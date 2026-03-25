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

import httpx
import psycopg2
import psycopg2.extras

DATABASE_URL = os.environ.get("DATABASE_URL")
OPENROUTER_API_KEY = os.environ.get("OPENROUTER_API_KEY")
NEWS_API_KEY = os.environ.get("NEWS_API_KEY")

OPENROUTER_URL = "https://openrouter.ai/api/v1/chat/completions"

REWRITE_PROMPT = """You are an editor at a factual news organization. Your job is to rewrite
news articles to remove all clickbait, sensationalism, exaggeration, and engagement bait.
Keep all facts intact. Use clear, direct language.

Rewrite the following:

Original title: {title}
Original summary: {summary}

Respond ONLY with valid JSON (no markdown, no code fences):
{{"title": "your rewritten title", "summary": "your rewritten summary"}}"""


def get_db():
    if not DATABASE_URL:
        print("ERROR: DATABASE_URL not set", file=sys.stderr)
        sys.exit(1)
    conn = psycopg2.connect(DATABASE_URL)
    conn.autocommit = False
    return conn


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
            title = article.get("title", "").strip()
            summary = article.get("description", "").strip()
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
                        (title, summary, content, source_name, source_url, image_url,
                         category, published_at, original_title, original_summary, fetch_status)
                       VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, 'raw')
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


def rewrite_articles():
    """Process raw articles through active LLM models via OpenRouter."""
    if not OPENROUTER_API_KEY:
        print("ERROR: OPENROUTER_API_KEY not set", file=sys.stderr)
        sys.exit(1)

    conn = get_db()
    cur = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)

    # Get active models
    cur.execute("SELECT id, slug, openrouter_model_id FROM llm_models WHERE is_active = true ORDER BY id")
    models = cur.fetchall()
    if len(models) < 2:
        print("ERROR: Need at least 2 active LLM models", file=sys.stderr)
        sys.exit(1)

    # Get articles that need rewrites
    cur.execute(
        """SELECT a.id, COALESCE(a.original_title, a.title) as title,
                  COALESCE(a.original_summary, a.summary) as summary
           FROM articles a
           WHERE a.fetch_status = 'raw'
             OR (SELECT COUNT(*) FROM article_rewrites ar
                 WHERE ar.article_id = a.id AND ar.processing_status = 'completed') < 2
           ORDER BY a.published_at DESC
           LIMIT 50"""
    )
    articles = cur.fetchall()

    if not articles:
        print("No articles to process")
        return

    print(f"Processing {len(articles)} articles through {len(models)} models")

    client = httpx.Client(timeout=60)

    for article in articles:
        for model in models:
            # Check if rewrite already exists
            cur.execute(
                """SELECT id FROM article_rewrites
                   WHERE article_id = %s AND llm_model_id = %s AND processing_status = 'completed'""",
                (article["id"], model["id"]),
            )
            if cur.fetchone():
                continue

            print(f"  [{model['slug']}] Processing: {article['title'][:60]}...")

            prompt = REWRITE_PROMPT.format(
                title=article["title"],
                summary=article["summary"],
            )

            try:
                resp = client.post(
                    OPENROUTER_URL,
                    headers={
                        "Authorization": f"Bearer {OPENROUTER_API_KEY}",
                        "Content-Type": "application/json",
                    },
                    json={
                        "model": model["openrouter_model_id"],
                        "messages": [{"role": "user", "content": prompt}],
                        "temperature": 0.3,
                    },
                )
                resp.raise_for_status()
                data = resp.json()

                content = data["choices"][0]["message"]["content"]
                # Strip markdown fences if present
                content = content.strip()
                if content.startswith("```"):
                    content = content.split("\n", 1)[1] if "\n" in content else content[3:]
                if content.endswith("```"):
                    content = content[:-3]
                content = content.strip()

                result = json.loads(content)
                prompt_tokens = data.get("usage", {}).get("prompt_tokens")
                completion_tokens = data.get("usage", {}).get("completion_tokens")

                # Upsert rewrite
                cur.execute(
                    """INSERT INTO article_rewrites
                        (article_id, llm_model_id, rewritten_title, rewritten_summary,
                         processing_status, prompt_tokens, completion_tokens)
                       VALUES (%s, %s, %s, %s, 'completed', %s, %s)
                       ON CONFLICT (article_id, llm_model_id) DO UPDATE SET
                         rewritten_title = EXCLUDED.rewritten_title,
                         rewritten_summary = EXCLUDED.rewritten_summary,
                         processing_status = 'completed',
                         prompt_tokens = EXCLUDED.prompt_tokens,
                         completion_tokens = EXCLUDED.completion_tokens""",
                    (
                        article["id"], model["id"],
                        result["title"], result["summary"],
                        prompt_tokens, completion_tokens,
                    ),
                )
                conn.commit()

            except Exception as e:
                print(f"    ERROR: {e}")
                # Record failure
                cur.execute(
                    """INSERT INTO article_rewrites
                        (article_id, llm_model_id, rewritten_title, rewritten_summary,
                         processing_status, error_message)
                       VALUES (%s, %s, '', '', 'failed', %s)
                       ON CONFLICT (article_id, llm_model_id) DO UPDATE SET
                         processing_status = 'failed', error_message = EXCLUDED.error_message""",
                    (article["id"], model["id"], str(e)),
                )
                conn.commit()
                continue

            # Rate limit courtesy
            time.sleep(0.5)

        # Update article status if both rewrites are done
        cur.execute(
            """UPDATE articles SET fetch_status = 'processed'
               WHERE id = %s
                 AND (SELECT COUNT(*) FROM article_rewrites
                      WHERE article_id = %s AND processing_status = 'completed') >= 2""",
            (article["id"], article["id"]),
        )
        conn.commit()

    client.close()
    cur.close()
    conn.close()
    print("Done processing articles")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Fetch and process news articles")
    parser.add_argument("--fetch", action="store_true", help="Fetch new articles from NewsAPI")
    parser.add_argument("--rewrite", action="store_true", help="Rewrite articles via OpenRouter LLMs")
    args = parser.parse_args()

    if not args.fetch and not args.rewrite:
        args.fetch = True
        args.rewrite = True

    if args.fetch:
        fetch_news()
    if args.rewrite:
        rewrite_articles()
