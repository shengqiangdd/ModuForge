package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// LLM Cache Integration — Optimized LLM Call Pipeline
//
// Integrates:
// 1. Prefix Cache — Stable prompt ordering for maximum cache hits
// 2. Semantic Cache — Skip LLM for similar prompts
// 3. Context Condenser — Smart summarization
// 4. Session Learning — Pattern reuse
//
// Call flow:
// 1. Check semantic cache → if hit, return cached response
// 2. Build optimized prompt with prefix ordering
// 3. Check prefix cache → if hit, reuse cached prefix
// 4. Call LLM with optimized prompt
// 5. Cache response for future semantic matches
// 6. Record successful pattern for session learning
// ═══════════════════════════════════════════════════════════════════

// OptimizedLLMCall wraps the LLM call with caching optimizations.
type OptimizedLLMCall struct {
	runner          *AgentRunner
	prefixCache     *PrefixCache
	semanticCache   *SemanticCache
	contextCondenser *ContextCondenser
	sessionLearner  *SessionLearner
}

// NewOptimizedLLMCall creates a new optimized LLM call wrapper.
func NewOptimizedLLMCall(runner *AgentRunner) *OptimizedLLMCall {
	return &OptimizedLLMCall{
		runner:          runner,
		prefixCache:     runner.prefixCache,
		semanticCache:   runner.semanticCache,
		contextCondenser: runner.contextCondenser,
		sessionLearner:  runner.sessionLearner,
	}
}

// CallLLM performs an optimized LLM call with all caching layers.
func (o *OptimizedLLMCall) CallLLM(
	ctx context.Context,
	messages []map[string]interface{},
	tools []ToolDef,
	w SSEWriter,
	userID, reqProviderID, reqModel string,
	cfg RunConfig,
) (*LLMResponse, error) {
	startTime := time.Now()

	// Step 1: Check semantic cache
	if cachedResponse, found := o.checkSemanticCache(messages); found {
		log.Printf("[OptimizedLLM] Semantic cache HIT (took %v)", time.Since(startTime))
		return o.buildResponseFromCached(cachedResponse), nil
	}

	// Step 2: Build optimized prompt with prefix ordering
	optimizedMessages := o.buildOptimizedPrompt(messages, tools, cfg)

	// Step 3: Check prefix cache for the stable prefix
	prefixKey := o.getPrefixKey(optimizedMessages)
	if _, found := o.prefixCache.Get(prefixKey); found {
		log.Printf("[OptimizedLLM] Prefix cache HIT for key %s", prefixKey[:8])
		// Prefix is cached, but we still need to call LLM for the full response
		// This just means the LLM provider can reuse the cached prefix computation
	}

	// Step 4: Call LLM with optimized prompt
	llmResp, err := o.runner.callLLMWithTools(ctx, optimizedMessages, tools, w, userID, reqProviderID, reqModel, cfg)
	if err != nil {
		return nil, err
	}

	// Step 5: Cache response for semantic matching
	if llmResp.Content != "" {
		o.cacheSemanticResponse(messages, llmResp.Content)
	}

	// Step 6: Record successful pattern for session learning
	o.recordPattern(messages, llmResp)

	log.Printf("[OptimizedLLM] Completed in %v (messages=%d, tools=%d)", time.Since(startTime), len(messages), len(tools))
	return llmResp, nil
}

// checkSemanticCache checks if we have a cached response for similar prompts.
func (o *OptimizedLLMCall) checkSemanticCache(messages []map[string]interface{}) (string, bool) {
	// Build a cache key from the conversation
	key := o.buildSemanticCacheKey(messages)
	return o.semanticCache.Get(key)
}

// cacheSemanticResponse stores the response for future semantic matching.
func (o *OptimizedLLMCall) cacheSemanticResponse(messages []map[string]interface{}, response string) {
	key := o.buildSemanticCacheKey(messages)
	o.semanticCache.Put(key, response)
}

// buildSemanticCacheKey creates a key for semantic caching.
// Uses the last few messages for similarity matching.
func (o *OptimizedLLMCall) buildSemanticCacheKey(messages []map[string]interface{}) string {
	// Use the last 3 messages for similarity (most relevant context)
	var keyParts []string
	start := len(messages) - 3
	if start < 0 {
		start = 0
	}

	for _, msg := range messages[start:] {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if content == "" {
			continue
		}
		// Truncate for key generation
		if len(content) > 500 {
			content = content[:500]
		}
		keyParts = append(keyParts, role+":"+content)
	}

	return strings.Join(keyParts, "|||")
}

