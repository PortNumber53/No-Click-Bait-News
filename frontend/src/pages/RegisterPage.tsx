import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../contexts/auth-context';
import './AuthPage.css';

export function RegisterPage() {
  const { register, isLoading, error, clearError } = useAuth();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    register(email.trim(), password, name.trim());
  };

  return (
    <div className="auth-page">
      <form className="auth-form" onSubmit={handleSubmit}>
        <img className="auth-form__logo" src="/noclickbait-news-logo.png" alt="" />
        <h1 className="auth-form__title">Create Account</h1>
        <p className="auth-form__subtitle">Join for unbiased news.</p>

        {error && <p className="auth-form__error">{error}</p>}

        <label className="field">
          <span className="field__label">Name</span>
          <input
            type="text"
            className="field__input"
            value={name}
            onChange={e => setName(e.target.value)}
            required
          />
        </label>

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
            minLength={8}
          />
        </label>

        <button type="submit" className="btn btn--filled auth-form__submit" disabled={isLoading}>
          {isLoading ? 'Creating account...' : 'Sign Up'}
        </button>

        <Link to="/login" className="auth-form__link" onClick={clearError}>
          Already have an account? Sign in
        </Link>
      </form>
    </div>
  );
}
