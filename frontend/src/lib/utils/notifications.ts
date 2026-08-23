export interface Notification {
	id: string;
	type: 'success' | 'error' | 'warning' | 'info';
	message: string;
	action?: {
		label: string;
		onClick: () => void;
	};
	duration?: number;
	timestamp: number;
}

export interface NotificationOptions {
	message: string;
	action?: {
		label: string;
		onClick: () => void;
	};
	duration?: number;
}

export class NotificationManager {
	private notifications: Notification[] = [];
	private listeners: Array<(notifications: Notification[]) => void> = [];
	private counter = 0;

	/**
	 * Show a notification
	 */
	show(notification: Omit<Notification, 'id' | 'timestamp'>): string {
		const id = `toast_${++this.counter}_${Date.now()}`;
		const full: Notification = {
			...notification,
			id,
			timestamp: Date.now()
		};

		this.notifications = [...this.notifications, full];
		this.notify();

		// Auto-dismiss if duration > 0
		const duration = notification.duration ?? 3000;
		if (duration > 0) {
			setTimeout(() => this.dismiss(id), duration);
		}

		return id;
	}

	/**
	 * Show success notification
	 */
	success(message: string, options?: { action?: Notification['action']; duration?: number }): string {
		return this.show({
			type: 'success',
			message,
			action: options?.action,
			duration: options?.duration
		});
	}

	/**
	 * Show error notification
	 */
	error(message: string, options?: { action?: Notification['action']; duration?: number }): string {
		return this.show({
			type: 'error',
			message,
			action: options?.action,
			duration: options?.duration ?? 5000
		});
	}

	/**
	 * Show warning notification
	 */
	warning(message: string, options?: { duration?: number }): string {
		return this.show({
			type: 'warning',
			message,
			duration: options?.duration ?? 4000
		});
	}

	/**
	 * Show info notification
	 */
	info(message: string, options?: { duration?: number }): string {
		return this.show({
			type: 'info',
			message,
			duration: options?.duration
		});
	}

	/**
	 * Dismiss a specific notification
	 */
	dismiss(id: string): void {
		this.notifications = this.notifications.filter((n) => n.id !== id);
		this.notify();
	}

	/**
	 * Clear all notifications
	 */
	clearAll(): void {
		this.notifications = [];
		this.notify();
	}

	/**
	 * Get current notifications
	 */
	getNotifications(): Notification[] {
		return [...this.notifications];
	}

	/**
	 * Subscribe to notification changes
	 */
	subscribe(listener: (notifications: Notification[]) => void): () => void {
		this.listeners.push(listener);
		return () => {
			this.listeners = this.listeners.filter((l) => l !== listener);
		};
	}

	private notify(): void {
		for (const listener of this.listeners) {
			listener([...this.notifications]);
		}
	}
}

// Singleton instance
export const notifications = new NotificationManager();
