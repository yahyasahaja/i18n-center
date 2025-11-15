/**
 * Deployment stage for translations
 */
export type DeploymentStage = 'draft' | 'staging' | 'production';

/**
 * Configuration options for i18n-center client
 */
export interface I18nCenterConfig {
  /** API base URL (e.g., 'https://api.example.com/api') */
  apiUrl: string;
  /** API token for authentication */
  apiToken?: string;
  /** Default locale (default: 'en') */
  defaultLocale?: string;
  /** Default deployment stage (default: 'production') */
  defaultStage?: DeploymentStage;
  /** Cache TTL in milliseconds (default: 3600000 = 1 hour) */
  cacheTTL?: number;
  /** Enable caching (default: true) */
  enableCache?: boolean;
}

/**
 * Translation data structure (nested JSON object)
 */
export type TranslationData = Record<string, any>;

/**
 * Cache entry for translations
 */
interface CacheEntry {
  data: TranslationData;
  timestamp: number;
  ttl: number;
}

/**
 * Cache storage (in-memory by default, can be extended)
 */
export interface CacheStorage {
  get(key: string): TranslationData | null;
  set(key: string, data: TranslationData, ttl: number): void;
  clear(): void;
}

/**
 * Next.js integration configuration
 */
export interface NextJsI18nConfig {
  /** i18n-center client instance */
  client: any; // I18nCenterClient (avoid circular import)
  /** Application code (required) */
  applicationCode: string;
  /** Component codes to preload */
  componentCodes: string[];
  /** Default locale */
  defaultLocale?: string;
  /** Default stage */
  defaultStage?: DeploymentStage;
}

