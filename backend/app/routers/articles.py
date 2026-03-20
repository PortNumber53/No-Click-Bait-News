from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy.orm import Session

from app.database import get_db
from app.dependencies import get_current_user_optional
from app.models.article import Article
from app.models.user import User
from app.schemas.article import ArticleFeedResponse, ArticleResponse

router = APIRouter(prefix="/articles", tags=["articles"])


@router.get("/feed", response_model=ArticleFeedResponse)
def get_feed(
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=50),
    category: str | None = Query(None),
    user: User | None = Depends(get_current_user_optional),
    db: Session = Depends(get_db),
):
    query = db.query(Article).order_by(Article.published_at.desc())

    if category:
        query = query.filter(Article.category == category)

    # Non-premium users only see non-premium articles
    has_premium = False
    if user and user.subscription and user.subscription.tier:
        has_premium = user.subscription.tier.has_premium_access
    if not has_premium:
        query = query.filter(Article.is_premium.is_(False))

    total = query.count()
    offset = (page - 1) * page_size
    articles = query.offset(offset).limit(page_size).all()
    has_more = offset + page_size < total

    return ArticleFeedResponse(
        articles=[ArticleResponse.model_validate(a) for a in articles],
        page=page,
        page_size=page_size,
        has_more=has_more,
    )


@router.get("/{article_id}", response_model=ArticleResponse)
def get_article(
    article_id: str,
    user: User | None = Depends(get_current_user_optional),
    db: Session = Depends(get_db),
):
    article = db.query(Article).filter(Article.id == article_id).first()
    if not article:
        raise HTTPException(status_code=404, detail="Article not found")

    if article.is_premium:
        has_premium = False
        if user and user.subscription and user.subscription.tier:
            has_premium = user.subscription.tier.has_premium_access
        if not has_premium:
            raise HTTPException(status_code=403, detail="Premium subscription required")

    article.view_count += 1
    db.commit()
    return ArticleResponse.model_validate(article)
