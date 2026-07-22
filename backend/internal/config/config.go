package config

import "os"

type Config struct {
	Port         string // 监听端口
	JWTSecret    string // JWT 签名密钥
	DatabasePath string // SQLite 数据库路径
	StoragePath  string // 文件存储路径
	// Legacy single-provider config (backward compatible)
	LLMApiKey   string // LLM API Key
	LLMEndpoint string // LLM API 端点
	LLMModel    string // LLM 模型名称
	// Multi-provider config
	LLMProvider string // 当前使用的提供商 ID
	LLMModelID  string // 当前使用的模型 ID
	// Provider API Keys
	OpenAIApiKey    string // OpenAI API key
	AnthropicApiKey string // Anthropic API key
	GoogleApiKey    string // Google API key
	DeepSeekApiKey  string // DeepSeek API key
	QwenApiKey      string // 通义千问 API key
	OpenCodeApiKey  string // OpenCode Zen 和 Go 共用
	OllamaEndpoint  string // Ollama 本地端点
	DockerEndpoint  string // Docker 端点（空 = 不启用 Docker 构建）
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", ":8080"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		DatabasePath:   getEnv("DATABASE_PATH", "data/moduforge.db"),
		StoragePath:    getEnv("STORAGE_PATH", "data/storage"),
		LLMApiKey:      getEnv("LLM_API_KEY", ""),
		LLMEndpoint:    getEnv("LLM_ENDPOINT", "https://api.openai.com/v1"),
		LLMModel:       getEnv("LLM_MODEL", "gpt-4o-mini"),
		LLMProvider:    getEnv("LLM_PROVIDER", ""),
		LLMModelID:     getEnv("LLM_MODEL_ID", ""),
		OpenAIApiKey:   getEnv("OPENAI_API_KEY", ""),
		AnthropicApiKey: getEnv("ANTHROPIC_API_KEY", ""),
		GoogleApiKey:    getEnv("GOOGLE_API_KEY", ""),
		DeepSeekApiKey:  getEnv("DEEPSEEK_API_KEY", ""),
		QwenApiKey:      getEnv("QWEN_API_KEY", ""),
		OpenCodeApiKey:  getEnv("OPENCODE_API_KEY", ""),
		OllamaEndpoint:  getEnv("OLLAMA_ENDPOINT", "http://localhost:11434"),
		DockerEndpoint:  getEnv("DOCKER_ENDPOINT", ""),
	}
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
