const CACHE_NAME = 'moduforge-v7';

const STATIC_CACHE_URLS = [
  '/',
  '/index.html',
  '/manifest.json',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(STATIC_CACHE_URLS);
    })
  );
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k))
      );
    }).then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // Only handle same-origin requests
  if (url.origin !== self.location.origin) return;

  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request).catch(() => caches.match('/'))
    );
    return;
  }

  if (url.pathname.startsWith('/api/')) {
    // API responses are NEVER cached — caching poll/build responses here can
    // queue up Cache API writes and stall the page's fetch() forever, which
    // froze the build UI at RUNNING. Route straight to network.
    event.respondWith(fetch(event.request).catch(() => new Response('Network error', { status: 503 })));
    return;
  }

  // Hashed assets (js/css with content hash) are immutable — safe to cache.
  // But always try network first so new deploys propagate immediately;
  // fall back to cache only when offline.
  if (url.pathname.match(/\.(js|css|png|svg|ico|json|woff2?)$/)) {
    event.respondWith(networkFirst(event.request));
    return;
  }

  event.respondWith(networkFirst(event.request));
});

async function networkFirst(request) {
  try {
    const response = await fetch(request);
    // Only cache successful responses — never cache 4xx/5xx errors
    if (response.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    const cached = await caches.match(request);
    return cached || new Response('Offline', { status: 503 });
  }
}
