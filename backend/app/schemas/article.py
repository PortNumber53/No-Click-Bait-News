from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class ArticleResponse(BaseModel):
    id: UUID
    title: str
    summary: str
    content: str | None = None
    source_name: str
    source_url: str
    image_url: str | None = None
    category: str | None = None
    published_at: datetime
    is_premium: bool
    view_count: int

    model_config = {"from_attributes": True}


class ArticleFeedResponse(BaseModel):
    articles: list[ArticleResponse]
    page: int
    page_size: int
    has_more: bool
