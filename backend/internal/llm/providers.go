package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Provider 定义 LLM 提供商
type Provider struct {
	Name        string   `json:"name"`
	ID          string   `json:"id"`
	Endpoint    string   `json:"endpoint"`
	Models      []Model  `json:"models"`
	RequiresKey bool     `json:"requires_key"`
	IsFree      bool     `json:"is_free"`
	Tier        string   `json:"tier"` // "free", "paid", "subscription"
}

// Model 定义可用模型
type Model struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Provider       string  `json:"provider"`
	MaxTokens      int     `json:"max_tokens"`
	SupportsStream bool    `json:"supports_stream"`
	PriceInput     float64 `json:"price_input_per_m"`  // USD per 1M tokens
	PriceOutput    float64 `json:"price_output_per_m"` // USD per 1M tokens
}

// RemoteModel 从远程 API 获取的模型信息
type RemoteModel struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

// GetProviders 返回所有支持的提供商
func GetProviders() []Provider {
	return []Provider{
		// === OpenCode Zen（合并免费+付费）===
		{
			Name: "OpenCode Zen", ID: "opencode-zen",
			Endpoint: "https://opencode.ai/zen/v1/chat/completions",
			RequiresKey: false, IsFree: true, Tier: "free",
			Models: []Model{
				// — 免费模型 (price=0) —
				{ID: "big-pickle", Name: "Big Pickle", Provider: "opencode-zen", MaxTokens: 16384, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "deepseek-v4-flash-free", Name: "DeepSeek V4 Flash Free", Provider: "opencode-zen", MaxTokens: 16384, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "mimo-v2.5-free", Name: "MiMo V2.5 Free", Provider: "opencode-zen", MaxTokens: 16384, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "nemotron-3-ultra-free", Name: "Nemotron 3 Ultra Free", Provider: "opencode-zen", MaxTokens: 16384, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "north-mini-code-free", Name: "North Mini Code Free", Provider: "opencode-zen", MaxTokens: 16384, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "laguna-s-2.1-free", Name: "Laguna S 2.1 Free", Provider: "opencode-zen", MaxTokens: 16384, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				// — GPT 系列 —
				{ID: "gpt-5.6-sol", Name: "GPT 5.6 Sol", Provider: "opencode-zen", MaxTokens: 1100000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 30.0},
				{ID: "gpt-5.6-terra", Name: "GPT 5.6 Terra", Provider: "opencode-zen", MaxTokens: 128000, SupportsStream: true, PriceInput: 2.5, PriceOutput: 15.0},
				{ID: "gpt-5.6-luna", Name: "GPT 5.6 Luna", Provider: "opencode-zen", MaxTokens: 128000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 6.0},
				{ID: "gpt-5.5", Name: "GPT 5.5", Provider: "opencode-zen", MaxTokens: 128000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 30.0},
				{ID: "gpt-5.5-pro", Name: "GPT 5.5 Pro", Provider: "opencode-zen", MaxTokens: 128000, SupportsStream: true, PriceInput: 30.0, PriceOutput: 180.0},
				{ID: "gpt-5.4-mini", Name: "GPT 5.4 Mini", Provider: "opencode-zen", MaxTokens: 400000, SupportsStream: true, PriceInput: 0.75, PriceOutput: 5.0},
				{ID: "gpt-5.4-nano", Name: "GPT 5.4 Nano", Provider: "opencode-zen", MaxTokens: 400000, SupportsStream: true, PriceInput: 0.20, PriceOutput: 1.25},
				{ID: "gpt-5.3-codex", Name: "GPT 5.3 Codex", Provider: "opencode-zen", MaxTokens: 400000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 9.0},
				{ID: "gpt-5.1-codex-mini", Name: "GPT 5.1 Codex Mini", Provider: "opencode-zen", MaxTokens: 400000, SupportsStream: true, PriceInput: 0.25, PriceOutput: 2.0},
				// — Claude 系列 —
				{ID: "claude-fable-5", Name: "Claude Fable 5", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 10.0, PriceOutput: 50.0},
				{ID: "claude-opus-4-8", Name: "Claude Opus 4.8", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 25.0},
				{ID: "claude-opus-4-1", Name: "Claude Opus 4.1", Provider: "opencode-zen", MaxTokens: 200000, SupportsStream: true, PriceInput: 15.0, PriceOutput: 75.0},
				{ID: "claude-sonnet-5", Name: "Claude Sonnet 5", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 10.0},
				{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 3.0, PriceOutput: 15.0},
				{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Provider: "opencode-zen", MaxTokens: 200000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 5.0},
				// — Gemini 系列 —
				{ID: "gemini-3.5-flash", Name: "Gemini 3.5 Flash", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 1.5, PriceOutput: 9.0},
				{ID: "gemini-3.1-pro", Name: "Gemini 3.1 Pro", Provider: "opencode-zen", MaxTokens: 2000000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 12.0},
				{ID: "gemini-3-flash", Name: "Gemini 3 Flash", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0.50, PriceOutput: 3.0},
				// — DeepSeek —
				{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0.14, PriceOutput: 0.28},
				{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 4.0},
				// — Kimi —
				{ID: "kimi-k2.7-code", Name: "Kimi K2.7 Code", Provider: "opencode-zen", MaxTokens: 262000, SupportsStream: true, PriceInput: 0.95, PriceOutput: 4.0},
				{ID: "kimi-k2.6", Name: "Kimi K2.6", Provider: "opencode-zen", MaxTokens: 262000, SupportsStream: true, PriceInput: 0.95, PriceOutput: 4.0},
				// — Qwen —
				{ID: "qwen3.7-plus", Name: "Qwen3.7 Plus", Provider: "opencode-zen", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.6, PriceOutput: 3.6},
				// — GLM —
				{ID: "glm-5.2", Name: "GLM 5.2", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 4.0},
				// — Grok —
				{ID: "grok-4.5", Name: "Grok 4.5", Provider: "opencode-zen", MaxTokens: 500000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 6.0},
				// — MiniMax —
				{ID: "minimax-m3", Name: "MiniMax M3", Provider: "opencode-zen", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0.30, PriceOutput: 1.20},
				{ID: "minimax-m2.7", Name: "MiniMax M2.7", Provider: "opencode-zen", MaxTokens: 200000, SupportsStream: true, PriceInput: 0.30, PriceOutput: 1.20},
			},
		},
		// === OpenCode Go (订阅制) ===
		{
			Name: "OpenCode Go", ID: "opencode-go",
			Endpoint: "https://opencode.ai/zen/go/v1/chat/completions",
			RequiresKey: false, IsFree: false, Tier: "subscription",
			Models: []Model{
				// — MiMo 系列 —
				{ID: "mimo-v2.5", Name: "MiMo V2.5", Provider: "opencode-go", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "mimo-v2.5-pro", Name: "MiMo V2.5 Pro", Provider: "opencode-go", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				// — DeepSeek 系列 —
				{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Provider: "opencode-go", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", Provider: "opencode-go", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				// — Kimi 系列 —
				{ID: "kimi-k3", Name: "Kimi K3", Provider: "opencode-go", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "kimi-k2.7-code", Name: "Kimi K2.7 Code", Provider: "opencode-go", MaxTokens: 262000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "kimi-k2.6", Name: "Kimi K2.6", Provider: "opencode-go", MaxTokens: 262000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				// — Qwen 系列 —
				{ID: "qwen3.7-plus", Name: "Qwen3.7 Plus", Provider: "opencode-go", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				// — GLM 系列 —
				{ID: "glm-5.2", Name: "GLM 5.2", Provider: "opencode-go", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "glm-5.1", Name: "GLM 5.1", Provider: "opencode-go", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				// — Grok 系列 —
				{ID: "grok-4.5", Name: "Grok 4.5", Provider: "opencode-go", MaxTokens: 500000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				// — MiniMax —
				{ID: "minimax-m3", Name: "MiniMax M3", Provider: "opencode-go", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "minimax-m2.7", Name: "MiniMax M2.7", Provider: "opencode-go", MaxTokens: 200000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				// — Hy3 —
				{ID: "hy3", Name: "Hy3", Provider: "opencode-go", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
			},
		},
		// === OpenAI ===
		{
			Name: "OpenAI", ID: "openai",
			Endpoint: "https://api.openai.com/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "gpt-5.6-sol", Name: "GPT 5.6 Sol", Provider: "openai", MaxTokens: 1000000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 30.0},
				{ID: "gpt-5.6-luna", Name: "GPT 5.6 Luna", Provider: "openai", MaxTokens: 1000000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 6.0},
				{ID: "gpt-5.5", Name: "GPT 5.5", Provider: "openai", MaxTokens: 1000000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 30.0},
				{ID: "gpt-5.4-mini", Name: "GPT 5.4 Mini", Provider: "openai", MaxTokens: 400000, SupportsStream: true, PriceInput: 0.75, PriceOutput: 4.50},
				{ID: "gpt-5.4-nano", Name: "GPT 5.4 Nano", Provider: "openai", MaxTokens: 400000, SupportsStream: true, PriceInput: 0.20, PriceOutput: 1.25},
				{ID: "o3", Name: "o3", Provider: "openai", MaxTokens: 200000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 8.0},
				{ID: "o4-mini", Name: "o4-mini", Provider: "openai", MaxTokens: 200000, SupportsStream: true, PriceInput: 1.1, PriceOutput: 4.4},
			},
		},
		// === Anthropic (Claude) ===
		{
			Name: "Anthropic", ID: "anthropic",
			Endpoint: "https://api.anthropic.com/v1/messages",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "claude-opus-4-8", Name: "Claude Opus 4.8", Provider: "anthropic", MaxTokens: 1000000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 25.0},
				{ID: "claude-opus-4-7", Name: "Claude Opus 4.7", Provider: "anthropic", MaxTokens: 1000000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 25.0},
				{ID: "claude-sonnet-5", Name: "Claude Sonnet 5", Provider: "anthropic", MaxTokens: 1000000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 10.0},
				{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", Provider: "anthropic", MaxTokens: 1000000, SupportsStream: true, PriceInput: 3.0, PriceOutput: 15.0},
				{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: "anthropic", MaxTokens: 1000000, SupportsStream: true, PriceInput: 3.0, PriceOutput: 15.0},
				{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Provider: "anthropic", MaxTokens: 200000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 5.0},
				{ID: "claude-fable-5", Name: "Claude Fable 5", Provider: "anthropic", MaxTokens: 1000000, SupportsStream: true, PriceInput: 10.0, PriceOutput: 50.0},
			},
		},
		// === Google (Gemini) ===
		{
			Name: "Google", ID: "google",
			Endpoint: "https://generativelanguage.googleapis.com/v1beta/models",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "gemini-3.5-flash", Name: "Gemini 3.5 Flash", Provider: "google", MaxTokens: 1000000, SupportsStream: true, PriceInput: 1.5, PriceOutput: 9.0},
				{ID: "gemini-3.1-pro", Name: "Gemini 3.1 Pro", Provider: "google", MaxTokens: 2000000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 12.0},
				{ID: "gemini-3-flash", Name: "Gemini 3 Flash", Provider: "google", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0.50, PriceOutput: 3.0},
				{ID: "gemini-3.1-flash-lite", Name: "Gemini 3.1 Flash Lite", Provider: "google", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0.25, PriceOutput: 1.50},
				{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: "google", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0.15, PriceOutput: 0.60},
				{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: "google", MaxTokens: 1000000, SupportsStream: true, PriceInput: 1.25, PriceOutput: 10.0},
			},
		},
		// === DeepSeek ===
		{
			Name: "DeepSeek", ID: "deepseek",
			Endpoint: "https://api.deepseek.com/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Provider: "deepseek", MaxTokens: 1000000, SupportsStream: true, PriceInput: 0.14, PriceOutput: 0.28},
				{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", Provider: "deepseek", MaxTokens: 1000000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 4.0},
				{ID: "deepseek-r1", Name: "DeepSeek R1", Provider: "deepseek", MaxTokens: 64000, SupportsStream: true, PriceInput: 0.55, PriceOutput: 2.19},
			},
		},
		// === xAI / Grok ===
		{
			Name: "xAI (Grok)", ID: "xai",
			Endpoint: "https://api.x.ai/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "grok-4.5", Name: "Grok 4.5", Provider: "xai", MaxTokens: 500000, SupportsStream: true, PriceInput: 2.0, PriceOutput: 6.0},
				{ID: "grok-4.3", Name: "Grok 4.3", Provider: "xai", MaxTokens: 1000000, SupportsStream: true, PriceInput: 1.25, PriceOutput: 2.50},
				{ID: "grok-4.1-fast", Name: "Grok 4.1 Fast", Provider: "xai", MaxTokens: 2000000, SupportsStream: true, PriceInput: 0.20, PriceOutput: 0.50},
			},
		},
		// === Ollama (本地) ===
		{
			Name: "Ollama (本地)", ID: "ollama",
			Endpoint: "http://localhost:11434/v1/chat/completions",
			RequiresKey: false, IsFree: true, Tier: "free",
			Models: []Model{
				{ID: "qwen3-coder:7b", Name: "Qwen3 Coder 7B", Provider: "ollama", MaxTokens: 32000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "qwen3:8b", Name: "Qwen3 8B", Provider: "ollama", MaxTokens: 32000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "deepseek-v3:latest", Name: "DeepSeek V3", Provider: "ollama", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "llama4-scout:latest", Name: "Llama 4 Scout", Provider: "ollama", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "codellama:13b", Name: "CodeLlama 13B", Provider: "ollama", MaxTokens: 16000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
			},
		},
		// === 阿里云百炼 (Aliyun Bailian) ===
		{
			Name: "阿里云百炼", ID: "aliyun-bailian",
			Endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "qwen3.7-max", Name: "Qwen3.7 Max", Provider: "aliyun-bailian", MaxTokens: 128000, SupportsStream: true, PriceInput: 1.66, PriceOutput: 5.0},
				{ID: "qwen3.7-plus", Name: "Qwen3.7 Plus", Provider: "aliyun-bailian", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.80, PriceOutput: 2.40},
				{ID: "qwen3-max", Name: "Qwen3 Max", Provider: "aliyun-bailian", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.35, PriceOutput: 1.38},
				{ID: "qwen3-plus", Name: "Qwen3 Plus", Provider: "aliyun-bailian", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.11, PriceOutput: 0.28},
				{ID: "qwen-turbo", Name: "Qwen Turbo", Provider: "aliyun-bailian", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.04, PriceOutput: 0.12},
			},
		},
		// === 阿里云灵积 (Aliyun Lingji) ===
		{
			Name: "阿里云灵积", ID: "aliyun-lingji",
			Endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "qwen3.7-max", Name: "Qwen3.7 Max", Provider: "aliyun-lingji", MaxTokens: 128000, SupportsStream: true, PriceInput: 1.66, PriceOutput: 5.0},
				{ID: "qwen3.7-plus", Name: "Qwen3.7 Plus", Provider: "aliyun-lingji", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.80, PriceOutput: 2.40},
				{ID: "qwen3-max", Name: "Qwen3 Max", Provider: "aliyun-lingji", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.35, PriceOutput: 1.38},
				{ID: "qwen-turbo", Name: "Qwen Turbo", Provider: "aliyun-lingji", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.04, PriceOutput: 0.12},
			},
		},
		// === 小米 MiMo (Xiaomi MiMo) ===
		{
			Name: "小米 MiMo", ID: "xiaomi-mimo",
			Endpoint: "https://api.mimo.xiaomi.com/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "MiMo-v2.5-Pro", Name: "MiMo V2.5 Pro", Provider: "xiaomi-mimo", MaxTokens: 128000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 3.0},
				{ID: "MiMo-7B-RL", Name: "MiMo 7B RL", Provider: "xiaomi-mimo", MaxTokens: 16384, SupportsStream: true, PriceInput: 0.50, PriceOutput: 1.50},
			},
		},
		// === 智谱 AI (Zhipu AI) ===
		{
			Name: "智谱 AI", ID: "zhipu",
			Endpoint: "https://open.bigmodel.cn/api/paas/v4/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "glm-5", Name: "GLM-5", Provider: "zhipu", MaxTokens: 200000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 10.0},
				{ID: "glm-4.7", Name: "GLM-4.7", Provider: "zhipu", MaxTokens: 200000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 10.0},
				{ID: "glm-4-6", Name: "GLM-4.6", Provider: "zhipu", MaxTokens: 200000, SupportsStream: true, PriceInput: 5.0, PriceOutput: 10.0},
				{ID: "glm-4-flashx", Name: "GLM-4 FlashX", Provider: "zhipu", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.10, PriceOutput: 0.10},
				{ID: "glm-4-air", Name: "GLM-4 Air", Provider: "zhipu", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.50, PriceOutput: 0.25},
			},
		},
		// === 月之暗面 (Moonshot / Kimi) ===
		{
			Name: "月之暗面", ID: "moonshot",
			Endpoint: "https://api.moonshot.cn/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "kimi-k2.7-code", Name: "Kimi K2.7 Code", Provider: "moonshot", MaxTokens: 262000, SupportsStream: true, PriceInput: 0.95, PriceOutput: 4.0},
				{ID: "kimi-k2.6", Name: "Kimi K2.6", Provider: "moonshot", MaxTokens: 262000, SupportsStream: true, PriceInput: 0.95, PriceOutput: 4.0},
				{ID: "kimi-k2.5", Name: "Kimi K2.5", Provider: "moonshot", MaxTokens: 262000, SupportsStream: true, PriceInput: 0.60, PriceOutput: 3.0},
				{ID: "moonshot-v1-128k", Name: "Moonshot V1 128K", Provider: "moonshot", MaxTokens: 128000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 3.0},
			},
		},
		// === 百川智能 (Baichuan) ===
		{
			Name: "百川智能", ID: "baichuan",
			Endpoint: "https://api.baichuan-ai.com/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "Baichuan-M3-Plus", Name: "Baichuan M3 Plus", Provider: "baichuan", MaxTokens: 192000, SupportsStream: true, PriceInput: 0.80, PriceOutput: 2.0},
				{ID: "Baichuan4", Name: "Baichuan4", Provider: "baichuan", MaxTokens: 32000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 3.0},
				{ID: "baichuan3-turbo", Name: "Baichuan3 Turbo", Provider: "baichuan", MaxTokens: 32000, SupportsStream: true, PriceInput: 0.50, PriceOutput: 1.50},
			},
		},
		// === 零一万物 (Lingyiwanwu / Yi) ===
		{
			Name: "零一万物", ID: "lingyiwanwu",
			Endpoint: "https://api.lingyiwanwu.com/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "yi-large", Name: "Yi Large", Provider: "lingyiwanwu", MaxTokens: 32000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 3.0},
				{ID: "yi-large-turbo", Name: "Yi Large Turbo", Provider: "lingyiwanwu", MaxTokens: 16000, SupportsStream: true, PriceInput: 0.40, PriceOutput: 1.0},
				{ID: "yi-medium", Name: "Yi Medium", Provider: "lingyiwanwu", MaxTokens: 32000, SupportsStream: true, PriceInput: 0.50, PriceOutput: 1.50},
			},
		},
		// === 讯飞星火 (iFlytek Spark) ===
		{
			Name: "讯飞星火", ID: "xfyun",
			Endpoint: "https://spark-api-open.xf-yun.com/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "spark-4.0-ultra", Name: "Spark 4.0 Ultra", Provider: "xfyun", MaxTokens: 128000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 4.0},
				{ID: "spark-4.0", Name: "Spark 4.0", Provider: "xfyun", MaxTokens: 32000, SupportsStream: true, PriceInput: 0.50, PriceOutput: 2.0},
				{ID: "spark-max", Name: "Spark Max", Provider: "xfyun", MaxTokens: 32000, SupportsStream: true, PriceInput: 0.30, PriceOutput: 1.0},
			},
		},
		// === 硅基流动 (SiliconFlow) ===
		{
			Name: "硅基流动", ID: "siliconflow",
			Endpoint: "https://api.siliconflow.cn/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "Qwen/Qwen3-8B", Name: "Qwen3 8B", Provider: "siliconflow", MaxTokens: 32000, SupportsStream: true, PriceInput: 0.20, PriceOutput: 0.60},
				{ID: "DeepSeek-V3", Name: "DeepSeek V3", Provider: "siliconflow", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.50, PriceOutput: 1.50},
				{ID: "Pro/deepseek-ai/DeepSeek-V3", Name: "DeepSeek V3 (Pro)", Provider: "siliconflow", MaxTokens: 128000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 3.0},
			},
		},
		// === Groq ===
		{
			Name: "Groq", ID: "groq",
			Endpoint: "https://api.groq.com/openai/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "openai/gpt-oss-120b", Name: "GPT-OSS 120B", Provider: "groq", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "qwen/qwen3.6-27b", Name: "Qwen3.6 27B", Provider: "groq", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "meta-llama/llama-4-scout-17b-16e-instruct", Name: "Llama 4 Scout 17B", Provider: "groq", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "meta-llama/llama-4-maverick-17b-128e-instruct", Name: "Llama 4 Maverick 17B", Provider: "groq", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
				{ID: "openai/gpt-oss-20b", Name: "GPT-OSS 20B", Provider: "groq", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
			},
		},
		// === Together ===
		{
			Name: "Together", ID: "together",
			Endpoint: "https://api.together.xyz/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "MiniMax-M3", Name: "MiniMax M3", Provider: "together", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.69, PriceOutput: 2.19},
				{ID: "THUDM/GLM-5.2-32B-A12B", Name: "GLM-5.2 32B", Provider: "together", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.14, PriceOutput: 0.56},
				{ID: "Qwen/Qwen3.6-235B-A22B-Instruct-2507", Name: "Qwen3.6 235B", Provider: "together", MaxTokens: 128000, SupportsStream: true, PriceInput: 1.0, PriceOutput: 3.0},
				{ID: "meta-llama/Llama-4-Maverick-17B-128E-Instruct", Name: "Llama 4 Maverick", Provider: "together", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.20, PriceOutput: 0.60},
				{ID: "deepseek-ai/DeepSeek-R1", Name: "DeepSeek R1", Provider: "together", MaxTokens: 128000, SupportsStream: true, PriceInput: 0.55, PriceOutput: 2.19},
			},
		},
		// === OpenRouter ===
		{
			Name: "OpenRouter", ID: "openrouter",
			Endpoint: "https://openrouter.ai/api/v1/chat/completions",
			RequiresKey: true, IsFree: false, Tier: "paid",
			Models: []Model{
				{ID: "auto", Name: "Auto (根据路由选择)", Provider: "openrouter", MaxTokens: 128000, SupportsStream: true, PriceInput: 0, PriceOutput: 0},
			},
		},
	}
}

