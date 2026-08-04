package agent

import (
	"context"
	"time"
)

// startKeepalive spawns a goroutine that periodically emits an empty SSE
// "think" event so the frontend idle timeout and proxy/CDN keepalive never
// fire during long-running operations (LLM streaming, tool execution, retry
// backoff sleeps). The goroutine exits as soon as done is closed or ctx is
// cancelled.
func startKeepalive(ctx context.Context, w SSEWriter, done <-chan struct{}, interval time.Duration) {
	go func() {
		incGoroutines()
		defer decGoroutines()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = w.WriteSSE(map[string]interface{}{"type": "step", "step": "think", "content": ""})
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}
