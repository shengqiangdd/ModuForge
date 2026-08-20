package agent

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
)

// llmMaxRetries is the maximum number of retries for transient LLM errors (429, 5xx, network).
// Free models get more retries since 429 is expected behavior.
const llmMaxRetries = 5

// isLLMRetryableError returns true for errors worth retrying (rate limit, server errors, network).
func isLLMRetryableError(statusCode int) bool {
	return statusCode == 429 || statusCode == 502 || statusCode == 503 || statusCode == 504
}

// llmRetryBackoff returns the sleep duration for attempt number (1-based).
// Uses exponential backoff: 3s, 6s, 12s, 24s. For 429 with Retry-After header, uses that instead.
func llmRetryBackoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if d, err := time.ParseDuration(retryAfter + "s"); err == nil && d > 0 && d <= 120*time.Second {
			return d
		}
	}
	return time.Duration(3<<uint(attempt)) * time.Second // 3s, 6s, 12s, 24s
}

// ═══════════════════════════════════════════════════════════════════
// Circuit Breaker — for free model providers only
//
// Tracks consecutive failures per provider. After threshold failures,
// temporarily skips the provider for cooldown period.
// Optimization 29: Adaptive cooldown — longer cooldown for more failures
// ═══════════════════════════════════════════════════════════════════

// Circuit breaker: consecutive failures before skipping a free model provider
// P2-Optimization: Increased from 3 to 5 to be less aggressive — many failures
// are transient (429 rate limits, network blips) and recover quickly.
const circuitBreakerThreshold = 5

// Circuit breaker: base cooldown period after breaker opens
const circuitBreakerBaseCooldown = 60 * time.Second

type circuitBreaker struct {
	mu            sync.Mutex
	failures      map[string]int       // providerID -> consecutive failures
	lastFailure   map[string]time.Time // providerID -> last failure time
	breakerActive map[string]bool      // providerID -> is breaker open?
}

var globalCircuitBreaker = &circuitBreaker{
	failures:      make(map[string]int),
	lastFailure:   make(map[string]time.Time),
	breakerActive: make(map[string]bool),
}

// adaptiveCooldown returns cooldown proportional to failure count:
// 3 failures → 60s, 5 → 120s, 10 → 300s, 15+ → 600s
func adaptiveCooldown(failures int) time.Duration {
	switch {
	case failures >= 15:
		return 600 * time.Second
	case failures >= 10:
		return 300 * time.Second
	case failures >= 5:
		return 120 * time.Second
	default:
		return circuitBreakerBaseCooldown
	}
}

// RecordSuccess resets the failure counter for a provider.
func (cb *circuitBreaker) RecordSuccess(providerID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures[providerID] = 0
	cb.breakerActive[providerID] = false
}

// RecordFailure increments the failure counter and opens the breaker if threshold reached.
func (cb *circuitBreaker) RecordFailure(providerID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures[providerID]++
	cb.lastFailure[providerID] = time.Now()
	if cb.failures[providerID] >= circuitBreakerThreshold {
		cb.breakerActive[providerID] = true
		cooldown := adaptiveCooldown(cb.failures[providerID])
		log.Printf("[CircuitBreaker] OPEN for provider %s after %d failures (cooldown=%v)",
			providerID, cb.failures[providerID], cooldown)
	}
}

// IsOpen returns true if the circuit breaker is open (provider should be skipped).
func (cb *circuitBreaker) IsOpen(providerID string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if !cb.breakerActive[providerID] {
		return false
	}
	// Adaptive cooldown based on failure count
	cooldown := adaptiveCooldown(cb.failures[providerID])
	if time.Since(cb.lastFailure[providerID]) > cooldown {
		log.Printf("[CircuitBreaker] cooldown elapsed for provider %s (was %v), half-opening", providerID, cooldown)
		cb.breakerActive[providerID] = false
		cb.failures[providerID] = 0
		return false
	}
	return true
}

// ═══════════════════════════════════════════════════════════════════
// Rate Limit Tracker — client-side tracking to avoid 429s
// ═══════════════════════════════════════════════════════════════════

type rateLimitTracker struct {
	mu           sync.Mutex
	requests     []time.Time   // timestamps of recent requests
	maxPerMinute int           // max requests per minute (0 = no limit)
	minInterval  time.Duration // minimum interval between requests
}

var globalRateLimiter = &rateLimitTracker{
	maxPerMinute: 0, // disabled by default, can be configured per model
	minInterval:  0,
}

// ConfigureForModel sets rate limits based on model tier.
func (r *rateLimitTracker) ConfigureForModel(modelName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	lower := strings.ToLower(modelName)
	// Free models: minimal client-side limits — rely on server 429 + Retry-After
	if strings.Contains(lower, "free") || strings.Contains(lower, "mini") || strings.Contains(lower, "lite") {
		r.maxPerMinute = 0              // no client-side cap, trust server
		r.minInterval = 2 * time.Second // tiny 2s gap to avoid bursts
		log.Printf("[RateLimit] configured for free model: no cap, %v min interval", r.minInterval)
	} else {
		// Paid models: no limits
		r.maxPerMinute = 0
		r.minInterval = 0
	}
}

// WaitForSlot blocks until it's safe to make a request.
func (r *rateLimitTracker) WaitForSlot(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.maxPerMinute == 0 && r.minInterval == 0 {
		return nil // no limits
	}

	now := time.Now()

	// Clean up old requests (older than 1 minute)
	cutoff := now.Add(-1 * time.Minute)
	newRequests := make([]time.Time, 0, len(r.requests))
	for _, t := range r.requests {
		if t.After(cutoff) {
			newRequests = append(newRequests, t)
		}
	}
	r.requests = newRequests

	// Check rate limit
	if r.maxPerMinute > 0 && len(r.requests) >= r.maxPerMinute {
		// Wait until oldest request falls out of window
		oldest := r.requests[0]
		waitTime := oldest.Add(1*time.Minute).Sub(now) + 100*time.Millisecond
		if waitTime > 0 {
			log.Printf("[RateLimit] rate limit reached, waiting %v", waitTime)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Check minimum interval
	if r.minInterval > 0 && len(r.requests) > 0 {
		lastRequest := r.requests[len(r.requests)-1]
		elapsed := now.Sub(lastRequest)
		if elapsed < r.minInterval {
			waitTime := r.minInterval - elapsed
			log.Printf("[RateLimit] min interval not met, waiting %v", waitTime)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Record this request
	r.requests = append(r.requests, time.Now())
	return nil
}

// Record429 records a 429 error and adjusts limits.
func (r *rateLimitTracker) Record429() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// On 429, be more conservative: reduce limit by 25%
	if r.maxPerMinute > 4 {
		r.maxPerMinute = r.maxPerMinute * 3 / 4
		log.Printf("[RateLimit] 429 received, reduced limit to %d req/min", r.maxPerMinute)
	}
	// Increase minimum interval
	if r.minInterval < 10*time.Second {
		r.minInterval += 2 * time.Second
		log.Printf("[RateLimit] increased interval to %v", r.minInterval)
	}
}
