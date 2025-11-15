/**
 * i18ncenter-js
 *
 * JavaScript/TypeScript SDK for i18n-center translation service
 *
 * @example
 * ```typescript
 * import { I18nCenterClient, createTranslator } from 'i18ncenter-js';
 *
 * const client = new I18nCenterClient({
 *   apiUrl: 'https://api.example.com/api',
 *   apiToken: 'your-token',
 * });
 *
 * const t = createTranslator(client, 'my_app', 'pdp_form', 'en', 'production');
 *
 * // Use in async function
 * const label = await t('form.name.label');
 * ```
 *
 * @example Next.js Integration
 * ```typescript
 * import { withTranslations } from 'i18ncenter-js/nextjs';
 *
 * export const getServerSideProps = withTranslations(
 *   {
 *     client: new I18nCenterClient({ apiUrl: '...', apiToken: '...' }),
 *     applicationCode: 'my_app',
 *     componentCodes: ['pdp_form', 'checkout'],
 *   },
 *   async (context, { t }) => {
 *     const label = t('pdp_form', 'form.name.label');
 *     return { props: { label } };
 *   }
 * );
 * ```
 */

export { I18nCenterClient } from './client';
export { createTranslator, createSyncTranslator } from './translator';
export { preloadTranslations, withTranslations, getLocaleFromContext } from './nextjs';
export type {
  I18nCenterConfig,
  DeploymentStage,
  TranslationData,
  CacheStorage,
  TranslateOptions,
  NextJsI18nConfig,
} from './types';

// Re-export Next.js types
export type { GetServerSidePropsContext } from 'next';