// buildOptimizedPrompt reorders messages for optimal prefix caching.
func (o *OptimizedLLMCall) buildOptimizedPrompt(
	messages []map[string]interface{},
	tools []ToolDef,
	cfg RunConfig,
) []map[string]interface{} {
	if len(messages) == 0 {
		return messages
	}

	// Separate messages by type
	var systemMessages []map[string]interface{}
	var userMessages []map[string]interface{}
	var assistantMessages []map[string]interface{}
	var toolMessages []map[string]interface{}

	for _, msg := range messages {
		role, _ := msg["role"].(string)
		switch role {
		case "system":
			systemMessages = append(systemMessages, msg)
		case "user":
			userMessages = append(userMessages, msg)
		case "assistant":
			assistantMessages = append(assistantMessages, msg)
		case "tool":
			toolMessages = append(toolMessages, msg)
		}
	}

	// Reorder for optimal prefix caching:
	// 1. System messages (most stable)
	// 2. Tool definitions as system message (stable)
	// 3. Assistant messages (stable within session)
	// 4. User messages (variable)
	// 5. Tool messages (variable, but follows assistant)

	var optimized []map[string]interface{}

	// 1. System messages first (stable prefix)
	optimized = append(optimized, systemMessages...)

	// 2. Add tool definitions as a system message if tools are present
	if len(tools) > 0 {
		toolDefContent := o.buildToolDefinitionsContent(tools)
		optimized = append(optimized, map[string]interface{}{
			"role":    "system",
			"content": toolDefContent,
		})
	}

	// 3. Interleave assistant and tool messages (maintains conversation flow)
	// But keep them before the latest user message
	assistantIdx := 0
	toolIdx := 0
	for assistantIdx < len(assistantMessages) || toolIdx < len(toolMessages) {
		// Add assistant message
		if assistantIdx < len(assistantMessages) {
			optimized = append(optimized, assistantMessages[assistantIdx])
			assistantIdx++
		}

		// Add tool messages that follow this assistant message
		for toolIdx < len(toolMessages) {
			toolMsg := toolMessages[toolIdx]
			toolCallID, _ := toolMsg["tool_call_id"].(string)
			// Check if this tool message has a matching assistant tool_call
			if toolCallID != "" {
				optimized = append(optimized, toolMsg)
				toolIdx++
			} else {
				break
			}
		}
	}

	// 4. User messages last (most variable)
	// Keep all but the last user message in the stable section
	if len(userMessages) > 1 {
		for _, msg := range userMessages[:len(userMessages)-1] {
			optimized = append(optimized, msg)
		}
	}

	// 5. Last user message at the very end (changes every request)
	if len(userMessages) > 0 {
		optimized = append(optimized, userMessages[len(userMessages)-1])
	}

	return optimized
}

// buildToolDefinitionsContent creates a single content block with all tool definitions.
func (o *OptimizedLLMCall) buildToolDefinitionsContent(tools []ToolDef) string {
	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")

	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("### %s\n", tool.Function.Name))
		if tool.Function.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n", tool.Function.Description))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// getPrefixKey generates a cache key for the stable prefix.
func (o *OptimizedLLMCall) getPrefixKey(messages []map[string]interface{}) string {
	// Use the first 3 messages (system + tools + first user) as the prefix key
	var prefixParts []string
	for i := 0; i < 3 && i < len(messages); i++ {
		content, _ := messages[i]["content"].(string)
		if content != "" {
			prefixParts = append(prefixParts, content)
		}
	}

	combined := strings.Join(prefixParts, "|||")
	key := o.prefixCache.GetCacheKey(combined)
	return key
}

// buildResponseFromCached creates an LLMResponse from a cached string.
func (o *OptimizedLLMCall) buildResponseFromCached(cached string) *LLMResponse {
	return &LLMResponse{
		Content:   cached,
		ToolCalls: nil,
	}
}

// recordPattern records the successful pattern for session learning.
func (o *OptimizedLLMCall) recordPattern(messages []map[string]interface{}, resp *LLMResponse) {
	if resp == nil {
		return
	}

	// Extract task type from first user message
	taskType := "general"
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		if role == "user" {
			content, _ := msg["content"].(string)
			if content != "" {
				taskType = o.classifyTask(content)
				break
			}
		}
	}

	// Extract tool sequence from response
	var toolSequence []string
	if resp.ToolCalls != nil {
		for _, tc := range resp.ToolCalls {
			toolSequence = append(toolSequence, tc.Function.Name)
		}
	}

	// Extract file types from messages
	var fileTypes []string
	for _, msg := range messages {
		content, _ := msg["content"].(string)
		if strings.Contains(content, ".rs") {
			fileTypes = append(fileTypes, "rust")
		}
		if strings.Contains(content, ".go") {
			fileTypes = append(fileTypes, "go")
		}
		if strings.Contains(content, ".cpp") || strings.Contains(content, ".c") {
			fileTypes = append(fileTypes, "cpp")
		}
	}

	// Deduplicate file types
	uniqueTypes := make(map[string]bool)
	for _, ft := range fileTypes {
		uniqueTypes[ft] = true
	}
	var dedupedTypes []string
	for ft := range uniqueTypes {
		dedupedTypes = append(dedupedTypes, ft)
	}

	// Record the pattern
	if len(toolSequence) > 0 {
		o.sessionLearner.RecordPattern(taskType, toolSequence, dedupedTypes)
	}
}

