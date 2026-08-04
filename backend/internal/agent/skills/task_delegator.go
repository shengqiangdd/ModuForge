package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
)

// TaskDelegatorSkill allows agent to spawn sub-agents for parallel work
type TaskDelegatorSkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("task_delegator", func(deps *registry.Deps) registry.Skill {
		return &TaskDelegatorSkill{db: deps.DB}
	})
}

func (s *TaskDelegatorSkill) Name() string {
	return "task_delegator"
}

func (s *TaskDelegatorSkill) Description() string {
	return `Delegate tasks to sub-agents for parallel execution. Input: {"action": "spawn|status|result|cancel", "task_id": "...", "task": "...", "tools": [...], "timeout": 300}`
}

type SubTask struct {
	ID          string   `json:"id"`
	ParentID    string   `json:"parent_id"`
	Task        string   `json:"task"`
	Status      string   `json:"status"` // pending, running, completed, failed, cancelled
	Result      string   `json:"result,omitempty"`
	Error       string   `json:"error,omitempty"`
	Tools       []string `json:"tools,omitempty"` // tool whitelist for sub-agent
	Timeout     int      `json:"timeout"`         // seconds
	CreatedAt   string   `json:"created_at"`
	StartedAt   string   `json:"started_at,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
}

func (s *TaskDelegatorSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	taskID, _ := input["task_id"].(string)
	parentID, _ := input["parent_id"].(string)

	switch action {
	case "spawn":
		return s.spawnTask(parentID, input)
	case "status":
		return s.getStatus(taskID)
	case "result":
		return s.getResult(taskID)
	case "cancel":
		return s.cancelTask(taskID)
	case "list":
		return s.listTasks(parentID)
	default:
		return "", fmt.Errorf("unknown action: %s (use spawn|status|result|cancel|list)", action)
	}
}

func (s *TaskDelegatorSkill) ensureTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sub_tasks (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL,
			task TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			result TEXT DEFAULT '',
			error TEXT DEFAULT '',
			tools TEXT DEFAULT '[]',
			timeout INTEGER DEFAULT 300,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME
		)
	`)
	return err
}

func (s *TaskDelegatorSkill) spawnTask(parentID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	task, _ := input["task"].(string)
	tools, _ := input["tools"].([]interface{})
	timeout := 300
	if t, ok := input["timeout"].(float64); ok {
		timeout = int(t)
	}

	if task == "" {
		return "", fmt.Errorf("task is required")
	}
	if parentID == "" {
		return "", fmt.Errorf("parent_id is required")
	}

	// Generate task ID
	taskID := fmt.Sprintf("task_%s_%d", parentID, time.Now().UnixMilli())

	// Convert tools to JSON
	toolsJSON := "[]"
	if tools != nil {
		b, _ := json.Marshal(tools)
		toolsJSON = string(b)
	}

	// Insert task
	_, err := s.db.Exec(`
		INSERT INTO sub_tasks (id, parent_id, task, tools, timeout)
		VALUES (?, ?, ?, ?, ?)
	`, taskID, parentID, task, toolsJSON, timeout)
	if err != nil {
		return "", fmt.Errorf("spawn task: %w", err)
	}

	// In a real implementation, this would spawn a goroutine to execute the task
	// For now, we just record it and mark it as pending
	// The actual execution would be handled by the AgentRunner

	result := map[string]interface{}{
		"action":  "spawn",
		"success": true,
		"task_id": taskID,
		"status":  "pending",
		"message": fmt.Sprintf("Task spawned with ID: %s", taskID),
		"note":    "Task will be executed asynchronously. Use 'status' to check progress.",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TaskDelegatorSkill) getStatus(taskID string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}

	var task SubTask
	var toolsJSON string
	var startedAt, completedAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, parent_id, task, status, result, error, tools, timeout, created_at, started_at, completed_at
		FROM sub_tasks WHERE id = ?
	`, taskID).Scan(&task.ID, &task.ParentID, &task.Task, &task.Status, &task.Result, &task.Error, &toolsJSON, &task.Timeout, &task.CreatedAt, &startedAt, &completedAt)
	if err != nil {
		return "", fmt.Errorf("task not found: %w", err)
	}

	if startedAt.Valid {
		task.StartedAt = startedAt.String
	}
	if completedAt.Valid {
		task.CompletedAt = completedAt.String
	}

	// Parse tools
	json.Unmarshal([]byte(toolsJSON), &task.Tools)

	result := map[string]interface{}{
		"action": "status",
		"task":   task,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TaskDelegatorSkill) getResult(taskID string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}

	var status, result, errMsg string
	err := s.db.QueryRow("SELECT status, result, error FROM sub_tasks WHERE id = ?", taskID).Scan(&status, &result, &errMsg)
	if err != nil {
		return "", fmt.Errorf("task not found: %w", err)
	}

	if status != "completed" && status != "failed" {
		return "", fmt.Errorf("task not finished (status: %s)", status)
	}

	output := map[string]interface{}{
		"action":  "result",
		"task_id": taskID,
		"status":  status,
	}
	if status == "completed" {
		output["result"] = result
	} else {
		output["error"] = errMsg
	}

	b, _ := json.MarshalIndent(output, "", "  ")
	return string(b), nil
}

func (s *TaskDelegatorSkill) cancelTask(taskID string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}

	_, err := s.db.Exec(`
		UPDATE sub_tasks SET status = 'cancelled', completed_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status IN ('pending', 'running')
	`, taskID)
	if err != nil {
		return "", fmt.Errorf("cancel task: %w", err)
	}

	result := map[string]interface{}{
		"action":  "cancel",
		"success": true,
		"task_id": taskID,
		"message": "Task cancelled",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TaskDelegatorSkill) listTasks(parentID string) (string, error) {
	if parentID == "" {
		return "", fmt.Errorf("parent_id is required")
	}

	rows, err := s.db.Query(`
		SELECT id, task, status, created_at
		FROM sub_tasks WHERE parent_id = ?
		ORDER BY created_at DESC
	`, parentID)
	if err != nil {
		return "", fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []map[string]interface{}
	for rows.Next() {
		var id, task, status, createdAt string
		if err := rows.Scan(&id, &task, &status, &createdAt); err == nil {
			tasks = append(tasks, map[string]interface{}{
				"id":         id,
				"task":       task,
				"status":     status,
				"created_at": createdAt,
			})
		}
	}

	result := map[string]interface{}{
		"action":  "list",
		"success": true,
		"tasks":   tasks,
		"count":   len(tasks),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TaskDelegatorSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
