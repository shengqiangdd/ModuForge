package agent

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestAutoRecallMemory verifies that relevant past memories are injected into
// the prompt while irrelevant ones are filtered out.
func TestAutoRecallMemory(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(dir, "mem.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE memory_v2 (
		id TEXT PRIMARY KEY, user_id TEXT NOT NULL, session_id TEXT,
		content TEXT NOT NULL, category TEXT DEFAULT 'episodic',
		tier TEXT DEFAULT 'short_term', importance INTEGER DEFAULT 5,
		tags TEXT DEFAULT '[]', access_count INTEGER DEFAULT 0,
		last_accessed DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Relevant memory about OpenCode Zen provider behavior
	_, err = db.Exec(`INSERT INTO memory_v2 (id, user_id, content, category, tier, importance) VALUES
		('m1', 'u1', 'OpenCode Zen 免费模型 mimo-v2.5-free 有每日配额，用尽后返回 429 限流错误', 'semantic', 'long_term', 8),
		('m2', 'u1', '用户偏好：不要裁剪免费模型的写工具，保持完整能力', 'semantic', 'long_term', 9),
		('m3', 'u1', '昨天部署了 ModuForge 第三十六轮：容器内存上限优化', 'episodic', 'short_term', 6),
		('m4', 'u1', '今天天气晴朗适合户外活动', 'episodic', 'short_term', 2)`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	r := &AgentRunner{db: db}
	cfg := RunConfig{UserID: "u1"}

	// Query about LLM quota issues should surface the quota memory, not weather.
	out := r.autoRecallMemory(cfg, "免费模型 429 配额用尽怎么办", 3)
	if out == "" {
		t.Fatal("expected non-empty recall for quota query")
	}
	if !strings.Contains(out, "429") {
		t.Errorf("recall should mention 429 quota memory, got: %s", out)
	}
	if strings.Contains(out, "天气") {
		t.Error("irrelevant weather memory should not be recalled")
	}

	// Empty task should return empty.
	if r.autoRecallMemory(cfg, "", 3) != "" {
		t.Error("empty task should return empty recall")
	}

	// Different user should get nothing.
	if r.autoRecallMemory(RunConfig{UserID: "nobody"}, "429 配额", 3) != "" {
		t.Error("other user should have no recall")
	}
}// TestAutoStoreMemory verifies the auto-store loop: short answers are skipped,
// long answers are persisted to memory_v2 (table auto-created), and near
// duplicates within 24h are deduped.
func TestAutoStoreMemory(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(dir, "memstore.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	r := &AgentRunner{db: db}

	// Too short -> no store, table may not even exist yet.
	r.autoStoreMemory("u1", "s1", "OK done")
	// Long answer -> auto-creates table and stores.
	long := "完成了 ModuForge 第三十七轮优化：自动记忆召回与存储闭环，记忆系统现在不依赖模型主动调用技能，任务后自动落库、任务前自动召回相关记忆。"
	r.autoStoreMemory("u1", "s1", long)

	var cnt int
	err = db.QueryRow(`SELECT COUNT(*) FROM memory_v2 WHERE user_id='u1'`).Scan(&cnt)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected 1 stored memory, got %d", cnt)
	}

	// Same content again -> dedupe (0 new rows).
	r.autoStoreMemory("u1", "s2", long)
	err = db.QueryRow(`SELECT COUNT(*) FROM memory_v2 WHERE user_id='u1'`).Scan(&cnt)
	if err != nil {
		t.Fatalf("query count 2: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected dedupe (1 row), got %d", cnt)
	}

	// Different content -> stores.
	r.autoStoreMemory("u1", "s3", "另一条不同内容的记忆：用户偏好使用 OpenCode Zen 的免费模型且不裁剪工具。")
	err = db.QueryRow(`SELECT COUNT(*) FROM memory_v2 WHERE user_id='u1'`).Scan(&cnt)
	if err != nil {
		t.Fatalf("query count 3: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("expected 2 rows, got %d", cnt)
	}

	// Different user -> separate.
	r.autoStoreMemory("u2", "s9", "这是另一个用户的完全不同的长内容记忆，用来验证用户隔离。")
	err = db.QueryRow(`SELECT COUNT(*) FROM memory_v2 WHERE user_id='u2'`).Scan(&cnt)
	if err != nil {
		t.Fatalf("query count 4: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected 1 row for u2, got %d", cnt)
	}

	// Verify category/tier defaults.
	var cat, tier string
	err = db.QueryRow(`SELECT category, tier FROM memory_v2 WHERE user_id='u1' LIMIT 1`).Scan(&cat, &tier)
	if err != nil {
		t.Fatalf("scan defaults: %v", err)
	}
	if cat != "episodic" || tier != "short_term" {
		t.Fatalf("expected episodic/short_term, got %s/%s", cat, tier)
	}
}