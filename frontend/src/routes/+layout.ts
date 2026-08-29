// Pure SPA: no SSR, no prerender. Every route is client-rendered and talks to
// the Go API directly.
export const ssr = false;
export const prerender = false;
export const trailingSlash = 'never';
