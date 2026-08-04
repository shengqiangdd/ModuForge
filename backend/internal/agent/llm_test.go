package agent

import (
	"testing"
)

func testToolCall(name, args string) LLMToolCall {
	return LLMToolCall{
		ID:   "call_1",
		Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: name, Arguments: args},
	}
}

func TestRepairToolCalls_KeepsEmptyArgs(t *testing.T) {
	got := repairToolCalls([]LLMToolCall{testToolCall("read_file", "")})
	if len(got) != 1 {
		t.Fatalf("expected empty-args call to be kept, got %d calls", len(got))
	}
}

func TestRepairToolCalls_KeepsValidJSON(t *testing.T) {
	args := `{"query":"foo","path":"src"}`
	got := repairToolCalls([]LLMToolCall{testToolCall("grep_search", args)})
	if len(got) != 1 {
		t.Fatalf("expected valid call kept, got %d calls", len(got))
	}
	if got[0].Function.Arguments != args {
		t.Fatalf("valid JSON should pass through unchanged, got %q", got[0].Function.Arguments)
	}
}

func TestRepairToolCalls_FixesMissingColons(t *testing.T) {
	got := repairToolCalls([]LLMToolCall{testToolCall("write_file", `{"path" "a.go","content" "x"}`)})
	if len(got) != 1 {
		t.Fatalf("expected colon-fix to repair the call, got %d calls", len(got))
	}
	if got[0].Function.Arguments != `{"path": "a.go","content": "x"}` {
		t.Fatalf("expected colon-fixed args, got %q", got[0].Function.Arguments)
	}
}

func TestRepairToolCalls_FixesUnescapedNewlines(t *testing.T) {
	args := "{\"path\":\"a.go\",\"content\":\"l1\nl2\"}"
	got := repairToolCalls([]LLMToolCall{testToolCall("write_file", args)})
	if len(got) != 1 {
		t.Fatalf("expected newline-fix to repair the call, got %d calls", len(got))
	}
	if got[0].Function.Arguments != "{\"path\":\"a.go\",\"content\":\"l1\\nl2\"}" {
		t.Fatalf("expected escaped newline, got %q", got[0].Function.Arguments)
	}
}

func TestRepairToolCalls_DropsUnrecoverableJSON(t *testing.T) {
	got := repairToolCalls([]LLMToolCall{testToolCall("bash", "this is not json")})
	if len(got) != 0 {
		t.Fatalf("expected unrecoverable call to be dropped, got %#v", got)
	}
}

func TestRepairToolCalls_DropsEmptyName(t *testing.T) {
	got := repairToolCalls([]LLMToolCall{testToolCall("", `{}`)})
	if len(got) != 0 {
		t.Fatalf("expected empty-name call to be dropped, got %#v", got)
	}
}

func TestResolveLLMConfig_UsesRunConfig(t *testing.T) {
	r := NewAgentRunner(nil, "default-key", "http://default:8080/v1/chat/completions", "default-model", nil)
	cfg := RunConfig{
		UserID:        "u1",
		LLMEndpoint:   "http://custom:9090/v1/chat/completions",
		LLMApiKey:     "custom-key",
		LLMModel:      "custom-model",
		MaxIterations: 10,
		MaxResultLen:  1024,
		Mode:          ModeAct,
	}
	ep, key, model := r.resolveLLMConfig("u1", "", "", cfg)
	if ep != "http://custom:9090/v1/chat/completions" || key != "custom-key" || model != "custom-model" {
		t.Fatalf("expected RunConfig values, got (%s, %s, %s)", ep, key, model)
	}
}

func TestResolveLLMConfig_ReqModelOverridesRunConfigModel(t *testing.T) {
	r := NewAgentRunner(nil, "k", "http://e/v1/chat/completions", "m", nil)
	cfg := RunConfig{
		UserID:        "u1",
		LLMEndpoint:   "http://e/v1/chat/completions",
		LLMApiKey:     "k",
		LLMModel:      "m",
		MaxIterations: 10,
		MaxResultLen:  1024,
		Mode:          ModeAct,
	}
	_, _, model := r.resolveLLMConfig("u1", "", "req-model", cfg)
	if model != "req-model" {
		t.Fatalf("reqModel should override RunConfig model, got %q", model)
	}
}

func TestResolveLLMConfig_FallsBackToRunnerDefaults(t *testing.T) {
	r := NewAgentRunner(nil, "default-key", "http://default:8080/v1/chat/completions", "default-model", nil)
	ep, key, model := r.resolveLLMConfig("u1", "", "")
	if ep != "http://default:8080/v1/chat/completions" || key != "default-key" || model != "default-model" {
		t.Fatalf("expected runner defaults, got (%s, %s, %s)", ep, key, model)
	}
}

func TestResolveLLMConfig_ReqModelOverridesFallbackModel(t *testing.T) {
	r := NewAgentRunner(nil, "k", "http://e/v1/chat/completions", "m", nil)
	_, _, model := r.resolveLLMConfig("u1", "", "req-model")
	if model != "req-model" {
		t.Fatalf("reqModel should override fallback model, got %q", model)
	}
}
