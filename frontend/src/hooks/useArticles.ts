import { useState, useCallback, useRef, useEffect } from 'react';
import { api } from '../services/api';
import type { Article } from '../types';

export function useArticles() {
  const [articles, setArticles] = useState<Article[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string | undefined>();
  const pageRef = useRef(1);
  const loadingRef = useRef(false);
  const generationRef = useRef(0);

  const loadArticles = useCallback(async (refresh = false, signal?: AbortSignal) => {
    if (loadingRef.current && !refresh) return;
    if (!refresh && !hasMore) return;
    const generation = refresh ? ++generationRef.current : generationRef.current;

    loadingRef.current = true;
    setIsLoading(true);
    setError(null);

    const page = refresh ? 1 : pageRef.current;

    try {
      const feed = await api.getFeed(page, 20, selectedCategory, signal);
      if (generation !== generationRef.current) return;
      setArticles(prev => refresh ? feed.articles : [...prev, ...feed.articles]);
      setHasMore(feed.has_more);
      pageRef.current = page + 1;
    } catch (e) {
      if (signal?.aborted || generation !== generationRef.current) return;
      setError(e instanceof Error ? e.message : 'Failed to load articles');
    } finally {
      if (generation === generationRef.current) {
        setIsLoading(false);
        loadingRef.current = false;
      }
    }
  }, [selectedCategory, hasMore]);

  const changeCategory = useCallback((category: string | undefined) => {
    if (category === selectedCategory) return;
    setSelectedCategory(category);
    setArticles([]);
    setHasMore(true);
    pageRef.current = 1;
  }, [selectedCategory]);

  useEffect(() => {
    const controller = new AbortController();
    loadArticles(true, controller.signal);
    return () => controller.abort();
  }, [selectedCategory]); // eslint-disable-line react-hooks/exhaustive-deps

  return { articles, isLoading, hasMore, error, selectedCategory, changeCategory, loadArticles };
}
