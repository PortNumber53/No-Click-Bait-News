"""
Fetch raw news articles and process them through 2 LLMs via OpenRouter
to remove clickbait. Each article gets two independent rewrites for blind A/B comparison.

Usage:
    python scripts/process_news.py [--fetch] [--rewrite]

    --fetch    Fetch new articles from NewsAPI and insert as raw
    --rewrite  Process raw articles through LLMs

Env vars: DATABASE_URL, OPENROUTER_API_KEY, NEWS_API_KEY
"""
import argparse
import json
import os
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

from dotenv import load_dotenv
import httpx
import psycopg2
from markdownify import markdownify as md
import re
import psycopg2.extras

# Load .env from the backend directory
load_dotenv(Path(__file__).resolve().parent.parent / ".env")

DATABASE_URL = os.environ.get("DATABASE_URL")
OPENROUTER_API_KEY = os.environ.get("OPENROUTER_API_KEY")
NEWS_API_KEY = os.environ.get("NEWS_API_KEY")

OPENROUTER_URL = "https://openrouter.ai/api/v1/chat/completions"

REWRITE_PROMPT = """You are an editor at a factual news organization. Your job is to rewrite
news articles to remove all clickbait, sensationalism, exaggeration, and engagement bait.
Keep all facts intact. Use clear, direct language.

You must produce:
1. A factual, non-clickbait title
2. A comprehensive summary (2-4 sentences) that gives readers enough information to decide whether to read further
3. A full rewritten article in markdown format that preserves all facts but removes sensationalism

Rewrite the following:

Original title: {title}
Original summary: {summary}
{content_section}
Respond ONLY with valid JSON (no markdown fences around the JSON):
{{"title": "rewritten title", "summary": "comprehensive factual summary", "content": "full rewritten article in markdown"}}"""


def get_db():
    if not DATABASE_URL:
        print("ERROR: DATABASE_URL not set", file=sys.stderr)
        sys.exit(1)
    conn = psycopg2.connect(DATABASE_URL)
    conn.autocommit = False
    return conn


def fetch_article_content(url: str, client: httpx.Client) -> str | None:
    """Fetch article HTML and convert to markdown, preserving formatting."""
    try:
        resp = client.get(url, follow_redirects=True, timeout=15)
        resp.raise_for_status()
        content_type = resp.headers.get("content-type", "")
        if "html" not in content_type:
            return None

        html = resp.text

        # Try to extract from <article> tag first, fall back to <body>
        article_match = re.search(r"<article[^>]*>(.*?)</article>", html, re.DOTALL | re.IGNORECASE)
        if article_match:
            html = article_match.group(1)
        else:
            body_match = re.search(r"<body[^>]*>(.*?)</body>", html, re.DOTALL | re.IGNORECASE)
            if body_match:
                html = body_match.group(1)

        # Strip script/style/nav tags before converting
        html = re.sub(r"<(script|style|nav|header|footer|aside|noscript)[^>]*>.*?</\1>", "", html, flags=re.DOTALL | re.IGNORECASE)

        # Convert to markdown
        text = md(html, heading_style="ATX", strip=["img", "figure", "figcaption", "iframe", "video", "audio"])

        # Clean up excessive whitespace
        text = re.sub(r"\n{3,}", "\n\n", text)
        text = text.strip()

        # Skip if too short (probably a paywall or bot protection page)
        if len(text) < 200:
            print(f"    Skipped (too short: {len(text)} chars, likely paywalled/bot-protected): {url[:60]}")
            return None

        # Truncate very long articles to avoid huge LLM costs
        if len(text) > 15000:
            text = text[:15000] + "\n\n[Article truncated]"

        return text
    except Exception as e:
        print(f"    Could not fetch content from {url}: {e}")
        return None


