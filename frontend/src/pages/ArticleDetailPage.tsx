import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { MarkdownContent } from '../components/MarkdownContent';
import { api, ApiError } from '../services/api';
import type { Article, ComparisonData } from '../types';
import { ComparisonCard } from '../components/ComparisonCard';
import './ArticleDetailPage.css';

export function ArticleDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [article, setArticle] = useState<Article | null>(null);
  const [comparison, setComparison] = useState<ComparisonData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    api.getArticle(id)
      .then(setArticle)
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load article'));

    // Load comparison data (may not exist for all articles)
    api.getComparison(id)
      .then(setComparison)
      .catch((e) => {
        // 404 is expected if article hasn't been processed yet
        if (!(e instanceof ApiError && e.status === 404)) {
          console.error('Failed to load comparison:', e);
        }
      });
  }, [id]);

  if (error) {
    return (
      <div className="detail__error">
        <p>{error}</p>
        <button className="btn btn--tonal" onClick={() => navigate(-1)}>Go Back</button>
      </div>
    );
  }

  if (!article) {
    return <div className="detail__loading"><div className="spinner" /></div>;
  }

  const date = new Date(article.published_at).toLocaleDateString('en-US', {
    year: 'numeric', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit',
  });
  const isRewritePending = article.rewrite_status === 'pending';
  const isRewriteFailed = article.rewrite_status === 'failed';
  const categories = article.categories?.length ? article.categories : article.category ? [article.category] : [];

  return (
    <div className="detail">
      {article.image_url && (
        <div className="detail__hero">
          <img src={article.image_url} alt="" />
        </div>
      )}
      <div className="detail__content">
        <button className="detail__back" onClick={() => navigate(-1)}>&larr; Back</button>
        <div className="detail__badges">
          {categories.slice(0, 3).map(category => (
            <span key={category} className="detail__chip">{category}</span>
          ))}
          {article.is_premium && <span className="detail__chip detail__chip--premium">&#9733; Premium</span>}
        </div>
        <h1 className="detail__title">{article.title}</h1>
        <div className="detail__meta">
          <span>{article.source_name}</span>
          <span>{date}</span>
        </div>

        {comparison && <ComparisonCard comparison={comparison} showContent />}

        <hr className="detail__divider" />
        <div className="detail__summary">
          <MarkdownContent markdown={article.summary} />
        </div>
        {article.content && (
          article.original_content ? (
            <div className="detail__columns">
              <section className="detail__column">
                <h2 className="detail__column-title">AI Rewrite</h2>
                {isRewritePending ? (
                  <div className="detail__processing">
                    <div className="spinner" />
                    <span>Processing AI rewrite...</span>
                  </div>
                ) : isRewriteFailed ? (
                  <div className="detail__processing detail__processing--failed">
                    AI rewrite failed. The original article is still available.
                  </div>
                ) : (
                  <MarkdownContent markdown={article.content} />
                )}
              </section>
              <section className="detail__column detail__column--original">
                <h2 className="detail__column-title">Original</h2>
                <MarkdownContent markdown={article.original_content} />
              </section>
            </div>
          ) : (
            <div className="detail__body">
              <MarkdownContent markdown={article.content} />
            </div>
          )
        )}
        <a
          href={article.source_url}
          target="_blank"
          rel="noopener noreferrer"
          className="btn btn--outlined detail__source-link"
        >
          Read Original Source &#8599;
        </a>
      </div>
    </div>
  );
}
