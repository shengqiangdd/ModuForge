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
		entries:   make(map[string]*PrefixEntry),
		maxEntries: maxEntries,
		ttl:       ttl,
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

// GetStats returns cache statistics.
func (pc *PrefixCache) GetStats() map[string]interface{} {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	total := pc.hits + pc.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(pc.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"entries":  len(pc.entries),
		"hits":     pc.hits,
		"misses":   pc.misses,
		"hit_rate": fmt.Sprintf("%.1f%%", hitRate),
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
	maxEntries  int
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
		entries:            make([]SemanticEntry, 0, maxEntries),
		maxEntries:         maxEntries,
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

// GetStats returns cache statistics.
func (sc *SemanticCache) GetStats() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	total := sc.hits + sc.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(sc.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"entries":  len(sc.entries),
		"hits":     sc.hits,
		"misses":   sc.misses,
		"hit_rate": fmt.Sprintf("%.1f%%", hitRate),
		"threshold": sc.similarityThreshold,
	}
}

// Invalidate clears all cache entries.
func (sc *SemanticCache) Invalidate() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.entries = sc.entries[:0]
	sc.hits = 0
	sc.misses = 0
}

// ═══════════════════════════════════════════════════════════════════
// Context Condenser — LLM-based context summarization
//
// Based on OpenHands LLMSummarizingCondenser:
// 1. Keep recent messages intact
// 2. Summarize older messages with LLM
// 3. Preserve key information (file paths, decisions, errors)
// 4. Maintain continuity across summarizations
//
// Reference: https://docs.openhands.dev/sdk/guides/context-condenser
// ═══════════════════════════════════════════════════════════════════

// ContextCondenser manages intelligent context compression.
type ContextCondenser struct {
	// Configuration
	maxContextLength int // Max messages before condensing
	keepRecent       int // Number of recent messages to keep intact
	keepFirst        int // Number of first messages to keep (system prompt)

	// State
	summaryHistory []string // Previous summaries for continuity
	mu             sync.RWMutex
}

// NewContextCondenser creates a new context condenser.
func NewContextCondenser(maxContextLength, keepRecent, keepFirst int) *ContextCondenser {
	if maxContextLength <= 0 {
		maxContextLength = 30
	}
	if keepRecent <= 0 {
		keepRecent = 6
	}
	if keepFirst <= 0 {
		keepFirst = 1 // Always keep system prompt
	}
	return &ContextCondenser{
		maxContextLength: maxContextLength,
		keepRecent:       keepRecent,
		keepFirst:        keepFirst,
		summaryHistory:   make([]string, 0),
	}
}

// ShouldCondense checks if the conversation needs condensing.
func (cc *ContextCondenser) ShouldCondense(conversation []map[string]interface{}) bool {
	return len(conversation) > cc.maxContextLength
}

// Condense performs intelligent context compression.
// Returns the condensed conversation and whether compression was applied.
func (cc *ContextCondenser) Condense(
	conversation []map[string]interface{},
	llmCaller func(messages []map[string]string) (string, error),
) ([]map[string]interface{}, bool) {
	if !cc.ShouldCondense(conversation) {
		return conversation, false
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Split conversation into three parts:
	// 1. First messages (system prompt, etc.) - always keep
	// 2. Middle messages (to be summarized)
	// 3. Recent messages (to keep in full)

	if len(conversation) <= cc.keepFirst+cc.keepRecent {
		return conversation, false
	}

	firstMessages := conversation[:cc.keepFirst]
	middleMessages := conversation[cc.keepFirst : len(conversation)-cc.keepRecent]
	recentMessages := conversation[len(conversation)-cc.keepRecent:]

	// Build summary of middle messages
	summary, err := cc.buildSummary(middleMessages, llmCaller)
	if err != nil {
		log.Printf("[ContextCondenser] summary failed: %v, using heuristic", err)
		summary = cc.heuristicSummary(middleMessages)
	}

	// Build condensed conversation
	condensed := make([]map[string]interface{}, 0, len(firstMessages)+1+len(recentMessages))
	condensed = append(condensed, firstMessages...)

	// Add summary as system message
	condensed = append(condensed, map[string]interface{}{
		"role":    "system",
		"content": fmt.Sprintf("[Context Summary]\n%s", summary),
	})

	condensed = append(condensed, recentMessages...)

	log.Printf("[ContextCondenser] condensed %d messages to %d (kept %d first, %d recent, summary=%d chars)",
		len(conversation), len(condensed), len(firstMessages), len(recentMessages), len(summary))

	return condensed, true
}

// buildSummary creates an LLM-based summary of messages.
func (cc *ContextCondenser) buildSummary(
	messages []map[string]interface{},
	llmCaller func(messages []map[string]string) (string, error),
) (string, error) {
	if llmCaller == nil {
		return "", fmt.Errorf("no LLM caller provided")
	}

	// Build the conversation text
	var convText strings.Builder
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if content == "" {
			continue
		}

		label := "User"
		switch role {
		case "assistant":
			label = "Agent"
		case "system":
			label = "System"
		case "tool":
			label = "Tool Result"
		}

		// Truncate very long messages
		if len(content) > 2000 {
			content = content[:2000] + "...[truncated]"
		}

		convText.WriteString(fmt.Sprintf("%s: %s\n\n", label, content))
	}

	// Include previous summary for continuity
	var prompt []map[string]string
	if len(cc.summaryHistory) > 0 {
		lastSummary := cc.summaryHistory[len(cc.summaryHistory)-1]
		prompt = append(prompt, map[string]string{
			"role":    "system",
			"content": fmt.Sprintf("You are a conversation summarizer. You have a previous summary:\n\n%s\n\nNow summarize the following NEW messages, incorporating key points from the previous summary.", lastSummary),
		})
	} else {
		prompt = append(prompt, map[string]string{
			"role":    "system",
			"content": `You are a conversation summarizer for a coding agent. Summarize the conversation, preserving:
- File paths mentioned or modified
- Key decisions and their reasons
- Errors encountered and how they were resolved
- Current work in progress
- User constraints and requirements

Be concise but complete. Output ONLY the summary text, no labels.`,
		})
	}

	prompt = append(prompt, map[string]string{
		"role":    "user",
		"content": convText.String(),
	})

	summary, err := llmCaller(prompt)
	if err != nil {
		return "", err
	}

	// Store summary for future continuity
	cc.summaryHistory = append(cc.summaryHistory, summary)
	if len(cc.summaryHistory) > 5 {
		cc.summaryHistory = cc.summaryHistory[len(cc.summaryHistory)-5:]
	}

	return summary, nil
}