def fetch_news():
    """Fetch top headlines from NewsAPI and insert as raw articles."""
    if not NEWS_API_KEY or NEWS_API_KEY.startswith("your_"):
        print("ERROR: NEWS_API_KEY not set or is placeholder", file=sys.stderr)
        sys.exit(1)

    conn = get_db()
    cur = conn.cursor()

    categories = ["technology", "science", "business", "health", "sports", "general"]
    inserted = 0

    for category in categories:
        print(f"Fetching {category} headlines...")
        resp = httpx.get(
            "https://newsapi.org/v2/top-headlines",
            params={
                "apiKey": NEWS_API_KEY,
                "country": "us",
                "category": category,
                "pageSize": 10,
            },
            timeout=30,
        )
        resp.raise_for_status()
        data = resp.json()

        for article in data.get("articles", []):
            title = (article.get("title") or "").strip()
            summary = (article.get("description") or "").strip()
            if not title or not summary or title == "[Removed]":
                continue

            source_name = article.get("source", {}).get("name", "Unknown")
            source_url = article.get("url", "")
            image_url = article.get("urlToImage")
            published_at = article.get("publishedAt", datetime.now(timezone.utc).isoformat())
            content = article.get("content")

            # Map NewsAPI category to our categories
            cat_map = {"general": "World"}
            display_category = cat_map.get(category, category.capitalize())

            try:
                cur.execute(
                    """INSERT INTO articles
                        (id, title, summary, content, source_name, source_url, image_url,
                         category, published_at, is_premium, original_title, original_summary, fetch_status)
                       VALUES (gen_random_uuid(), %s, %s, %s, %s, %s, %s, %s, %s, false, %s, %s, 'raw')
                       ON CONFLICT DO NOTHING""",
                    (
                        title, summary, content, source_name, source_url, image_url,
                        display_category, published_at, title, summary,
                    ),
                )
                if cur.rowcount > 0:
                    inserted += 1
            except Exception as e:
                print(f"  Skip duplicate or error: {e}")
                conn.rollback()
                continue

    conn.commit()
    cur.close()
    conn.close()
    print(f"Fetched and inserted {inserted} new articles")


class RateLimitedError(Exception):
    """Raised when a model is persistently rate-limited and should be put in cooldown."""
    pass


def call_openrouter(client, model_id, prompt, max_retries=3):
    """Call OpenRouter with exponential backoff on 429s.
    Raises RateLimitedError after exhausting retries so the caller can apply cooldown."""
    for attempt in range(max_retries):
        resp = client.post(
            OPENROUTER_URL,
            headers={
                "Authorization": f"Bearer {OPENROUTER_API_KEY}",
                "Content-Type": "application/json",
            },
            json={
                "model": model_id,
                "messages": [{"role": "user", "content": prompt}],
                "temperature": 0.3,
            },
        )
        if resp.status_code == 429:
            wait = min(2 ** attempt * 3, 30)
            retry_after = resp.headers.get("retry-after")
            if retry_after:
                try:
                    wait = max(wait, int(retry_after))
                except ValueError:
                    pass
            print(f"    Rate limited, waiting {wait}s (attempt {attempt + 1}/{max_retries})")
            time.sleep(wait)
            continue
        resp.raise_for_status()
        return resp.json()
    raise RateLimitedError(f"Rate limited after {max_retries} retries")


