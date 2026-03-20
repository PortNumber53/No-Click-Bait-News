from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.database import get_db
from app.models.subscription import SubscriptionTier, UserSubscription
from app.models.user import User
from app.schemas.user import TokenResponse, UserCreate, UserLogin, UserResponse
from app.services.auth import create_access_token, hash_password, verify_password
from app.services.stripe_service import create_stripe_customer

router = APIRouter(prefix="/auth", tags=["auth"])


@router.post("/register", response_model=TokenResponse, status_code=status.HTTP_201_CREATED)
def register(data: UserCreate, db: Session = Depends(get_db)):
    if db.query(User).filter(User.email == data.email).first():
        raise HTTPException(status_code=400, detail="Email already registered")

    user = User(
        email=data.email,
        hashed_password=hash_password(data.password),
        name=data.name,
    )
    db.add(user)
    db.flush()

    # Create Stripe customer
    stripe_id = create_stripe_customer(user)
    user.stripe_customer_id = stripe_id

    # Assign free tier
    free_tier = db.query(SubscriptionTier).filter(SubscriptionTier.name == "free").first()
    if free_tier:
        sub = UserSubscription(user_id=user.id, tier_id=free_tier.id, status="active")
        db.add(sub)

    db.commit()
    db.refresh(user)

    token = create_access_token(user.id)
    tier_name = user.subscription.tier.name if user.subscription else "free"
    return TokenResponse(
        access_token=token,
        user=UserResponse(
            id=user.id,
            email=user.email,
            name=user.name,
            created_at=user.created_at,
            subscription_tier=tier_name,
        ),
    )


@router.post("/login", response_model=TokenResponse)
def login(data: UserLogin, db: Session = Depends(get_db)):
    user = db.query(User).filter(User.email == data.email).first()
    if not user or not verify_password(data.password, user.hashed_password):
        raise HTTPException(status_code=401, detail="Invalid credentials")

    token = create_access_token(user.id)
    tier_name = user.subscription.tier.name if user.subscription else "free"
    return TokenResponse(
        access_token=token,
        user=UserResponse(
            id=user.id,
            email=user.email,
            name=user.name,
            created_at=user.created_at,
            subscription_tier=tier_name,
        ),
    )
