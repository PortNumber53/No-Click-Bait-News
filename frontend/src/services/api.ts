import type { Article, ArticleFeed, AuthResponse, ComparisonData, SubscriptionTier, User, VoteStats } from '../types';

const API_BASE = '/api/v1';

function getToken(): string | null {
  return localStorage.getItem('access_token');
}

function headers(auth = false): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' };
  if (auth) {
    const token = getToken();
    if (token) h['Authorization'] = `Bearer ${token}`;
  }
  return h;
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    if (res.status === 401) {
      localStorage.removeItem('access_token');
      localStorage.removeItem('user');
      window.dispatchEvent(new Event('auth:unauthorized'));
    }
    const body = await res.json().catch(() => ({ detail: res.statusText }));
    throw new ApiError(res.status, body.detail ?? 'Request failed');
  }
  return res.json();
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export const api = {
  login(email: string, password: string) {
    return request<AuthResponse>(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ email, password }),
    });
  },

  register(email: string, password: string, name: string) {
    return request<AuthResponse>(`${API_BASE}/auth/register`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ email, password, name }),
    });
  },

  getMe() {
    return request<User>(`${API_BASE}/auth/me`, {
      headers: headers(true),
    });
  },

  getFeed(page = 1, pageSize = 20, category?: string, signal?: AbortSignal) {
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(pageSize),
    });
    if (category) params.set('category', category);
    return request<ArticleFeed>(
      `${API_BASE}/articles/feed?${params}`,
      { headers: headers(true), signal },
    );
  },

  getMyArticles(page = 1, pageSize = 20) {
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(pageSize),
    });
    return request<ArticleFeed>(
      `${API_BASE}/articles/my?${params}`,
      { headers: headers(true) },
    );
  },

  getArticle(id: string) {
    return request<Article>(
      `${API_BASE}/articles/${id}`,
      { headers: headers(true) },
    );
  },

  fetchArticleUrl(url: string) {
    return request<Article>(
      `${API_BASE}/articles/fetch`,
      {
        method: 'POST',
        headers: headers(true),
        body: JSON.stringify({ url }),
      },
    );
  },

  getSubscriptionTiers() {
    return request<SubscriptionTier[]>(
      `${API_BASE}/subscriptions/tiers`,
      { headers: headers(true) },
    );
  },

  createCheckout(tierId: number) {
    return request<{ checkout_url: string }>(
      `${API_BASE}/subscriptions/checkout`,
      {
        method: 'POST',
        headers: headers(true),
        body: JSON.stringify({ tier_id: tierId }),
      },
    );
  },

  createBillingPortal() {
    return request<{ portal_url: string }>(
      `${API_BASE}/subscriptions/portal`,
      {
        method: 'POST',
        headers: headers(true),
      },
    );
  },

  getComparison(articleId: string) {
    return request<ComparisonData>(
      `${API_BASE}/articles/${articleId}/comparison`,
      { headers: headers(true) },
    );
  },

  submitVote(articleId: string, chosenRewriteId: string, otherRewriteId: string) {
    return request<VoteStats>(
      `${API_BASE}/articles/${articleId}/vote`,
      {
        method: 'POST',
        headers: headers(true),
        body: JSON.stringify({
          chosen_rewrite_id: chosenRewriteId,
          other_rewrite_id: otherRewriteId,
        }),
      },
    );
  },

  getVoteStats(articleId: string) {
    return request<VoteStats>(
      `${API_BASE}/articles/${articleId}/vote-stats`,
      { headers: headers(true) },
    );
  },
};
