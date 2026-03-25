const API_BACKEND = 'http://localhost:21011';

interface AppEnv extends Env {
  ASSETS: { fetch(request: Request): Promise<Response> };
  BACKEND_ORIGIN?: string;
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const backendOrigin = env.BACKEND_ORIGIN || API_BACKEND;

    if (url.pathname.startsWith('/api/')) {
      const backendUrl = new URL(url.pathname + url.search, backendOrigin);
      const headers = new Headers(request.headers);
      headers.set('Host', new URL(backendOrigin).host);

      return fetch(backendUrl.toString(), {
        method: request.method,
        headers,
        body: request.method !== 'GET' && request.method !== 'HEAD' ? request.body : undefined,
      });
    }

    // Serve static assets; fall back to index.html for SPA routes
    const response = await env.ASSETS.fetch(request);
    if (response.status === 404) {
      // SPA fallback: serve index.html for client-side routing
      return env.ASSETS.fetch(new Request(new URL('/', request.url), request));
    }
    return response;
  },
} satisfies ExportedHandler<AppEnv>;
