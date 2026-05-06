# i18ncenter-js

JavaScript/TypeScript SDK for the i18n-center translation service. Provides a simple, type-safe interface for fetching and using translations in Next.js applications.

> **Compatibility note:** This SDK targets **Next.js Pages Router** (`getServerSideProps` / `getStaticProps`). It is **not compatible** with the Next.js App Router. For App Router projects, modify `i18n/request.ts` directly using the raw `I18nCenterClient`.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Authentication](#authentication-api-key)
- [React Hook Integration](#react-hook-integration-recommended-for-client-components)
- [Locale Detection](#locale-detection)
- [Next.js Integration (Server-Side)](#nextjs-integration-server-side)
- [Client-Side Usage](#client-side-usage-with-react)
- [CMS Content](#cms-content)
  - [Rich text image srcset](#rich-text-image-srcset)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Caching](#caching)
- [TypeScript Support](#typescript-support)

---

## Installation

```bash
npm install i18ncenter-js
# or
yarn add i18ncenter-js
```

---

## Quick Start

### Basic Usage

```typescript
import { I18nCenterClient, createTranslator } from 'i18ncenter-js';

// Initialize client
const client = new I18nCenterClient({
  apiUrl: 'https://api.example.com/api',
  apiToken: 'sk_xxxxxxxx...', // Application API key (required for translations API)
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

---

## Authentication (API key)

The translations API requires an **application API key**:

1. In the i18n-center dashboard, open an application and go to the **API Keys** section (super_admin only).
2. Click **Add API Key** and copy the key (it is shown only once; format `sk_...`).
3. Pass it as `apiToken` when creating the client. The client sends it as `Authorization: Bearer <key>`.

The same key is used for all translation endpoints (bulk, by-tag, by-page, CMS). The key is scoped to one application.

---

## React Hook Integration (Recommended for Client Components)

### Using `useTranslation` Hook

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

**Safety Guarantees:** The `t()` function is **always safe** and **never throws errors**:
- ✅ Always returns a `string` (never `undefined` or `null`)
- ✅ Never throws exceptions
- ✅ If translation not found: returns `defaultValue` (if provided) or the `path` itself
- ✅ Handles network errors gracefully (returns fallback value)
- ✅ Safe to use in render functions without try-catch blocks

**Example:**
```typescript
// Safe - always returns a string
const label = t('pdp_form.title'); // Returns translation or 'pdp_form.title'
const label2 = t('pdp_form.unknown', { defaultValue: 'Default Text' }); // Returns 'Default Text' if not found
```

---

## Locale Detection

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

---

## Next.js Integration (Server-Side)

### Option 1: Using `getServerSideProps`

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

### Option 2: Manual Preloading

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

---

## Client-Side Usage (with React)

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

---

## CMS Content

i18n-center includes a headless CMS feature. Each **CMS item** has a unique `identifier`, belongs to an application, and is backed by a **template** that defines typed fields. The SDK exposes one method to fetch published CMS content.

### Field types

| Type | Description | SDK value |
|------|-------------|-----------|
| `text` | Single-line plain text | `string` |
| `textarea` | Multi-line plain text | `string` |
| `rich_text` | Rich text — stored and returned as an **HTML string** | `string` (HTML) |
| `json` | Arbitrary JSON object | `object` |
| `ld_json` | Structured data (JSON-LD) | `object` |

> ⚠️ `rich_text` values are raw HTML. Never render them with `{value}` — use `dangerouslySetInnerHTML` or a sanitisation library.

### `getCmsContent`

```typescript
getCmsContent(
  applicationId: string,   // Application UUID (not code)
  identifier: string,      // CMS item identifier, e.g. 'flash_banner'
  locale?: string,         // Default: client.defaultLocale
  stage?: DeploymentStage  // Default: client.defaultStage ('production')
): Promise<CmsContent>
```

**`CmsContent` type:**

```typescript
type CmsContent = {
  identifier: string;
  locale: string;
  stage: DeploymentStage;
  data: Record<string, any>;  // field_key → value map (types depend on template)
}
```

The endpoint called is:
```
GET /api/applications/:id/cms/:identifier?locale=en&stage=production
```
Authentication is the same application API key used for translations.

### Basic example

```typescript
import { I18nCenterClient } from 'i18ncenter-js';

const client = new I18nCenterClient({
  apiUrl: process.env.I18N_CENTER_API_URL!,
  apiToken: process.env.I18N_CENTER_API_TOKEN,
});

const content = await client.getCmsContent(
  'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx', // applicationId (UUID)
  'flash_banner',
  'en',
  'production'
);

console.log(content.data.title);      // "Flash Sale!"         (text field)
console.log(content.data.body);       // "<p>Up to 50% off…</p>" (rich_text — HTML string)
console.log(content.data.cta_label);  // "Shop Now"            (text field)
console.log(content.data.config);     // { "bg": "#ff0000" }   (json field)
```

### Next.js Pages Router — `getServerSideProps` example

```typescript
// pages/home.tsx
import { GetServerSideProps } from 'next';
import { I18nCenterClient } from 'i18ncenter-js';

const client = new I18nCenterClient({
  apiUrl: process.env.I18N_CENTER_API_URL!,
  apiToken: process.env.I18N_CENTER_API_TOKEN,
  defaultStage: 'production',
});

const APP_ID = process.env.I18N_CENTER_APP_ID!; // Application UUID

export const getServerSideProps: GetServerSideProps = async (context) => {
  const locale = (context.locale ?? context.query.locale ?? 'en') as string;

  const [banner, categoryDetail] = await Promise.all([
    client.getCmsContent(APP_ID, 'flash_banner', locale),
    client.getCmsContent(APP_ID, 'category_detail', locale),
  ]);

  return {
    props: {
      banner: banner.data,
      categoryDetail: categoryDetail.data,
      locale,
    },
  };
};

export default function HomePage({ banner, categoryDetail, locale }) {
  return (
    <main>
      {/* plain text field */}
      <h2>{banner.title}</h2>

      {/* rich_text field — render as HTML */}
      <div dangerouslySetInnerHTML={{ __html: categoryDetail.content }} />

      {/* json field */}
      <p style={{ color: banner.config?.textColor }}>
        {banner.cta_label}
      </p>
    </main>
  );
}
```

### Handling missing or draft content

```typescript
try {
  const content = await client.getCmsContent(APP_ID, 'flash_banner', 'en', 'production');
  // use content.data…
} catch (err) {
  if (err.message.includes('not found')) {
    // No production content published yet — show fallback UI
  } else {
    throw err;
  }
}
```

### Stage workflow

CMS items follow the same `draft → staging → production` promotion workflow as translations. Always fetch `stage: 'production'` in production code. Use `'staging'` for QA and `'draft'` only in local development.

---

### Rich text image srcset

`rich_text` fields can contain images uploaded through the i18n-center admin UI. These images are
served via **PixelShift** (`img.lapakgaming.com`), LapakGaming's on-the-fly image CDN.

**Why srcset is not stored in the HTML:**
- Image width is stored as a CSS percentage (`width:50%`) — the pixel size is container-dependent
  and varies per viewport, so no fixed pixel value can be embedded at save time.
- Storing srcset inline would bloat each `rich_text` field 5× per image for no render-time benefit.

**Recommended approach — generate srcset at render time:**

PixelShift accepts `w=` and `f=` query params on any image URL. Use the utility below to
post-process `rich_text` HTML before rendering it, replacing bare PixelShift `src` values with
a full `srcset` that lets the browser pick the right resolution automatically.

```typescript
const PIXELSHIFT_HOST = 'img.lapakgaming.com'

/**
 * Enriches PixelShift <img> tags in a rich_text HTML string with responsive srcset.
 *
 * Before: <img src="https://img.lapakgaming.com/?src=…" style="…">
 * After:  <img src="…&w=1080&f=webp"
 *              srcset="…&w=480&f=webp 480w, …&w=720&f=webp 720w, …&w=1080&f=webp 1080w, …&w=1440&f=webp 1440w"
 *              sizes="(max-width:640px) 480px,(max-width:1024px) 720px,1080px"
 *              style="…">
 */
export function addPixelShiftSrcset(html: string): string {
  return html.replace(
    /<img([^>]*?) src="(https:\/\/img\.lapakgaming\.com\/\?[^"]+)"([^>]*)>/g,
    (_match, before: string, src: string, after: string) => {
      // Strip any existing w= or f= so we start from a clean base URL.
      const base = src.replace(/&(w|f)=[^&]*/g, '')
      const breakpoints = [480, 720, 1080, 1440]
      const srcset = breakpoints.map((w) => `${base}&w=${w}&f=webp ${w}w`).join(', ')
      const fallback = `${base}&w=1080&f=webp`
      const sizes = '(max-width:640px) 480px,(max-width:1024px) 720px,1080px'
      return `<img${before} src="${fallback}" srcset="${srcset}" sizes="${sizes}"${after}>`
    }
  )
}
```

**Usage with `dangerouslySetInnerHTML`:**

```tsx
import { addPixelShiftSrcset } from '@/utils/pixelshift'

export function RichTextRenderer({ html }: { html: string }) {
  return (
    <div
      className="prose"
      dangerouslySetInnerHTML={{ __html: addPixelShiftSrcset(html) }}
    />
  )
}
```

> The Cloudflare cache key for PixelShift includes all query params (`w`, `f`, `q`, etc.), so
> each srcset variant (`480w`, `720w`, etc.) is cached separately at the edge. After the first
> request for each breakpoint, subsequent loads are served from Cloudflare with zero origin cost.

---

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
  apiUrl: string;               // Required: API base URL
  apiToken?: string;            // Optional: Bearer token
  defaultLocale?: string;       // Default: 'en'
  defaultStage?: DeploymentStage; // Default: 'production'
  cacheTTL?: number;            // Default: 3600000 (1 hour), in milliseconds
  enableCache?: boolean;        // Default: true
});
```

**Methods:**

| Method | Description |
|--------|-------------|
| `getTranslation(applicationCode, componentCode, locale?, stage?)` | Single component translation |
| `getMultipleTranslations(applicationCode, componentCodes, locale?, stage?)` | Multiple components in one API call |
| `getTranslationsByTag(applicationId, tagCode, locale?, stage?)` | All components tagged with `tagCode` |
| `getTranslationsByPage(applicationId, pageCode, locale?, stage?)` | All components associated with `pageCode` |
| `getCmsContent(applicationId, identifier, locale?, stage?)` | Fetch CMS item content |
| `clearCache()` | Clear the in-process memory cache |

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

---

## Configuration

### Environment Variables

```env
# .env.local
I18N_CENTER_API_URL=https://api.example.com/api
I18N_CENTER_API_TOKEN=your-token-here
I18N_CENTER_APP_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx  # Application UUID for CMS
```

### Next.js Environment Variables

For client-side usage, prefix with `NEXT_PUBLIC_`:

```env
NEXT_PUBLIC_I18N_CENTER_API_URL=https://api.example.com/api
NEXT_PUBLIC_I18N_CENTER_API_TOKEN=your-token-here
```

---

## Caching

### Default: in-process memory cache

By default the SDK uses an **in-process `Map`-based memory cache** (not Redis). This cache is local to each Node.js process/dyno and is lost on restart. It is suitable for most SSR use-cases where translations change infrequently.

```typescript
// Default behaviour — cache lives in Node.js process memory
const client = new I18nCenterClient({
  cacheTTL: 3600000, // 1 hour (default)
  enableCache: true, // true by default
});
```

### Cache keys used by the SDK

| Method | SDK cache key format |
|--------|----------------------|
| `getTranslation` / `getMultipleTranslations` | `i18n:{applicationCode}:{componentCode}:{locale}:{stage}` |
| `getTranslationsByTag` | `bytag:{applicationId}:{tagCode}:{locale}:{stage}` |
| `getTranslationsByPage` | `bypage:{applicationId}:{pageCode}:{locale}:{stage}` |
| `getCmsContent` | `cms:{applicationId}:{identifier}:{locale}:{stage}` |

> ⚠️ **README correction:** an earlier version of this document listed the key as `i18n:{componentCode}:{locale}:{stage}` — the `applicationCode` segment was missing. The correct key is shown above.

### Collision analysis — safe to share a Redis instance with the backend

If you implement a custom `CacheStorage` backed by the same Redis instance that the i18n-center backend uses, **there is no key collision risk**. The backend and the SDK use completely different key formats:

| Lookup | Backend key | SDK key | Risk |
|--------|-------------|---------|------|
| Single translation | `translation:{componentUUID}:{locale}:{stage}` | `i18n:{appCode}:{componentCode}:{locale}:{stage}` | **None** — different prefix; backend uses UUIDs, SDK uses codes |
| By-tag | `translations:bytag:{appUUID}:{tag}:{locale}:{stage}` | `bytag:{appId}:{tag}:{locale}:{stage}` | **None** — `translations:bytag:` vs `bytag:` |
| By-page | `translations:bypage:{appUUID}:{page}:{locale}:{stage}` | `bypage:{appId}:{page}:{locale}:{stage}` | **None** — `translations:bypage:` vs `bypage:` |
| CMS | *(backend does not cache CMS content)* | `cms:{appId}:{identifier}:{locale}:{stage}` | **None** |

Both sets of keys are also semantically different (backend stores server-side computed data, SDK stores the HTTP response payload), so even accidental overlap would not cause correctness issues — just a stale read that would be rejected at deserialization.

**Recommendation:** If you still want belt-and-suspenders isolation, configure the backend and the SDK to use different Redis databases (`REDIS_DB=0` for backend, `DB: 1` in the custom cache implementation).

### Custom cache (e.g. Redis)

Implement the `CacheStorage` interface to plug in any cache backend:

```typescript
import { CacheStorage, TranslationData } from 'i18ncenter-js';
import { createClient } from 'redis';

class RedisCache implements CacheStorage {
  private client = createClient({ url: process.env.REDIS_URL });

  async connect() { await this.client.connect(); }

  get(key: string): TranslationData | null {
    // Note: CacheStorage.get is synchronous in the current interface.
    // For async Redis, use a local Map as L1 and populate it in a background
    // prefetch, or extend the interface to support Promises.
    return null; // implement as needed
  }

  set(key: string, data: TranslationData, ttl: number): void {
    this.client.set(key, JSON.stringify(data), { PX: ttl }); // PX = milliseconds
  }

  clear(): void {
    // scan and delete keys with your prefix
  }
}

const client = new I18nCenterClient(config, new RedisCache());
```

---

## TypeScript Support

Full TypeScript support with type definitions included.

**Exported types:**

```typescript
import {
  CmsContent,          // Returned by getCmsContent
  DeploymentStage,     // 'draft' | 'staging' | 'production'
  TranslationData,     // Record<string, any> — component translation map
  I18nCenterConfig,    // Constructor config type
  CacheStorage,        // Interface for custom cache implementations
} from 'i18ncenter-js';
```

---

## Examples

See the `/examples` directory for more examples:
- Basic usage
- Next.js integration
- Client-side React hooks
- Custom caching
- CMS content rendering

---

## License

MIT
