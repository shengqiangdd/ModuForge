package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
)

// ===== Custom Skills =====

type CustomSkill struct {
	ID          int64  `json:"id"`
	UserID      string `json:"user_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	InputSchema string `json:"input_schema"`
	IsPublic    bool   `json:"is_public"`
	CreatedAt   string `json:"created_at"`
}

func (h *AgentHandler) ListCustomSkills(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	rows, err := h.db.Conn.Query(
		"SELECT id, user_id, name, description, prompt, input_schema, is_public, created_at FROM custom_skills WHERE user_id = ? OR is_public = 1 ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()

	var skills []CustomSkill
	for rows.Next() {
		var s CustomSkill
		var isPub int
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Description, &s.Prompt, &s.InputSchema, &isPub, &s.CreatedAt); err != nil {
			continue
		}
		s.IsPublic = isPub == 1
		if s.UserID != userID {
			s.UserID = ""
		}
		skills = append(skills, s)
	}
	if skills == nil {
		skills = []CustomSkill{}
	}
	return c.JSON(fiber.Map{"skills": skills})
}

func (h *AgentHandler) CreateCustomSkill(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		InputSchema string `json:"input_schema"`
		IsPublic    bool   `json:"is_public"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Name == "" || req.Description == "" || req.Prompt == "" {
		return ValidationError(c, "name, description, and prompt are required")
	}
	if len(req.Name) > 100 {
		return ValidationError(c, "name too long (max 100)")
	}
	if len(req.Description) > 500 {
		return ValidationError(c, "description too long (max 500)")
	}
	if len(req.Prompt) > 10000 {
		return ValidationError(c, "prompt too long (max 10000)")
	}
	if req.InputSchema == "" {
		req.InputSchema = "{}"
	}

	isPub := 0
	if req.IsPublic {
		isPub = 1
	}

	result, err := h.db.Conn.Exec(
		"INSERT INTO custom_skills (user_id, name, description, prompt, input_schema, is_public) VALUES (?, ?, ?, ?, ?, ?)",
		userID, req.Name, req.Description, req.Prompt, req.InputSchema, isPub,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}

	id, _ := result.LastInsertId()
	return c.Status(201).JSON(fiber.Map{"id": id, "status": "ok"})
}

func (h *AgentHandler) UpdateCustomSkill(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	skillID := c.Params("id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		InputSchema string `json:"input_schema"`
		IsPublic    bool   `json:"is_public"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}

	isPub := 0
	if req.IsPublic {
		isPub = 1
	}

	_, err := h.db.Conn.Exec(
		"UPDATE custom_skills SET name=?, description=?, prompt=?, input_schema=?, is_public=? WHERE id=? AND user_id=?",
		req.Name, req.Description, req.Prompt, req.InputSchema, isPub, skillID, userID,
	)
	if err != nil {
		return InternalError(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AgentHandler) DeleteCustomSkill(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}

	skillID := c.Params("id")
	_, err := h.db.Conn.Exec("DELETE FROM custom_skills WHERE id=? AND user_id=?", skillID, userID)
	if err != nil {
		return InternalError(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// ─── Skill Evolution (6) ───

func (h *AgentHandler) GetSkillEvolution(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}
	skillID := c.Params("id")

	var stats struct {
		TotalRuns   int     `json:"total_runs"`
		SuccessRate float64 `json:"success_rate"`
		AvgDuration float64 `json:"avg_duration_ms"`
		LastRunAt   string  `json:"last_run_at"`
	}
	h.db.Conn.QueryRow(`
		SELECT COUNT(*), 
			COALESCE(AVG(CASE WHEN success=1 THEN 1.0 ELSE 0.0 END), 0),
			COALESCE(AVG(duration_ms), 0),
			COALESCE(MAX(created_at), '')
		FROM skill_evolution WHERE skill_id=? AND user_id=?
	`, skillID, userID).Scan(&stats.TotalRuns, &stats.SuccessRate, &stats.AvgDuration, &stats.LastRunAt)

	// Recent runs
	rows, err := h.db.Conn.Query(`
		SELECT id, input, output, success, duration_ms, feedback, created_at
		FROM skill_evolution WHERE skill_id=? AND user_id=? ORDER BY created_at DESC LIMIT 20
	`, skillID, userID)
	var history []map[string]interface{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var input, output, feedback, createdAt string
			var success int
			var durationMs int
			if err := rows.Scan(&id, &input, &output, &success, &durationMs, &feedback, &createdAt); err == nil {
				history = append(history, map[string]interface{}{
					"id":          id,
					"input":       input,
					"output":      output,
					"success":     success == 1,
					"duration_ms": durationMs,
					"feedback":    feedback,
					"created_at":  createdAt,
				})
			}
		}
	}

	return c.JSON(fiber.Map{
		"stats":   stats,
		"history": history,
	})
}

func (h *AgentHandler) RecordSkillEvolution(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}
	skillID := c.Params("id")
	var req struct {
		Input      string `json:"input"`
		Output     string `json:"output"`
		Success    bool   `json:"success"`
		DurationMs int    `json:"duration_ms"`
		Feedback   string `json:"feedback"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	success := 0
	if req.Success {
		success = 1
	}
	_, err := h.db.Conn.Exec(
		"INSERT INTO skill_evolution (skill_id, user_id, input, output, success, duration_ms, feedback) VALUES (?, ?, ?, ?, ?, ?, ?)",
		skillID, userID, req.Input, req.Output, success, req.DurationMs, req.Feedback,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("记录失败: %v", err)})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AgentHandler) GetSkillOptimization(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return Unauthorized(c, "未授权")
	}
	skillID := c.Params("id")

	// Get skill info
	var prompt string
	err := h.db.Conn.QueryRow("SELECT prompt FROM custom_skills WHERE id=? AND user_id=?", skillID, userID).Scan(&prompt)
	if err != nil {
		return NotFound(c, "skill not found")
	}

	// Get stats
	var totalRuns, successRuns int
	var avgDuration float64
	h.db.Conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END), 0), COALESCE(AVG(duration_ms), 0)
		FROM skill_evolution WHERE skill_id=? AND user_id=?
	`, skillID, userID).Scan(&totalRuns, &successRuns, &avgDuration)

	// Generate suggestions
	var suggestions []string
	if totalRuns == 0 {
		suggestions = append(suggestions, "尚无执行记录，请先运行该技能以获取优化建议")
	} else {
		rate := float64(0)
		if totalRuns > 0 {
			rate = float64(successRuns) / float64(totalRuns) * 100
		}
		if rate < 80 {
			suggestions = append(suggestions, fmt.Sprintf("成功率 %.0f%% 偏低，建议优化 prompt 以提高成功率", rate))
		}
		if avgDuration > 10000 {
			suggestions = append(suggestions, fmt.Sprintf("平均耗时 %.0fms 较长，考虑精简 prompt 或使用更快的模型", avgDuration))
		}
		if totalRuns > 10 && rate >= 90 {
			suggestions = append(suggestions, "该技能运行稳定，表现良好")
		}
		if totalRuns > 5 {
			suggestions = append(suggestions, fmt.Sprintf("已运行 %d 次，可考虑基于历史成功案例创建更多变体技能", totalRuns))
		}
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "继续使用并收集更多数据以获得优化建议")
	}

	return c.JSON(fiber.Map{
		"prompt":       prompt,
		"total_runs":   totalRuns,
		"success_rate": fmt.Sprintf("%.0f%%", float64(successRuns)/float64(totalRuns)*100),
		"avg_duration": fmt.Sprintf("%.0fms", avgDuration),
		"suggestions":  suggestions,
	})
}
