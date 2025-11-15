/**
 * Example: Using useTranslation hook in Next.js
 *
 * This example shows how to use the useTranslation hook with TranslationProvider
 * in a Next.js page component.
 */

import { GetServerSideProps } from 'next';
import { I18nCenterClient, withTranslations, TranslationProvider, useTranslation } from 'i18ncenter-js';

// Initialize client (should be in a separate file and reused)
const client = new I18nCenterClient({
  apiUrl: process.env.I18N_CENTER_API_URL || 'http://localhost:8080/api',
  apiToken: process.env.I18N_CENTER_API_TOKEN,
  defaultLocale: 'en',
  defaultStage: 'production',
});

interface ProductPageProps {
  __i18n: {
    translations: Record<string, any>;
    locale: string;
    stage: string;
    applicationCode: string;
  };
}

/**
 * Example 1: Using withTranslations to preload translations
 */
export const getServerSideProps = withTranslations(
  {
    client,
    applicationCode: 'my_app',
    componentCodes: ['pdp_form', 'checkout'],
  },
  async (context) => {
    // You can use the t function here if needed
    // const { t } = await preloadTranslations(...);
    return { props: {} };
  }
);

/**
 * Page component - wraps children with TranslationProvider
 */
function ProductPage({ __i18n }: ProductPageProps) {
  return (
    <TranslationProvider
      translations={__i18n.translations}
      applicationCode={__i18n.applicationCode}
      locale={__i18n.locale}
      stage={__i18n.stage}
      client={client} // Optional: enables client-side fetching for missing translations
      componentCodes={['pdp_form', 'checkout']} // Optional: components to preload on client
    >
      <ProductContent />
    </TranslationProvider>
  );
}

/**
 * Component using translations via useTranslation hook
 */
function ProductContent() {
  const { t, locale, setLocale } = useTranslation();

  return (
    <div>
      <h1>{t('pdp_form.title')}</h1>
      <p>{t('pdp_form.description', { variables: { name: 'John' } })}</p>
      <button>{t('pdp_form.button.add_to_cart')}</button>
      <button>{t('checkout.button.submit')}</button>

      {/* Example: Changing locale */}
      <select value={locale} onChange={(e) => setLocale(e.target.value)}>
        <option value="en">English</option>
        <option value="id">Indonesian</option>
        <option value="fr">French</option>
      </select>
    </div>
  );
}

export default ProductPage;

/**
 * Example 2: Using in a nested component
 */
function ProductDetails() {
  const { t } = useTranslation();

  return (
    <div>
      <h2>{t('pdp_form.product_details.title')}</h2>
      <p>{t('pdp_form.product_details.description')}</p>
      <ul>
        <li>{t('pdp_form.product_details.feature_1')}</li>
        <li>{t('pdp_form.product_details.feature_2')}</li>
      </ul>
    </div>
  );
}

/**
 * Example 3: Using with template variables
 */
function ProductGreeting({ userName }: { userName: string }) {
  const { t } = useTranslation();

  return (
    <div>
      {/* Translation: "Hello {name}, welcome to our store!" */}
      <p>{t('pdp_form.greeting', { variables: { name: userName } })}</p>
    </div>
  );
}

/**
 * Example 4: Using with default value
 */
function OptionalTranslation() {
  const { t } = useTranslation();

  return (
    <div>
      {/* If translation doesn't exist, returns "Fallback Text" */}
      <p>{t('pdp_form.optional.text', { defaultValue: 'Fallback Text' })}</p>
    </div>
  );
}

