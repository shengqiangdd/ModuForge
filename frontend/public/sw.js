const CACHE_NAME = 'moduforge-v2';
const STATIC_ASSETS = ['/', '/index.html', '/app.css', '/app.js'];
const FONT_CACHE = 'moduforge-fonts-v1';
const FONT_ASSETS = [
  '/fonts/MaterialSymbolsOutlined.woff2',
  '/fonts/Inter-Latin.woff2',
  '/fonts/Inter-LatinExt.woff2',
  '/fonts/JetBrainsMono-Regular.woff2',
];
const API_PATTERN = /^https?:\/\/.*\/api\//;

// Install: pre-cache static assets and fonts
self.addEventListener('install', (event) => {
  event.waitUntil(
    Promise.all([
      caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS)),
      caches.open(FONT_CACHE).then((cache) => cache.addAll(FONT_ASSETS)),
    ])
  );
  self.skipWaiting();
});

// Activate: clean old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((k) => k !== CACHE_NAME && k !== FONT_CACHE)
          .map((k) => caches.delete(k))
      )
    )
  );
  self.clients.claim();
});

// Fetch: routing strategies
self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // API: network-first
  if (API_PATTERN.test(request.url)) {
    event.respondWith(networkFirst(request));
    return;
  }

  // Fonts: cache-first (fonts rarely change)
  if (url.pathname.startsWith('/fonts/') || request.destination === 'font') {
    event.respondWith(cacheFirst(request, FONT_CACHE));
    return;
  }

  // Static: cache-first
  if (url.origin === self.location.origin) {
    event.respondWith(cacheFirst(request));
    return;
  }

  // External: network-first with cache fallback
  event.respondWith(networkFirst(request));
});

async function networkFirst(request) {
  try {
    const response = await fetch(request);
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

async function cacheFirst(request, cacheName = CACHE_NAME) {
  const cached = await caches.match(request);
  if (cached) return cached;
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(cacheName);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    return new Response('Offline', { status: 503 });
  }
}
