from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.routers import articles, auth, subscriptions

app = FastAPI(
    title="No-Click Bait News API",
    description="Backend API for the No-Click Bait News mobile app",
    version="1.0.0",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(auth.router, prefix="/api/v1")
app.include_router(articles.router, prefix="/api/v1")
app.include_router(subscriptions.router, prefix="/api/v1")


@app.get("/health")
def health_check():
    return {"status": "ok"}
