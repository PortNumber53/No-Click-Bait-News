import type { Article } from '../types';
import './ArticleCard.css';

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

interface Props {
  article: Article;
  onClick: () => void;
}

export function ArticleCard({ article, onClick }: Props) {
  return (
    <article className="article-card" onClick={onClick}>
      {article.image_url && (
        <div className="article-card__image">
          <img src={article.image_url} alt="" loading="lazy" />
        </div>
      )}
      <div className="article-card__body">
        <div className="article-card__meta">
          {article.category && (
            <span className="article-card__category">{article.category}</span>
          )}
          {article.is_premium && (
            <span className="article-card__premium" title="Premium">&#9733;</span>
          )}
          <span className="article-card__time">{timeAgo(article.published_at)}</span>
        </div>
        <h3 className="article-card__title">{article.title}</h3>
        <p className="article-card__summary">{article.summary}</p>
        <span className="article-card__source">{article.source_name}</span>
      </div>
    </article>
  );
}
