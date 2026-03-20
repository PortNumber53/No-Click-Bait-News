import stripe
from fastapi import APIRouter, Depends, Header, HTTPException, Request
from sqlalchemy.orm import Session

from app.config import STRIPE_WEBHOOK_SECRET
from app.database import get_db
from app.dependencies import get_current_user
from app.models.subscription import SubscriptionTier
from app.models.user import User
from app.schemas.subscription import (
    CheckoutSessionResponse,
    CreateCheckoutRequest,
    SubscriptionTierResponse,
)
from app.services.stripe_service import (
    handle_checkout_completed,
    handle_subscription_updated,
    create_checkout_session,
)

router = APIRouter(prefix="/subscriptions", tags=["subscriptions"])


@router.get("/tiers", response_model=list[SubscriptionTierResponse])
def list_tiers(db: Session = Depends(get_db)):
    tiers = db.query(SubscriptionTier).all()
    return [SubscriptionTierResponse.model_validate(t) for t in tiers]


@router.post("/checkout", response_model=CheckoutSessionResponse)
def create_checkout(
    data: CreateCheckoutRequest,
    user: User = Depends(get_current_user),
    db: Session = Depends(get_db),
):
    tier = db.query(SubscriptionTier).filter(SubscriptionTier.id == data.tier_id).first()
    if not tier or not tier.stripe_price_id:
        raise HTTPException(status_code=400, detail="Invalid subscription tier")

    session = create_checkout_session(
        user=user,
        tier=tier,
        success_url="noclickbaitnews://subscription/success",
        cancel_url="noclickbaitnews://subscription/cancel",
    )
    return CheckoutSessionResponse(checkout_url=session.url, session_id=session.id)


@router.post("/webhook")
async def stripe_webhook(
    request: Request,
    stripe_signature: str = Header(alias="stripe-signature"),
    db: Session = Depends(get_db),
):
    payload = await request.body()
    try:
        event = stripe.Webhook.construct_event(payload, stripe_signature, STRIPE_WEBHOOK_SECRET)
    except (ValueError, stripe.error.SignatureVerificationError):
        raise HTTPException(status_code=400, detail="Invalid webhook signature")

    if event["type"] == "checkout.session.completed":
        handle_checkout_completed(event["data"]["object"], db)
    elif event["type"] in ("customer.subscription.updated", "customer.subscription.deleted"):
        handle_subscription_updated(event["data"]["object"], db)

    return {"status": "ok"}