// classifyTask classifies the task type from user input.
func (o *OptimizedLLMCall) classifyTask(content string) string {
	lower := strings.ToLower(content)

	if strings.Contains(lower, "fix") || strings.Contains(lower, "error") || strings.Contains(lower, "bug") {
		return "fix_error"
	}
	if strings.Contains(lower, "add") || strings.Contains(lower, "create") || strings.Contains(lower, "implement") {
		return "add_feature"
	}
	if strings.Contains(lower, "refactor") || strings.Contains(lower, "optimize") {
		return "refactor"
	}
	if strings.Contains(lower, "test") {
		return "add_test"
	}
	if strings.Contains(lower, "explain") || strings.Contains(lower, "what") {
		return "explain"
	}

	return "general"
}

// ═══════════════════════════════════════════════════════════════════
// Cache Statistics API
// ═══════════════════════════════════════════════════════════════════

// GetCacheStats returns comprehensive cache statistics.
func (r *AgentRunner) GetCacheStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Prefix cache stats
	if r.prefixCache != nil {
		stats["prefix_cache"] = r.prefixCache.GetStats()
	}

	// Semantic cache stats
	if r.semanticCache != nil {
		stats["semantic_cache"] = r.semanticCache.GetStats()
	}

	// Context condenser stats
	if r.contextCondenser != nil {
		stats["context_condenser"] = r.contextCondenser.GetStats()
	}

	// Session learner stats
	if r.sessionLearner != nil {
		stats["session_learner"] = r.sessionLearner.GetStats()
	}

	// Tool result cache stats (per session)
	toolCacheStats := make(map[string]interface{})
	r.sessionCaches.Range(func(key, value interface{}) bool {
		sessionID, ok := key.(string)
		if !ok {
			return true
		}
		cache, ok := value.(*toolResultCache)
		if !ok {
			return true
		}
		cache.mu.RLock()
		toolCacheStats[sessionID] = map[string]interface{}{
			"entries": len(cache.entries),
			"max":     cache.maxSize,
		}
		cache.mu.RUnlock()
		return true
	})
	stats["tool_result_caches"] = toolCacheStats

	return stats
}

// InvalidateAllCaches clears all cache layers.
func (r *AgentRunner) InvalidateAllCaches() {
	if r.prefixCache != nil {
		r.prefixCache.Invalidate()
	}
	if r.semanticCache != nil {
		r.semanticCache.Invalidate()
	}
	// Note: We don't invalidate context condenser as it maintains continuity
	// Note: We don't invalidate session learner as it learns over time
}

// ═══════════════════════════════════════════════════════════════════
// Context Condenser Integration
// ═══════════════════════════════════════════════════════════════════

// ShouldCondenseContext checks if the conversation needs condensing.
func (r *AgentRunner) ShouldCondenseContext(conversation []map[string]interface{}) bool {
	if r.contextCondenser == nil {
		return false
	}
	return r.contextCondenser.ShouldCondense(conversation)
}

// CondenseContext performs intelligent context compression.
func (r *AgentRunner) CondenseContext(
	conversation []map[string]interface{},
	cfg RunConfig,
) ([]map[string]interface{}, bool) {
	if r.contextCondenser == nil {
		return conversation, false
	}

	// Create an LLM caller for summarization
	llmCaller := func(messages []map[string]string) (string, error) {
		// Build a simple request to the LLM for summarization
		var prompt []map[string]interface{}
		for _, msg := range messages {
			prompt = append(prompt, map[string]interface{}{
				"role":    msg["role"],
				"content": msg["content"],
			})
		}

		// Use the existing callLLMSummary method
		summaryPrompt := make([]map[string]string, len(messages))
		for i, msg := range messages {
			summaryPrompt[i] = map[string]string{
				"role":    msg["role"],
				"content": msg["content"],
			}
		}

		return r.callLLMSummary(context.Background(), cfg, summaryPrompt)
	}

	return r.contextCondenser.Condense(conversation, llmCaller)
}
