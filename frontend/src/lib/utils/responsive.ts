export enum Breakpoint {
	MOBILE = 375,
	TABLET = 768,
	DESKTOP = 1024,
	LARGE = 1440
}

/**
 * Get current viewport width (browser only)
 */
export function getViewportWidth(): number {
	if (typeof window === 'undefined') return Breakpoint.DESKTOP;
	return window.innerWidth;
}

/**
 * Get current viewport height (browser only)
 */
export function getViewportHeight(): number {
	if (typeof window === 'undefined') return 800;
	return window.innerHeight;
}

/**
 * Get viewport size
 */
export function getViewportSize(): { width: number; height: number } {
	return {
		width: getViewportWidth(),
		height: getViewportHeight()
	};
}

/**
 * Check if current viewport is mobile
 */
export function isMobile(): boolean {
	return getViewportWidth() < Breakpoint.TABLET;
}

/**
 * Check if current viewport is tablet
 */
export function isTablet(): boolean {
	const w = getViewportWidth();
	return w >= Breakpoint.TABLET && w < Breakpoint.DESKTOP;
}

/**
 * Check if current viewport is desktop
 */
export function isDesktop(): boolean {
	return getViewportWidth() >= Breakpoint.DESKTOP;
}

/**
 * Match media query
 */
export function matchMedia(query: string): boolean {
	if (typeof window === 'undefined') return false;
	return window.matchMedia(query).matches;
}

/**
 * Svelte 5 compatible reactive breakpoint hook
 * Returns a readable store-like object with current breakpoint info
 */
export function useBreakpoint() {
	let width = $state(getViewportWidth());

	const mobile = $derived(width < Breakpoint.TABLET);
	const tablet = $derived(width >= Breakpoint.TABLET && width < Breakpoint.DESKTOP);
	const desktop = $derived(width >= Breakpoint.DESKTOP);
	const large = $derived(width >= Breakpoint.LARGE);

	if (typeof window !== 'undefined') {
		const handler = () => {
			width = window.innerWidth;
		};
		window.addEventListener('resize', handler);
	}

	return {
		get width() { return width; },
		get mobile() { return mobile; },
		get tablet() { return tablet; },
		get desktop() { return desktop; },
		get large() { return large; }
	};
}
