export interface Article {
  id: string;
  title: string;
  summary: string;
  content?: string;
  source_name: string;
  source_url: string;
  image_url?: string;
  category?: string;
  published_at: string;
  is_premium: boolean;
  view_count: number;
}

export interface ArticleFeed {
  articles: Article[];
  page: number;
  page_size: number;
  has_more: boolean;
}

export interface SubscriptionTier {
  id: number;
  name: string;
  price_monthly: number;
  max_articles_per_day: number;
  has_premium_access: boolean;
}

export interface User {
  id: string;
  email: string;
  name: string;
  created_at: string;
  subscription_tier: string;
}

export interface AuthResponse {
  access_token: string;
  user: User;
}
