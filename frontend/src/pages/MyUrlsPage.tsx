import { useCallback, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArticleCard } from '../components/ArticleCard';
import { ShimmerCard } from '../components/ShimmerCard';
import { useMyArticles } from '../hooks/useMyArticles';
import './HomePage.css';

export function MyUrlsPage() {
  const { articles, isLoading, hasMore, error, loadArticles } = useMyArticles();
  const navigate = useNavigate();
  const sentinelRef = useRef<HTMLDivElement>(null);

  const loadMore = useCallback(() => {
    if (!isLoading && hasMore) loadArticles(false);
  }, [isLoading, hasMore, loadArticles]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return;
    const observer = new IntersectionObserver(
      ([entry]) => { if (entry.isIntersecting) loadMore(); },
      { rootMargin: '300px' },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loadMore]);

  if (articles.length === 0 && isLoading) {
    return (
      <div className="home__list">
        {Array.from({ length: 5 }, (_, i) => <ShimmerCard key={i} />)}
      </div>
    );
  }

  if (articles.length === 0 && error) {
    return (
      <div className="home__empty">
        <p className="home__error">{error}</p>
        <button className="btn btn--tonal" onClick={() => loadArticles(true)}>Retry</button>
      </div>
    );
  }

  if (articles.length === 0) {
    return (
      <div className="home__empty">
        <p>No submitted URLs found</p>
      </div>
    );
  }

  return (
    <div className="home__list">
      {articles.map(article => (
        <ArticleCard
          key={article.id}
          article={article}
          onClick={() => navigate(`/article/${article.id}`)}
        />
      ))}
      {hasMore && (
        <div ref={sentinelRef} className="home__loading">
          <div className="spinner" />
        </div>
      )}
    </div>
  );
}
