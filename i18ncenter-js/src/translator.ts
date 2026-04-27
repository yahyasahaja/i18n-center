import { I18nCenterClient } from './client';
import { DeploymentStage, TranslationData } from './types';

/**
 * Get nested value from an object using dot notation path.
 * Returns the raw value (string or object) without coercion.
 */
function getNestedValue(obj: any, path: string): unknown {
  const keys = path.split('.');
  let current: unknown = obj;

  for (const key of keys) {
    if (current == null || typeof current !== 'object') {
      return null;
    }
    current = (current as Record<string, unknown>)[key];
  }

  return current ?? null;
}

/**
 * Translation function options
 */
export interface TranslateOptions {
  /** Locale override */
  locale?: string;
  /** Stage override */
  stage?: DeploymentStage;
  /** Default value if translation not found */
  defaultValue?: string;
  /** Replace template variables (e.g., {name} -> value) */
  variables?: Record<string, string | number>;
}

/**
 * Create a translation function (t) for a specific component
 */
export function createTranslator(
  client: I18nCenterClient,
  applicationCode: string,
  componentCode: string,
  defaultLocale?: string,
  defaultStage?: DeploymentStage
) {
  // Cache for component translations
  let cachedTranslation: TranslationData | null = null;
  let cachedLocale: string | undefined;
  let cachedStage: DeploymentStage | undefined;

  /**
   * Translation function
   * Usage: t('form.name.label') or t<{b: string}>('a')
   *
   * When T is string (default): returns the translation string, or defaultValue/path on miss.
   * When T is an object type: returns the nested object at that path.
   *
   * Template variable substitution only applies when the resolved value is a string.
   */
  async function t<T = string>(path: string, options?: TranslateOptions): Promise<T> {
    try {
      const locale = options?.locale || defaultLocale || client['config'].defaultLocale;
      const stage = options?.stage || defaultStage || client['config'].defaultStage;

      // Check if we need to reload translation
      if (!cachedTranslation || cachedLocale !== locale || cachedStage !== stage) {
        try {
          cachedTranslation = await client.getTranslation(applicationCode, componentCode, locale, stage);
          cachedLocale = locale;
          cachedStage = stage;
        } catch (error) {
          return (options?.defaultValue ?? path) as unknown as T;
        }
      }

      const value = getNestedValue(cachedTranslation, path);

      if (value === null || value === undefined) {
        return (options?.defaultValue ?? path) as unknown as T;
      }

      // Template variable substitution only for string values
      if (typeof value === 'string' && options?.variables) {
        let result = value;
        for (const [key, val] of Object.entries(options.variables)) {
          result = result.replace(new RegExp(`\\{${key}\\}`, 'g'), String(val));
          result = result.replace(new RegExp(`\\[${key}\\]`, 'g'), String(val));
        }
        return result as unknown as T;
      }

      return value as unknown as T;
    } catch (error) {
      return (options?.defaultValue ?? path) as unknown as T;
    }
  }

  /**
   * Preload translation (useful for SSR)
   */
  async function preload(locale?: string, stage?: DeploymentStage): Promise<void> {
    const loc = locale || defaultLocale || client['config'].defaultLocale;
    const stg = stage || defaultStage || client['config'].defaultStage;
    cachedTranslation = await client.getTranslation(applicationCode, componentCode, loc, stg);
    cachedLocale = loc;
    cachedStage = stg;
  }

  /**
   * Get raw translation data
   */
  async function getRaw(locale?: string, stage?: DeploymentStage): Promise<TranslationData> {
    const loc = locale || defaultLocale || client['config'].defaultLocale;
    const stg = stage || defaultStage || client['config'].defaultStage;
    return await client.getTranslation(applicationCode, componentCode, loc, stg);
  }

  return {
    t,
    preload,
    getRaw,
  };
}

/**
 * Synchronous translation function (requires preloaded data)
 * 
 * @returns Always returns a string. Never throws errors.
 * - Returns the translation value if found
 * - Returns `defaultValue` if provided and translation not found
 * - Returns the `path` itself if translation not found and no defaultValue
 */
export function createSyncTranslator(translationData: TranslationData) {
  return function t<T = string>(path: string, options?: { defaultValue?: string; variables?: Record<string, string | number> }): T {
    try {
      const value = getNestedValue(translationData, path);

      if (value === null || value === undefined) {
        return (options?.defaultValue ?? path) as unknown as T;
      }

      if (typeof value === 'string' && options?.variables) {
        let result = value;
        for (const [key, val] of Object.entries(options.variables)) {
          result = result.replace(new RegExp(`\\{${key}\\}`, 'g'), String(val));
          result = result.replace(new RegExp(`\\[${key}\\]`, 'g'), String(val));
        }
        return result as unknown as T;
      }

      return value as unknown as T;
    } catch (error) {
      return (options?.defaultValue ?? path) as unknown as T;
    }
  };
}

