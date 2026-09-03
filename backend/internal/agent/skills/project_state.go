package skills

import (
	"encoding/json"
	"fmt"
	"time"
)

// Project state tracking functions — file tracking, progress tracking,
// project state queries, and task status management.

func (s *ContextManagerSkill) trackFile(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	filePath, _ := input["path"].(string)
	action, _ := input["action"].(string) // created, modified, deleted
	projectID, _ := input["project_id"].(string)

	if filePath == "" || action == "" {
		return "", fmt.Errorf("path and action are required")
	}

	// Store file tracking entry
	key := fmt.Sprintf("file:%s:%s", projectID, filePath)
	value := fmt.Sprintf(`{"path":"%s","action":"%s","timestamp":"%s"}`, filePath, action, time.Now().Format(time.RFC3339))

	_, err := s.db.Exec(`
		INSERT INTO context_entries (id, session_id, category, key, value, importance)
		VALUES (?, ?, 'project', ?, ?, 7)
		ON CONFLICT(id) DO UPDATE SET
			value = excluded.value,
			accessed_at = CURRENT_TIMESTAMP
	`, fmt.Sprintf("file_%s_%s", sessionID, filePath), sessionID, key, value)
	if err != nil {
		return "", fmt.Errorf("track_file: %w", err)
	}

	result := map[string]interface{}{
		"action":      "track_file",
		"success":     true,
		"path":        filePath,
		"file_action": action,
		"project_id":  projectID,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) trackProgress(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)
	status, _ := input["status"].(string) // pending, in_progress, completed, failed
	progress, _ := input["progress"].(float64)
	projectID, _ := input["project_id"].(string)

	if taskID == "" || status == "" {
		return "", fmt.Errorf("task_id and status are required")
	}

	// Store progress entry
	key := fmt.Sprintf("progress:%s:%s", projectID, taskID)
	value := fmt.Sprintf(`{"task_id":"%s","status":"%s","progress":%.1f,"timestamp":"%s"}`,
		taskID, status, progress, time.Now().Format(time.RFC3339))

	_, err := s.db.Exec(`
		INSERT INTO context_entries (id, session_id, category, key, value, importance)
		VALUES (?, ?, 'project', ?, ?, 8)
		ON CONFLICT(id) DO UPDATE SET
			value = excluded.value,
			accessed_at = CURRENT_TIMESTAMP
	`, fmt.Sprintf("progress_%s_%s", sessionID, taskID), sessionID, key, value)
	if err != nil {
		return "", fmt.Errorf("track_progress: %w", err)
	}

	result := map[string]interface{}{
		"action":     "track_progress",
		"success":    true,
		"task_id":    taskID,
		"status":     status,
		"progress":   progress,
		"project_id": projectID,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) getProjectState(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	projectID, _ := input["project_id"].(string)
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}

	// Get all files for this project
	rows, err := s.db.Query(`
		SELECT key, value, importance, created_at, accessed_at
		FROM context_entries
		WHERE session_id = ? AND key LIKE ? AND category = 'project'
		ORDER BY accessed_at DESC
	`, sessionID, fmt.Sprintf("file:%s:%%", projectID))
	if err != nil {
		return "", fmt.Errorf("get_project_state: %w", err)
	}
	defer rows.Close()

	var files []map[string]interface{}
	for rows.Next() {
		var key, value, createdAt, accessedAt string
		var importance int
		if err := rows.Scan(&key, &value, &importance, &createdAt, &accessedAt); err == nil {
			// Parse the value JSON
			var fileData map[string]interface{}
			if json.Unmarshal([]byte(value), &fileData) == nil {
				files = append(files, fileData)
			}
		}
	}

	// Get progress for this project
	progressRows, err := s.db.Query(`
		SELECT key, value, importance, created_at, accessed_at
		FROM context_entries
		WHERE session_id = ? AND key LIKE ? AND category = 'project'
		ORDER BY accessed_at DESC
	`, sessionID, fmt.Sprintf("progress:%s:%%", projectID))
	if err != nil {
		return "", fmt.Errorf("get_project_state: %w", err)
	}
	defer progressRows.Close()

	var progress []map[string]interface{}
	for progressRows.Next() {
		var key, value, createdAt, accessedAt string
		var importance int
		if err := progressRows.Scan(&key, &value, &importance, &createdAt, &accessedAt); err == nil {
			// Parse the value JSON
			var progressData map[string]interface{}
			if json.Unmarshal([]byte(value), &progressData) == nil {
				progress = append(progress, progressData)
			}
		}
	}

	result := map[string]interface{}{
		"action":     "get_project_state",
		"success":    true,
		"project_id": projectID,
		"files":      files,
		"progress":   progress,
		"file_count": len(files),
		"task_count": len(progress),
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}

func (s *ContextManagerSkill) updateTaskStatus(sessionID string, input map[string]interface{}) (string, error) {
	if err := s.ensureTable(); err != nil {
		return "", err
	}

	taskID, _ := input["task_id"].(string)
	status, _ := input["status"].(string)
	result_, _ := input["result"].(string)
	projectID, _ := input["project_id"].(string)

	if taskID == "" || status == "" {
		return "", fmt.Errorf("task_id and status are required")
	}

	// Update or create task status entry
	key := fmt.Sprintf("task_status:%s:%s", projectID, taskID)
	value := fmt.Sprintf(`{"task_id":"%s","status":"%s","result":"%s","timestamp":"%s"}`,
		taskID, status, result_, time.Now().Format(time.RFC3339))

	_, err := s.db.Exec(`
		INSERT INTO context_entries (id, session_id, category, key, value, importance)
		VALUES (?, ?, 'project', ?, ?, 9)
		ON CONFLICT(id) DO UPDATE SET
			value = excluded.value,
			accessed_at = CURRENT_TIMESTAMP
	`, fmt.Sprintf("task_status_%s_%s", sessionID, taskID), sessionID, key, value)
	if err != nil {
		return "", fmt.Errorf("update_task_status: %w", err)
	}

	result := map[string]interface{}{
		"action":     "update_task_status",
		"success":    true,
		"task_id":    taskID,
		"status":     status,
		"result":     result_,
		"project_id": projectID,
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), nil
}
