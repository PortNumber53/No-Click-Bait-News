# No-Click Bait News

A news reader app that delivers factual, no-clickbait news with an infinite scrolling UX.

## Architecture

```
├── backend/          # FastAPI backend with PostgreSQL
│   ├── app/
│   │   ├── models/       # SQLAlchemy models (User, Article, Subscription)
│   │   ├── routers/      # API endpoints (auth, articles, subscriptions)
│   │   ├── schemas/      # Pydantic request/response schemas
│   │   ├── services/     # Business logic (auth, Stripe)
│   │   └── main.py       # FastAPI app entry point
│   ├── alembic/          # Database migrations
│   └── seed.py           # Seed data script
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
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env   # Edit with your credentials
python seed.py          # Seed the database
uvicorn app.main:app --reload --host 0.0.0.0 --port 21011
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
| GET | `/api/v1/subscriptions/tiers` | List subscription tiers |
| POST | `/api/v1/subscriptions/checkout` | Create Stripe checkout |
| POST | `/api/v1/subscriptions/webhook` | Stripe webhook handler |
