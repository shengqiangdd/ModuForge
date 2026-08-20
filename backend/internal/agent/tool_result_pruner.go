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

func NewToolResultPruner() *ToolResultPruner {
	return &ToolResultPruner{
		threshold:      2000, // Prune results > 2KB
		keepHead:       1500, // Keep first 1.5KB of content
		preserveWrites: true, // Always keep write/edit results
	}
}

// PruneResult compresses a tool result if it's too large.
// Returns (pruned_content, was_pruned).
func (tp *ToolResultPruner) PruneResult(content string, toolName string) (string, bool) {
	// Never prune write/edit results (they show what changed)
	if tp.preserveWrites {
		if toolName == "write_file" || toolName == "edit_file" {
			return content, false
		}
		if toolName == "bash" && len(content) <= 3000 {
			return content, false
		}
		if toolName == "bash" && len(content) > 3000 {
			return tp.pruneBashResult(content), true
		}
	}

	if len(content) <= tp.threshold {
		return content, false
	}

	// For read_file results: keep head + summary
	if toolName == "read_file" || toolName == "file_reader" {
		return tp.pruneReadFileResult(content), true
	}

	// For grep/glob results: keep head + count
	if toolName == "grep_search" || toolName == "glob_search" {
		return tp.pruneSearchResult(content), true
	}

	// Generic pruning: keep head + truncation marker
	pruned := content[:tp.keepHead]
	pruned += fmt.Sprintf("\n... [truncated, %d chars total]", len(content))
	return pruned, true
}

// pruneReadFileResult keeps the first N lines + file metadata.
func (tp *ToolResultPruner) pruneReadFileResult(content string) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	keepLines := 40
	if keepLines > totalLines {
		keepLines = totalLines
	}

	pruned := strings.Join(lines[:keepLines], "\n")
	if totalLines > keepLines {
		pruned += fmt.Sprintf("\n... [file has %d lines total, showing first %d for context]", totalLines, keepLines)
	}
	return pruned
}

// pruneSearchResult keeps the first N matches + summary count.
func (tp *ToolResultPruner) pruneSearchResult(content string) string {
	lines := strings.Split(content, "\n")
	totalMatches := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			totalMatches++
		}
	}

	keepLines := 30
	if keepLines > len(lines) {
		keepLines = len(lines)
	}

	pruned := strings.Join(lines[:keepLines], "\n")
	if totalMatches > keepLines {
		pruned += fmt.Sprintf("\n... [%d total matches, showing first %d]", totalMatches, keepLines)
	}
	return pruned
}

// pruneBashResult keeps the command + last N chars of output.
func (tp *ToolResultPruner) pruneBashResult(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	cmd := lines[0]
	if len(lines) <= 30 {
		return content
	}

	lastLines := lines[len(lines)-30:]
	return cmd + "\n... [output truncated] ...\n" + strings.Join(lastLines, "\n")
}

// ----- DifferentialCache -----

// DifferentialCache caches file content hashes to skip redundant read_file calls.
type DifferentialCache struct {
	entries map[string]map[string]*diffCacheEntry // sessionID -> path -> entry
	mu      sync.RWMutex
	ttl     time.Duration
}

type diffCacheEntry struct {
	hash      string
	content   string
	timestamp time.Time
	fileSize  int64
}

// NewDifferentialCache creates a new differential cache.
