# i18ncenter-js

JavaScript/TypeScript SDK for the i18n-center translation service. Provides a simple, type-safe interface for fetching and using translations in Next.js applications.

## Installation

```bash
npm install i18ncenter-js
# or
yarn add i18ncenter-js
```

## Quick Start

### Basic Usage

```typescript
import { I18nCenterClient, createTranslator } from 'i18ncenter-js';

// Initialize client
const client = new I18nCenterClient({
  apiUrl: 'https://api.example.com/api',
  apiToken: 'your-api-token', // Optional, if API requires auth
  defaultLocale: 'en',
  defaultStage: 'production',
});

// Create translator for a component (application code is required)
const t = createTranslator(client, 'my_app', 'pdp_form', 'en', 'production');

// Use in async function
async function MyComponent() {
  const label = await t('form.name.label');
  return <div>{label}</div>;
}
```

### React Hook Integration (Recommended for Client Components)

#### Using `useTranslation` Hook

```tsx
// app/product/[id]/page.tsx
import { GetServerSideProps } from 'next';
import { I18nCenterClient, withTranslations, TranslationProvider, useTranslation } from 'i18ncenter-js';

const client = new I18nCenterClient({
  apiUrl: process.env.I18N_CENTER_API_URL!,
  apiToken: process.env.I18N_CENTER_API_TOKEN,
});

export const getServerSideProps = withTranslations(
  {
    client,
    applicationCode: 'my_app',
    componentCodes: ['pdp_form', 'checkout'],
  },
  async (context) => {
    return { props: {} };
  }
);

// Page component
function ProductPage({ __i18n }: { __i18n: any }) {
  return (
    <TranslationProvider
      translations={__i18n.translations}
      applicationCode={__i18n.applicationCode}
      locale={__i18n.locale}
      stage={__i18n.stage}
      client={client} // Optional: for client-side fetching
      componentCodes={['pdp_form', 'checkout']}
    >
      <ProductContent />
    </TranslationProvider>
  );
}

// Component using translations
function ProductContent() {
  const { t } = useTranslation();

  return (
    <div>
      <h1>{t('pdp_form.title')}</h1>
      <p>{t('pdp_form.description', { variables: { name: 'John' } })}</p>
      <button>{t('pdp_form.button.add_to_cart')}</button>
    </div>
  );
}

export default ProductPage;
```

**Path Format:** The `t` function supports the format `[componentCode].[path.to.key]`:
- `t('pdp_form.title')` → Gets `translations.pdp_form.title`
- `t('pdp_form.form.name.label')` → Gets `translations.pdp_form.form.name.label`
- `t('checkout.button.submit')` → Gets `translations.checkout.button.submit`

### Locale Detection

The `withTranslations` helper automatically detects the locale from multiple sources (in priority order):

1. **Next.js locale** (from `next-i18next` or `next-intl`)
2. **Query parameter** (`?locale=en-us`)
3. **URL path pattern** (`/en-us/pdp`, `/en_us/pdp`, `/en/pdp`, `/id/pdp`)
   - Supports both hyphen (`en-us`) and underscore (`en_us`) formats
   - Normalizes to hyphen format (e.g., `en_us` → `en-us`)
   - Supports language-only codes (e.g., `en`, `id`, `fr`)
4. **Accept-Language header** (browser language preference)
5. **Default locale** (from config or `'en'`)

**Examples:**
- `https://example.com/en-us/pdp` → locale: `en-us`
- `https://example.com/en_us/pdp` → locale: `en-us` (normalized)
- `https://example.com/id/pdp` → locale: `id`
- `https://example.com/pdp?locale=fr` → locale: `fr` (query param takes priority)

### Next.js Integration (Server-Side)

#### Option 1: Using `getServerSideProps`

```typescript
// pages/product/[id].tsx
import { GetServerSideProps } from 'next';
import { I18nCenterClient, withTranslations } from 'i18ncenter-js/nextjs';

const client = new I18nCenterClient({
  apiUrl: process.env.I18N_CENTER_API_URL!,
  apiToken: process.env.I18N_CENTER_API_TOKEN,
});

export const getServerSideProps = withTranslations(
  {
    client,
    applicationCode: 'my_app', // Application code (required)
    componentCodes: ['pdp_form', 'checkout'],
  },
  async (context, { t, locale }) => {
    // Use translations synchronously
    const productNameLabel = t('pdp_form', 'form.name.label');
    const addToCartLabel = t('pdp_form', 'button.add_to_cart');

    return {
      props: {
        productNameLabel,
        addToCartLabel,
        locale,
      },
    };
  }
);

function ProductPage({ productNameLabel, addToCartLabel }) {
  return (
    <div>
      <label>{productNameLabel}</label>
      <button>{addToCartLabel}</button>
    </div>
  );
}
```

#### Option 2: Manual Preloading

```typescript
import { GetServerSideProps } from 'next';
import { I18nCenterClient, preloadTranslations } from 'i18ncenter-js/nextjs';

const client = new I18nCenterClient({
  apiUrl: process.env.I18N_CENTER_API_URL!,
  apiToken: process.env.I18N_CENTER_API_TOKEN,
});

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
```

### Client-Side Usage (with React)

