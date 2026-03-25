export interface Article {
  id: string;
  title: string;
  summary: string;
  content?: string;
  original_content?: string;
  rewrite_status: 'pending' | 'complete' | 'failed' | string;
  source_name: string;
  source_url: string;
  image_url?: string;
  category?: string;
  categories?: string[];
  llm_rewrite_version: number;
  published_at: string;
  is_premium: boolean;
  view_count: number;
  rewrites?: RewriteVersion[];
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
  is_current: boolean;
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

export interface RewriteVersion {
  id: string;
  model_name: string;
  title: string;
  summary: string;
  content?: string;
}

export interface ComparisonData {
  article_id: string;
  original_title: string;
  source_name: string;
  source_url: string;
  image_url?: string;
  category?: string;
  published_at: string;
  version_a: RewriteVersion;
  version_b: RewriteVersion;
  user_vote: 'a' | 'b' | null;
}

export interface VoteStats {
  article_id: string;
  version_a_id: string;
  version_a_name: string;
  version_a_votes: number;
  version_b_id: string;
  version_b_name: string;
  version_b_votes: number;
}
