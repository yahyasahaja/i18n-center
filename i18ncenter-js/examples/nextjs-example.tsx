/**
 * Example: Next.js Page with i18n-center translations
 *
 * This example shows how to use i18ncenter-js in a Next.js page
 * with getServerSideProps for SSR translations.
 */

import { GetServerSideProps } from 'next';
import { I18nCenterClient, withTranslations } from 'i18ncenter-js/nextjs';

// Initialize client (should be in a separate file and reused)
const client = new I18nCenterClient({
  apiUrl: process.env.I18N_CENTER_API_URL || 'http://localhost:8080/api',
  apiToken: process.env.I18N_CENTER_API_TOKEN,
  defaultLocale: 'en',
  defaultStage: 'production',
});

interface ProductPageProps {
  productNameLabel: string;
  addToCartLabel: string;
  priceLabel: string;
  locale: string;
}

/**
 * Example 1: Using withTranslations helper
 */
export const getServerSideProps = withTranslations(
  {
    client,
    applicationCode: 'my_app', // Application code (required)
    componentCodes: ['pdp_form', 'checkout'], // Preload these components
  },
  async (context, { t, locale }) => {
    // Use translations synchronously (already preloaded)
    const productNameLabel = t('pdp_form', 'form.name.label');
    const addToCartLabel = t('pdp_form', 'button.add_to_cart');
    const priceLabel = t('pdp_form', 'form.price.label');

    // You can also use template variables
    // const greeting = t('pdp_form', 'greeting', { variables: { name: 'John' } });

    return {
      props: {
        productNameLabel,
        addToCartLabel,
        priceLabel,
        locale,
      },
    };
  }
);

function ProductPage({ productNameLabel, addToCartLabel, priceLabel, locale }: ProductPageProps) {
  return (
    <div>
      <h1>Product Page ({locale})</h1>
      <form>
        <label>{productNameLabel}</label>
        <input type="text" name="name" />

        <label>{priceLabel}</label>
        <input type="number" name="price" />

        <button>{addToCartLabel}</button>
      </form>
    </div>
  );
}

export default ProductPage;

/**
 * Example 2: Manual preloading (more control)
 */
/*
import { preloadTranslations } from 'i18ncenter-js/nextjs';

export const getServerSideProps: GetServerSideProps = async (context) => {
  const { t, translations, locale } = await preloadTranslations(
    {
      client,
      applicationCode: 'my_app', // Application code (required)
      componentCodes: ['pdp_form'],
    },
    context
  );

  const label = t('pdp_form', 'form.name.label');

  return {
    props: {
      label,
      translations, // Pass to client if needed
      locale,
    },
  };
};
*/

