/**
 * Example: Basic usage of i18ncenter-js
 *
 * This example shows basic usage without Next.js integration.
 */

import { I18nCenterClient, createTranslator } from 'i18ncenter-js';

// Initialize client
const client = new I18nCenterClient({
  apiUrl: 'https://api.example.com/api',
  apiToken: 'your-api-token', // Optional
  defaultLocale: 'en',
  defaultStage: 'production',
  cacheTTL: 3600000, // 1 hour
  enableCache: true,
});

// Create translator for a component (application code is required)
const t = createTranslator(client, 'my_app', 'pdp_form', 'en', 'production');

// Example 1: Simple translation
async function example1() {
  const label = await t('form.name.label');
  console.log(label); // e.g., "Product Name"
}

// Example 2: Translation with template variables
async function example2() {
  const greeting = await t('greeting', {
    variables: { name: 'John' }
  });
  console.log(greeting); // e.g., "Hello John!" (if translation is "Hello {name}!")
}

// Example 3: Translation with default value
async function example3() {
  const label = await t('form.unknown.field', {
    defaultValue: 'Default Label'
  });
  console.log(label); // "Default Label" if translation not found
}

// Example 4: Preload translation (useful for batch operations)
async function example4() {
  await t.preload('en', 'production');

  // Now all subsequent calls are instant (from cache)
  const label1 = await t('form.name.label');
  const label2 = await t('form.price.label');
  const label3 = await t('button.submit');
}

// Example 5: Get raw translation data
async function example5() {
  const rawData = await t.getRaw('en', 'production');
  console.log(rawData); // Full translation object
}

// Example 6: Multiple components
async function example6() {
  const translations = await client.getMultipleTranslations(
    'my_app', // Application code (required)
    ['pdp_form', 'checkout', 'cart'],
    'en',
    'production'
  );

  console.log(translations.pdp_form); // Translation data for pdp_form
  console.log(translations.checkout); // Translation data for checkout
  console.log(translations.cart);     // Translation data for cart
}

// Example 7: Clear cache
function example7() {
  client.clearCache();
}

// Run examples
async function main() {
  try {
    await example1();
    await example2();
    await example3();
    await example4();
    await example5();
    await example6();
    example7();
  } catch (error) {
    console.error('Error:', error);
  }
}

// main();

