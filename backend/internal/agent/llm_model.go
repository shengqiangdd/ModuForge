package agent

import (
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════════════════
// Model Tier & Max Tokens
// ═══════════════════════════════════════════════════════════════════

// defaultModelMaxTokens maps known model name substrings to their default max output tokens.
// Used when the user hasn't explicitly configured max_tokens for a model.
var defaultModelMaxTokens = map[string]int{
	// OpenAI
	"o1":          32768,
	"o3":          32768,
	"o4-mini":     32768,
	"gpt-4o":      16384,
	"gpt-4-turbo": 4096,
	"gpt-4":       8192,
	"gpt-3.5":     4096,
	// Anthropic
	"claude-3.5-sonnet": 8192,
	"claude-3-opus":     4096,
	"claude-3-haiku":    8192,
	"claude-4":          16384,
	"claude":            8192,
	// Google
	"gemini-2.5-pro":   65536,
	"gemini-2.5-flash": 65536,
	"gemini-2.0":       8192,
	"gemini-1.5-pro":   8192,
	"gemini":           8192,
	// DeepSeek
	"deepseek-v3":    8192,
	"deepseek-v2.5":  8192,
	"deepseek-coder": 8192,
	"deepseek":       8192,
	// Qwen
	"qwen-max":   8192,
	"qwen-plus":  8192,
	"qwen-turbo": 8192,
	"qwen":       8192,
	// Meta
	"llama-3.1-405b": 4096,
	"llama-3.1-70b":  4096,
	"llama-3.1-8b":   4096,
	"llama":          4096,
	// Mistral
	"mistral-large":  8192,
	"mistral-medium": 8192,
	"mistral":        8192,
	// Default for unknown models
	"_default": 8192,
}

// resolveModelMaxTokens returns the max output tokens for a model name.
// It checks the model name against known patterns (case-insensitive substring match).
func resolveModelMaxTokens(modelName string) int {
	lower := strings.ToLower(modelName)
	// Longest match first (more specific patterns)
	bestLen := 0
	bestVal := defaultModelMaxTokens["_default"]
	for pattern, val := range defaultModelMaxTokens {
		if pattern == "_default" {
			continue
		}
		if strings.Contains(lower, pattern) && len(pattern) > bestLen {
			bestLen = len(pattern)
			bestVal = val
		}
	}
	return bestVal
}

// Model tier determines context handling aggressiveness.
// Tier 0 (free/weak): small context, aggressive compaction, smart truncation
// Tier 1 (mid): moderate context, standard compaction
// Tier 2 (strong/paid): large context, lazy compaction, no truncation
type ModelTier int

const (
	TierFree   ModelTier = 0 // free models, small context (deepseek-v4-flash-free, etc.)
	TierMid    ModelTier = 1 // mid-tier models (deepseek-v3, qwen-turbo, etc.)
	TierStrong ModelTier = 2 // strong paid models (gpt-4o, claude, gemini-pro, etc.)
)

// modelTierCache caches tier resolution results (model names don't change at runtime).
var modelTierCache sync.Map

func resolveModelTier(modelName string) ModelTier {
	// Fast path: cached
	if cached, ok := modelTierCache.Load(modelName); ok {
		return cached.(ModelTier)
	}
	// Slow path: compute and cache
	lower := strings.ToLower(modelName)
	// Free/weak models — aggressive limits
	freePatterns := []string{"free", "mini", "flash-free", "lite", "nano"}
	for _, p := range freePatterns {
		if strings.Contains(lower, p) {
			modelTierCache.Store(modelName, TierFree)
			return TierFree
		}
	}
	// Strong models — generous limits
	strongPatterns := []string{"gpt-4o", "gpt-4-turbo", "claude-3.5", "claude-4", "claude-3-opus",
		"gemini-2.5-pro", "gemini-1.5-pro", "o1", "o3", "deepseek-r1", "qwen-max"}
	for _, p := range strongPatterns {
		if strings.Contains(lower, p) {
			modelTierCache.Store(modelName, TierStrong)
			return TierStrong
		}
	}
	// Everything else is mid-tier
	modelTierCache.Store(modelName, TierMid)
	return TierMid
}

// compactionThresholdForTier returns the context compaction threshold for a model tier.
// For free models with 16K context, we must be much more aggressive to leave room for
// system prompt (~800) + tool definitions (~1840) + output (~4096) = ~6700 tokens overhead.
func compactionThresholdForTier(tier ModelTier) int {
	switch tier {
	case TierFree:
		return 8000 // very aggressive: 16K context - 6700 overhead = ~9K for conversation
	case TierMid:
		return 30000 // moderate
	case TierStrong:
		return 100000 // generous: let strong models use their full context
	default:
		return 30000
	}
}

// maxResultLenForTier returns the tool result size limit for a model tier.
func maxResultLenForTier(tier ModelTier) int {
	switch tier {
	case TierFree:
		return 12000 // small: minimize context bloat
	case TierMid:
		return 24000 // moderate
	case TierStrong:
		return 48000 // generous: let strong models read large files
	default:
		return 24000
	}
}
