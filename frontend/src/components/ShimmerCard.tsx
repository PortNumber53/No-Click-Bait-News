import './ShimmerCard.css';

export function ShimmerCard() {
  return (
    <div className="shimmer-card">
      <div className="shimmer-card__image shimmer" />
      <div className="shimmer-card__body">
        <div className="shimmer-card__meta">
          <div className="shimmer shimmer--tag" />
          <div className="shimmer shimmer--time" />
        </div>
        <div className="shimmer shimmer--title" />
        <div className="shimmer shimmer--title shimmer--short" />
        <div className="shimmer shimmer--text" />
        <div className="shimmer shimmer--text shimmer--medium" />
        <div className="shimmer shimmer--source" />
      </div>
    </div>
  );
}
