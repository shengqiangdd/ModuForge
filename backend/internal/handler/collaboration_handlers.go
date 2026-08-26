package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// 协作管理器（全局）
var collabManager = code.NewCollaborationManager()

// HandleCreateCollaboration 创建协作会话
func (h *AIHandler) HandleCreateCollaboration(c fiber.Ctx) error {
	type request struct {
		FileName string `json:"file_name"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.FileName == "" {
		return BadRequest(c, "File name is required")
	}

	session := collabManager.CreateSession(req.FileName)

	return c.Status(200).JSON(fiber.Map{
		"valid":   true,
		"session": session,
	})
}

// HandleJoinCollaboration 加入协作会话
func (h *AIHandler) HandleJoinCollaboration(c fiber.Ctx) error {
	type request struct {
		SessionID string `json:"session_id"`
		UserID    string `json:"user_id"`
		Username  string `json:"username"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.SessionID == "" || req.UserID == "" || req.Username == "" {
		return BadRequest(c, "Session ID, User ID, and Username are required")
	}

	success := collabManager.JoinSession(req.SessionID, req.UserID, req.Username)
	if !success {
		return BadRequest(c, "Session not found")
	}

	return c.Status(200).JSON(fiber.Map{
		"valid": true,
		"message": "Joined session successfully",
	})
}

// HandleLeaveCollaboration 离开协作会话
func (h *AIHandler) HandleLeaveCollaboration(c fiber.Ctx) error {
	type request struct {
		SessionID string `json:"session_id"`
		UserID    string `json:"user_id"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.SessionID == "" || req.UserID == "" {
		return BadRequest(c, "Session ID and User ID are required")
	}

	collabManager.LeaveSession(req.SessionID, req.UserID)

	return c.Status(200).JSON(fiber.Map{
		"valid":   true,
		"message": "Left session successfully",
	})
}

// HandleUpdateCursor 更新光标位置
func (h *AIHandler) HandleUpdateCursor(c fiber.Ctx) error {
	type request struct {
		SessionID string `json:"session_id"`
		UserID    string `json:"user_id"`
		Line      int    `json:"line"`
		Column    int    `json:"column"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.SessionID == "" || req.UserID == "" {
		return BadRequest(c, "Session ID and User ID are required")
	}

	collabManager.UpdateCursor(req.SessionID, req.UserID, req.Line, req.Column)

	return c.Status(200).JSON(fiber.Map{
		"valid": true,
	})
}

// HandleApplyCollaborationChange 应用协作变更
func (h *AIHandler) HandleApplyCollaborationChange(c fiber.Ctx) error {
	type request struct {
		SessionID string `json:"session_id"`
		UserID    string `json:"user_id"`
		Username  string `json:"username"`
		Type      string `json:"type"`
		Position  int    `json:"position"`
		Content   string `json:"content"`
		Length    int    `json:"length"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.SessionID == "" || req.UserID == "" {
		return BadRequest(c, "Session ID and User ID are required")
	}

	change := code.CollaborationChange{
		UserID:   req.UserID,
		Username: req.Username,
		Type:     req.Type,
		Position: req.Position,
		Content:  req.Content,
		Length:   req.Length,
	}

	success := collabManager.ApplyChange(req.SessionID, change)
	if !success {
		return BadRequest(c, "Session not found")
	}

	return c.Status(200).JSON(fiber.Map{
		"valid": true,
	})
}

// HandleGetActiveSessions 获取活跃协作会话
func (h *AIHandler) HandleGetActiveSessions(c fiber.Ctx) error {
	sessions := collabManager.GetActiveSessions()

	return c.Status(200).JSON(fiber.Map{
		"valid":    true,
		"sessions": sessions,
	})
}

// HandleGenerateAPIDoc 生成API文档
func (h *AIHandler) HandleGenerateAPIDoc(c fiber.Ctx) error {
	var req code.APIDocRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.Code == "" {
		return BadRequest(c, "Code is required")
	}

	generator := code.NewAPIDocGenerator()
	result := generator.GenerateDoc(req)

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}

// HandleDetectDuplication 检测代码重复
func (h *AIHandler) HandleDetectDuplication(c fiber.Ctx) error {
	var req code.DuplicationRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if len(req.Files) == 0 {
		return BadRequest(c, "At least one file is required")
	}

	detector := code.NewDuplicationDetector()
	result := detector.Detect(req)

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}
