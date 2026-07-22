package config

import "os"

type Config struct {
	Port           string // 监听端口
	JWTSecret      string // JWT 签名密钥
	DatabasePath   string // SQLite 数据库路径
	StoragePath    string // 文件存储路径
	LLMApiKey      string // LLM API Key
	LLMEndpoint    string // LLM API 端点
	LLMModel       string // LLM 模型名称
	DockerEndpoint string // Docker 端点（空 = 不启用 Docker 构建）
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
		DockerEndpoint: getEnv("DOCKER_ENDPOINT", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
