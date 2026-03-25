import './CategoryFilter.css';

const CATEGORIES = [
  { value: undefined, label: 'All' },
  { value: 'Technology', label: 'Technology' },
  { value: 'Science', label: 'Science' },
  { value: 'Business', label: 'Business' },
  { value: 'Health', label: 'Health' },
  { value: 'Sports', label: 'Sports' },
  { value: 'World', label: 'World' },
] as const;

interface Props {
  selected: string | undefined;
  onChange: (category: string | undefined) => void;
}

export function CategoryFilter({ selected, onChange }: Props) {
  return (
    <div className="category-filter">
      <div className="category-filter__scroll">
        {CATEGORIES.map(cat => (
          <button
            key={cat.label}
            className={`category-chip ${selected === cat.value ? 'category-chip--active' : ''}`}
            onClick={() => onChange(cat.value)}
          >
            {cat.label}
          </button>
        ))}
      </div>
    </div>
  );
}
