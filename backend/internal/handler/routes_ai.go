package handler

import (
	"github.com/moduforge/backend/internal/service"
)

func registerAIRoutes(ctx *routeContext) {
	db := ctx.db
	cfg := ctx.cfg
	fileRepo := ctx.fileRepo

	// AI Analytics (protected — admin sees all data, normal user sees only own data)
	analyticsH := NewAIAnalyticsHandler(db.Conn)
	ctx.r("GET", "/analytics/overview", analyticsH.Overview)
	ctx.r("GET", "/analytics/users", analyticsH.Users)
	ctx.r("GET", "/analytics/models", analyticsH.Models)
	ctx.r("GET", "/analytics/timeline", analyticsH.Timeline)
	ctx.r("GET", "/analytics/modes", analyticsH.Modes)

	aiSvc := service.NewAIServiceWithDB(cfg, db.Conn)
	aiStreamSvc := service.NewAIStreamServiceWithConfig(cfg)

	aiH := NewAIHandler(aiSvc, cfg, db)
	aiH.SetMemoryStore(service.NewMemoryStore(db.Conn))
	aiH.SetFileContentRepo(fileRepo)
	aiStreamH := NewAIStreamHandler(aiStreamSvc)

	// AI prompts (public)
	ctx.api.Get("/ai/prompts", OptionalAuth(cfg.JWTSecret), aiH.GetPrompts)

	// LLM Providers (public)
	ctx.api.Get("/llm/providers", OptionalAuth(cfg.JWTSecret), CacheMiddleware(ctx.cache), aiH.ListProviders)
	ctx.api.Get("/llm/refresh", aiH.RefreshModels)

	// ── PROTECTED ──

	// LLM config
	ctx.r("POST", "/llm/config", aiH.UpdateLLMConfig)
	ctx.r("GET", "/llm/config", aiH.GetLLMConfig)

	// Provider config
	ctx.r("PUT", "/llm/provider-config", aiH.SaveProviderConfig)
	ctx.r("GET", "/llm/provider-configs", aiH.GetProviderConfigs)
	ctx.r("DELETE", "/llm/provider-config/:id", aiH.DeleteProviderConfig)

	// Custom providers
	ctx.r("POST", "/llm/custom-providers", aiH.CreateCustomProvider)
	ctx.r("GET", "/llm/custom-providers", aiH.GetCustomProviders)
	ctx.r("PUT", "/llm/custom-providers/:id", aiH.UpdateCustomProvider)
	ctx.r("DELETE", "/llm/custom-providers/:id", aiH.DeleteCustomProvider)

	// AI prompts (protected)
	ctx.r("PUT", "/ai/prompts", aiH.UpdatePrompt)
	ctx.r("POST", "/ai/prompts/:mode/reset", aiH.ResetPrompt)

	// MD-based prompts
	ctx.r("GET", "/md-prompts", NewMDPromptsHandler().ListMDPrompts)
	ctx.r("GET", "/md-prompts/:name", NewMDPromptsHandler().GetMDPrompt)
	ctx.r("PUT", "/md-prompts/:name", NewMDPromptsHandler().UpdateMDPrompt)
	ctx.r("POST", "/md-prompts/:name/reset", NewMDPromptsHandler().ResetMDPrompt)
	ctx.r("POST", "/md-prompts/reload", NewMDPromptsHandler().ReloadMDPrompts)

	// Skills API
	ctx.r("GET", "/skills", NewSkillsHandler(nil).ListSkills)
	ctx.r("GET", "/skills/:name", NewSkillsHandler(nil).GetSkill)
	ctx.r("POST", "/skills/:name/execute", NewSkillsHandler(nil).ExecuteSkill)

	// AI capability
	ctx.r("GET", "/ai/capability", aiH.GetAICapability)
	ctx.r("GET", "/ai/history/:session_id", aiH.GetHistory)
	ctx.r("DELETE", "/ai/history/:session_id", aiH.DeleteHistory)

	// AI memory
	ctx.r("GET", "/ai/memory", aiH.ListMemory)
	ctx.r("DELETE", "/ai/memory/:type/:key", aiH.DeleteMemory)
	ctx.r("DELETE", "/ai/memory", aiH.ClearMemory)

	// Memory V2
	ctx.r("GET", "/ai/memory/project/:project_id", aiH.GetProjectKnowledge)
	ctx.r("POST", "/ai/memory/project/:project_id", aiH.SaveProjectKnowledge)
	ctx.r("DELETE", "/ai/memory/project/:project_id/:category/:key", aiH.DeleteProjectKnowledge)
	ctx.r("GET", "/ai/memory/summaries/:project_id", aiH.GetProjectSummaries)
	ctx.r("POST", "/ai/memory/summarize", aiH.GenerateSummary)

	// Agent
	agentH := NewAgentHandler(cfg, db)
	aiH.SetRunner(agentH.GetRunner())
	ctx.r("POST", "/agent/run", agentH.Run)
	ctx.r("GET", "/agent/skills", agentH.ListSkills)
	ctx.r("GET", "/agent/mcp/status", agentH.ListMCPStatus)
	ctx.r("GET", "/agent/mcp/servers", agentH.ListMCPServers)
	ctx.r("POST", "/agent/mcp/servers", agentH.AddMCPServer)
	ctx.r("PUT", "/agent/mcp/servers/:name", agentH.UpdateMCPServer)
	ctx.r("DELETE", "/agent/mcp/servers/:name", agentH.DeleteMCPServer)
	ctx.r("POST", "/agent/mcp/servers/:name/reconnect", agentH.ReconnectMCPServer)
	ctx.r("GET", "/agent/mcp/policies", agentH.ListMCPPolicies)
	ctx.r("PUT", "/agent/mcp/policies/:server/:tool", agentH.SetMCPPolicy)
	ctx.r("POST", "/agent/mcp/confirm", agentH.ConfirmMCPApproval)
	ctx.r("POST", "/agent/mcp/test", agentH.TestMCPTool)
	ctx.r("GET", "/agent/custom-skills", agentH.ListCustomSkills)
	ctx.r("POST", "/agent/custom-skills", agentH.CreateCustomSkill)
	ctx.r("PUT", "/agent/custom-skills/:id", agentH.UpdateCustomSkill)
	ctx.r("DELETE", "/agent/custom-skills/:id", agentH.DeleteCustomSkill)
	ctx.r("POST", "/agent/custom-skills/:id/execute", agentH.ExecuteCustomSkill)
	ctx.r("GET", "/agent/custom-skills/:id/evolution", agentH.GetSkillEvolution)
	ctx.r("POST", "/agent/custom-skills/:id/evolution", agentH.RecordSkillEvolution)
	ctx.r("GET", "/agent/custom-skills/:id/optimize", agentH.GetSkillOptimization)
	ctx.r("GET", "/agent/stats", agentH.GetToolStats)
	ctx.r("GET", "/agent/metrics", agentH.GetAgentMetrics)
	ctx.r("GET", "/agent/cache", agentH.GetCacheStats)
	ctx.r("GET", "/agent/audit", agentH.GetAuditHistory)
	ctx.r("GET", "/agent/denials", agentH.GetPermissionDenials)
	ctx.r("GET", "/agent/security/audit", agentH.GetSecurityAuditLog)
	ctx.r("GET", "/agent/security/rules", agentH.GetSecurityRules)
	ctx.r("POST", "/agent/security/check", agentH.CheckCommandSecurity)
	ctx.r("GET", "/agent/session/:sessionId", agentH.GetSessionState)
	ctx.r("GET", "/agent/sessions", agentH.ListSessions)
	ctx.r("GET", "/agent/sessions/:id", agentH.GetSession)

	// Build progress
	ctx.r("GET", "/builds/:id/progress", aiH.StreamBuildProgress)

	// AI rate-limited
	ctx.rA("POST", "/ai/generate", aiH.GenerateModule)
	ctx.rA("POST", "/ai/chat", aiH.Chat)
	ctx.rA("POST", "/ai/repair", aiH.RepairBuild)
	ctx.rA("POST", "/ai/gather", aiH.GatherRequirements)
	ctx.rA("POST", "/ai/stream", aiStreamH.StreamChat)
	ctx.rA("POST", "/ai/compare", aiH.CompareModels)
	ctx.rA("POST", "/ai/auto-build", aiH.AutoBuild)

	// AI conversations
	ctx.r("GET", "/ai/conversations", aiH.ListConversations)
	ctx.r("POST", "/ai/conversations", aiH.SaveConversation)
	ctx.r("GET", "/ai/conversations/:id", aiH.GetConversation)
	ctx.r("DELETE", "/ai/conversations/:id", aiH.DeleteConversation)
	ctx.r("GET", "/ai/sessions", aiH.ListSessions)
	ctx.r("GET", "/ai/sessions/:session_id/messages", aiH.GetSessionMessages)
	ctx.r("DELETE", "/ai/sessions/:session_id", aiH.DeleteSession)
	ctx.r("PUT", "/ai/sessions/:session_id/title", aiH.RenameSession)
	ctx.r("GET", "/ai/sessions/:session_id/export", aiH.ExportSession)
	ctx.r("GET", "/ai/sessions/search", aiH.SearchSessions)
	ctx.r("POST", "/ai/diff", aiH.ComputeDiff)
}
