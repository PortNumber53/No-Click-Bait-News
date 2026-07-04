import { useState, useCallback, useRef, useEffect } from 'react';
import { api } from '../services/api';
import type { Article } from '../types';

export function useMyArticles() {
  const [articles, setArticles] = useState<Article[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const pageRef = useRef(1);
  const loadingRef = useRef(false);

  const loadArticles = useCallback(async (refresh = false) => {
    if (loadingRef.current && !refresh) return;
    if (!refresh && !hasMore) return;

    loadingRef.current = true;
    setIsLoading(true);
    setError(null);

    const page = refresh ? 1 : pageRef.current;

    try {
      const feed = await api.getMyArticles(page, 20);
      setArticles(prev => refresh ? feed.articles : [...prev, ...feed.articles]);
      setHasMore(feed.has_more);
      pageRef.current = page + 1;
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load submitted URLs');
    } finally {
      setIsLoading(false);
      loadingRef.current = false;
    }
  }, [hasMore]);

  useEffect(() => {
    loadArticles(true);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return { articles, isLoading, hasMore, error, loadArticles };
}
