"""Seed the database with subscription tiers and sample articles."""
from datetime import datetime, timedelta

from app.database import SessionLocal, engine, Base
from app.models import Article, SubscriptionTier


def seed():
    Base.metadata.create_all(bind=engine)
    db = SessionLocal()

    # Seed subscription tiers
    tiers = [
        SubscriptionTier(
            name="free",
            price_monthly=0,
            max_articles_per_day=10,
            has_premium_access=False,
        ),
        SubscriptionTier(
            name="basic",
            stripe_price_id="price_basic_monthly",
            price_monthly=4.99,
            max_articles_per_day=50,
            has_premium_access=False,
        ),
        SubscriptionTier(
            name="premium",
            stripe_price_id="price_premium_monthly",
            price_monthly=9.99,
            max_articles_per_day=999,
            has_premium_access=True,
        ),
    ]

    for tier in tiers:
        existing = db.query(SubscriptionTier).filter(SubscriptionTier.name == tier.name).first()
        if not existing:
            db.add(tier)

    # Seed sample articles
    categories = ["Technology", "Science", "Business", "Health", "Sports", "World"]
    now = datetime.utcnow()

    for i in range(60):
        cat = categories[i % len(categories)]
        article = Article(
            title=f"Sample {cat} Article #{i + 1}: Important Developments Today",
            summary=f"A straightforward summary of key {cat.lower()} developments without sensationalism.",
            content=f"Full article content for {cat.lower()} article #{i + 1}. "
            "This is a detailed, factual report without clickbait headlines.",
            source_name="No-Click Bait News",
            source_url=f"https://example.com/articles/{i + 1}",
            image_url=f"https://picsum.photos/seed/{i + 1}/800/400",
            category=cat,
            published_at=now - timedelta(hours=i),
            is_premium=(i % 5 == 0),
        )
        db.add(article)

    db.commit()
    db.close()
    print("Database seeded successfully.")


if __name__ == "__main__":
    seed()
