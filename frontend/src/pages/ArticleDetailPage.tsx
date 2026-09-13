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
  const [biasError, setBiasError] = useState<string | null>(null);
  const [loadingBias, setLoadingBias] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    let pollTimer: ReturnType<typeof setTimeout> | undefined;
    let pollsRemaining = 30;

    const loadComparison = () => api.getComparison(id)
      .then(data => { if (!cancelled) setComparison(data); })
      .catch((e) => {
        if (!(e instanceof ApiError && e.status === 404)) {
          console.error('Failed to load comparison:', e);
        }
      });

    const loadArticle = () => api.getArticle(id)
      .then(data => {
        if (cancelled) return;
        setArticle(data);
        if (data.rewrite_status === 'pending' && pollsRemaining-- > 0) {
          pollTimer = setTimeout(loadArticle, 2000);
        } else {
          void loadComparison();
        }
      })
      .catch(e => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load article');
      });

    void loadArticle();
    return () => {
      cancelled = true;
      if (pollTimer) clearTimeout(pollTimer);
    };
  }, [id]);

  const revealBiasReasoning = async (rewriteId: string) => {
    if (!article || loadingBias) return;
    setLoadingBias(rewriteId);
    setBiasError(null);
    try {
      const revealed = await api.revealBiasReasoning(article.id, rewriteId);
      setArticle(current => current ? {
        ...current,
        rewrites: current.rewrites?.map(rewrite => rewrite.id === rewriteId ? {
          ...rewrite,
          bias_label: revealed.bias_label,
          bias_reasoning: revealed.bias_reasoning,
          bias_reasoning_available: true,
          bias_reasoning_unlocked: true,
        } : rewrite),
      } : current);
    } catch (reason) {
      setBiasError(reason instanceof Error ? reason.message : 'Failed to open bias explanation');
    } finally {
      setLoadingBias(null);
    }
  };

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

        {article.rewrites?.some(rewrite => rewrite.bias_label) && (
          <section className="detail__bias-section" aria-labelledby="bias-heading">
            <div className="detail__bias-heading">
              <div>
                <span className="detail__bias-eyebrow">AI analysis</span>
                <h2 id="bias-heading">Bias check</h2>
              </div>
              <span className="detail__bias-caveat">Signal, not a verdict</span>
            </div>
            <div className="detail__bias-grid">
              {article.rewrites.filter(rewrite => rewrite.bias_label).map((rewrite, index) => (
                <article className="detail__bias-card" key={rewrite.id}>
                  <span className="detail__bias-model">Version {String.fromCharCode(65 + index)}</span>
                  <strong>{rewrite.bias_label}</strong>
                  {rewrite.bias_reasoning_unlocked && rewrite.bias_reasoning ? (
                    <p>{rewrite.bias_reasoning}</p>
                  ) : rewrite.bias_reasoning_available ? (
                    <>
                      <p className="detail__bias-access">Paid plans include every explanation. Free accounts can open five each day.</p>
                      <button
                        className="btn btn--tonal"
                        disabled={loadingBias === rewrite.id}
                        onClick={() => void revealBiasReasoning(rewrite.id)}
                      >
                        {loadingBias === rewrite.id ? 'Opening…' : 'Why this rating?'}
                      </button>
                    </>
                  ) : null}
                </article>
              ))}
            </div>
            {biasError && <p className="detail__bias-error" role="alert">{biasError}</p>}
          </section>
        )}

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
