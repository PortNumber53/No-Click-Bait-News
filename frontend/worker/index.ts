const API_BACKEND = 'http://localhost:21011';

export default {
  async fetch(request) {
    const url = new URL(request.url);

    if (url.pathname.startsWith('/api/')) {
      const backendUrl = new URL(url.pathname + url.search, API_BACKEND);
      const headers = new Headers(request.headers);
      headers.set('Host', new URL(API_BACKEND).host);

      return fetch(backendUrl.toString(), {
        method: request.method,
        headers,
        body: request.method !== 'GET' && request.method !== 'HEAD' ? request.body : undefined,
      });
    }

    return new Response(null, { status: 404 });
  },
} satisfies ExportedHandler<Env>;
