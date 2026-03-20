import uuid
from datetime import datetime, timezone

from sqlalchemy import Boolean, Column, DateTime, ForeignKey, Integer, Numeric, String
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from app.database import Base


def _utcnow():
    return datetime.now(timezone.utc)


class SubscriptionTier(Base):
    __tablename__ = "subscription_tiers"

    id = Column(Integer, primary_key=True, autoincrement=True)
    name = Column(String, unique=True, nullable=False)  # free, basic, premium
    stripe_price_id = Column(String, unique=True, nullable=True)
    price_monthly = Column(Numeric(10, 2), nullable=False, default=0)
    max_articles_per_day = Column(Integer, nullable=False, default=10)
    has_premium_access = Column(Boolean, default=False)
    created_at = Column(DateTime, default=_utcnow)


class UserSubscription(Base):
    __tablename__ = "user_subscriptions"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    user_id = Column(UUID(as_uuid=True), ForeignKey("users.id"), unique=True, nullable=False)
    tier_id = Column(Integer, ForeignKey("subscription_tiers.id"), nullable=False)
    stripe_subscription_id = Column(String, unique=True, nullable=True)
    status = Column(String, nullable=False, default="active")  # active, canceled, past_due
    current_period_start = Column(DateTime, nullable=True)
    current_period_end = Column(DateTime, nullable=True)
    created_at = Column(DateTime, default=_utcnow)
    updated_at = Column(DateTime, default=_utcnow, onupdate=_utcnow)

    user = relationship("User", back_populates="subscription")
    tier = relationship("SubscriptionTier")
