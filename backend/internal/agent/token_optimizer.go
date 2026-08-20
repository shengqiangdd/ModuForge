package agent

import (
	"encoding/json"
	"time"
)

// ===========================================================================
// TokenOptimizer — 综合 Token 优化引擎 (Coordinator)
//
// Coordinates all sub-modules:
//   - TokenEstimator: fast character-based token estimation
//   - ToolResultPruner: compress old tool results
//   - DifferentialCache: skip redundant read_file results
//   - PromptChunker: load only necessary prompt modules
//   - ConversationOptimizer: token-aware history pruning
// ===========================================================================

// TokenOptimizer coordinates all sub-modules.
type TokenOptimizer struct {
	estimator     *TokenEstimator
	pruner        *ToolResultPruner
	diffCache     *DifferentialCache
	promptChunker *PromptChunker
	convoOpt      *ConversationOptimizer
	prefixCache   *PrefixCache
}

// NewTokenOptimizer creates a new TokenOptimizer with all sub-modules.
func NewTokenOptimizer(promptsDir string) *TokenOptimizer {
	return &TokenOptimizer{
		estimator:     &TokenEstimator{},
		pruner:        NewToolResultPruner(),
		diffCache:     NewDifferentialCache(2 * time.Minute),
		promptChunker: NewPromptChunker(promptsDir),
		convoOpt:      NewConversationOptimizer(),
		prefixCache:   NewPrefixCache(100, 5*time.Minute),
	}
}

// OptimizeConversation prunes old tool results.
func (to *TokenOptimizer) OptimizeConversation(
	conversation []map[string]interface{},
	maxTokens int,
) []map[string]interface{} {
	return to.convoOpt.OptimizeConversation(conversation, maxTokens)
}

// OptimizeToolResultForHistory compresses a tool result before adding to history.
func (to *TokenOptimizer) OptimizeToolResultForHistory(
	content string,
	toolName string,
	resultLen int,
) string {
	return to.convoOpt.OptimizeToolResultForHistory(content, toolName, resultLen)
}

// RecordToolResult caches a tool result.
func (to *TokenOptimizer) RecordToolResult(sessionID, toolName, content string, args map[string]interface{}) {
	to.convoOpt.RecordToolResult(sessionID, toolName, content, args)
}

// CheckDifferential returns cached content if file hasn't changed.
func (to *TokenOptimizer) CheckDifferential(sessionID, path string) (string, bool) {
	return to.convoOpt.CheckDifferential(sessionID, path)
}

// BuildModePrompt constructs a prompt with only necessary modules.
func (to *TokenOptimizer) BuildModePrompt(mode string) string {
	return to.promptChunker.BuildModePrompt(mode)
}

// InvalidatePromptCache clears the prompt cache.
func (to *TokenOptimizer) InvalidatePromptCache() {
	to.promptChunker.InvalidateCache()
}

// EstimateTokens returns estimated token count.
func (to *TokenOptimizer) EstimateTokens(text string) int {
	return to.estimator.EstimateTokens(text)
}

// EstimateConversationTokens estimates total tokens in conversation.
func (to *TokenOptimizer) EstimateConversationTokens(messages []map[string]interface{}) int {
	return to.estimator.EstimateConversationTokens(messages)
}

// GetStats returns all optimization statistics.
func (to *TokenOptimizer) GetStats() map[string]interface{} {
	return to.convoOpt.GetStats()
}

// ParseToolArgs parses JSON tool arguments.
func ParseToolArgs(argsJSON string) map[string]interface{} {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
		return args
	}
	return nil
}
