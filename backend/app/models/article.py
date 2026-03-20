import uuid
from datetime import datetime

from sqlalchemy import Boolean, Column, DateTime, Integer, String, Text
from sqlalchemy.dialects.postgresql import UUID

from app.database import Base


class Article(Base):
    __tablename__ = "articles"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    title = Column(String, nullable=False)
    summary = Column(Text, nullable=False)
    content = Column(Text, nullable=True)
    source_name = Column(String, nullable=False)
    source_url = Column(String, nullable=False)
    image_url = Column(String, nullable=True)
    category = Column(String, nullable=True, index=True)
    published_at = Column(DateTime, nullable=False, index=True)
    is_premium = Column(Boolean, default=False)
    view_count = Column(Integer, default=0)
    created_at = Column(DateTime, default=datetime.utcnow)
