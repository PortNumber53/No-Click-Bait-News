import type { ArticleFeed, AuthResponse, SubscriptionTier } from '../types';

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

  getFeed(page = 1, pageSize = 20, category?: string) {
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(pageSize),
    });
    if (category) params.set('category', category);
    return request<ArticleFeed>(
      `${API_BASE}/articles/feed?${params}`,
      { headers: headers(true) },
    );
  },

  getArticle(id: string) {
    return request<import('../types').Article>(
      `${API_BASE}/articles/${id}`,
      { headers: headers(true) },
    );
  },

  getSubscriptionTiers() {
    return request<SubscriptionTier[]>(
      `${API_BASE}/subscriptions/tiers`,
      { headers: headers() },
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
};
