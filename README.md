# No-Click Bait News

A news reader app that delivers factual, no-clickbait news with an infinite scrolling UX.

## Architecture

```
├── backend/          # Go API backend with PostgreSQL
│   ├── handlers/         # API endpoints (auth, articles, subscriptions)
│   ├── middleware/       # Auth middleware
│   ├── models/           # API/data models
│   ├── services/         # Business logic (migrations, Stripe, TinyFish)
│   └── main.go           # API entry point and subcommands
│
├── frontend/         # React SPA and Cloudflare Worker API proxy
│
├── mobile/           # Flutter mobile app
│   └── lib/
│       ├── models/       # Data models
│       ├── providers/    # State management (Provider)
│       ├── screens/      # App screens
│       ├── services/     # API client
│       └── widgets/      # Reusable UI components
```

## Backend Setup

```bash
cd backend
export DATABASE_URL="postgresql://postgres:postgres@localhost:5432/noclickbait"
export JWT_SECRET_KEY="dev-secret"
export STRIPE_SECRET_KEY="sk_test_..."
export STRIPE_WEBHOOK_SECRET_SNAPSHOT="whsec_..."
export TINYFISH_API_KEY="tf_..." # optional for server startup
go run . migrate
go run .
```

### Fetch Article Content

Article detail requests automatically use TinyFish Fetch to populate `articles.content`
from `articles.source_url` when content is empty and `TINYFISH_API_KEY` is configured.
User-submitted URLs are fetched with TinyFish, rewritten through an OpenAI-compatible
chat completions API, and stored with both the rewritten markdown and original
TinyFish markdown.

To backfill existing rows with empty content:

```bash
cd backend
TINYFISH_API_KEY="tf_..." go run . fetch-content 100
```

TinyFish settings:

| Env var | Default | Description |
|---------|---------|-------------|
| `TINYFISH_API_KEY` | unset | Enables TinyFish Fetch |
| `TINYFISH_FETCH_ENDPOINT` | `https://api.fetch.tinyfish.ai` | Fetch API endpoint |
| `TINYFISH_FETCH_FORMAT` | `markdown` | `markdown`, `html`, or `json` |
| `TINYFISH_FETCH_TTL` | `3600` | Cache freshness tolerance in seconds; use `0` for live fetches |
| `TINYFISH_FETCH_TIMEOUT_MS` | `45000` | Per-URL timeout sent to TinyFish |
| `LLM_API_KEY` | unset | API key for the OpenAI-compatible rewrite API |
| `LLM_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible API base URL |
| `LLM_MODEL` | unset | Chat completions model used for article rewrites |
| `LLM_MODELS` | unset | Optional comma-separated model list; two or more models enable blind comparisons |
| `LLM_TEMPERATURE` | `0.2` | Rewrite sampling temperature |
| `LLM_MAX_TOKENS` | `3000` | Maximum rewrite output tokens |
| `LLM_REWRITE_STALE_ON_START_LIMIT` | `100` | Number of outdated article rewrites to queue when the API starts |
| `LLM_REWRITE_MAX_ATTEMPTS` | `3` | Durable rewrite attempts before marking an article failed |
| `CHECKOUT_RETURN_ORIGIN` | `https://ncbnews.truvis.co` | Trusted Stripe checkout return origin |

Rewrite work is persisted in PostgreSQL and survives API restarts. Configure at
least two models through `LLM_MODELS` (or a comma-separated `LLM_MODEL`) to populate
the comparison and voting views. `backend/scripts/process_news.py` is retained only
for legacy/manual backfills; it is not part of the deployed request pipeline.

For FreeLLMAPI, map the Hermes-style model config like this:

```bash
LLM_BASE_URL=http://192.168.68.180:30001/v1
LLM_API_KEY=freellmapi-your-key
LLM_MODEL=auto
```

## Mobile Setup

```bash
cd mobile
flutter pub get
flutter run
```

## Features

- **Infinite scroll** news feed with pull-to-refresh
- **Category filtering** (Technology, Science, Business, Health, Sports, World)
- **Shimmer loading** placeholders for smooth UX
- **User authentication** with JWT tokens
- **Stripe subscriptions** with 3 tiers: Free, Basic ($4.99/mo), Premium ($9.99/mo)
- **Premium content** gating based on subscription tier
- **Dark mode** support
- **Material 3** design system

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/register` | Register new user |
| POST | `/api/v1/auth/login` | Login |
| GET | `/api/v1/articles/feed` | Paginated article feed |
| GET | `/api/v1/articles/{id}` | Single article detail |
| POST | `/api/v1/articles/fetch` | Fetch and rewrite a submitted URL |
| GET | `/api/v1/articles/{id}/comparison` | Blind rewrite comparison |
| POST | `/api/v1/articles/{id}/vote` | Vote on the presented rewrite pair |
| GET | `/api/v1/subscriptions/tiers` | List subscription tiers |
| POST | `/api/v1/subscriptions/checkout` | Create Stripe checkout |
| POST | `/webhook/stripe/snapshot` | Stripe snapshot webhook handler |
| POST | `/webhook/stripe/thin` | Optional Stripe thin webhook handler |
