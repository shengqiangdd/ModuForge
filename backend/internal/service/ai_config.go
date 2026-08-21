package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/prompts"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/storage"
)

// ─── HTTP Client Configuration ───

var httpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 2 * time.Minute, // 等待 LLM 响应头的超时
	},
	// 不设 Timeout：LLM 流式生成可能持续很久，Timeout 会强制取消读取
}

// ─── AIService Struct & Constructors ───

// AIService is the main AI service that orchestrates LLM calls, prompt
// management, and project file persistence.
type AIService struct {
	cfg       *config.Config
	db        *sql.DB
	convStore *ConversationStore
	s3        *storage.S3Adapter // optional S3-compatible storage for project files
}

func NewAIService(cfg *config.Config) *AIService {
	return &AIService{cfg: cfg, convStore: NewConversationStore()}
}

func NewAIServiceWithDB(cfg *config.Config, db *sql.DB) *AIService {
	return &AIService{cfg: cfg, db: db, convStore: NewConversationStore()}
}

// SetS3 injects the optional S3 adapter used to persist generated project files.
func (s *AIService) SetS3(adapter *storage.S3Adapter) {
	s.s3 = adapter
}

// ─── Utility Functions ───


// ensureChatCompletionsURL ensures the endpoint ends with /chat/completions.
func ensureChatCompletionsURL(endpoint string) string {
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		return endpoint + "/chat/completions"
	}
	return endpoint
}

// marshalJSON is a convenience wrapper around json.Marshal that returns nil on error.
func marshalJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// decodeJSON decodes a JSON response body into the target struct.
func decodeJSON(r interface{ Read([]byte) (int, error) }, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

// ─── Prompt Management ───

// defaultSystemPrompt 已迁移到 MD 文件系统（prompts/prompts.go）
// 现在通过 prompts.Load(mode) 加载，支持运行时修改
func defaultSystemPrompt(mode string) string {
	prompt, err := prompts.Load(mode)
	if err != nil {
		return ""
	}
	return prompt.Full
}

// loadPrompt 加载提示词：用户数据库覆盖 → MD文件默认
func (s *AIService) loadPrompt(mode, userID string) string {
	// 1. 优先加载用户自定义提示词（数据库）
	if s.db != nil && userID != "" {
		var content string
		err := s.db.QueryRow(`SELECT content FROM ai_prompts WHERE mode=? AND user_id=?`, mode, userID).Scan(&content)
		if err == nil && content != "" {
			return content
		}
	}

	// 2. 回退到MD文件系统
	prompt, err := prompts.Load(mode)
	if err != nil {
		// 3. 最终回退：返回空字符串（不应该发生）
		fmt.Printf("[WARN] Failed to load prompt for mode %s: %v\n", mode, err)
		return ""
	}

	return prompt.Full
}

// ensurePromptsTable 确保 ai_prompts 表存在
func (s *AIService) ensurePromptsTable() error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS ai_prompts (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		mode       TEXT NOT NULL,
		user_id    TEXT NOT NULL DEFAULT '',
		content    TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE(mode, user_id)
	)`)
	if err != nil {
		return fmt.Errorf("unmarshal ai config: %w", err)
	}
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('generate', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('chat', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('repair', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('gather', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('agent', '')`)
	return nil
}

// GetPrompts 返回提示词
func (s *AIService) GetPrompts(userID string) ([]domain.AIPrompt, error) {
	if s.db == nil {
		return defaultPrompts(), nil
	}
	if err := s.ensurePromptsTable(); err != nil {
		return defaultPrompts(), nil
	}
	rows, err := s.db.Query(
		`SELECT id, mode, user_id, content, updated_at FROM ai_prompts WHERE user_id='' OR user_id=? ORDER BY mode, user_id`,
		userID,
	)
	if err != nil {
		return defaultPrompts(), nil
	}
	defer rows.Close()

	type promptRow struct {
		domain.AIPrompt
		rowUserID string
	}
	var allRows []promptRow
	for rows.Next() {
		var p promptRow
		if err := rows.Scan(&p.ID, &p.Mode, &p.rowUserID, &p.Content, &p.UpdatedAt); err != nil {
			continue
		}
		allRows = append(allRows, p)
	}

	merged := make(map[string]domain.AIPrompt)
	for _, r := range allRows {
		if r.rowUserID != "" && r.rowUserID == userID {
			merged[r.Mode] = r.AIPrompt
		} else if r.rowUserID == "" {
			if _, has := merged[r.Mode]; !has {
				merged[r.Mode] = r.AIPrompt
			}
		}
	}

	modes := []string{"generate", "chat", "repair", "gather", "agent"}
	var prompts []domain.AIPrompt
	for _, m := range modes {
		if p, ok := merged[m]; ok {
			if p.Content == "" {
				if def := defaultSystemPrompt(m); def != "" {
					p.Content = def
				}
			}
			prompts = append(prompts, p)
		} else {
			if def := defaultSystemPrompt(m); def != "" {
				prompts = append(prompts, domain.AIPrompt{Mode: m, Content: def})
			}
		}
	}
	return prompts, nil
}

func defaultPrompts() []domain.AIPrompt {
	var prompts []domain.AIPrompt
	for _, m := range []string{"generate", "chat", "repair", "gather", "agent"} {
		prompts = append(prompts, domain.AIPrompt{Mode: m, Content: defaultSystemPrompt(m)})
	}
	return prompts
}

// UpdatePrompt 更新提示词
func (s *AIService) UpdatePrompt(mode, content, userID string) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	if userID == "" {
		return fmt.Errorf("user_id required")
	}
	if err := s.ensurePromptsTable(); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO ai_prompts (mode, user_id, content, updated_at) VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(mode, user_id) DO UPDATE SET content=?, updated_at=datetime('now')`,
		mode, userID, content, content,
	)
	return err
}

// ResetPrompt 重置为默认提示词
func (s *AIService) ResetPrompt(mode, userID string) error {
	if s.db == nil {
		return nil
	}
	if userID != "" {
		_, err := s.db.Exec(`DELETE FROM ai_prompts WHERE mode=? AND user_id=?`, mode, userID)
		return err
	}
	return nil
}

// ─── Message Building Helpers ───

// buildMessageArray constructs the full message array for LLM API calls,
// including system prompt, bounded history, and current user turn.
func buildMessageArray(systemPrompt, userPrompt string, history []Message, convStore *ConversationStore) []Message {
	if len(history) > 0 {
		// 先压缩历史消息，避免 token 浪费
		compressed := convStore.CompressMessages(systemPrompt, history)
		// CompressMessages 返回的已经包含 system prompt，直接用
		compressed = append(compressed, Message{Role: "user", Content: userPrompt})
		return compressed
	}
	return []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
}

// buildHistoryMessages constructs the message array with bounded history for
// streaming chat calls.
func buildHistoryMessages(systemPrompt, userPrompt string, history []Message) []map[string]string {
	msgs := []map[string]string{{"role": "system", "content": systemPrompt}}
	if len(history) > 0 {
		// ~4 chars per token is a safe upper bound for CJK + code.
		for _, m := range historyForLLM(history, userPrompt, 20_000) {
			msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
		}
	}
	msgs = append(msgs, map[string]string{"role": "user", "content": userPrompt})
	return msgs
}