```typescript
'use client';

import { useEffect, useState } from 'react';
import { I18nCenterClient, createTranslator } from 'i18ncenter-js';

const client = new I18nCenterClient({
  apiUrl: process.env.NEXT_PUBLIC_I18N_CENTER_API_URL!,
  apiToken: process.env.NEXT_PUBLIC_I18N_CENTER_API_TOKEN,
});

export function MyComponent() {
  const [t, setT] = useState<ReturnType<typeof createTranslator> | null>(null);

  useEffect(() => {
    const translator = createTranslator(client, 'my_app', 'pdp_form', 'en', 'production');
    translator.preload(); // Preload translation
    setT(translator);
  }, []);

  if (!t) return <div>Loading...</div>;

  return (
    <div>
      <button onClick={async () => {
        const label = await t.t('form.name.label');
        alert(label);
      }}>
        Get Translation
      </button>
    </div>
  );
}
```

## API Reference

### `TranslationProvider`

React context provider that makes translations available to child components via `useTranslation`.

```tsx
<TranslationProvider
  translations={translations}        // From SSR props (__i18n.translations)
  applicationCode="my_app"           // Required
  locale="en"                        // Optional, default: 'en'
  stage="production"                 // Optional, default: 'production'
  client={client}                    // Optional: for client-side fetching
  componentCodes={['pdp_form']}      // Optional: components to preload
>
  {children}
</TranslationProvider>
```

### `useTranslation`

React hook to access translations in components.

```tsx
const { t, translations, locale, stage, setLocale, setStage } = useTranslation();

// Usage
const label = t('pdp_form.form.name.label');
const greeting = t('pdp_form.greeting', { variables: { name: 'John' } });
```

**Returns:**
- `t(path, options?)` - Translation function
  - `path`: Format `[componentCode].[path.to.key]` (e.g., `'pdp_form.title'`)
  - `options.defaultValue`: Fallback if translation not found
  - `options.variables`: Template variables to replace
- `translations` - All loaded translations
- `locale` - Current locale
- `stage` - Current deployment stage
- `setLocale(locale)` - Change locale (triggers refetch if client provided)
- `setStage(stage)` - Change stage (triggers refetch if client provided)

### `I18nCenterClient`

Main client for interacting with the i18n-center API.

```typescript
const client = new I18nCenterClient({
  apiUrl: string;              // Required: API base URL
  apiToken?: string;            // Optional: Bearer token
  defaultLocale?: string;        // Default: 'en'
  defaultStage?: DeploymentStage; // Default: 'production'
  cacheTTL?: number;            // Default: 3600000 (1 hour)
  enableCache?: boolean;         // Default: true
});
```

**Methods:**

- `getTranslation(componentCode, locale?, stage?)`: Get translation for a single component
- `getMultipleTranslations(componentCodes, locale?, stage?)`: Get translations for multiple components
- `clearCache()`: Clear the cache

### `createTranslator(client, applicationCode, componentCode, defaultLocale?, defaultStage?)`

Creates a translation function for a specific component. **Application code is required** to differentiate components with the same code in different applications.

```typescript
const t = createTranslator(client, 'my_app', 'pdp_form', 'en', 'production');

// Usage
const label = await t('form.name.label');
const labelWithVars = await t('form.greeting', {
  variables: { name: 'John' }
});
```

**Translation Path Syntax:**

Use dot notation to access nested translation values:
- `'form.name.label'` → `translation.form.name.label`
- `'button.submit'` → `translation.button.submit`

**Template Variables:**

Support for `{variable}` and `[variable]` syntax:
```typescript
// Translation: "Hello {name}!"
await t('greeting', { variables: { name: 'John' } });
// Returns: "Hello John!"
```

### `withTranslations(config, getServerSidePropsFn?)`

Next.js helper for preloading translations in `getServerSideProps`.

```typescript
export const getServerSideProps = withTranslations(
  {
    client,
    componentCodes: ['pdp_form'],
  },
  async (context, { t, locale, stage }) => {
    // Use t() synchronously here
    const label = t('pdp_form', 'form.name.label');
    return { props: { label } };
  }
);
```

## Configuration

### Environment Variables

```env
# .env.local
I18N_CENTER_API_URL=https://api.example.com/api
I18N_CENTER_API_TOKEN=your-token-here
```

### Next.js Environment Variables

For client-side usage, prefix with `NEXT_PUBLIC_`:

```env
NEXT_PUBLIC_I18N_CENTER_API_URL=https://api.example.com/api
NEXT_PUBLIC_I18N_CENTER_API_TOKEN=your-token-here
```

## Caching

The SDK includes built-in in-memory caching:

- **Default TTL**: 1 hour
- **Cache Key**: `i18n:{componentCode}:{locale}:{stage}`
- **Automatic**: Cache is checked before API calls
- **Manual Clear**: `client.clearCache()`

For custom caching (e.g., Redis), implement the `CacheStorage` interface:

```typescript
import { CacheStorage } from 'i18ncenter-js';

class RedisCache implements CacheStorage {
  // Implement get, set, clear
}

const client = new I18nCenterClient(config, new RedisCache());
```

## TypeScript Support

Full TypeScript support with type definitions included.

## Examples

See the `/examples` directory for more examples:
- Basic usage
- Next.js integration
- Client-side React hooks
- Custom caching

## License

MIT

