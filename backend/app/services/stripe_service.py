import stripe
from sqlalchemy.orm import Session

from app.config import STRIPE_SECRET_KEY
from app.models.subscription import SubscriptionTier, UserSubscription
from app.models.user import User

stripe.api_key = STRIPE_SECRET_KEY


def create_stripe_customer(user: User) -> str:
    customer = stripe.Customer.create(
        email=user.email,
        name=user.name,
        metadata={"user_id": str(user.id)},
    )
    return customer.id


def create_checkout_session(
    user: User, tier: SubscriptionTier, success_url: str, cancel_url: str
) -> stripe.checkout.Session:
    if not user.stripe_customer_id:
        raise ValueError("User has no Stripe customer ID")

    session = stripe.checkout.Session.create(
        customer=user.stripe_customer_id,
        payment_method_types=["card"],
        line_items=[{"price": tier.stripe_price_id, "quantity": 1}],
        mode="subscription",
        success_url=success_url,
        cancel_url=cancel_url,
        metadata={"user_id": str(user.id), "tier_id": str(tier.id)},
    )
    return session


def handle_checkout_completed(session_data: dict, db: Session) -> None:
    user_id = session_data["metadata"]["user_id"]
    tier_id = int(session_data["metadata"]["tier_id"])
    stripe_subscription_id = session_data.get("subscription")

    sub = db.query(UserSubscription).filter(UserSubscription.user_id == user_id).first()
    if sub:
        sub.tier_id = tier_id
        sub.stripe_subscription_id = stripe_subscription_id
        sub.status = "active"
    else:
        sub = UserSubscription(
            user_id=user_id,
            tier_id=tier_id,
            stripe_subscription_id=stripe_subscription_id,
            status="active",
        )
        db.add(sub)
    db.commit()


def handle_subscription_updated(subscription_data: dict, db: Session) -> None:
    stripe_sub_id = subscription_data["id"]
    status = subscription_data["status"]

    sub = (
        db.query(UserSubscription)
        .filter(UserSubscription.stripe_subscription_id == stripe_sub_id)
        .first()
    )
    if sub:
        sub.status = status
        db.commit()
