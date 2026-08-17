package agent

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TestMonthlyCostCap verifies the monthly cost aggregation and limit logic.
func TestMonthlyCostCap(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cost_test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE ai_usage_daily (
		date TEXT, user_id TEXT, llm_call_count INTEGER DEFAULT 0,
		llm_token_usage INTEGER DEFAULT 0, tool_call_count INTEGER DEFAULT 0,
		error_count INTEGER DEFAULT 0, retry_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (date, user_id)
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	month := time.Now().Format("2006-01")
	// 1M tokens this month at gpt-5.5 price (input $5, output $30 per 1M) -> avg $17.5
	_, err = db.Exec(`INSERT INTO ai_usage_daily (date, user_id, llm_token_usage) VALUES (?, 'u1', 1000000)`,
		month+"-01")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	_, err = db.Exec(`INSERT INTO ai_usage_daily (date, user_id, llm_token_usage) VALUES (?, 'u1', 1000000)`,
		month+"-15")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Other user should not count.
	_, err = db.Exec(`INSERT INTO ai_usage_daily (date, user_id, llm_token_usage) VALUES (?, 'u2', 9000000)`,
		month+"-15")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	r := &AgentRunner{db: db, monthlyCostLimit: 30}
	info := r.calcMonthlyCost("u1", 5, 30)
	if info.Tokens != 2_000_000 {
		t.Fatalf("tokens = %d, want 2000000", info.Tokens)
	}
	if info.EstimatedCost < 34.9 || info.EstimatedCost > 35.1 {
		t.Fatalf("estimated cost = %.4f, want ~35.0", info.EstimatedCost)
	}
	if !info.Exceeded {
		t.Fatal("expected exceeded=true with limit 30 < cost 35")
	}

	// Free model (price 0) never trips the cap.
	free := r.calcMonthlyCost("u1", 0, 0)
	if free.EstimatedCost != 0 || free.Exceeded {
		t.Fatalf("free model should not accrue cost: %+v", free)
	}

	// Unset limit never trips.
	unlimited := &AgentRunner{db: db}
	if unlimited.MonthlyCostExceeded("u1", 30, 30) {
		t.Fatal("no limit should never exceed")
	}
}

// TestModelPricer verifies known/free/unknown model price resolution.
func TestModelPricer(t *testing.T) {
	pi, po := ModelPricer("gpt-5.5")
	if pi <= 0 || po <= 0 {
		t.Fatalf("gpt-5.5 price = (%v,%v), want positive", pi, po)
	}
	pi, po = ModelPricer("mimo-v2.5-free")
	if pi != 0 || po != 0 {
		t.Fatalf("free model price = (%v,%v), want 0", pi, po)
	}
	pi, po = ModelPricer("nonexistent-model-xyz")
	if pi != 0 || po != 0 {
		t.Fatalf("unknown model price = (%v,%v), want 0", pi, po)
	}
	pi, po = ModelPricer("")
	if pi != 0 || po != 0 {
		t.Fatalf("empty model price = (%v,%v), want 0", pi, po)
	}
}