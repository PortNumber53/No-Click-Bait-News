interface AppEnv extends Env {
  ASSETS: Fetcher;
  BACKEND_ORIGIN: string;
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    // Proxy API and webhook routes to the Go backend
    if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/webhook/')) {
      const backendUrl = new URL(url.pathname + url.search, env.BACKEND_ORIGIN);
      const headers = new Headers(request.headers);
      headers.set('Host', new URL(env.BACKEND_ORIGIN).host);

      return fetch(backendUrl.toString(), {
        method: request.method,
        headers,
        body: request.method !== 'GET' && request.method !== 'HEAD' ? request.body : undefined,
      });
    }

    // Everything else: serve static assets (SPA fallback configured in wrangler.jsonc)
    return env.ASSETS.fetch(request);
  },
} satisfies ExportedHandler<AppEnv>;
