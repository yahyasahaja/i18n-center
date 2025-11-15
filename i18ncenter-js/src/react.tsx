// React is a peer dependency - types will be available when installed
// @ts-ignore
import React, { createContext, useContext, useMemo, useState, useEffect, ReactNode } from 'react';
import { I18nCenterClient } from './client';
import { TranslationData, DeploymentStage } from './types';
import { createSyncTranslator } from './translator';

/**
 * Translation context value
 */
interface TranslationContextValue {
  translations: Record<string, TranslationData>;
  locale: string;
  stage: DeploymentStage;
  applicationCode: string;
  client?: I18nCenterClient;
  setTranslations: (translations: Record<string, TranslationData>) => void;
  setLocale: (locale: string) => void;
  setStage: (stage: DeploymentStage) => void;
}

const TranslationContext = createContext<TranslationContextValue | null>(null);

/**
 * Props for TranslationProvider
 */
export interface TranslationProviderProps {
  /** Initial translations (from SSR props) */
  translations?: Record<string, TranslationData>;
  /** Application code */
  applicationCode: string;
  /** Initial locale */
  locale?: string;
  /** Initial stage */
  stage?: DeploymentStage;
  /** Client instance (optional, for client-side fetching) */
  client?: I18nCenterClient;
  /** Component codes to preload (if client is provided) */
  componentCodes?: string[];
  /** Children */
  children: ReactNode;
}

/**
 * Translation Provider component
 *
 * @example
 * ```tsx
 * // In your page component (from getServerSideProps)
 * function MyPage({ __i18n }) {
 *   return (
 *     <TranslationProvider
 *       translations={__i18n.translations}
 *       applicationCode="my_app"
 *       locale={__i18n.locale}
 *       stage={__i18n.stage}
 *     >
 *       <MyComponent />
 *     </TranslationProvider>
 *   );
 * }
 * ```
 */
export function TranslationProvider({
  translations: initialTranslations = {},
  applicationCode,
  locale: initialLocale = 'en',
  stage: initialStage = 'production',
  client,
  componentCodes = [],
  children,
}: TranslationProviderProps) {
  const [translations, setTranslations] = useState<Record<string, TranslationData>>(initialTranslations);
  const [locale, setLocale] = useState(initialLocale);
  const [stage, setStage] = useState<DeploymentStage>(initialStage);

  // Client-side preload if client is provided and translations are missing
  useEffect(() => {
    if (client && componentCodes.length > 0) {
      const missingCodes = componentCodes.filter(code => !translations[code]);
      if (missingCodes.length > 0) {
        client
          .getMultipleTranslations(applicationCode, missingCodes, locale, stage)
          .then((fetched: Record<string, TranslationData>) => {
            setTranslations((prev: Record<string, TranslationData>) => ({ ...prev, ...fetched }));
          })
          .catch((err: Error) => {
            console.error('Failed to fetch translations:', err);
          });
      }
    }
  }, [client, applicationCode, componentCodes, locale, stage, translations]);

  const value = useMemo<TranslationContextValue>(
    () => ({
      translations,
      locale,
      stage,
      applicationCode,
      client,
      setTranslations,
      setLocale,
      setStage,
    }),
    [translations, locale, stage, applicationCode, client]
  );

  return <TranslationContext.Provider value={value}>{children}</TranslationContext.Provider>;
}

/**
 * Hook to use translations in React components
 *
 * @example
 * ```tsx
 * function MyComponent() {
 *   const { t } = useTranslation();
 *
 *   return (
 *     <div>
 *       <h1>{t('pdp_form.title')}</h1>
 *       <p>{t('pdp_form.description', { variables: { name: 'John' } })}</p>
 *     </div>
 *   );
 * }
 * ```
 *
 * @example With path format: [component].[a].[b]
 * ```tsx
 * const label = t('pdp_form.form.name.label');
 * ```
 */
export function useTranslation() {
  const context = useContext(TranslationContext);

  if (!context) {
    throw new Error('useTranslation must be used within a TranslationProvider');
  }

  const { translations, locale, stage, applicationCode, client } = context;

  /**
   * Translation function
   * Supports path format: [component].[a].[b] or [componentCode].[path]
   *
   * @param path - Translation path in format "componentCode.path.to.key" or "componentCode.path"
   * @param options - Translation options
   */
  const t = useMemo(() => {
    return (path: string, options?: { defaultValue?: string; variables?: Record<string, string | number> }): string => {
      // Parse path: "componentCode.path.to.key"
      const parts = path.split('.');
      if (parts.length < 2) {
        return options?.defaultValue || path;
      }

      const componentCode = parts[0];
      const translationPath = parts.slice(1).join('.');

      // Get translation data for the component
      const translationData = translations[componentCode];

      if (!translationData) {
        // If not found and client is available, try to fetch it (async, but we return immediately)
        if (client) {
          client
            .getTranslation(applicationCode, componentCode, locale, stage)
            .then((data: TranslationData) => {
              context.setTranslations({ ...translations, [componentCode]: data });
            })
            .catch((err: Error) => {
              console.error(`Failed to fetch translation for component ${componentCode}:`, err);
            });
        }
        return options?.defaultValue || path;
      }

      // Use sync translator
      const syncT = createSyncTranslator(translationData);
      return syncT(translationPath, options);
    };
  }, [translations, locale, stage, applicationCode, client, context]);

  return {
    t,
    translations,
    locale,
    stage,
    setLocale: context.setLocale,
    setStage: context.setStage,
  };
}

