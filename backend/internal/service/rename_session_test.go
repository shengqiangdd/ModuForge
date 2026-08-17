package service

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRenameSession(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 建表（ai_history.go 中使用的结构）
	if _, err := db.Exec(`CREATE TABLE ai_conversations (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		title TEXT DEFAULT '',
		mode TEXT DEFAULT '',
		messages TEXT DEFAULT '[]',
		model TEXT DEFAULT '',
		project_id TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}

	// 1) 更新已有记录
	if _, err := db.Exec(`INSERT INTO ai_conversations (id, user_id, title) VALUES ('sess-1', 'u1', '旧标题')`); err != nil {
		t.Fatal(err)
	}
	if err := RenameSession(db, "sess-1", "u1", "新标题"); err != nil {
		t.Fatalf("rename existing: %v", err)
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM ai_conversations WHERE id='sess-1'`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "新标题" {
		t.Fatalf("expected 新标题, got %q", title)
	}

	// 2) 会话不存在 ai_conversations 记录时创建 stub
	if err := RenameSession(db, "sess-2", "u1", "新建标题"); err != nil {
		t.Fatalf("rename non-existing: %v", err)
	}
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ai_conversations WHERE id='sess-2' AND user_id='u1'`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("expected stub row, got %d", cnt)
	}

	// 3) 用户隔离：其他用户不能改
	if err := RenameSession(db, "sess-1", "u2", "劫持"); err != nil {
		t.Fatalf("rename other user should be no-op, got err: %v", err)
	}
	if err := db.QueryRow(`SELECT title FROM ai_conversations WHERE id='sess-1'`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "新标题" {
		t.Fatalf("title changed by other user: %q", title)
	}
}
