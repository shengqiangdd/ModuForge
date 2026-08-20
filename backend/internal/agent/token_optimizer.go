package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ===========================================================================
// TokenOptimizer — 综合 Token 优化引擎
//
// 目标：省 Token + 提升缓存命中 + 高效编码
//
// 五个子模块：
//   1. TokenEstimator      — 字符级快速 token 估算（无需 tokenizer，100x faster）
//   2. ToolResultPruner    — 旧工具结果压缩为摘要，保留关键信息
//   3. DifferentialCache   — 文件未变时跳过重复 read_file 结果
//   4. PromptChunker       — 按模式只加载必要指令，减少系统提示 token
//   5. ConversationOptimizer — token 感知的历史裁剪
//
// 整合点：
//   - Run() 主循环: OptimizeConversation() 裁剪历史
//   - Tool 执行后: RecordToolResult() 缓存
//   - read_file 前: CheckDifferential() 跳过重复读
//   - buildSystemPromptForMode: PromptChunker 按模式加载
// ===========================================================================

// ----- TokenEstimator -----

// TokenEstimator provides fast character-based token estimation.
// English: ~4 chars/token, Code: ~3 chars/token, Chinese: ~1.5 chars/token.
// Accuracy: ±15% vs tiktoken, but 100x faster (no network/disk overhead).
type TokenEstimator struct {
	cache sync.Map // hash -> estimated tokens
}

// EstimateTokens returns an estimated token count for the given text.

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
