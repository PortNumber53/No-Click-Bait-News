import type { User } from '../types';

const TOKEN_KEY = 'access_token';
const USER_KEY = 'user';

function activeStorage(): Storage | null {
  if (localStorage.getItem(TOKEN_KEY)) return localStorage;
  if (sessionStorage.getItem(TOKEN_KEY)) return sessionStorage;
  return null;
}

export function getStoredToken(): string | null {
  return activeStorage()?.getItem(TOKEN_KEY) ?? null;
}

export function getStoredUser(): User | null {
  const storage = activeStorage();
  const stored = storage?.getItem(USER_KEY);
  if (!storage || !stored) return null;
  try {
    return JSON.parse(stored) as User;
  } catch {
    clearStoredAuth();
    return null;
  }
}

export function storeAuthSession(token: string, user: User, remember: boolean) {
  clearStoredAuth();
  const storage = remember ? localStorage : sessionStorage;
  storage.setItem(TOKEN_KEY, token);
  storage.setItem(USER_KEY, JSON.stringify(user));
}

export function updateStoredUser(user: User) {
  activeStorage()?.setItem(USER_KEY, JSON.stringify(user));
}

export function clearStoredAuth() {
  for (const storage of [localStorage, sessionStorage]) {
    storage.removeItem(TOKEN_KEY);
    storage.removeItem(USER_KEY);
  }
}
