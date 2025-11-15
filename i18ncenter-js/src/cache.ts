import { TranslationData, CacheStorage } from './types';

/**
 * In-memory cache implementation
 */
class MemoryCache implements CacheStorage {
  private cache: Map<string, { data: TranslationData; expires: number }> = new Map();

  get(key: string): TranslationData | null {
    const entry = this.cache.get(key);
    if (!entry) {
      return null;
    }

    // Check if expired
    if (Date.now() > entry.expires) {
      this.cache.delete(key);
      return null;
    }

    return entry.data;
  }

  set(key: string, data: TranslationData, ttl: number): void {
    const expires = Date.now() + ttl;
    this.cache.set(key, { data, expires });
  }

  clear(): void {
    this.cache.clear();
  }
}

/**
 * Create a cache key for a translation request
 * Includes application code to differentiate components with the same code in different applications
 */
export function createCacheKey(
  applicationCode: string,
  componentCode: string,
  locale: string,
  stage: string
): string {
  return `i18n:${applicationCode}:${componentCode}:${locale}:${stage}`;
}

/**
 * Default cache instance
 */
export const defaultCache = new MemoryCache();

