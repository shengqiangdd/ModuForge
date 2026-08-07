package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseModuleOutput_JSONBlock(t *testing.T) {
	raw := "some text ```json\n{\"files\": [{\"path\": \"test.txt\", \"content\": \"hello\"}]}\n``` more text"
	files, err := ParseModuleOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "test.txt" {
		t.Errorf("expected path test.txt, got %s", files[0].Path)
	}
	if files[0].Content != "hello" {
		t.Errorf("expected content hello, got %s", files[0].Content)
	}
}

func TestParseModuleOutput_BareJSON(t *testing.T) {
	raw := `{"files": [{"path": "a.txt", "content": "b"}]}`
	files, err := ParseModuleOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
}

func TestParseModuleOutput_NoJSON(t *testing.T) {
	_, err := ParseModuleOutput("just some random text")
	if err == nil {
		t.Fatal("expected error for no JSON")
	}
}

func TestParseModuleOutput_EmptyFiles(t *testing.T) {
	raw := `{"files": []}`
	_, err := ParseModuleOutput(raw)
	if err == nil {
		t.Fatal("expected error for empty files")
	}
}

func TestNewGateway(t *testing.T) {
	g := NewGateway("sk-test", "https://api.example.com/v1", "gpt-4")
	if g == nil {
		t.Fatal("expected non-nil gateway")
	}
	if g.modelID != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", g.modelID)
	}
	if g.apiKey != "sk-test" {
		t.Errorf("expected apiKey sk-test, got %s", g.apiKey)
	}
	expectedEndpoint := "https://api.example.com/v1/chat/completions"
	if g.endpoint != expectedEndpoint {
		t.Errorf("expected endpoint %s, got %s", expectedEndpoint, g.endpoint)
	}
}

func TestNewGatewayFromProvider_Unknown(t *testing.T) {
	_, err := NewGatewayFromProvider("nonexistent", "model", "key")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewGatewayFromProvider_MissingKey(t *testing.T) {
	_, err := NewGatewayFromProvider("openai", "gpt-4", "")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestNewGatewayFromProvider_Google(t *testing.T) {
	g, err := NewGatewayFromProvider("google", "gemini-pro", "sk-google")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(g.endpoint, "gemini-pro:generateContent") {
		t.Errorf("expected google-style endpoint, got %s", g.endpoint)
	}
	if !strings.Contains(g.endpoint, "key=sk-google") {
		t.Errorf("expected api key in endpoint, got %s", g.endpoint)
	}
}

func TestGateway_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		resp := ChatResponse{
			Choices: []struct {
				Message Message `json:"message"`
			}{
				{Message: Message{Role: "assistant", Content: "Hello!"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	g := NewGateway("sk-test", server.URL, "test-model")
	// Override to remove /chat/completions suffix since test server handles it
	g.endpoint = server.URL

	result, err := g.Chat([]Message{{Role: "user", Content: "Hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello!" {
		t.Errorf("expected Hello!, got %s", result)
	}
}

func TestGateway_ChatAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	g := NewGateway("wrong-key", server.URL, "test-model")
	g.endpoint = server.URL

	_, err := g.Chat([]Message{{Role: "user", Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error for auth failure")
	}
}

func TestGateway_Model(t *testing.T) {
	g := NewGateway("", "", "gpt-4o")
	if g.Model() != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", g.Model())
	}
}