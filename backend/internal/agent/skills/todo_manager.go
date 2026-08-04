package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/agent/registry"
)

// TodoManagerSkill manages todo lists for agent task tracking
type TodoManagerSkill struct {
	db *sql.DB
}

func init() {
	registry.RegisterFactory("todo_manager", func(deps *registry.Deps) registry.Skill {
		return &TodoManagerSkill{db: deps.DB}
	})
}

func (s *TodoManagerSkill) Name() string {
	return "todo_manager"
}

func (s *TodoManagerSkill) Description() string {
	return `Manage todo lists for task tracking. Input: {"action": "create|update|read|delete|complete", "todo_id": "...", "title": "...", "description": "...", "status": "pending|in_progress|completed|cancelled", "priority": "low|medium|high|urgent", "items": [...]}`
}

type TodoItem struct {
	ID          string `json:"id"`
	TodoID      string `json:"todo_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`   // pending, in_progress, completed, cancelled
	Priority    string `json:"priority"` // low, medium, high, urgent
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	CompletedAt string `json:"completed_at,omitempty"`
}

type TodoList struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	SessionID   string     `json:"session_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"` // active, completed, archived
	Items       []TodoItem `json:"items"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

func (s *TodoManagerSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	todoID, _ := input["todo_id"].(string)
	userID, _ := input["user_id"].(string)
	sessionID, _ := input["session_id"].(string)

	switch action {
	case "create":
		return s.createTodo(userID, sessionID, input)
	case "update":
		return s.updateTodo(todoID, input)
	case "read":
		return s.readTodo(todoID, userID, sessionID)
	case "delete":
		return s.deleteTodo(todoID)
	case "complete":
		return s.completeTodo(todoID)
	case "list":
		return s.listTodos(userID, sessionID)
	case "add_item":
		return s.addItem(todoID, input)
	case "update_item":
		return s.updateItem(input)
	case "complete_item":
		return s.completeItem(input)
	default:
		return "", fmt.Errorf("unknown action: %s (use create|update|read|delete|complete|list|add_item|update_item|complete_item)", action)
	}
}

func (s *TodoManagerSkill) ensureTables() error {
	// Create todo_lists table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS todo_lists (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			session_id TEXT,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create todo_lists table: %w", err)
	}

	// Create todo_items table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS todo_items (
			id TEXT PRIMARY KEY,
			todo_id TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			status TEXT DEFAULT 'pending',
			priority TEXT DEFAULT 'medium',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			FOREIGN KEY (todo_id) REFERENCES todo_lists(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("create todo_items table: %w", err)
	}

	return nil
}

func (s *TodoManagerSkill) createTodo(userID, sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	title, _ := input["title"].(string)
	description, _ := input["description"].(string)
	items, _ := input["items"].([]interface{})

	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	// Generate todo ID
	todoID := fmt.Sprintf("todo_%s_%d", userID, time.Now().UnixMilli())

	// Insert todo list
	_, err := s.db.Exec(`
		INSERT INTO todo_lists (id, user_id, session_id, title, description)
		VALUES (?, ?, ?, ?, ?)
	`, todoID, userID, sessionID, title, description)
	if err != nil {
		return "", fmt.Errorf("create todo: %w", err)
	}

	// Insert items if provided
	var createdItems []TodoItem
	for i, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemTitle, _ := itemMap["title"].(string)
		itemDesc, _ := itemMap["description"].(string)
		itemPriority, _ := itemMap["priority"].(string)
		if itemPriority == "" {
			itemPriority = "medium"
		}

		itemID := fmt.Sprintf("item_%s_%d", todoID, i)
		_, err := s.db.Exec(`
				INSERT INTO todo_items (id, todo_id, title, description, priority)
				VALUES (?, ?, ?, ?, ?)
			`, itemID, todoID, itemTitle, itemDesc, itemPriority)
		if err != nil {
			return "", fmt.Errorf("create item: %w", err)
		}

		createdItems = append(createdItems, TodoItem{
			ID:          itemID,
			TodoID:      todoID,
			Title:       itemTitle,
			Description: itemDesc,
			Status:      "pending",
			Priority:    itemPriority,
		})
	}

	result := map[string]interface{}{
		"action":  "create",
		"success": true,
		"todo_id": todoID,
		"title":   title,
		"items":   createdItems,
		"message": fmt.Sprintf("Todo list '%s' created with %d items", title, len(createdItems)),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) updateTodo(todoID string, input map[string]interface{}) (string, error) {
	if todoID == "" {
		return "", fmt.Errorf("todo_id is required")
	}

	title, _ := input["title"].(string)
	description, _ := input["description"].(string)
	status, _ := input["status"].(string)

	var updates []string
	var args []interface{}

	if title != "" {
		updates = append(updates, "title = ?")
		args = append(args, title)
	}
	if description != "" {
		updates = append(updates, "description = ?")
		args = append(args, description)
	}
	if status != "" {
		updates = append(updates, "status = ?")
		args = append(args, status)
	}

	if len(updates) == 0 {
		return "", fmt.Errorf("no fields to update")
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, todoID)

	query := fmt.Sprintf("UPDATE todo_lists SET %s WHERE id = ?", strings.Join(updates, ", "))
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return "", fmt.Errorf("update todo: %w", err)
	}

	result := map[string]interface{}{
		"action":  "update",
		"success": true,
		"todo_id": todoID,
		"message": "Todo list updated",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) readTodo(todoID, userID, sessionID string) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	var todo TodoList
	var status string

	// Try to find by todo_id first
	if todoID != "" {
		err := s.db.QueryRow(`
			SELECT id, user_id, session_id, title, description, status, created_at, updated_at
			FROM todo_lists WHERE id = ?
		`, todoID).Scan(&todo.ID, &todo.UserID, &todo.SessionID, &todo.Title, &todo.Description, &status, &todo.CreatedAt, &todo.UpdatedAt)
		if err != nil {
			return "", fmt.Errorf("todo not found: %w", err)
		}
		todo.Status = status
	} else if userID != "" {
		// Find most recent active todo for user
		err := s.db.QueryRow(`
			SELECT id, user_id, session_id, title, description, status, created_at, updated_at
			FROM todo_lists WHERE user_id = ? AND status = 'active'
			ORDER BY updated_at DESC LIMIT 1
		`, userID).Scan(&todo.ID, &todo.UserID, &todo.SessionID, &todo.Title, &todo.Description, &status, &todo.CreatedAt, &todo.UpdatedAt)
		if err != nil {
			return "", fmt.Errorf("no active todo found: %w", err)
		}
		todo.Status = status
	} else {
		return "", fmt.Errorf("todo_id or user_id is required")
	}

	// Get items
	rows, err := s.db.Query(`
		SELECT id, todo_id, title, description, status, priority, created_at, updated_at, completed_at
		FROM todo_items WHERE todo_id = ? ORDER BY 
		CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 END,
		created_at
	`, todo.ID)
	if err != nil {
		return "", fmt.Errorf("get items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item TodoItem
		var completedAt sql.NullString
		if err := rows.Scan(&item.ID, &item.TodoID, &item.Title, &item.Description, &item.Status, &item.Priority, &item.CreatedAt, &item.UpdatedAt, &completedAt); err == nil {
			if completedAt.Valid {
				item.CompletedAt = completedAt.String
			}
			todo.Items = append(todo.Items, item)
		}
	}

	// Calculate progress
	total := len(todo.Items)
	completed := 0
	for _, item := range todo.Items {
		if item.Status == "completed" {
			completed++
		}
	}
	progress := 0.0
	if total > 0 {
		progress = float64(completed) / float64(total) * 100
	}

	result := map[string]interface{}{
		"action":    "read",
		"success":   true,
		"todo":      todo,
		"total":     total,
		"completed": completed,
		"progress":  fmt.Sprintf("%.1f%%", progress),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) deleteTodo(todoID string) (string, error) {
	if todoID == "" {
		return "", fmt.Errorf("todo_id is required")
	}

	// Delete items first
	_, err := s.db.Exec("DELETE FROM todo_items WHERE todo_id = ?", todoID)
	if err != nil {
		return "", fmt.Errorf("delete items: %w", err)
	}

	// Delete todo list
	_, err = s.db.Exec("DELETE FROM todo_lists WHERE id = ?", todoID)
	if err != nil {
		return "", fmt.Errorf("delete todo: %w", err)
	}

	result := map[string]interface{}{
		"action":  "delete",
		"success": true,
		"todo_id": todoID,
		"message": "Todo list deleted",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) completeTodo(todoID string) (string, error) {
	if todoID == "" {
		return "", fmt.Errorf("todo_id is required")
	}

	// Mark all items as completed
	_, err := s.db.Exec(`
		UPDATE todo_items SET status = 'completed', completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE todo_id = ? AND status != 'completed'
	`, todoID)
	if err != nil {
		return "", fmt.Errorf("complete items: %w", err)
	}

	// Mark todo as completed
	_, err = s.db.Exec(`
		UPDATE todo_lists SET status = 'completed', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, todoID)
	if err != nil {
		return "", fmt.Errorf("complete todo: %w", err)
	}

	result := map[string]interface{}{
		"action":  "complete",
		"success": true,
		"todo_id": todoID,
		"message": "Todo list completed",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) listTodos(userID, sessionID string) (string, error) {
	if err := s.ensureTables(); err != nil {
		return "", err
	}

	query := "SELECT id, title, description, status, created_at, updated_at FROM todo_lists WHERE user_id = ?"
	args := []interface{}{userID}

	if sessionID != "" {
		query += " AND (session_id = ? OR session_id IS NULL)"
		args = append(args, sessionID)
	}

	query += " ORDER BY updated_at DESC LIMIT 10"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return "", fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	var todos []map[string]interface{}
	for rows.Next() {
		var id, title, description, status, createdAt, updatedAt string
		if err := rows.Scan(&id, &title, &description, &status, &createdAt, &updatedAt); err == nil {
			todos = append(todos, map[string]interface{}{
				"id":          id,
				"title":       title,
				"description": description,
				"status":      status,
				"created_at":  createdAt,
				"updated_at":  updatedAt,
			})
		}
	}

	result := map[string]interface{}{
		"action":  "list",
		"success": true,
		"todos":   todos,
		"count":   len(todos),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) addItem(todoID string, input map[string]interface{}) (string, error) {
	if todoID == "" {
		return "", fmt.Errorf("todo_id is required")
	}

	title, _ := input["title"].(string)
	description, _ := input["description"].(string)
	priority, _ := input["priority"].(string)

	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	if priority == "" {
		priority = "medium"
	}

	// Get max item number
	var maxNum int
	s.db.QueryRow("SELECT COALESCE(MAX(CAST(SUBSTR(id, LENGTH(id) - 2) AS INTEGER)), 0) FROM todo_items WHERE todo_id = ?", todoID).Scan(&maxNum)
	itemID := fmt.Sprintf("%s_%03d", todoID, maxNum+1)

	_, err := s.db.Exec(`
		INSERT INTO todo_items (id, todo_id, title, description, priority)
		VALUES (?, ?, ?, ?, ?)
	`, itemID, todoID, title, description, priority)
	if err != nil {
		return "", fmt.Errorf("add item: %w", err)
	}

	// Update todo's updated_at
	s.db.Exec("UPDATE todo_lists SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", todoID)

	result := map[string]interface{}{
		"action":  "add_item",
		"success": true,
		"item_id": itemID,
		"title":   title,
		"message": fmt.Sprintf("Item '%s' added", title),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) updateItem(input map[string]interface{}) (string, error) {
	itemID, _ := input["item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("item_id is required")
	}

	title, _ := input["title"].(string)
	description, _ := input["description"].(string)
	status, _ := input["status"].(string)
	priority, _ := input["priority"].(string)

	var updates []string
	var args []interface{}

	if title != "" {
		updates = append(updates, "title = ?")
		args = append(args, title)
	}
	if description != "" {
		updates = append(updates, "description = ?")
		args = append(args, description)
	}
	if status != "" {
		updates = append(updates, "status = ?")
		args = append(args, status)
	}
	if priority != "" {
		updates = append(updates, "priority = ?")
		args = append(args, priority)
	}

	if len(updates) == 0 {
		return "", fmt.Errorf("no fields to update")
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, itemID)

	query := fmt.Sprintf("UPDATE todo_items SET %s WHERE id = ?", strings.Join(updates, ", "))
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return "", fmt.Errorf("update item: %w", err)
	}

	result := map[string]interface{}{
		"action":  "update_item",
		"success": true,
		"item_id": itemID,
		"message": "Item updated",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) completeItem(input map[string]interface{}) (string, error) {
	itemID, _ := input["item_id"].(string)
	if itemID == "" {
		return "", fmt.Errorf("item_id is required")
	}

	_, err := s.db.Exec(`
		UPDATE todo_items SET status = 'completed', completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, itemID)
	if err != nil {
		return "", fmt.Errorf("complete item: %w", err)
	}

	// Check if all items in the todo are completed
	var todoID string
	s.db.QueryRow("SELECT todo_id FROM todo_items WHERE id = ?", itemID).Scan(&todoID)
	if todoID != "" {
		var total, completed int
		s.db.QueryRow("SELECT COUNT(*) FROM todo_items WHERE todo_id = ?", todoID).Scan(&total)
		s.db.QueryRow("SELECT COUNT(*) FROM todo_items WHERE todo_id = ? AND status = 'completed'", todoID).Scan(&completed)

		if total == completed && total > 0 {
			// All items completed, mark todo as completed
			s.db.Exec("UPDATE todo_lists SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id = ?", todoID)
		}
	}

	result := map[string]interface{}{
		"action":  "complete_item",
		"success": true,
		"item_id": itemID,
		"message": "Item completed",
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *TodoManagerSkill) Metadata() registry.SkillMeta {
	return registry.SkillMeta{
		ReadOnly:  false,
		Essential: false,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
