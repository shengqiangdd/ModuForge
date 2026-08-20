package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// Prefix Cache — LLM Prompt Prefix Optimization
//
// Based on Anthropic/OpenAI prefix caching best practices:
// 1. Stable content (system prompt, tool definitions) FIRST
// 2. Variable content (user messages, tool results) LAST
// 3. Cache prefix hashes to detect changes
// 4. Maximize cache hit rate across requests
//
// Reference: https://www.digitalapplied.com/blog/prompt-caching-2026
// ═══════════════════════════════════════════════════════════════════

// PrefixCache caches the prefix portion of prompts for LLM requests.
type PrefixCache struct {
	// Cache entries: prefix hash -> cached prefix content
	entries map[string]*PrefixEntry
	mu      sync.RWMutex

	// Statistics
	hits   int64
	misses int64

	// Configuration
	maxEntries int
	ttl        time.Duration
}

// PrefixEntry is a single cached prefix.
type PrefixEntry struct {
	Hash      string    // SHA256 of prefix content
	Content   string    // The prefix content
	CreatedAt time.Time // When this entry was created
	HitCount  int64     // How many times this prefix was used
}

// NewPrefixCache creates a new prefix cache.
func NewPrefixCache(maxEntries int, ttl time.Duration) *PrefixCache {
	if maxEntries <= 0 {
		maxEntries = 100
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute // Default 5-min TTL for cache warming
	}
	return &PrefixCache{
		entries:    make(map[string]*PrefixEntry),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

// BuildOptimizedPrompt constructs a prompt with optimal ordering for prefix caching.
//
// Order (from first to last):
// 1. System prompt (stable, cacheable)
// 2. Tool definitions (stable, cacheable)
// 3. Project context (stable per project, cacheable)
// 4. Conversation history (variable, not cacheable)
// 5. Current user message (changes every request)
//
// This ordering ensures the stable prefix is reused across requests.
func (pc *PrefixCache) BuildOptimizedPrompt(
	systemPrompt string,
	toolDefs []ToolDef,
	projectContext string,
	history []map[string]interface{},
	currentMessage string,
) []map[string]string {
	var messages []map[string]string

	// 1. System prompt (MOST stable - stays the same across all requests)
	messages = append(messages, map[string]string{
		"role":    "system",
		"content": systemPrompt,
	})

	// 2. Tool definitions as a single system message (stable)
	if len(toolDefs) > 0 {
		toolContent := pc.buildToolDefsContent(toolDefs)
		if toolContent != "" {
			messages = append(messages, map[string]string{
				"role":    "system",
				"content": toolContent,
			})
		}
	}

	// 3. Project context (stable per project session)
	if projectContext != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": projectContext,
		})
	}

	// 4. Conversation history (variable)
	for _, msg := range history {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if role == "" || content == "" {
			continue
		}
		// Skip system messages (already added above)
		if role == "system" {
			continue
		}
		// Only include user and assistant messages in history
		if role == "user" || role == "assistant" {
			messages = append(messages, map[string]string{
				"role":    role,
				"content": content,
			})
		}
	}

	// 5. Current user message (MOST variable - changes every request)
	if currentMessage != "" {
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": currentMessage,
		})
	}

	return messages
}

