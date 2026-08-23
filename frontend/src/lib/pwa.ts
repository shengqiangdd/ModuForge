let installPrompt: any = null;

/**
 * Register service worker
 */
export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
	if (!('serviceWorker' in navigator)) return null;

	try {
		const reg = await navigator.serviceWorker.register('/sw.js');
		console.log('SW registered:', reg.scope);
		return reg;
	} catch (err) {
		console.error('SW registration failed:', err);
		return null;
	}
}

/**
 * Unregister service worker
 */
export async function unregisterServiceWorker(): Promise<boolean> {
	if (!('serviceWorker' in navigator)) return false;

	try {
		const regs = await navigator.serviceWorker.getRegistrations();
		for (const reg of regs) {
			await reg.unregister();
		}
		return true;
	} catch {
		return false;
	}
}

/**
 * Check if online
 */
export function isOnline(): boolean {
	return navigator.onLine;
}

/**
 * Listen for online/offline changes
 */
export function onOnlineChange(callback: (online: boolean) => void): () => void {
	const handler = () => callback(navigator.onLine);
	window.addEventListener('online', handler);
	window.addEventListener('offline', handler);
	return () => {
		window.removeEventListener('online', handler);
		window.removeEventListener('offline', handler);
	};
}

/**
 * Request notification permission
 */
export async function requestNotificationPermission(): Promise<NotificationPermission> {
	if (!('Notification' in window)) return 'denied';
	if (Notification.permission === 'granted') return 'granted';
	return Notification.requestPermission();
}

/**
 * Send desktop notification
 */
export function sendNotification(title: string, options?: NotificationOptions): Notification | null {
	if (Notification.permission !== 'granted') return null;
	return new Notification(title, options);
}

/**
 * Capture install prompt
 */
export function getInstallPrompt(): Promise<any> {
	return new Promise((resolve) => {
		if (installPrompt) {
			resolve(installPrompt);
			return;
		}
		window.addEventListener('beforeinstallprompt', (e: any) => {
			e.preventDefault();
			installPrompt = e;
			resolve(e);
		}, { once: true });
	});
}

/**
 * Check if running as standalone PWA
 */
export function isStandalone(): boolean {
	return (
		window.matchMedia('(display-mode: standalone)').matches ||
		(window.navigator as any).standalone === true
	);
}
