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


// ----- ConversationOptimizer -----

// ConversationOptimizer performs token-aware conversation pruning.
type ConversationOptimizer struct {
	estimator *TokenEstimator
	pruner    *ToolResultPruner
	diffCache *DifferentialCache

	stats struct {
		mu               sync.Mutex
		tokensSaved      int64
		resultsPruned    int64
		differentialHits int64
	}
}

// NewConversationOptimizer creates a new optimizer.
func NewConversationOptimizer() *ConversationOptimizer {
	return &ConversationOptimizer{
		estimator: &TokenEstimator{},
		pruner:    NewToolResultPruner(),
		diffCache: NewDifferentialCache(2 * time.Minute),
	}
}

// OptimizeConversation prunes old tool results in the conversation to save tokens.
// Keeps the last N messages in full and prunes older tool results.
func (co *ConversationOptimizer) OptimizeConversation(
	conversation []map[string]interface{},
	maxTokens int,
) []map[string]interface{} {
	if len(conversation) <= 4 {
		return conversation
	}

	// Estimate current token count
	currentTokens := co.estimator.EstimateConversationTokens(conversation)
	if currentTokens <= maxTokens {
		return conversation
	}

	// Strategy: Keep last 6 messages in full, prune older tool results
	result := make([]map[string]interface{}, len(conversation))
	copy(result, conversation)

	preserveCount := 6
	if preserveCount > len(result) {
		preserveCount = len(result)
	}

	pruned := 0
	for i := 0; i < len(result)-preserveCount; i++ {
		msg := result[i]
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		// Only prune tool results
		if role == "tool" || (role == "user" && strings.HasPrefix(content, "Result of ")) {
			toolName := extractToolNameFromContent(content)
			prunedContent, wasPruned := co.pruner.PruneResult(content, toolName)
			if wasPruned {
				result[i] = map[string]interface{}{
					"role":    role,
					"content": prunedContent,
				}
				pruned++
				saved := len(content) - len(prunedContent)
				co.stats.mu.Lock()
				co.stats.tokensSaved += int64(saved / 4)
				co.stats.resultsPruned++
				co.stats.mu.Unlock()
			}
		}
	}

	if pruned > 0 {
		newTokens := co.estimator.EstimateConversationTokens(result)
		log.Printf("[TokenOptimizer] Pruned %d tool results, saved ~%d tokens (%d -> %d)",
			pruned, currentTokens-newTokens, currentTokens, newTokens)
	}

	return result
}

// OptimizeToolResultForHistory compresses a tool result before adding to history.
func (co *ConversationOptimizer) OptimizeToolResultForHistory(
	content string,
	toolName string,
	resultLen int,
) string {
	if len(content) <= resultLen/2 {
		return content
	}

	// For read_file: summarize with head + line count
	if toolName == "read_file" {
		lines := strings.Split(content, "\n")
		if len(lines) > 30 {
			head := strings.Join(lines[:30], "\n")
			return head + fmt.Sprintf("\n... [%d lines total]", len(lines))
		}
	}

	// For grep: keep first 20 matches + count
	if toolName == "grep_search" {
		lines := strings.Split(content, "\n")
		if len(lines) > 20 {
			head := strings.Join(lines[:20], "\n")
			return head + fmt.Sprintf("\n... [%d matches total]", len(lines))
		}
	}

	// Generic: truncate at half budget
	half := resultLen / 2
	if len(content) > half {
		return content[:half] + fmt.Sprintf("\n... [truncated %d/%d chars]", len(content), half)
	}

	return content
}

// RecordToolResult caches a tool result for differential comparison.
func (co *ConversationOptimizer) RecordToolResult(sessionID, toolName, content string, args map[string]interface{}) {
	if toolName == "read_file" || toolName == "file_reader" {
		if path, ok := args["path"].(string); ok {
			co.diffCache.Store(sessionID, path, content)
		}
	}
	if toolName == "write_file" || toolName == "edit_file" {
		if path, ok := args["path"].(string); ok {
			co.diffCache.Invalidate(sessionID, path)
		}
	}
}

// CheckDifferential returns cached content if file hasn't changed.
func (co *ConversationOptimizer) CheckDifferential(sessionID, path string) (string, bool) {
	content, found := co.diffCache.CheckFile(sessionID, path)
	if found {
		co.stats.mu.Lock()
		co.stats.differentialHits++
		co.stats.mu.Unlock()
		log.Printf("[TokenOptimizer] Differential cache HIT for %s", path)
	}
	return content, found
}

// GetStats returns optimization statistics.
func (co *ConversationOptimizer) GetStats() map[string]interface{} {
	co.stats.mu.Lock()
	defer co.stats.mu.Unlock()
	return map[string]interface{}{
		"tokens_saved":      co.stats.tokensSaved,
		"results_pruned":    co.stats.resultsPruned,
		"differential_hits": co.stats.differentialHits,
	}
}

// extractToolNameFromContent attempts to extract the tool name from a result message.

// extractToolNameFromContent attempts to extract the tool name from a result message.
func extractToolNameFromContent(content string) string {
	re := regexp.MustCompile(`Result of (\w+):`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	re2 := regexp.MustCompile(`\[(\w+)\]`)
	matches2 := re2.FindStringSubmatch(content)
	if len(matches2) > 1 {
		return matches2[1]
	}
	return "unknown"
}
