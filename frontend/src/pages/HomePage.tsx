import { useCallback, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useArticles } from '../hooks/useArticles';
import { ArticleCard } from '../components/ArticleCard';
import { CategoryFilter } from '../components/CategoryFilter';
import { ShimmerCard } from '../components/ShimmerCard';
import './HomePage.css';

export function HomePage() {
  const { articles, isLoading, hasMore, error, selectedCategory, changeCategory, loadArticles } = useArticles();
  const navigate = useNavigate();
  const sentinelRef = useRef<HTMLDivElement>(null);

  // Infinite scroll via IntersectionObserver
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
      <>
        <CategoryFilter selected={selectedCategory} onChange={changeCategory} />
        <div className="home__list">
          {Array.from({ length: 5 }, (_, i) => <ShimmerCard key={i} />)}
        </div>
      </>
    );
  }

  if (articles.length === 0 && error) {
    return (
      <>
        <CategoryFilter selected={selectedCategory} onChange={changeCategory} />
        <div className="home__empty">
          <p className="home__error">{error}</p>
          <button className="btn btn--tonal" onClick={() => loadArticles(true)}>Retry</button>
        </div>
      </>
    );
  }

  if (articles.length === 0) {
    return (
      <>
        <CategoryFilter selected={selectedCategory} onChange={changeCategory} />
        <div className="home__empty">
          <p>No articles found</p>
        </div>
      </>
    );
  }

  return (
    <>
      <CategoryFilter selected={selectedCategory} onChange={changeCategory} />
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
    </>
  );
}
