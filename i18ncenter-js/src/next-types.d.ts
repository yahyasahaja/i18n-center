/**
 * Minimal type declarations for Next.js
 * This allows the library to be type-checked without requiring Next.js to be installed
 */

declare module 'next' {
  export interface GetServerSidePropsContext {
    params?: Record<string, string | string[]>;
    query: Record<string, string | string[]>;
    req: {
      url?: string;
      headers: Record<string, string | string[] | undefined>;
    };
    res: any;
    resolvedUrl?: string;
    locale?: string;
  }
}

