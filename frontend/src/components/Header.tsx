import { useState } from 'react';
import type { FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/auth-context';
import { api } from '../services/api';
import './Header.css';

export function Header() {
  const { isAuthenticated, logout } = useAuth();
  const navigate = useNavigate();
  const [url, setUrl] = useState('');
  const [isFetching, setIsFetching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleFetchArticle(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = url.trim();
    if (!trimmed || isFetching) return;

    setIsFetching(true);
    setError(null);

    try {
      const article = await api.fetchArticleUrl(trimmed);
      setUrl('');
      navigate(`/article/${article.id}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch URL');
    } finally {
      setIsFetching(false);
    }
  }

  return (
    <header className="header">
      <Link to="/" className="header__title">No-Click Bait News</Link>
      {isAuthenticated && (
        <nav className="header__actions">
          <form className="header__url-form" onSubmit={handleFetchArticle}>
            <input
              className="header__url-input"
              type="url"
              inputMode="url"
              value={url}
              onChange={(event) => setUrl(event.target.value)}
              placeholder="Paste news URL"
              aria-label="News URL"
              disabled={isFetching}
            />
            <button
              className="header__btn header__btn--icon"
              type="submit"
              title="Fetch URL"
              aria-label="Fetch URL"
              disabled={isFetching || !url.trim()}
            >
              {isFetching ? <span className="header__spinner" /> : '+'}
            </button>
            {error && <span className="header__error" role="status">{error}</span>}
          </form>
          <Link className="header__btn header__link" to="/my-urls">
            My URLs
          </Link>
          <button
            className="header__btn"
            onClick={() => navigate('/subscriptions')}
            title="Subscriptions"
          >
            &#9830;
          </button>
          <button
            className="header__btn"
            onClick={logout}
            title="Sign Out"
          >
            Sign Out
          </button>
        </nav>
      )}
    </header>
  );
}
