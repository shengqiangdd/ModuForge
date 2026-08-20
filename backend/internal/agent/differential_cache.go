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

func NewDifferentialCache(ttl time.Duration) *DifferentialCache {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &DifferentialCache{
		entries: make(map[string]map[string]*diffCacheEntry),
		ttl:     ttl,
	}
}

// CheckFile returns cached content if file hasn't changed, nil otherwise.
func (dc *DifferentialCache) CheckFile(sessionID, path string) (string, bool) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	sessions, ok := dc.entries[sessionID]
	if !ok {
		return "", false
	}
	entry, ok := sessions[path]
	if !ok {
		return "", false
	}

	// Check TTL
	if time.Since(entry.timestamp) > dc.ttl {
		return "", false
	}

	// Check if file has changed (size)
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	if info.Size() != entry.fileSize {
		return "", false
	}

	// Content hash check
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	if hashStr == entry.hash {
		return entry.content, true
	}

	return "", false
}

// Store caches file content for future comparison.
func (dc *DifferentialCache) Store(sessionID, path, content string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if _, ok := dc.entries[sessionID]; !ok {
		dc.entries[sessionID] = make(map[string]*diffCacheEntry)
	}

	info, err := os.Stat(path)
	var fileSize int64
	if err == nil {
		fileSize = info.Size()
	}

	hash := sha256.Sum256([]byte(content))
	dc.entries[sessionID][path] = &diffCacheEntry{
		hash:      hex.EncodeToString(hash[:]),
		content:   content,
		timestamp: time.Now(),
		fileSize:  fileSize,
	}
}

// Invalidate removes a file from cache (after write/edit).
func (dc *DifferentialCache) Invalidate(sessionID, path string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if sessions, ok := dc.entries[sessionID]; ok {
		delete(sessions, path)
	}
}

// Cleanup removes expired entries.
func (dc *DifferentialCache) Cleanup() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	for sid, sessions := range dc.entries {
		for path, entry := range sessions {
			if time.Since(entry.timestamp) > dc.ttl {
				delete(sessions, path)
			}
		}
		if len(sessions) == 0 {
			delete(dc.entries, sid)
		}
	}
}

// ----- PromptChunker -----

// PromptChunker loads only the necessary instruction modules per mode.
type PromptChunker struct {
	promptsDir string
	cache      map[string]string // mode -> concatenated prompt
	mu         sync.RWMutex
}

// NewPromptChunker creates a chunker.
