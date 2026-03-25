import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../services/api';
import type { Article } from '../types';
import './ArticleDetailPage.css';

export function ArticleDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [article, setArticle] = useState<Article | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    api.getArticle(id)
      .then(setArticle)
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load article'));
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
          {article.category && <span className="detail__chip">{article.category}</span>}
          {article.is_premium && <span className="detail__chip detail__chip--premium">&#9733; Premium</span>}
        </div>
        <h1 className="detail__title">{article.title}</h1>
        <div className="detail__meta">
          <span>{article.source_name}</span>
          <span>{date}</span>
        </div>
        <hr className="detail__divider" />
        <p className="detail__summary">{article.summary}</p>
        {article.content && <div className="detail__body">{article.content}</div>}
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
