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
  const categories = article.categories?.length ? article.categories : article.category ? [article.category] : [];
  const rewrites = article.rewrites ?? [];
  const hasRewrites = rewrites.length >= 2;

  return (
    <article className="article-card" onClick={onClick}>
      <div className="article-card__meta">
        {categories.slice(0, 3).map(category => (
          <span key={category} className="article-card__category">{category}</span>
        ))}
        {article.is_premium && (
          <span className="article-card__premium" title="Premium">&#9733;</span>
        )}
        <span className="article-card__source">{article.source_name}</span>
        <span className="article-card__time">{timeAgo(article.published_at)}</span>
      </div>

      {article.image_url && (
        <div className="article-card__image">
          <img src={article.image_url} alt="" loading="lazy" />
        </div>
      )}
      {hasRewrites ? (
        <div className="article-card__columns">
          <div className="article-card__col article-card__col--rewrite">
            <span className="article-card__col-label">{rewrites[0].model_name}</span>
            <h3 className="article-card__title">{rewrites[0].title}</h3>
            <p className="article-card__summary">{rewrites[0].summary}</p>
          </div>
          <div className="article-card__col article-card__col--original">
            <span className="article-card__col-label">Original</span>
            <h3 className="article-card__title">{article.title}</h3>
            <p className="article-card__summary">{article.summary}</p>
          </div>
          <div className="article-card__col article-card__col--rewrite">
            <span className="article-card__col-label">{rewrites[1].model_name}</span>
            <h3 className="article-card__title">{rewrites[1].title}</h3>
            <p className="article-card__summary">{rewrites[1].summary}</p>
          </div>
        </div>
      ) : (
        <div className="article-card__body">
          <h3 className="article-card__title">{article.title}</h3>
          <p className="article-card__summary">{article.summary}</p>
        </div>
      )}
    </article>
  );
}
