package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactSensitiveArgs(t *testing.T) {
	in := map[string]interface{}{
		"repo":        "owner/repo",
		"issue_title": "hello",
		"token":       "ghp_1234567890abcdefghij",
		"api_key":     "sk-abcdef1234567890",
		"config":      map[string]interface{}{"password": "p@ssw0rd", "retries": 3},
	}
	out := redactSensitiveArgs(in)
	if out["repo"] != "owner/repo" {
		t.Fatalf("non-sensitive leaked: %v", out["repo"])
	}
	if out["token"] == "ghp_1234567890abcdefghij" || out["token"].(string) != "ghp_***ij" {
		t.Fatalf("token not masked: %v", out["token"])
	}
	if out["api_key"].(string) != "sk-a***90" {
		t.Fatalf("api_key not masked: %v", out["api_key"])
	}
	nested := out["config"].(map[string]interface{})
	if nested["password"] != "***" {
		t.Fatalf("nested password not masked: %v", nested["password"])
	}
	if nested["retries"] != 3 {
		t.Fatalf("nested non-sensitive changed: %v", nested["retries"])
	}
}

func TestUserFriendlyError402(t *testing.T) {
	err := errors.New(`LLM error (HTTP 402) provider=rhythm model=deepseek-v4-flash: {"error":{"message":"insufficient balance"}}`)
	msg := userFriendlyError(err)
	if !strings.Contains(msg, "余额") || !strings.Contains(msg, "rhythm") {
		t.Fatalf("402 message not friendly: %s", msg)
	}
}

func TestUserFriendlyError429(t *testing.T) {
	err := errors.New(`LLM error (HTTP 429) provider=mimo model=mimo-v2.5-free: {"error":{"message":"quota exceeded"}}`)
	msg := userFriendlyError(err)
	if !strings.Contains(msg, "限流") || !strings.Contains(msg, "免费") {
		t.Fatalf("429 message not friendly: %s", msg)
	}
}