// buildToolDefsContent converts tool definitions to a single content block.
func (pc *PrefixCache) buildToolDefsContent(toolDefs []ToolDef) string {
	if len(toolDefs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")

	for _, def := range toolDefs {
		sb.WriteString(fmt.Sprintf("### %s\n", def.Function.Name))
		if def.Function.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n", def.Function.Description))
		}
		if len(def.Function.Parameters) > 0 {
			paramsJSON, _ := json.Marshal(def.Function.Parameters)
			sb.WriteString(fmt.Sprintf("Parameters: %s\n", string(paramsJSON)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetCacheKey generates a cache key for a prefix.
func (pc *PrefixCache) GetCacheKey(prefix string) string {
	hash := sha256.Sum256([]byte(prefix))
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes for shorter key
}

// Get retrieves a cached prefix by key.
func (pc *PrefixCache) Get(key string) (string, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	entry, ok := pc.entries[key]
	if !ok {
		pc.misses++
		return "", false
	}

	// Check TTL
	if time.Since(entry.CreatedAt) > pc.ttl {
		pc.misses++
		return "", false
	}

	entry.HitCount++
	pc.hits++
	return entry.Content, true
}

// Put stores a prefix in the cache.
func (pc *PrefixCache) Put(key, content string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Evict if full (LRU-like: remove oldest)
	if len(pc.entries) >= pc.maxEntries {
		pc.evictOldest()
	}

	pc.entries[key] = &PrefixEntry{
		Hash:      pc.GetCacheKey(content),
		Content:   content,
		CreatedAt: time.Time{},
		HitCount:  0,
	}
}

// evictOldest removes the oldest entry (must hold lock).
func (pc *PrefixCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range pc.entries {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(pc.entries, oldestKey)
	}
}

// Invalidate clears all cache entries.
func (pc *PrefixCache) Invalidate() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.entries = make(map[string]*PrefixEntry)
	pc.hits = 0
	pc.misses = 0
}

// ═══════════════════════════════════════════════════════════════════
// Semantic Cache — Cache similar LLM responses
//
// Based on GPTCache pattern:
// 1. Embed the input prompt
// 2. Find similar cached prompts
// 3. Return cached response if similarity > threshold
// 4. Bypass LLM entirely for semantically similar queries
//
// Reference: https://github.com/zilliztech/GPTCache
// ═══════════════════════════════════════════════════════════════════

// SemanticCache caches LLM responses based on semantic similarity.
type SemanticCache struct {
	entries []SemanticEntry
	mu      sync.RWMutex

	// Configuration
	maxEntries          int
	similarityThreshold float64 // 0.0 - 1.0, higher = more strict

	// Statistics
	hits   int64
	misses int64
}

// SemanticEntry is a single cached semantic entry.
type SemanticEntry struct {
	PromptHash string    // Hash of the prompt
	Prompt     string    // Original prompt (for debugging)
	Response   string    // Cached LLM response
	CreatedAt  time.Time // When this entry was created
	HitCount   int64     // How many times this was used
}

// NewSemanticCache creates a new semantic cache.
func NewSemanticCache(maxEntries int, similarityThreshold float64) *SemanticCache {
	if maxEntries <= 0 {
		maxEntries = 500
	}
	if similarityThreshold <= 0 || similarityThreshold > 1 {
		similarityThreshold = 0.85 // Default 85% similarity threshold
	}
	return &SemanticCache{
		entries:             make([]SemanticEntry, 0, maxEntries),
		maxEntries:          maxEntries,
		similarityThreshold: similarityThreshold,
	}
}

// Get retrieves a cached response for a similar prompt.
func (sc *SemanticCache) Get(prompt string) (string, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	promptHash := sc.hashPrompt(prompt)

	for i := range sc.entries {
		entry := &sc.entries[i]
		if entry.PromptHash == promptHash {
			// Exact match
			entry.HitCount++
			sc.hits++
			log.Printf("[SemanticCache] EXACT HIT for prompt hash %s", promptHash[:8])
			return entry.Response, true
		}

		// Semantic similarity check (simplified: using token overlap)
		similarity := sc.calculateSimilarity(prompt, entry.Prompt)
		if similarity >= sc.similarityThreshold {
			entry.HitCount++
			sc.hits++
			log.Printf("[SemanticCache] SEMANTIC HIT (%.1f%%) for prompt hash %s", similarity*100, entry.PromptHash[:8])
			return entry.Response, true
		}
	}

	sc.misses++
	return "", false
}

// Put stores a prompt-response pair in the cache.
func (sc *SemanticCache) Put(prompt, response string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Don't cache empty responses
	if response == "" {
		return
	}

	// Don't cache error responses
	if strings.HasPrefix(response, "Error:") || strings.HasPrefix(response, "❌") {
		return
	}

	// Evict if full (remove oldest)
	if len(sc.entries) >= sc.maxEntries {
		sc.entries = sc.entries[1:] // Remove oldest
	}

	sc.entries = append(sc.entries, SemanticEntry{
		PromptHash: sc.hashPrompt(prompt),
		Prompt:     prompt,
		Response:   response,
		CreatedAt:  time.Now(),
		HitCount:   0,
	})

	log.Printf("[SemanticCache] PUT (total=%d)", len(sc.entries))
}

// hashPrompt creates a hash for exact matching.
func (sc *SemanticCache) hashPrompt(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", hash[:16])
}

// calculateSimilarity computes token overlap similarity between two prompts.
// This is a simplified version - production would use embeddings.
func (sc *SemanticCache) calculateSimilarity(a, b string) float64 {
	tokensA := tokenize(a)
	tokensB := tokenize(b)

	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}

	// Build sets
	setA := make(map[string]bool)
	for _, t := range tokensA {
		setA[t] = true
	}

	// Count overlap
	overlap := 0
	for _, t := range tokensB {
		if setA[t] {
			overlap++
		}
	}

	// Jaccard-like similarity
	union := len(tokensA) + len(tokensB) - overlap
	if union == 0 {
		return 0
	}

	return float64(overlap) / float64(union)
}

// tokenize splits text into tokens (simplified: by whitespace).
func tokenize(text string) []string {
	// Lowercase and split by whitespace
	text = strings.ToLower(text)
	return strings.Fields(text)
}

// Invalidate clears all cache entries.
func (sc *SemanticCache) Invalidate() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.entries = sc.entries[:0]
	sc.hits = 0
	sc.misses = 0
}
