import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import './AuthPage.css';

export function LoginPage() {
  const { login, isLoading, error, clearError } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    login(email.trim(), password);
  };

  return (
    <div className="auth-page">
      <form className="auth-form" onSubmit={handleSubmit}>
        <div className="auth-form__icon">&#128240;</div>
        <h1 className="auth-form__title">No-Click Bait News</h1>
        <p className="auth-form__subtitle">Just the facts. No clickbait.</p>

        {error && <p className="auth-form__error">{error}</p>}

        <label className="field">
          <span className="field__label">Email</span>
          <input
            type="email"
            className="field__input"
            value={email}
            onChange={e => setEmail(e.target.value)}
            required
          />
        </label>

        <label className="field">
          <span className="field__label">Password</span>
          <input
            type="password"
            className="field__input"
            value={password}
            onChange={e => setPassword(e.target.value)}
            required
            minLength={6}
          />
        </label>

        <button type="submit" className="btn btn--filled auth-form__submit" disabled={isLoading}>
          {isLoading ? 'Signing in...' : 'Sign In'}
        </button>

        <Link to="/register" className="auth-form__link" onClick={clearError}>
          Don't have an account? Sign up
        </Link>
      </form>
    </div>
  );
}
