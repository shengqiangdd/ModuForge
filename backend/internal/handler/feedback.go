package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent"
)

// HandleSubmitFeedback 提交用户反馈
func (h *AgentHandler) HandleSubmitFeedback(c *fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id"`
		MessageID string `json:"message_id"`
		Rating    int    `json:"rating"`
		Comment   string `json:"comment"`
		Accepted  bool   `json:"accepted"`
		Modified  bool   `json:"modified"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString("Invalid request body")
	}

	if req.Rating < 1 || req.Rating > 5 {
		return c.Status(400).SendString("Rating must be between 1 and 5")
	}

	runner := h.runner
	if runner == nil || runner.FeedbackCollector() == nil {
		return c.Status(503).SendString("Feedback collector not available")
	}

	runner.FeedbackCollector().RecordFeedback(agent.FeedbackRecord{
		SessionID: req.SessionID,
		MessageID: req.MessageID,
		Rating:    req.Rating,
		Comment:   req.Comment,
		Accepted:  req.Accepted,
		Modified:  req.Modified,
	})

	return c.JSON(fiber.Map{"success": true})
}

// HandleGetFeedbackStats 获取反馈统计
func (h *AgentHandler) HandleGetFeedbackStats(c *fiber.Ctx) error {
	runner := h.runner
	if runner == nil || runner.FeedbackCollector() == nil {
		return c.Status(503).SendString("Feedback collector not available")
	}

	stats := runner.FeedbackCollector().GetFeedbackStats()
	return c.JSON(stats)
}

// HandleGetRecentFeedbacks 获取最近的反馈
func (h *AgentHandler) HandleGetRecentFeedbacks(c *fiber.Ctx) error {
	runner := h.runner
	if runner == nil || runner.FeedbackCollector() == nil {
		return c.Status(503).SendString("Feedback collector not available")
	}

	limit := c.QueryInt("n", 20)
	feedbacks := runner.FeedbackCollector().GetRecentFeedbacks(limit)
	return c.JSON(feedbacks)
}
