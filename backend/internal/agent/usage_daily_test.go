package agent

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestPersistDailyUsage(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ai_usage_daily (
		date TEXT PRIMARY KEY,
		llm_call_count INTEGER DEFAULT 0,
		llm_token_usage INTEGER DEFAULT 0,
		tool_call_count INTEGER DEFAULT 0,
		error_count INTEGER DEFAULT 0,
		retry_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}

	r := NewAgentRunner(nil, "", "", "", db)
	// Simulate activity
	r.perfMetrics.RecordLLMCall(1200 * time.Millisecond)
	r.perfMetrics.RecordTokenUsage(5000)
	r.perfMetrics.RecordToolCall(300 * time.Millisecond)
	r.perfMetrics.RecordError()
	r.perfMetrics.RecordRetry()

	// First persist writes the delta
	r.persistDailyUsage("test-user")

	var calls, tokens, tools, errs, retries int64
	today := time.Now().Format("2006-01-02")
	if err := db.QueryRow(`SELECT llm_call_count, llm_token_usage, tool_call_count, error_count, retry_count
		FROM ai_usage_daily WHERE date=?`, today).Scan(&calls, &tokens, &tools, &errs, &retries); err != nil {
		t.Fatalf("no daily row after persist: %v", err)
	}
	if calls != 1 || tokens != 5000 || tools != 1 || errs != 1 || retries != 1 {
		t.Fatalf("daily row = calls=%d tokens=%d tools=%d errs=%d retries=%d, want 1/5000/1/1/1", calls, tokens, tools, errs, retries)
	}

	// Second persist without new activity must not double-count
	r.persistDailyUsage("test-user")
	if err := db.QueryRow(`SELECT llm_call_count, llm_token_usage FROM ai_usage_daily WHERE date=?`, today).Scan(&calls, &tokens); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || tokens != 5000 {
		t.Fatalf("idempotency broken: calls=%d tokens=%d, want 1/5000", calls, tokens)
	}

	// New activity accumulates on top
	r.perfMetrics.RecordTokenUsage(2500)
	r.persistDailyUsage("test-user")
	if err := db.QueryRow(`SELECT llm_token_usage FROM ai_usage_daily WHERE date=?`, today).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens != 7500 {
		t.Fatalf("accumulated tokens = %d, want 7500", tokens)
	}

	// GetDailyUsage returns the row (ascending order)
	days := r.GetDailyUsage(10)
	if len(days) != 1 || days[0]["date"] != today || days[0]["llm_token_usage"].(int64) != 7500 {
		t.Fatalf("GetDailyUsage = %+v, want 1 row with 7500 tokens", days)
	}
}
