const API_BACKEND = 'http://localhost:21011';

interface AppEnv extends Env {
  BACKEND_ORIGIN?: string;
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    // Only handle API routes — static assets and SPA fallback
    // are handled by Workers Static Assets (configured in wrangler.jsonc)
    if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/webhook/')) {
      const backendOrigin = env.BACKEND_ORIGIN || API_BACKEND;
      const backendUrl = new URL(url.pathname + url.search, backendOrigin);
      const headers = new Headers(request.headers);
      headers.set('Host', new URL(backendOrigin).host);

      return fetch(backendUrl.toString(), {
        method: request.method,
        headers,
        body: request.method !== 'GET' && request.method !== 'HEAD' ? request.body : undefined,
      });
    }

    // For non-API routes, return nothing — the assets platform serves static files
    return new Response('Not Found', { status: 404 });
  },
} satisfies ExportedHandler<AppEnv>;
