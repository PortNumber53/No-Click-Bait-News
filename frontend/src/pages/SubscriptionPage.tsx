import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../services/api';
import type { SubscriptionTier } from '../types';
import './SubscriptionPage.css';

export function SubscriptionPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [tiers, setTiers] = useState<SubscriptionTier[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [subscribing, setSubscribing] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isManaging, setIsManaging] = useState(false);

  useEffect(() => {
    api.getSubscriptionTiers()
      .then(setTiers)
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load subscription plans'))
      .finally(() => setIsLoading(false));
  }, []);

  const subscribe = async (tier: SubscriptionTier) => {
    setSubscribing(tier.id);
    setError(null);
    try {
      const data = await api.createCheckout(tier.id);
      if (data.checkout_url) {
        window.location.href = data.checkout_url;
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to start checkout');
    } finally {
      setSubscribing(null);
    }
  };

  const manageBilling = async () => {
    setIsManaging(true);
    setError(null);
    try {
      const data = await api.createBillingPortal();
      window.location.href = data.portal_url;
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to open Stripe billing');
      setIsManaging(false);
    }
  };

  if (isLoading) {
    return <div className="sub__loading"><div className="spinner" /></div>;
  }

  return (
    <div className="sub">
      <div className="sub__header">
        <button className="detail__back" onClick={() => navigate(-1)}>&larr; Back</button>
        <h1 className="sub__title">Subscription Plans</h1>
      </div>
      {searchParams.get('success') === 'true' && (
        <p className="sub__notice sub__notice--success" role="status">
          Stripe checkout completed. Your Unlimited access will appear as soon as payment is confirmed.
        </p>
      )}
      {searchParams.get('canceled') === 'true' && (
        <p className="sub__notice" role="status">Checkout canceled. Your plan was not changed.</p>
      )}
      {error && <p className="home__error" role="alert">{error}</p>}
      <div className="sub__grid">
        {tiers.map(tier => {
          const isPremium = tier.unlimited_reading;
          return (
            <div key={tier.id} className={`sub-card ${isPremium ? 'sub-card--featured' : ''} ${tier.is_current ? 'sub-card--current' : ''}`}>
              {isPremium && !tier.is_current && <span className="sub-card__badge">MOST POPULAR</span>}
              {tier.is_current && <span className="sub-card__badge sub-card__badge--current">YOUR PLAN</span>}
              <h2 className="sub-card__name">
                {isPremium ? 'Unlimited' : 'Free'}
              </h2>
              <div className="sub-card__price">
                <span className="sub-card__amount">${tier.price_monthly.toFixed(2)}</span>
                <span className="sub-card__period">/month</span>
              </div>
              <ul className="sub-card__features">
                <li>{isPremium ? 'Unlimited news reading' : '1 article per category, every day'}</li>
                {isPremium && <li>&#9733; Every category and premium story</li>}
              </ul>
              {tier.is_current && isPremium ? (
                <button
                  className="btn btn--outlined sub-card__btn"
                  onClick={manageBilling}
                  disabled={isManaging}
                >
                  {isManaging ? 'Opening Stripe...' : 'Manage billing with Stripe'}
                </button>
              ) : tier.is_current ? (
                <button className="btn btn--tonal sub-card__btn" disabled>
                  Current Plan
                </button>
              ) : tier.price_monthly > 0 ? (
                <button
                  className="btn btn--filled sub-card__btn"
                  onClick={() => subscribe(tier)}
                  disabled={subscribing === tier.id}
                >
                  {subscribing === tier.id ? 'Opening Stripe...' : 'Continue with Stripe'}
                </button>
              ) : (
                <button className="btn btn--tonal sub-card__btn" disabled>
                  Free
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
