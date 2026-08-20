package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent"
)

// ═══════════════════════════════════════════════════════════════════
// Statistics and monitoring endpoints
// ═══════════════════════════════════════════════════════════════════

// GetToolStats returns tool usage statistics.
func (h *AgentHandler) GetToolStats(c fiber.Ctx) error {
	stats := h.runner.GetToolStats()
	return c.JSON(fiber.Map{"stats": stats})
}

// GetAgentMetrics returns aggregated process-lifetime performance metrics
// plus daily usage history (from ai_usage_daily) for the observability UI.
func (h *AgentHandler) GetAgentMetrics(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	// Free models resolve to price 0 and never accumulate cost, so this is
	// safe for the built-in catalog as well as custom endpoints (unknown ->
	// price 0, conservative for the cap).
	pi, po := agent.ModelPricer(h.cfg.LLMModelID)
	costInfo := h.runner.CalcMonthlyCostInfo(uid, pi, po)
	prefixStats := map[string]interface{}{}
	if h.runner.PrefixCache() != nil {
		if s := h.runner.PrefixCache().GetStats(); s != nil {
			prefixStats = s
		}
	}
	return c.JSON(fiber.Map{
		"metrics":       h.runner.GetPerfMetrics(),
		"daily":         h.runner.GetDailyUsage(30, uid),
		"monthly_cost":  costInfo,
		"prefix_cache":  prefixStats,
	})
}

// GetAuditHistory returns recent audit entries.
func (h *AgentHandler) GetAuditHistory(c fiber.Ctx) error {
	toolName := c.Query("tool", "")
	limitStr := c.Query("limit", "50")
	limit := 50
	fmt.Sscanf(limitStr, "%d", &limit)
	entries := h.runner.GetAuditHistory(toolName, limit)
	return c.JSON(fiber.Map{"entries": entries})
}

// GetPermissionDenials returns recent permission denials.
func (h *AgentHandler) GetPermissionDenials(c fiber.Ctx) error {
	limitStr := c.Query("limit", "50")
	limit := 50
	fmt.Sscanf(limitStr, "%d", &limit)
	denials := h.runner.GetPermissionDenials(limit)
	return c.JSON(fiber.Map{"denials": denials})
}

// GetSecurityAuditLog returns recent security audit entries.
func (h *AgentHandler) GetSecurityAuditLog(c fiber.Ctx) error {
	limitStr := c.Query("limit", "100")
	limit := 100
	fmt.Sscanf(limitStr, "%d", &limit)
	entries := h.runner.GetSecurityAuditLog(limit)
	return c.JSON(fiber.Map{"entries": entries})
}

// GetSecurityRules returns all security rules.
func (h *AgentHandler) GetSecurityRules(c fiber.Ctx) error {
	rules := h.runner.GetSecurityRules()
	return c.JSON(fiber.Map{"rules": rules})
}

// CheckCommandSecurity checks a command against security rules.
func (h *AgentHandler) CheckCommandSecurity(c fiber.Ctx) error {
	var req struct {
		Command string `json:"command"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Command == "" {
		return BadRequest(c, "command required")
	}
	level, riskScore, rules := h.runner.CheckCommandSecurity(req.Command)
	return c.JSON(fiber.Map{
		"command":    req.Command,
		"level":      level,
		"risk_score": riskScore,
		"rules":      rules,
	})
}