// GetMergedProviders 返回所有提供商，合并用户配置（endpoint/api_key/模型 覆盖）并追加自定义提供商
func GetMergedProviders(userConfigs map[string]struct{ Endpoint, APIKey, ModelsJSON string }, customProviders []Provider) []Provider {
	providers := GetProviders()
	for i := range providers {
		if cfg, ok := userConfigs[providers[i].ID]; ok {
			if cfg.Endpoint != "" {
				providers[i].Endpoint = cfg.Endpoint
			}
			// Merge user-added models
			if cfg.ModelsJSON != "" {
				var extraModels []struct {
					ID          string  `json:"id"`
					Name        string  `json:"name"`
					MaxTokens   int     `json:"max_tokens"`
					PriceInput  float64 `json:"price_input_per_m"`
					PriceOutput float64 `json:"price_output_per_m"`
				}
				if json.Unmarshal([]byte(cfg.ModelsJSON), &extraModels) == nil {
					for _, em := range extraModels {
						m := Model{
							ID:             em.ID,
							Name:           em.Name,
							Provider:       providers[i].ID,
							MaxTokens:      em.MaxTokens,
							SupportsStream: true,
							PriceInput:     em.PriceInput,
							PriceOutput:    em.PriceOutput,
						}
						if m.MaxTokens <= 0 {
							m.MaxTokens = 8192 // safe default for unknown models
						}
						providers[i].Models = append(providers[i].Models, m)
					}
				}
			}
		}
	}
	providers = append(providers, customProviders...)
	return providers
}

// FindProvider 根据 ID 查找提供商
func FindProvider(providerID string) *Provider {
	providers := GetProviders()
	for _, p := range providers {
		if p.ID == providerID {
			pCopy := p
			return &pCopy
		}
	}
	return nil
}

// FindModel 查找指定提供商中的特定模型
func FindModel(providerID, modelID string) *Model {
	providers := GetProviders()
	for _, p := range providers {
		if p.ID == providerID {
			for _, m := range p.Models {
				if m.ID == modelID {
					mCopy := m
					return &mCopy
				}
			}
		}
	}
	return nil
}

// FetchRemoteModels 从 OpenCode Zen API 获取最新可用模型列表
func FetchRemoteModels() ([]RemoteModel, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://opencode.ai/zen/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []RemoteModel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Data, nil
}
