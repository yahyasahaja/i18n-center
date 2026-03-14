// Use native fetch (Node 18+) or node-fetch for older versions
declare const require: (id: string) => unknown;

let fetchFn: typeof fetch;
if (typeof fetch !== 'undefined') {
  fetchFn = fetch;
} else {
  // Fallback for Node < 18
  fetchFn = require('node-fetch') as typeof fetch;
}
import { I18nCenterConfig, DeploymentStage, TranslationData, CacheStorage } from './types';
import { createCacheKey, defaultCache } from './cache';

/**
 * i18n-center API client
 */
export class I18nCenterClient {
  private config: Required<Pick<I18nCenterConfig, 'apiUrl' | 'defaultLocale' | 'defaultStage' | 'cacheTTL' | 'enableCache'>> & {
    apiToken?: string;
  };
  private cache: CacheStorage;

  constructor(config: I18nCenterConfig, cache?: CacheStorage) {
    this.config = {
      apiUrl: config.apiUrl.replace(/\/$/, ''), // Remove trailing slash
      defaultLocale: config.defaultLocale || 'en',
      defaultStage: config.defaultStage || 'production',
      cacheTTL: config.cacheTTL || 3600000, // 1 hour
      enableCache: config.enableCache !== false,
      apiToken: config.apiToken,
    };
    this.cache = cache || defaultCache;
  }

  /**
   * Get translation for a single component
   */
  async getTranslation(
    applicationCode: string,
    componentCode: string,
    locale?: string,
    stage?: DeploymentStage
  ): Promise<TranslationData> {
    const loc = locale || this.config.defaultLocale;
    const stg = stage || this.config.defaultStage;

    // Check cache first
    if (this.config.enableCache) {
      const cacheKey = createCacheKey(applicationCode, componentCode, loc, stg);
      const cached = this.cache.get(cacheKey);
      if (cached) {
        return cached;
      }
    }

    // Fetch from API
    const url = `${this.config.apiUrl}/translations/bulk?application_code=${encodeURIComponent(applicationCode)}&component_codes=${encodeURIComponent(componentCode)}&locale=${loc}&stage=${stg}`;

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (this.config.apiToken) {
      headers['Authorization'] = `Bearer ${this.config.apiToken}`;
    }

    const response = await fetchFn(url, { headers });

    if (!response.ok) {
      if (response.status === 404) {
        throw new Error(`Translation not found for component: ${componentCode}, locale: ${loc}, stage: ${stg}`);
      }
      throw new Error(`Failed to fetch translation: ${response.statusText}`);
    }

    const data = await response.json() as Record<string, TranslationData>;
    const translation = data[componentCode];

    if (!translation) {
      throw new Error(`Translation not found for component: ${componentCode}`);
    }

    // Cache the result
    if (this.config.enableCache) {
      const cacheKey = createCacheKey(applicationCode, componentCode, loc, stg);
      this.cache.set(cacheKey, translation, this.config.cacheTTL);
    }

    return translation;
  }

  /**
   * Get translations for multiple components at once
   */
  async getMultipleTranslations(
    applicationCode: string,
    componentCodes: string[],
    locale?: string,
    stage?: DeploymentStage
  ): Promise<Record<string, TranslationData>> {
    const loc = locale || this.config.defaultLocale;
    const stg = stage || this.config.defaultStage;

    // Check cache for all components
    const results: Record<string, TranslationData> = {};
    const missingCodes: string[] = [];

    if (this.config.enableCache) {
      for (const code of componentCodes) {
        const cacheKey = createCacheKey(applicationCode, code, loc, stg);
        const cached = this.cache.get(cacheKey);
        if (cached) {
          results[code] = cached;
        } else {
          missingCodes.push(code);
        }
      }
    } else {
      missingCodes.push(...componentCodes);
    }

    // Fetch missing translations from API
    if (missingCodes.length > 0) {
      const codesParam = missingCodes.map(c => encodeURIComponent(c)).join(',');
      const url = `${this.config.apiUrl}/translations/bulk?application_code=${encodeURIComponent(applicationCode)}&component_codes=${codesParam}&locale=${loc}&stage=${stg}`;

      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };

      if (this.config.apiToken) {
        headers['Authorization'] = `Bearer ${this.config.apiToken}`;
      }

      const response = await fetchFn(url, { headers });

      if (!response.ok) {
        throw new Error(`Failed to fetch translations: ${response.statusText}`);
      }

      const data = await response.json() as Record<string, TranslationData>;

      // Cache and add to results
      for (const code of missingCodes) {
        const translation = data[code];
        if (translation) {
          results[code] = translation;

          if (this.config.enableCache) {
            const cacheKey = createCacheKey(applicationCode, code, loc, stg);
            this.cache.set(cacheKey, translation, this.config.cacheTTL);
          }
        }
      }
    }

