export default {
  async fetch(request, env): Promise<Response> {
    const url = new URL(request.url);

    // Proxy API and webhook routes to the Go backend
    if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/webhook/')) {
      try {
        const backendUrl = new URL(url.pathname + url.search, env.BACKEND_ORIGIN);
        const headers = new Headers(request.headers);
        headers.delete('host');
        return await fetch(backendUrl, {
          method: request.method,
          headers,
          body: request.method !== 'GET' && request.method !== 'HEAD' ? request.body : undefined,
          redirect: 'manual',
        });
      } catch (error) {
        console.error(JSON.stringify({
          message: 'backend proxy failed',
          method: request.method,
          path: url.pathname,
          error: error instanceof Error ? error.message : String(error),
        }));
        return Response.json({ detail: 'Backend service unavailable' }, { status: 502 });
      }
    }

    // Everything else: serve static assets (SPA fallback configured in wrangler.jsonc)
    return env.ASSETS.fetch(request);
  },
} satisfies ExportedHandler<Env>;
