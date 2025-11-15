import { GetServerSidePropsContext } from 'next';
import { I18nCenterClient } from './client';
import { createSyncTranslator } from './translator';
import { DeploymentStage, TranslationData, NextJsI18nConfig } from './types';

/**
 * Extract locale from URL path
 * Supports patterns like: /en-us/pdp, /en_us/pdp, /en/pdp, /id/pdp
 * 
 * @param pathname - URL path (e.g., "/en-us/pdp" or "/en_us/pdp")
 * @returns Locale string or null if not found
 */
function extractLocaleFromPath(pathname: string): string | null {
  // Remove leading slash and split
  const parts = pathname.split('/').filter(Boolean);
  if (parts.length === 0) {
    return null;
  }

  const firstSegment = parts[0];

  // Check if it matches locale patterns:
  // - en-us, en_us (language-country)
  // - en, id, fr (language only, 2-3 chars)
  // Pattern: 2-3 lowercase letters, optionally followed by - or _ and 2-3 lowercase letters
  const localePattern = /^([a-z]{2,3})([-_]([a-z]{2,3}))?$/i;
  const match = firstSegment.match(localePattern);

  if (match) {
    // Return normalized locale (use hyphen, lowercase)
    const lang = match[1].toLowerCase();
    const country = match[3] ? match[3].toLowerCase() : null;
    return country ? `${lang}-${country}` : lang;
  }

  return null;
}

/**
 * Get locale from Next.js context (supports next-i18next, next-intl, URL patterns, or custom)
 * 
 * Priority order:
 * 1. Next.js locale (from next-i18next or next-intl)
 * 2. Query parameter (?locale=en-us)
 * 3. URL path pattern (/en-us/pdp or /en_us/pdp)
 * 4. Accept-Language header
 * 5. Default locale
 */
export function getLocaleFromContext(
  context: GetServerSidePropsContext,
  defaultLocale: string = 'en'
): string {
  // Try next-i18next
  if (context.locale) {
    return context.locale;
  }

  // Try next-intl
  if ((context as any).locale) {
    return (context as any).locale;
  }

  // Try query parameter
  if (context.query.locale && typeof context.query.locale === 'string') {
    return context.query.locale;
  }

  // Try URL path pattern (e.g., /en-us/pdp, /en_us/pdp, /en/pdp)
  // Check resolvedUrl first (Pages Router)
  if (context.resolvedUrl) {
    const localeFromPath = extractLocaleFromPath(context.resolvedUrl);
    if (localeFromPath) {
      return localeFromPath;
    }
  }
  // Also check req.url (fallback)
  if (context.req?.url) {
    const urlPath = context.req.url.split('?')[0]; // Remove query string
    const localeFromPath = extractLocaleFromPath(urlPath);
    if (localeFromPath) {
      return localeFromPath;
    }
  }

  // Try Accept-Language header
  const acceptLanguage = context.req.headers['accept-language'];
  if (acceptLanguage) {
    const locale = acceptLanguage.split(',')[0].split('-')[0];
    return locale;
  }

  return defaultLocale;
}

/**
 * Preload translations in getServerSideProps
 * Returns translations and helper functions
 */
export async function preloadTranslations(
  config: NextJsI18nConfig,
  context: GetServerSidePropsContext
): Promise<{
  translations: Record<string, TranslationData>;
  t: (componentCode: string, path: string, options?: { defaultValue?: string; variables?: Record<string, string | number> }) => string;
  locale: string;
  stage: DeploymentStage;
}> {
  const locale = getLocaleFromContext(context, config.defaultLocale || 'en');
  const stage = config.defaultStage || 'production';

  // Preload all component translations
  const client = config.client as I18nCenterClient;
  const translations = await client.getMultipleTranslations(
    config.applicationCode,
    config.componentCodes,
    locale,
    stage
  );

  // Create synchronous translation function
  const t = (componentCode: string, path: string, options?: { defaultValue?: string; variables?: Record<string, string | number> }): string => {
    const translationData = translations[componentCode];
    if (!translationData) {
      return options?.defaultValue || path;
    }
    const syncT = createSyncTranslator(translationData);
    return syncT(path, options);
  };

  return {
    translations,
    t,
    locale,
    stage,
  };
}

/**
 * Create a getServerSideProps wrapper that preloads translations
 */
export function withTranslations(
  config: NextJsI18nConfig,
  getServerSidePropsFn?: (context: GetServerSidePropsContext, translations: {
    t: (componentCode: string, path: string, options?: { defaultValue?: string; variables?: Record<string, string | number> }) => string;
    locale: string;
    stage: DeploymentStage;
  }) => Promise<{ props: any }>
) {
  return async (context: GetServerSidePropsContext) => {
    const { t, translations, locale, stage } = await preloadTranslations(config, context);

    if (getServerSidePropsFn) {
      const result = await getServerSidePropsFn(context, { t, locale, stage });
      return {
        ...result,
        props: {
          ...result.props,
          __i18n: {
            translations,
            locale,
            stage,
            applicationCode: config.applicationCode,
          },
        },
      };
    }

    return {
      props: {
        __i18n: {
          translations,
          locale,
          stage,
          applicationCode: config.applicationCode,
        },
      },
    };
  };
}

