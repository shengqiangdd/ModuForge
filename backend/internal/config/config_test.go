package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear all relevant env vars
	for _, key := range []string{
		"PORT", "JWT_SECRET", "DATABASE_PATH", "STORAGE_PATH",
		"LLM_API_KEY", "LLM_ENDPOINT", "LLM_MODEL",
	} {
		os.Unsetenv(key)
	}

	cfg := Load()
	if cfg.Port != ":8080" {
		t.Errorf("expected :8080, got %s", cfg.Port)
	}
	if cfg.JWTSecret != "" {
		t.Errorf("expected empty default JWT secret, got %s", cfg.JWTSecret)
	}
	if cfg.DatabasePath != "data/moduforge.db" {
		t.Errorf("expected data/moduforge.db, got %s", cfg.DatabasePath)
	}
	if cfg.LLMEndpoint != "https://api.openai.com/v1" {
		t.Errorf("expected default LLM endpoint, got %s", cfg.LLMEndpoint)
	}
	if cfg.LLMModel != "gpt-4o-mini" {
		t.Errorf("expected default model gpt-4o-mini, got %s", cfg.LLMModel)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("PORT", ":9090")
	os.Setenv("JWT_SECRET", "custom-secret")
	os.Setenv("DATABASE_PATH", "/tmp/test.db")
	os.Setenv("STORAGE_PATH", "/tmp/storage")
	os.Setenv("LLM_API_KEY", "sk-test")
	os.Setenv("LLM_ENDPOINT", "https://custom.example.com/v1")
	os.Setenv("LLM_MODEL", "custom-model")
	defer func() {
		for _, key := range []string{
			"PORT", "JWT_SECRET", "DATABASE_PATH", "STORAGE_PATH",
			"LLM_API_KEY", "LLM_ENDPOINT", "LLM_MODEL",
		} {
			os.Unsetenv(key)
		}
	}()

	cfg := Load()
	if cfg.Port != ":9090" {
		t.Errorf("expected :9090, got %s", cfg.Port)
	}
	if cfg.JWTSecret != "custom-secret" {
		t.Errorf("expected custom-secret, got %s", cfg.JWTSecret)
	}
	if cfg.LLMApiKey != "sk-test" {
		t.Errorf("expected sk-test, got %s", cfg.LLMApiKey)
	}
}

func TestEffectiveLLMKey(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name: "no provider set, uses legacy key",
			cfg: &Config{
				LLMProvider: "",
				LLMApiKey:   "legacy-key",
			},
			expected: "legacy-key",
		},
		{
			name: "openai provider",
			cfg: &Config{
				LLMProvider:  "openai",
				OpenAIApiKey: "sk-openai",
			},
			expected: "sk-openai",
		},
		{
			name: "anthropic provider",
			cfg: &Config{
				LLMProvider:     "anthropic",
				AnthropicApiKey: "sk-ant",
			},
			expected: "sk-ant",
		},
		{
			name: "ollama returns empty",
			cfg: &Config{
				LLMProvider: "ollama",
			},
			expected: "",
		},
		{
			name: "unknown provider falls back to legacy",
			cfg: &Config{
				LLMProvider: "unknown",
				LLMApiKey:   "fallback-key",
			},
			expected: "fallback-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.EffectiveLLMKey(); got != tt.expected {
				t.Errorf("EffectiveLLMKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_EXISTS", "hello")
	defer os.Unsetenv("TEST_EXISTS")

	if got := getEnv("TEST_EXISTS", "fallback"); got != "hello" {
		t.Errorf("getEnv() = %q, want %q", got, "hello")
	}
	if got := getEnv("TEST_MISSING", "fallback"); got != "fallback" {
		t.Errorf("getEnv() = %q, want %q", got, "fallback")
	}
}