def rewrite_articles():
    """Process raw articles through active LLM models via OpenRouter."""
    if not OPENROUTER_API_KEY:
        print("ERROR: OPENROUTER_API_KEY not set", file=sys.stderr)
        sys.exit(1)

    conn = get_db()
    cur = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)

    # Get active models that aren't in cooldown
    cur.execute(
        """SELECT id, slug, openrouter_model_id, input_cost_per_million, output_cost_per_million
           FROM llm_models
           WHERE is_active = true
             AND (rate_limited_until IS NULL OR rate_limited_until < NOW())
           ORDER BY id"""
    )
    models = cur.fetchall()

    # Also report any models in cooldown
    cur.execute(
        """SELECT slug, rate_limited_until FROM llm_models
           WHERE is_active = true AND rate_limited_until IS NOT NULL AND rate_limited_until >= NOW()"""
    )
    for row in cur.fetchall():
        print(f"  Skipping {row['slug']} (rate limited until {row['rate_limited_until']})")
    if len(models) < 2:
        print("ERROR: Need at least 2 active LLM models", file=sys.stderr)
        sys.exit(1)
    num_models = len(models)

    # Get articles that need rewrites
    cur.execute(
        """SELECT a.id, COALESCE(a.original_title, a.title) as title,
                  COALESCE(a.original_summary, a.summary) as summary,
                  a.content
           FROM articles a
           WHERE a.fetch_status != 'processed'
              OR (SELECT COUNT(*) FROM article_rewrites ar
                  WHERE ar.article_id = a.id AND ar.processing_status = 'completed') < %s
           ORDER BY a.published_at DESC
           LIMIT 50""",
        (num_models,),
    )
    articles = cur.fetchall()

    if not articles:
        print("No articles to process")
        return

    print(f"Processing {len(articles)} articles through {len(models)} models")

    client = httpx.Client(timeout=60)
    cooled_models = set()  # model IDs put into cooldown this run

    for article in articles:
        for model in models:
            if model["id"] in cooled_models:
                continue

            # Check if rewrite already exists
            cur.execute(
                """SELECT id FROM article_rewrites
                   WHERE article_id = %s AND llm_model_id = %s AND processing_status = 'completed'""",
                (article["id"], model["id"]),
            )
            if cur.fetchone():
                continue

            print(f"  [{model['slug']}] Processing: {article['title'][:60]}...")

            content_section = ""
            if article.get("content"):
                content_section = f"\nOriginal content: {article['content']}\n"

            prompt = REWRITE_PROMPT.format(
                title=article["title"],
                summary=article["summary"],
                content_section=content_section,
            )

            try:
                data = call_openrouter(client, model["openrouter_model_id"], prompt)

                raw_content = data["choices"][0]["message"]["content"]
                # Strip markdown fences if present
                raw_content = raw_content.strip()
                if raw_content.startswith("```"):
                    raw_content = raw_content.split("\n", 1)[1] if "\n" in raw_content else raw_content[3:]
                if raw_content.endswith("```"):
                    raw_content = raw_content[:-3]
                raw_content = raw_content.strip()

                result = json.loads(raw_content)
                prompt_tokens = data.get("usage", {}).get("prompt_tokens")
                completion_tokens = data.get("usage", {}).get("completion_tokens")

                rewritten_title = result.get("title", "").strip()
                rewritten_summary = result.get("summary", "").strip()
                rewritten_content = result.get("content")
                if isinstance(rewritten_content, str):
                    rewritten_content = rewritten_content.strip() or None

                if not rewritten_title or not rewritten_summary:
                    print(f"    SKIP: empty title or summary from LLM")
                    continue

                # Upsert rewrite
                cur.execute(
                    """INSERT INTO article_rewrites
                        (article_id, llm_model_id, rewritten_title, rewritten_summary,
                         rewritten_content, processing_status, prompt_tokens, completion_tokens)
                       VALUES (%s, %s, %s, %s, %s, 'completed', %s, %s)
                       ON CONFLICT (article_id, llm_model_id) DO UPDATE SET
                         rewritten_title = EXCLUDED.rewritten_title,
                         rewritten_summary = EXCLUDED.rewritten_summary,
                         rewritten_content = EXCLUDED.rewritten_content,
                         processing_status = 'completed',
                         prompt_tokens = EXCLUDED.prompt_tokens,
                         completion_tokens = EXCLUDED.completion_tokens,
                         error_message = NULL""",
                    (
                        article["id"], model["id"],
                        rewritten_title, rewritten_summary, rewritten_content,
                        prompt_tokens, completion_tokens,
                    ),
                )
                conn.commit()

            except RateLimitedError as e:
                print(f"    COOLDOWN: {model['slug']} — pausing for 1 hour")
                cur.execute(
                    "UPDATE llm_models SET rate_limited_until = NOW() + INTERVAL '1 hour' WHERE id = %s",
                    (model["id"],),
                )
                conn.commit()
                cooled_models.add(model["id"])
                continue

            except Exception as e:
                print(f"    ERROR: {e}")
                # Don't store failed rows — just log and move on
                conn.rollback()
                continue

            # Rate limit courtesy (free models have tighter limits)
            time.sleep(2)

        # Update article status if both rewrites are done
        cur.execute(
            """UPDATE articles SET fetch_status = 'processed'
               WHERE id = %s
                 AND (SELECT COUNT(*) FROM article_rewrites
                      WHERE article_id = %s AND processing_status = 'completed') >= %s""",
            (article["id"], article["id"], num_models),
        )
        conn.commit()

    client.close()

    # Print cost summary
    cur.execute(
        """SELECT lm.display_name, lm.input_cost_per_million, lm.output_cost_per_million,
                  COALESCE(SUM(ar.prompt_tokens), 0) AS total_input,
                  COALESCE(SUM(ar.completion_tokens), 0) AS total_output
           FROM llm_models lm
           LEFT JOIN article_rewrites ar ON ar.llm_model_id = lm.id
           GROUP BY lm.id, lm.display_name, lm.input_cost_per_million, lm.output_cost_per_million
           ORDER BY lm.id"""
    )
    print("\n--- Cost Summary ---")
    for row in cur.fetchall():
        input_cost = row["total_input"] / 1_000_000 * float(row["input_cost_per_million"])
        output_cost = row["total_output"] / 1_000_000 * float(row["output_cost_per_million"])
        total = input_cost + output_cost
        print(f"  {row['display_name']}: {row['total_input']:,} in / {row['total_output']:,} out tokens — ${total:.4f}")

    cur.close()
    conn.close()
    print("Done processing articles")


