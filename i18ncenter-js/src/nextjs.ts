import { GetServerSidePropsContext } from 'next';
import { I18nCenterClient } from './client';
import { createSyncTranslator } from './translator';
import { DeploymentStage, TranslationData, NextJsI18nConfig } from './types';

/**
 * Get locale from Next.js context (supports next-i18next, next-intl, or custom)
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
    const { t, locale, stage } = await preloadTranslations(config, context);

    if (getServerSidePropsFn) {
      const result = await getServerSidePropsFn(context, { t, locale, stage });
      return {
        ...result,
        props: {
          ...result.props,
          __i18n: {
            locale,
            stage,
          },
        },
      };
    }

    return {
      props: {
        __i18n: {
          locale,
          stage,
        },
      },
    };
  };
}