// heuristicSummary creates a zero-LLM-cost summary.
func (cc *ContextCondenser) heuristicSummary(messages []map[string]interface{}) string {
	var fileChanges []string
	var decisions []string
	var errors []string
	var rounds int

	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		if role == "user" {
			rounds++
		}

		// Extract file changes
		if isFileChangeResult(content) {
			if fc := extractFileChange(content); fc != "" {
				fileChanges = append(fileChanges, fc)
			}
		}

		// Extract decisions
		if containsDecision(content) {
			if len(content) > 200 {
				content = content[:200]
			}
			decisions = append(decisions, content)
		}

		// Extract errors
		if strings.HasPrefix(content, "Error:") || strings.HasPrefix(content, "❌") {
			if len(content) > 150 {
				content = content[:150]
			}
			errors = append(errors, content)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Summary of %d messages (%d rounds):\n\n", len(messages), rounds))

	if len(fileChanges) > 0 {
		sb.WriteString(fmt.Sprintf("Files modified (%d):\n", len(fileChanges)))
		for _, fc := range fileChanges {
			if len(fileChanges) > 10 && sb.Len() > 500 {
				sb.WriteString(fmt.Sprintf("  ... and %d more files\n", len(fileChanges)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", fc))
		}
	}

	if len(decisions) > 0 {
		sb.WriteString(fmt.Sprintf("\nKey decisions (%d):\n", len(decisions)))
		for _, d := range decisions {
			sb.WriteString(fmt.Sprintf("  - %s\n", d))
		}
	}

	if len(errors) > 0 {
		sb.WriteString(fmt.Sprintf("\nErrors (%d):\n", len(errors)))
		for _, e := range errors {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	return sb.String()
}

// GetStats returns condenser statistics.
func (cc *ContextCondenser) GetStats() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return map[string]interface{}{
		"max_context_length": cc.maxContextLength,
		"keep_recent":        cc.keepRecent,
		"keep_first":         cc.keepFirst,
		"summary_count":      len(cc.summaryHistory),
	}
}

// ═══════════════════════════════════════════════════════════════════
// Session Learning — Learn from successful patterns
//
// Based on pattern learning research:
// 1. Track successful tool call sequences
// 2. Cache successful patterns for reuse
// 3. Suggest patterns for similar tasks
// 4. Reduce redundant exploration
// ═══════════════════════════════════════════════════════════════════

// SessionLearner tracks and learns from successful patterns.
type SessionLearner struct {
	patterns []LearnedPattern
	mu       sync.RWMutex

	// Configuration
	maxPatterns int
}

// LearnedPattern represents a learned successful pattern.
type LearnedPattern struct {
	TaskType    string   // e.g., "fix_rust_error", "add_feature"
	ToolSequence []string // Sequence of tools that worked
	FileTypes   []string // File types involved
	SuccessRate float64  // Historical success rate
	LastUsed    time.Time
}

// NewSessionLearner creates a new session learner.
func NewSessionLearner(maxPatterns int) *SessionLearner {
	if maxPatterns <= 0 {
		maxPatterns = 100
	}
	return &SessionLearner{
		patterns:    make([]LearnedPattern, 0, maxPatterns),
		maxPatterns: maxPatterns,
	}
}

// RecordPattern records a successful tool call sequence.
func (sl *SessionLearner) RecordPattern(taskType string, toolSequence []string, fileTypes []string) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	// Find existing pattern or create new
	for i := range sl.patterns {
		if sl.patterns[i].TaskType == taskType {
			sl.patterns[i].ToolSequence = toolSequence
			sl.patterns[i].FileTypes = fileTypes
			sl.patterns[i].SuccessRate = (sl.patterns[i].SuccessRate*0.9 + 1.0*0.1) // Exponential moving average
			sl.patterns[i].LastUsed = time.Now()
			return
		}
	}

	// New pattern
	if len(sl.patterns) >= sl.maxPatterns {
		sl.patterns = sl.patterns[1:] // Remove oldest
	}

	sl.patterns = append(sl.patterns, LearnedPattern{
		TaskType:     taskType,
		ToolSequence: toolSequence,
		FileTypes:    fileTypes,
		SuccessRate:  1.0,
		LastUsed:     time.Now(),
	})
}

// SuggestPattern suggests a pattern for a given task type.
func (sl *SessionLearner) SuggestPattern(taskType string) *LearnedPattern {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	for i := range sl.patterns {
		if sl.patterns[i].TaskType == taskType && sl.patterns[i].SuccessRate > 0.5 {
			return &sl.patterns[i]
		}
	}
	return nil
}

// GetStats returns learner statistics.
func (sl *SessionLearner) GetStats() map[string]interface{} {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	return map[string]interface{}{
		"patterns":    len(sl.patterns),
		"max_patterns": sl.maxPatterns,
	}
}
