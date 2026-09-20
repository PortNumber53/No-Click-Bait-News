import { useState, useCallback, useEffect, type ReactNode } from 'react';
import { api, ApiError } from '../services/api';
import type { User } from '../types';
import { AuthContext } from './auth-context';
import { clearStoredAuth, getStoredToken, getStoredUser, storeAuthSession, updateStoredUser } from '../services/authStorage';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(getStoredUser);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isAuthenticated = user !== null && !!getStoredToken();

  useEffect(() => {
    if (!getStoredToken()) return;
    api.getMe().then(currentUser => {
      updateStoredUser(currentUser);
      setUser(currentUser);
    }).catch(() => {
      clearStoredAuth();
      setUser(null);
    });
  }, []);

  useEffect(() => {
    const handleUnauthorized = () => setUser(null);
    window.addEventListener('auth:unauthorized', handleUnauthorized);
    return () => window.removeEventListener('auth:unauthorized', handleUnauthorized);
  }, []);

  const login = useCallback(async (email: string, password: string, rememberMe: boolean) => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.login(email, password, rememberMe);
      storeAuthSession(data.access_token, data.user, rememberMe);
      setUser(data.user);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Login failed');
    } finally {
      setIsLoading(false);
    }
  }, []);

  const register = useCallback(async (email: string, password: string, name: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.register(email, password, name);
      // Account creation keeps the existing persistent-session behavior. The
      // explicit choice is available on subsequent sign-ins.
      storeAuthSession(data.access_token, data.user, true);
      setUser(data.user);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Registration failed');
    } finally {
      setIsLoading(false);
    }
  }, []);

  const logout = useCallback(() => {
    clearStoredAuth();
    setUser(null);
  }, []);

  const clearError = useCallback(() => setError(null), []);

  return (
    <AuthContext.Provider value={{ user, isLoading, error, isAuthenticated, login, register, logout, clearError }}>
      {children}
    </AuthContext.Provider>
  );
}