    return results;
  }

  /**
   * Get translations for all components that have the given tag
   * @param applicationId - Application UUID
   * @param tagCode - Tag code (e.g. 'checkout', 'pdp')
   * @param locale - Optional locale (default from config)
   * @param stage - Optional stage (default from config)
   * @returns Map of component code -> translation data
   */
  async getTranslationsByTag(
    applicationId: string,
    tagCode: string,
    locale?: string,
    stage?: DeploymentStage
  ): Promise<Record<string, TranslationData>> {
    const loc = locale || this.config.defaultLocale;
    const stg = stage || this.config.defaultStage;
    const code = tagCode.trim().toLowerCase();
    if (!code) {
      throw new Error('Tag code is required');
    }

    const cacheKey = `bytag:${applicationId}:${code}:${loc}:${stg}`;
    if (this.config.enableCache) {
      const cached = this.cache.get(cacheKey);
      if (cached) {
        return cached as Record<string, TranslationData>;
      }
    }

    const pathTag = encodeURIComponent(code);
    const url = `${this.config.apiUrl}/applications/${encodeURIComponent(applicationId)}/translations/by-tag/${pathTag}?locale=${loc}&stage=${stg}`;
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.config.apiToken) {
      headers['Authorization'] = `Bearer ${this.config.apiToken}`;
    }
    const response = await fetchFn(url, { headers });
    if (!response.ok) {
      throw new Error(`Failed to fetch translations by tag: ${response.statusText}`);
    }
    const data = (await response.json()) as Record<string, TranslationData>;
    if (this.config.enableCache) {
      this.cache.set(cacheKey, data, this.config.cacheTTL);
    }
    return data;
  }

  /**
   * Get translations for all components that have the given page
   * @param applicationId - Application UUID
   * @param pageCode - Page code (e.g. 'home', 'cart')
   * @param locale - Optional locale (default from config)
   * @param stage - Optional stage (default from config)
   * @returns Map of component code -> translation data
   */
  async getTranslationsByPage(
    applicationId: string,
    pageCode: string,
    locale?: string,
    stage?: DeploymentStage
  ): Promise<Record<string, TranslationData>> {
    const loc = locale || this.config.defaultLocale;
    const stg = stage || this.config.defaultStage;
    const code = pageCode.trim().toLowerCase();
    if (!code) {
      throw new Error('Page code is required');
    }

    const cacheKey = `bypage:${applicationId}:${code}:${loc}:${stg}`;
    if (this.config.enableCache) {
      const cached = this.cache.get(cacheKey);
      if (cached) {
        return cached as Record<string, TranslationData>;
      }
    }

    const pathPage = encodeURIComponent(code);
    const url = `${this.config.apiUrl}/applications/${encodeURIComponent(applicationId)}/translations/by-page/${pathPage}?locale=${loc}&stage=${stg}`;
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.config.apiToken) {
      headers['Authorization'] = `Bearer ${this.config.apiToken}`;
    }
    const response = await fetchFn(url, { headers });
    if (!response.ok) {
      throw new Error(`Failed to fetch translations by page: ${response.statusText}`);
    }
    const data = (await response.json()) as Record<string, TranslationData>;
    if (this.config.enableCache) {
      this.cache.set(cacheKey, data, this.config.cacheTTL);
    }
    return data;
  }

  /**
   * Clear the cache
   */
  clearCache(): void {
    this.cache.clear();
  }
}

