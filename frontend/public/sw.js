const CACHE_NAME = 'moduforge-v1';
const STATIC_ASSETS = ['/', '/index.html', '/app.css', '/app.js'];
const API_PATTERN = /^https?:\/\/.*\/api\//;

// Install: pre-cache static assets
self.addEventListener('install', (event) => {
	event.waitUntil(
		caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS))
	);
	self.skipWaiting();
});

// Activate: clean old caches
self.addEventListener('activate', (event) => {
	event.waitUntil(
		caches.keys().then((keys) =>
			Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k)))
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
		return cached || offlineResponse();
	}
}

async function cacheFirst(request) {
	const cached = await caches.match(request);
	if (cached) return cached;
	try {
		const response = await fetch(request);
		if (response.ok) {
			const cache = await caches.open(CACHE_NAME);
			cache.put(request, response.clone());
		}
		return response;
	} catch {
		return offlineResponse();
	}
}

function offlineResponse() {
	return new Response('Offline', {
		status: 503,
		headers: { 'Content-Type': 'text/plain' }
	});
}
