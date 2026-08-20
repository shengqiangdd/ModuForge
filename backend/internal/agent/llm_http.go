package agent

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// llmHTTPTransport is a shared transport with connection pooling.
// Reusing TCP connections avoids the TLS handshake overhead on every LLM call,
// which is critical for free-tier providers that rate-limit per-connection.
var llmHTTPTransport = &http.Transport{
	MaxIdleConns:        30, // Optimization 41: increased from 10 to support concurrent agent sessions
	MaxIdleConnsPerHost: 10, // Optimization 41: increased from 5 to reuse more connections per LLM endpoint
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
	DisableKeepAlives:   false,
}

// llmHTTPClient is a shared HTTP client with timeouts for LLM API calls.
// Using http.DefaultClient (no timeout) caused requests to hang forever when
// the LLM endpoint was unresponsive, leading to connection drops and "network error" on the frontend.
var llmHTTPClient = &http.Client{
	Transport: llmHTTPTransport,
	Timeout:   180 * time.Second, // hard ceiling per request (increased for slow/free models with large context)
}

// LLMHTTPClient returns the shared HTTP client used for all LLM API calls.
// Skills should reuse this client to share connection pools and timeouts.
func LLMHTTPClient() *http.Client {
	return llmHTTPClient
}

// Optimization 35: HTTP connection pre-warm
// Proactively establishes a TCP+TLS connection to the LLM endpoint so the first
// real request doesn't pay the cold-start latency (200-500ms for TLS handshake).
// Called once per endpoint; subsequent calls are no-ops via sync.Once.
var prewarmOnce sync.Map // endpoint -> *sync.Once

// PrewarmConnection establishes an idle connection to the given endpoint.
// Safe to call concurrently; only performs the handshake once per endpoint.
func PrewarmConnection(endpoint string) {
	if endpoint == "" {
		return
	}
	val, _ := prewarmOnce.LoadOrStore(endpoint, &sync.Once{})
	once := val.(*sync.Once)
	once.Do(func() {
		go func() {
			// Strip /chat/completions suffix for the HEAD probe
			base := endpoint
			if idx := strings.Index(base, "/chat/completions"); idx > 0 {
				base = base[:idx]
			}
			req, err := http.NewRequest("HEAD", base, nil)
			if err != nil {
				return
			}
			resp, err := llmHTTPClient.Do(req)
			if err != nil {
				debugLog("prewarm failed for %s: %v", base, err)
				return
			}
			resp.Body.Close()
			debugLog("prewarmed connection to %s (status=%d)", base, resp.StatusCode)
		}()
	})
}

// handleLLMHTTPError converts an HTTP error response into an error and a
// retryable flag. It records 429 rate limits and circuit-breaker failures for
// free models, and extracts the Retry-After header when present.
func handleLLMHTTPError(resp *http.Response, modelTier ModelTier, reqProviderID, model string) (error, bool) {
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	errMsg := fmt.Sprintf("LLM error (HTTP %d) provider=%s model=%s: %s", resp.StatusCode, reqProviderID, model, string(respBody))
	err := errors.New(errMsg)
	if !isLLMRetryableError(resp.StatusCode) {
		return err, false
	}
	// Record 429 for rate limit tracking
	if resp.StatusCode == 429 {
		globalRateLimiter.Record429()
		// IMPORTANT: Do NOT record 429 as circuit breaker failure for free models.
		// 429 is expected behavior for free-tier providers — it just means "slow down".
		// Only record actual server errors (5xx) as failures.
		// Circuit breaker should only open on persistent server failures, not rate limits.
	}
	// Extract Retry-After header for 429
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		err = fmt.Errorf("%s\nRetry-After: %s", errMsg, retryAfter)
	}
	return err, true
}
