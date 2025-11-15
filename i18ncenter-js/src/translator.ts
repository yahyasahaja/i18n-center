import { I18nCenterClient } from './client';
import { DeploymentStage, TranslationData } from './types';

/**
 * Get nested value from an object using dot notation path
 */
function getNestedValue(obj: any, path: string): string | null {
  const keys = path.split('.');
  let current = obj;

  for (const key of keys) {
    if (current == null || typeof current !== 'object') {
      return null;
    }
    current = current[key];
  }

  if (typeof current === 'string') {
    return current;
  }

  // If it's an object, try to find a common 'text' or 'value' field
  if (typeof current === 'object' && current !== null) {
    return current.text || current.value || current.label || JSON.stringify(current);
  }

  return null;
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
   * Usage: t('form.name.label') or t('form.name.label', { variables: { name: 'John' } })
   * 
   * @returns Always returns a string. Never throws errors.
   * - Returns the translation value if found
   * - Returns `defaultValue` if provided and translation not found
   * - Returns the `path` itself if translation not found and no defaultValue
   */
  async function t(path: string, options?: TranslateOptions): Promise<string> {
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
          // If translation fetch fails, return defaultValue or path
          return options?.defaultValue || path;
        }
      }

      // Get the translation value
      let value = getNestedValue(cachedTranslation, path);

      if (value === null) {
        // Try to return default value or the path itself
        return options?.defaultValue || path;
      }

      // Replace template variables
      if (options?.variables) {
        for (const [key, val] of Object.entries(options.variables)) {
          // Support both {key} and [key] syntax
          value = value.replace(new RegExp(`\\{${key}\\}`, 'g'), String(val));
          value = value.replace(new RegExp(`\\[${key}\\]`, 'g'), String(val));
        }
      }

      return value;
    } catch (error) {
      // Safety net: always return a string, never throw
      return options?.defaultValue || path;
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
  return function t(path: string, options?: { defaultValue?: string; variables?: Record<string, string | number> }): string {
    try {
      let value = getNestedValue(translationData, path);

      if (value === null) {
        return options?.defaultValue || path;
      }

      // Replace template variables
      if (options?.variables) {
        for (const [key, val] of Object.entries(options.variables)) {
          value = value.replace(new RegExp(`\\{${key}\\}`, 'g'), String(val));
          value = value.replace(new RegExp(`\\[${key}\\]`, 'g'), String(val));
        }
      }

      return value;
    } catch (error) {
      // Safety net: always return a string, never throw
      return options?.defaultValue || path;
    }
  };
}

