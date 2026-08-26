package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent"
)

// HandleSubmitFeedback 提交用户反馈
func (h *AIHandler) HandleSubmitFeedback(c fiber.Ctx) error {
	var req struct {
		SessionID string `json:"session_id"`
		MessageID string `json:"message_id"`
		Rating    int    `json:"rating"`
		Comment   string `json:"comment"`
		Accepted  bool   `json:"accepted"`
		Modified  bool   `json:"modified"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.Rating < 1 || req.Rating > 5 {
		return BadRequest(c, "Rating must be between 1 and 5")
	}

	// 记录反馈
	h.runner.FeedbackCollector().RecordFeedback(agent.FeedbackRecord{
		SessionID: req.SessionID,
		MessageID: req.MessageID,
		Rating:    req.Rating,
		Comment:   req.Comment,
		Accepted:  req.Accepted,
		Modified:  req.Modified,
	})

	return c.Status(200).JSON(fiber.Map{"success": true})
}

// HandleGetFeedbackStats 获取反馈统计
func (h *AIHandler) HandleGetFeedbackStats(c fiber.Ctx) error {
	stats := h.runner.FeedbackCollector().GetFeedbackStats()
	return c.Status(200).JSON(stats)
}

// HandleGetRecentFeedbacks 获取最近的反馈
func (h *AIHandler) HandleGetRecentFeedbacks(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("n", "20"))
	feedbacks := h.runner.FeedbackCollector().GetRecentFeedbacks(limit)
	return c.Status(200).JSON(feedbacks)
}
