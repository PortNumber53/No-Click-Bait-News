import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../services/api';
import type { SubscriptionTier } from '../types';
import './SubscriptionPage.css';

export function SubscriptionPage() {
  const navigate = useNavigate();
  const [tiers, setTiers] = useState<SubscriptionTier[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [subscribing, setSubscribing] = useState<number | null>(null);

  useEffect(() => {
    api.getSubscriptionTiers()
      .then(setTiers)
      .catch(() => {})
      .finally(() => setIsLoading(false));
  }, []);

  const subscribe = async (tier: SubscriptionTier) => {
    setSubscribing(tier.id);
    try {
      const data = await api.createCheckout(tier.id);
      if (data.checkout_url) {
        window.open(data.checkout_url, '_blank');
      }
    } catch {
      // handled silently
    } finally {
      setSubscribing(null);
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
      <div className="sub__grid">
        {tiers.map(tier => {
          const isPremium = tier.name === 'premium';
          return (
            <div key={tier.id} className={`sub-card ${isPremium ? 'sub-card--featured' : ''}`}>
              {isPremium && <span className="sub-card__badge">MOST POPULAR</span>}
              <h2 className="sub-card__name">
                {tier.name.charAt(0).toUpperCase() + tier.name.slice(1)}
              </h2>
              <div className="sub-card__price">
                <span className="sub-card__amount">${tier.price_monthly.toFixed(2)}</span>
                <span className="sub-card__period">/month</span>
              </div>
              <ul className="sub-card__features">
                <li>{tier.max_articles_per_day} articles/day</li>
                {tier.has_premium_access && <li>&#9733; Premium content access</li>}
              </ul>
              {tier.price_monthly > 0 ? (
                <button
                  className="btn btn--filled sub-card__btn"
                  onClick={() => subscribe(tier)}
                  disabled={subscribing === tier.id}
                >
                  {subscribing === tier.id ? 'Loading...' : 'Subscribe'}
                </button>
              ) : (
                <button className="btn btn--tonal sub-card__btn" disabled>
                  Current Plan
                </button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
