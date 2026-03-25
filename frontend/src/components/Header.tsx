import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import './Header.css';

export function Header() {
  const { isAuthenticated, logout } = useAuth();
  const navigate = useNavigate();

  return (
    <header className="header">
      <Link to="/" className="header__title">No-Click Bait News</Link>
      {isAuthenticated && (
        <nav className="header__actions">
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
