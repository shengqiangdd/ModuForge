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


// ----- PromptChunker -----

// PromptChunker loads only the necessary instruction modules per mode.
type PromptChunker struct {
	promptsDir string
	cache      map[string]string // mode -> concatenated prompt
	mu         sync.RWMutex
}

// NewPromptChunker creates a chunker.
func NewPromptChunker(promptsDir string) *PromptChunker {
	return &PromptChunker{
		promptsDir: promptsDir,
		cache:      make(map[string]string),
	}
}

// ModeModules defines which prompt modules each mode needs.
// Instead of loading all 11 files (~15KB), load only 3-5 needed for the mode.
var ModeModules = map[string][]string{
	"code": {"base", "agent", "tools", "act", "errors"},
	"plan": {"base", "agent", "tools", "plan", "errors"},
	"free": {"base", "chat"},
	"chat": {"base", "chat"},
	"edit": {"base", "agent", "tools", "act", "errors"},
}

// BuildModePrompt constructs a prompt with only the modules needed for the mode.
func (pc *PromptChunker) BuildModePrompt(mode string) string {
	pc.mu.RLock()
	if cached, ok := pc.cache[mode]; ok {
		pc.mu.RUnlock()
		return cached
	}
	pc.mu.RUnlock()

	modules, ok := ModeModules[mode]
	if !ok {
		modules = ModeModules["code"]
	}

	var sb strings.Builder
	for _, mod := range modules {
		path := filepath.Join(pc.promptsDir, mod+".md")
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[PromptChunker] Warning: failed to load %s.md: %v", mod, err)
			continue
		}
		sb.WriteString(string(content))
		sb.WriteString("\n\n")
	}

	result := sb.String()

	pc.mu.Lock()
	pc.cache[mode] = result
	pc.mu.Unlock()

	return result
}

// InvalidateCache clears the cache.
func (pc *PromptChunker) InvalidateCache() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.cache = make(map[string]string)
}

