package config

import (
	"os"
	"strconv"
	"sync"
)

type Config struct {
	mu sync.RWMutex

	Port         string // 监听端口
	JWTSecret    string // JWT 签名密钥
	DatabasePath string // SQLite 数据库路径
	StoragePath  string // 文件存储路径
	S3Endpoint   string // S3-compatible storage endpoint (e.g. "seaweedfs:8333")
	S3AccessKey  string // S3 access key
	S3SecretKey  string // S3 secret key
	S3Bucket     string // S3 bucket name
	// Legacy single-provider config (backward compatible)
	LLMApiKey   string // LLM API Key
	LLMEndpoint string // LLM API 端点
	LLMModel    string // LLM 模型名称
	// Multi-provider config
	LLMProvider string // 当前使用的提供商 ID
	LLMModelID  string // 当前使用的模型 ID
	// Provider API Keys
	OpenAIApiKey     string  // OpenAI API key
	AnthropicApiKey  string  // Anthropic API key
	GoogleApiKey     string  // Google API key
	DeepSeekApiKey   string  // DeepSeek API key
	QwenApiKey       string  // 通义千问 API key
	OpenCodeApiKey   string  // OpenCode Zen 和 Go 共用
	XAIApiKey        string  // xAI / Grok API key
	OllamaEndpoint   string  // Ollama 本地端点
	DockerEndpoint   string  // Docker 端点（空 = 不启用 Docker 构建）
	ADBAddress       string  // ADB 地址（空 = 不启用 ADB 健康检查）
	WebhookSecret    string  // Git webhook HMAC secret
	GitHubWebhookSec string  // GitHub webhook secret
	RateLimitPublic  float64 // 公共路由限流 (req/min)
	RateLimitAuth    float64 // 认证路由限流 (req/min)
	RateLimitAI      float64 // AI 路由限流 (req/min)
	MonthlyCostLimit float64 // AI 月度成本上限 (USD)，0 = 不限制
}

func (c *Config) Lock()    { c.mu.Lock() }
func (c *Config) Unlock()  { c.mu.Unlock() }
func (c *Config) RLock()   { c.mu.RLock() }
func (c *Config) RUnlock() { c.mu.RUnlock() }

func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", ":8080"),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		DatabasePath:     getEnv("DATABASE_PATH", "data/moduforge.db"),
		StoragePath:      getEnv("STORAGE_PATH", "/data/storage"),
		S3Endpoint:       getEnv("S3_ENDPOINT", ""),
		S3AccessKey:      getEnv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:      getEnv("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:         getEnv("S3_BUCKET", "moduforge"),
		LLMApiKey:        getEnv("LLM_API_KEY", ""),
		LLMEndpoint:      getEnv("LLM_ENDPOINT", "https://api.openai.com/v1"),
		LLMModel:         getEnv("LLM_MODEL", "gpt-4o-mini"),
		LLMProvider:      getEnv("LLM_PROVIDER", ""),
		LLMModelID:       getEnv("LLM_MODEL_ID", ""),
		OpenAIApiKey:     getEnv("OPENAI_API_KEY", ""),
		AnthropicApiKey:  getEnv("ANTHROPIC_API_KEY", ""),
		GoogleApiKey:     getEnv("GOOGLE_API_KEY", ""),
		DeepSeekApiKey:   getEnv("DEEPSEEK_API_KEY", ""),
		QwenApiKey:       getEnv("QWEN_API_KEY", ""),
		OpenCodeApiKey:   getEnv("OPENCODE_API_KEY", ""),
		XAIApiKey:        getEnv("XAI_API_KEY", ""),
		OllamaEndpoint:   getEnv("OLLAMA_ENDPOINT", "http://localhost:11434"),
		DockerEndpoint:   getEnv("DOCKER_ENDPOINT", ""),
		ADBAddress:       getEnv("ADB_ADDRESS", ""),
		WebhookSecret:    getEnv("WEBHOOK_SECRET", ""),
		GitHubWebhookSec: getEnv("GITHUB_WEBHOOK_SECRET", ""),
		RateLimitPublic:  getEnvFloat("RATE_LIMIT_PUBLIC", 100),
		RateLimitAuth:    getEnvFloat("RATE_LIMIT_AUTH", 200),
		// AI 路由（含 SSE 长连接）放宽到 60 req/min：
		// - 前端 AI 页面高频操作（发送/停止/搜索/重命名）会快速消耗额度，
		//   20/min 在连续测试时会触发 RATE_LIMITED，影响体验。
		// - 仍保留抗滥用能力（60/min 对单用户足够，对脚本压力仍有限制）。
		// 通过环境变量 RATE_LIMIT_AI 可进一步调整。
		RateLimitAI: getEnvFloat("RATE_LIMIT_AI", 60),
		// AI 月度成本上限（USD）：当月估算成本超过该值后拒绝新任务。
		// 默认 0 = 不限制成本（避免意外拦截）。按当前模型单价 × 当月 token 估算。
		MonthlyCostLimit: getEnvFloat("AI_MONTHLY_COST_LIMIT", 0),
	}
}

// getEnvFloat reads a float env var with a fallback. Invalid values fall back.
func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

// EffectiveLLMKey 返回当前生效的 API Key（优先使用多提供商配置，回退到 legacy）
func (c *Config) EffectiveLLMKey() string {
	if c.LLMProvider != "" {
		switch c.LLMProvider {
		case "openai":
			return c.OpenAIApiKey
		case "anthropic":
			return c.AnthropicApiKey
		case "google":
			return c.GoogleApiKey
		case "deepseek":
			return c.DeepSeekApiKey
		case "qwen":
			return c.QwenApiKey
		case "opencode-zen", "opencode-zen-paid", "opencode-go":
			return c.OpenCodeApiKey
		case "xai":
			return c.XAIApiKey
		case "ollama":
			return "" // Ollama 不需要 key
		}
	}
	return c.LLMApiKey
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