def scrape_content():
    """Fetch full article content from source URLs for articles missing it."""
    conn = get_db()
    cur = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)

    # Scrape articles missing content OR with short NewsAPI snippets (< 500 chars)
    cur.execute(
        """SELECT id, source_url FROM articles
           WHERE source_url IS NOT NULL AND source_url != ''
             AND (content IS NULL OR content = '' OR LENGTH(content) < 500)
           ORDER BY published_at DESC
           LIMIT 100"""
    )
    articles = cur.fetchall()

    if not articles:
        print("No articles need content scraping")
        return

    print(f"Scraping content for {len(articles)} articles...")
    client = httpx.Client(
        headers={
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Accept-Language": "en-US,en;q=0.9",
        },
        timeout=15,
    )
    scraped = 0

    for article in articles:
        content = fetch_article_content(article["source_url"], client)
        if content:
            cur.execute(
                "UPDATE articles SET content = %s WHERE id = %s",
                (content, article["id"]),
            )
            conn.commit()
            scraped += 1
            print(f"  Scraped: {article['source_url'][:60]}... ({len(content)} chars)")
        time.sleep(1)  # Be polite

    client.close()
    cur.close()
    conn.close()
    print(f"Scraped content for {scraped}/{len(articles)} articles")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Fetch and process news articles")
    parser.add_argument("--fetch", action="store_true", help="Fetch new articles from NewsAPI")
    parser.add_argument("--scrape", action="store_true", help="Scrape full content from source URLs")
    parser.add_argument("--rewrite", action="store_true", help="Rewrite articles via OpenRouter LLMs")
    args = parser.parse_args()

    if not args.fetch and not args.scrape and not args.rewrite:
        args.fetch = True
        args.scrape = True
        args.rewrite = True

    if args.fetch:
        fetch_news()
    if args.scrape:
        scrape_content()
    if args.rewrite:
        rewrite_articles()
