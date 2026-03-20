from pydantic import BaseModel


class SubscriptionTierResponse(BaseModel):
    id: int
    name: str
    price_monthly: float
    max_articles_per_day: int
    has_premium_access: bool

    model_config = {"from_attributes": True}


class CreateCheckoutRequest(BaseModel):
    tier_id: int


class CheckoutSessionResponse(BaseModel):
    checkout_url: str
    session_id: str
