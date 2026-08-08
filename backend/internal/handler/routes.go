package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/middleware"
	"github.com/moduforge/backend/internal/service"
)

func reg3(api fiber.Router, method, path string, mw1, mw2, mw3 fiber.Handler, handler fiber.Handler) {
	switch method {
	case "GET":
		api.Get(path, mw1, mw2, mw3, handler)
	case "POST":
		api.Post(path, mw1, mw2, mw3, handler)
	case "PUT":
		api.Put(path, mw1, mw2, mw3, handler)
	case "DELETE":
		api.Delete(path, mw1, mw2, mw3, handler)
	}
}

func reg4(api fiber.Router, method, path string, mw1, mw2, mw3, mw4 fiber.Handler, handler fiber.Handler) {
	switch method {
	case "GET":
		api.Get(path, mw1, mw2, mw3, mw4, handler)
	case "POST":
		api.Post(path, mw1, mw2, mw3, mw4, handler)
	case "PUT":
		api.Put(path, mw1, mw2, mw3, mw4, handler)
	case "DELETE":
		api.Delete(path, mw1, mw2, mw3, mw4, handler)
	}
}

func RegisterRoutes(api fiber.Router, db *database.DB, cfg *config.Config) {
	// Rate limiters
	rateLimiter := middleware.NewRateLimiter()
	cache := GetCache()

	// Auth middleware (reusable — not in a group)
	authMW := middleware.APIKeyAuth(db.Conn)
	jwtMW := AuthMiddleware(cfg.JWTSecret)
	rateAuth := middleware.RateLimit(rateLimiter, cfg.RateLimitAuth, cfg.RateLimitAuth/60)
	rateAI := middleware.RateLimit(rateLimiter, cfg.RateLimitAI, cfg.RateLimitAI/60)
	adminMW := AdminOnly()

	// Services
	authSvc := service.NewAuthService(db.Conn, cfg)
	projectSvc := service.NewProjectService(db.Conn)
	buildSvc := service.NewBuildService(db.Conn, cfg)
	aiSvc := service.NewAIServiceWithDB(cfg, db.Conn)
	repoSvc := service.NewRepoService()
	templateSvc := service.NewTemplateService()
	translateSvc := service.NewTranslateService()
	aiStreamSvc := service.NewAIStreamService()
	buildLogSvc := service.NewBuildLogService(cfg.StoragePath + "/logs")
	emailSvc := service.NewEmailService(db.Conn)

	// Handlers
	authH := NewAuthHandler(authSvc)
	projectH := NewProjectHandlerWithDB(projectSvc, db.Conn)
	buildH := NewBuildHandler(buildSvc)
	buildScheduleSvc := service.NewBuildScheduleService(db.Conn, buildSvc)
	buildScheduleH := NewBuildScheduleHandler(buildScheduleSvc)
	buildScheduleSvc.StartScheduler(context.Background())
	aiH := NewAIHandler(aiSvc, cfg, db)
	aiH.SetMemoryStore(service.NewMemoryStore(db.Conn))
	repoH := NewRepoHandler(repoSvc)
	templateH := NewTemplateHandler(templateSvc)
	translateH := NewTranslateHandler(translateSvc)
	aiStreamH := NewAIStreamHandler(aiStreamSvc)
	buildLogH := NewBuildLogHandler(buildLogSvc)
	settingsH := NewSettingsHandler(emailSvc, db.Conn)

	moduleVersionH := NewModuleVersionHandler(db.Conn)
	templateMarketH := NewTemplateMarketHandler(db.Conn)
	dependencyH := NewDependencyHandler(db.Conn)

	signingH := NewSigningHandler(db.Conn, cfg.StoragePath+"/keys")
	vulnH := NewVulnerabilityHandler(db.Conn)
	permAuditH := NewPermissionAuditHandler(db.Conn)

	securitySvc := service.NewSecurityScanner()
	securityH := NewSecurityHandler(securitySvc, db.Conn)

	zipperSvc := service.NewZipperService(cfg.StoragePath+"/downloads", db.Conn)
	zipperH := NewZipperHandler(zipperSvc)

	adbSvc := service.NewADBService(db.Conn)
	adbH := NewADBHandler(adbSvc)

	updateSvc := service.NewUpdateService(db.Conn)
	updateH := NewUpdateHandler(updateSvc)

	gitSvc := service.NewGitManagerService(cfg.StoragePath + "/projects")
	gitH := NewGitHandler(gitSvc)

	marketSvc := service.NewSQLiteMarketService(db)
	marketH := NewMarketHandlerWithDB(marketSvc, cfg.StoragePath, db.Conn)

	analyticsSvc := service.NewAnalyticsService(db.Conn)
	analyticsH := NewAnalyticsHandler(analyticsSvc)
	tagsH := NewTagsHandler(db.Conn)
	pluginSvc := service.NewPluginService(db.Conn)
	pluginH := NewPluginHandler(pluginSvc)
	badgeSvc := service.NewBadgeService(db.Conn)
	badgeH := NewBadgeHandler(badgeSvc)
	dashboardH := NewDashboardHandler(db.Conn)
	healthH := NewHealthHandler(db.Conn)
	healthH.SetLLMURL(cfg.LLMEndpoint)
	healthH.SetADBAddr(cfg.ADBAddress)
	glossaryH := NewGlossaryHandler(db.Conn)
	validatorSvc := service.NewValidatorService()
	validatorH := NewValidatorHandler(validatorSvc)
	signerSvc := service.NewSignerService("data/keys")
	signerH := NewSignerHandler(signerSvc)
	benchH := NewBenchmarkHandler(adbSvc)
	mirrorH := NewMirrorHandler(adbSvc)
	crashH := NewCrashHandler(db.Conn)

	// Shorthand: r = register protected route
	r := func(method, path string, h fiber.Handler) { reg3(api, method, path, authMW, jwtMW, rateAuth, h) }
	rA := func(method, path string, h fiber.Handler) { reg3(api, method, path, authMW, jwtMW, rateAI, h) }
	rAdmin := func(method, path string, h fiber.Handler) { reg4(api, method, path, authMW, jwtMW, rateAuth, adminMW, h) }

	// ============================================================================
	// PUBLIC ROUTES
	// ============================================================================

	// OpenAPI documentation
	openapiH := NewOpenAPIHandler()
	api.Get("/openapi.json", openapiH.ServeJSON)
	api.Get("/openapi.yaml", openapiH.ServeYAML)
	api.Get("/docs", openapiH.ServeSwaggerUI)

	// Auth — stricter rate limiting for login/register/forgot-password
	// 10 requests per minute per IP for auth endpoints (anti-brute-force)
	rateAuthStrict := middleware.RateLimit(rateLimiter, 15, 10)
	api.Post("/auth/register", rateAuthStrict, authH.Register)
	api.Post("/auth/login", rateAuthStrict, authH.Login)
	api.Post("/auth/refresh", authH.Refresh)
	api.Post("/auth/verify-email", authH.VerifyEmail)
	api.Post("/auth/forgot-password", rateAuthStrict, authH.ForgotPassword)
	api.Post("/auth/reset-password", rateAuthStrict, authH.ResetPassword)
	// change-password requires authentication (moved from public to protected)
	r("POST", "/auth/change-password", authH.ChangePassword)

	// Templates
	api.Get("/templates", CacheMiddleware(cache), projectH.ListTemplates)
	api.Get("/templates/list", CacheMiddleware(cache), templateH.List)
	api.Get("/templates/market", templateMarketH.ListTemplates)
	api.Get("/templates/market/trending", templateMarketH.GetTrending)
	api.Get("/templates/market/categories", templateMarketH.GetCategories)
	api.Get("/templates/:name", CacheMiddleware(cache), templateH.Get)
	api.Post("/templates/recommend", templateH.Recommend)

	// AI prompts (public)
	api.Get("/ai/prompts", OptionalAuth(cfg.JWTSecret), aiH.GetPrompts)

	// LLM Providers
	api.Get("/llm/providers", OptionalAuth(cfg.JWTSecret), CacheMiddleware(cache), aiH.ListProviders)
	api.Get("/llm/refresh", aiH.RefreshModels)

	// Repo
	api.Post("/repo/fetch", repoH.Fetch)
	api.Post("/repo/files", repoH.FetchFiles)

	// Translate
	api.Post("/translate", translateH.Translate)
	api.Post("/translate/props", translateH.TranslateProps)

	// Validator
	api.Post("/validate", validatorH.ValidateFiles)
	api.Post("/validate/file", validatorH.ValidateFile)

	// Zipper
	api.Post("/build/zip", zipperH.BuildZip)
	api.Get("/build/downloads", zipperH.ListDownloads)

	// Module ZIP parse
	api.Post("/module/parse-zip", ParseModuleZip)

	// Signer (protected)
	r("POST", "/sign", signerH.Sign)
	api.Post("/verify", signerH.Verify)

	// ADB (public read)
	api.Get("/adb/check", adbH.CheckADB)
	api.Get("/adb/status", adbH.GetServerStatus)
	api.Get("/adb/devices", adbH.ListDevices)
	api.Get("/adb/device-info", adbH.GetDeviceInfo)
	api.Get("/adb/files", adbH.ListFiles)
	api.Get("/adb/apps", adbH.ListApps)
	api.Get("/adb/modules", adbH.ListInstalledModules)
	api.Get("/adb/modules/:name", adbH.GetModuleInfo)
	api.Get("/adb/logcat", adbH.GetLogcat)
	api.Post("/adb/benchmark", benchH.Benchmark)
	api.Get("/adb/mirror", mirrorH.Mirror)

	// Module update
	api.Post("/update/check", updateH.CheckUpdate)
	api.Post("/update/check-all", updateH.CheckAllUpdates)

	// Git (protected — project data)
	r("GET", "/git/commits", gitH.ListCommits)
	r("GET", "/git/diff", gitH.GetDiff)
	r("GET", "/git/head", gitH.GetCurrentHash)
	r("GET", "/git/branches", gitH.ListBranches)
	r("GET", "/git/branch", gitH.GetCurrentBranch)

	// Market (public)
	api.Get("/market/modules", CacheMiddleware(cache), marketH.ListModules)
	api.Get("/market/trending", CacheMiddleware(cache), marketH.Trending)
	api.Get("/market/categories", CacheMiddleware(cache), marketH.Categories)
	api.Get("/market/module/:slug", marketH.GetModule)
	api.Get("/market/module/:slug/reviews", marketH.GetReviews)
	api.Get("/market/module/:slug/dependencies", marketH.GetModuleDependencies)
	api.Post("/market/module/:slug/check-deps", marketH.CheckDependencyConflicts)
	api.Post("/market/compare", marketH.Compare)
	api.Get("/market/module/:slug/demo", marketH.GetDemo)
	api.Get("/market/module/:slug/changelogs", marketH.GetChangelogs)
	api.Get("/market/module/:slug/tags", tagsH.GetModuleTags)
	api.Get("/market/module/:slug/health", marketH.GetModuleHealth)
	api.Get("/market/module/:slug/install-stats", marketH.GetInstallStats)
	api.Get("/market/module/:slug/download", marketH.DownloadModule)
	api.Get("/market/stats/trending", marketH.GetTrending)

	// Build log
	api.Get("/build/log", buildLogH.GetBuildLog)

	// Analytics
	api.Get("/analytics/module-stats", analyticsH.ModuleStats)

	// Plugins (read)
	api.Get("/plugins", pluginH.List)

	// Tags
	api.Get("/tags", tagsH.List)

	// Badges (public)
	api.Get("/badges/definitions", badgeH.Definitions)
	api.Get("/badges/user/:id", badgeH.UserBadges)

	// Dashboard
	api.Get("/dashboard/widget-types", dashboardH.GetWidgetTypes)

	// Health
	api.Get("/health/system", healthH.Check)
	api.Get("/health/cache", healthH.CacheStats)

	// Glossary
	api.Get("/glossary", glossaryH.List)
	api.Get("/glossary/popular", glossaryH.Popular)
	api.Get("/glossary/:id", glossaryH.Get)

	// Crash report
	api.Post("/crash/report", crashH.Report)

	// ============================================================================
	// PROTECTED ROUTES — authMW + jwtMW + rateAuth applied inline
	// ============================================================================

	// 2FA
	r("POST", "/auth/2fa/setup", authH.Setup2FA)
	r("POST", "/auth/2fa/enable", authH.Enable2FA)
	r("POST", "/auth/2fa/disable", authH.Disable2FA)

	// Profile
	r("GET", "/auth/profile", authH.GetProfile)
	r("PUT", "/auth/profile", authH.UpdateProfile)
	r("POST", "/auth/avatar", authH.UploadAvatar)
	r("POST", "/auth/resend-verification", authH.ResendVerification)

	// LLM config
	r("POST", "/llm/config", aiH.UpdateLLMConfig)
	r("GET", "/llm/config", aiH.GetLLMConfig)

	// Provider config
	r("PUT", "/llm/provider-config", aiH.SaveProviderConfig)
	r("GET", "/llm/provider-configs", aiH.GetProviderConfigs)
	r("DELETE", "/llm/provider-config/:id", aiH.DeleteProviderConfig)

	// Custom providers
	r("POST", "/llm/custom-providers", aiH.CreateCustomProvider)
	r("GET", "/llm/custom-providers", aiH.GetCustomProviders)
	r("PUT", "/llm/custom-providers/:id", aiH.UpdateCustomProvider)
	r("DELETE", "/llm/custom-providers/:id", aiH.DeleteCustomProvider)

	// AI prompts
	r("PUT", "/ai/prompts", aiH.UpdatePrompt)
	r("POST", "/ai/prompts/:mode/reset", aiH.ResetPrompt)

	// MD-based prompts (embedded prompts for agent)
	r("GET", "/md-prompts", NewMDPromptsHandler().ListMDPrompts)
	r("GET", "/md-prompts/:name", NewMDPromptsHandler().GetMDPrompt)
	r("PUT", "/md-prompts/:name", NewMDPromptsHandler().UpdateMDPrompt)
	r("POST", "/md-prompts/:name/reset", NewMDPromptsHandler().ResetMDPrompt)
	r("POST", "/md-prompts/reload", NewMDPromptsHandler().ReloadMDPrompts)

	// Skills API (list and execute agent skills)
	r("GET", "/skills", NewSkillsHandler(nil).ListSkills)
	r("GET", "/skills/:name", NewSkillsHandler(nil).GetSkill)
	r("POST", "/skills/:name/execute", NewSkillsHandler(nil).ExecuteSkill)

	// AI capability scoring
	r("GET", "/ai/capability", aiH.GetAICapability)

	// AI history
	r("GET", "/ai/history/:session_id", aiH.GetHistory)
	r("DELETE", "/ai/history/:session_id", aiH.DeleteHistory)

	// AI memory
	r("GET", "/ai/memory", aiH.ListMemory)
	r("DELETE", "/ai/memory/:type/:key", aiH.DeleteMemory)
	r("DELETE", "/ai/memory", aiH.ClearMemory)

	// Memory V2 — project knowledge & session summaries
	r("GET", "/ai/memory/project/:project_id", aiH.GetProjectKnowledge)
	r("POST", "/ai/memory/project/:project_id", aiH.SaveProjectKnowledge)
	r("DELETE", "/ai/memory/project/:project_id/:category/:key", aiH.DeleteProjectKnowledge)
	r("GET", "/ai/memory/summaries/:project_id", aiH.GetProjectSummaries)
	r("POST", "/ai/memory/summarize", aiH.GenerateSummary)

	// ADB server control
	r("POST", "/adb/start-server", adbH.StartServer)
	r("POST", "/adb/kill-server", adbH.KillServer)

	// ADB connection
	r("POST", "/adb/connect", adbH.ConnectDevice)
	r("POST", "/adb/pair", adbH.PairDevice)
	r("GET", "/adb/diagnose", adbH.DiagnoseDevice)
	r("POST", "/adb/disconnect", adbH.DisconnectDevice)
	r("POST", "/adb/disconnect-all", adbH.DisconnectAll)

	// ADB saved devices
	r("GET", "/adb/saved-devices", adbH.GetSavedDevices)
	r("POST", "/adb/saved-devices", adbH.SaveDevice)
	r("DELETE", "/adb/saved-devices/:id", adbH.DeleteSavedDevice)

	// ADB shell/exec
	r("POST", "/adb/shell", adbH.RunShell)
	r("POST", "/adb/exec", adbH.RunExec)

	// ADB file management
	r("POST", "/adb/push", adbH.PushFile)
	r("POST", "/adb/pull", adbH.PullFile)
	r("POST", "/adb/delete", adbH.DeleteFile)
	r("POST", "/adb/mkdir", adbH.MakeDir)
	r("POST", "/adb/rename", adbH.RenameFile)
	r("POST", "/adb/file/read", adbH.ReadFile)
	r("POST", "/adb/file/write", adbH.WriteFile)
	r("POST", "/adb/file/copy", adbH.CopyFile)
	r("GET", "/adb/file/info", adbH.GetFileInfo)
	r("POST", "/adb/file/upload", adbH.UploadFile)
	r("GET", "/adb/file/download", adbH.DownloadFile)

	// ADB app management
	r("POST", "/adb/app/install", adbH.InstallApp)
	r("POST", "/adb/app/uninstall", adbH.UninstallApp)
	r("POST", "/adb/app/clear-data", adbH.ClearAppData)
	r("POST", "/adb/app/force-stop", adbH.ForceStopApp)
	r("POST", "/adb/app/launch", adbH.LaunchApp)
	r("POST", "/adb/app/toggle", adbH.ToggleApp)

	// ADB module management
	r("POST", "/adb/install", adbH.InstallModule)
	r("POST", "/adb/modules/:name/toggle", adbH.ToggleModule)
	r("POST", "/adb/modules/:name/uninstall", adbH.UninstallModule)
	r("POST", "/adb/module/install-url", adbH.InstallModuleFromURL)
	r("POST", "/adb/module/upload", adbH.UploadAndInstallModule)
	r("POST", "/adb/module/backup", adbH.BackupModule)
	r("POST", "/adb/module/restore", adbH.RestoreModule)
	r("GET", "/adb/module/check-update", adbH.CheckModuleUpdate)
	r("POST", "/adb/module/export", adbH.ExportModule)

	// ADB root manager
	r("GET", "/adb/root/managers", adbH.GetAvailableRootManagers)
	r("POST", "/adb/root/permission", adbH.ManageRootPermission)
	r("GET", "/adb/root/permissions", adbH.ListRootPermissions)
	r("GET", "/adb/root/modules", adbH.GetRootModules)

	// ADB screen
	r("GET", "/adb/screen/screenshot", adbH.ScreenshotBase64)
	r("GET", "/adb/screen/size", adbH.GetScreenSize)
	r("POST", "/adb/screen/tap", adbH.TapScreen)
	r("POST", "/adb/screen/swipe", adbH.SwipeScreen)
	r("POST", "/adb/screen/input", adbH.InputText)
	r("POST", "/adb/screen/key", adbH.KeyEvent)
	r("POST", "/adb/screen/record", adbH.ScreenRecord)

	// ADB screenshot
	screenshotH := NewScreenshotHandler(adbSvc)
	r("GET", "/adb/screenshot", screenshotH.Screenshot)
	r("GET", "/adb/screenshot/stream", screenshotH.StreamScreenshots)

	// ADB device ops
	r("POST", "/adb/reboot", adbH.RebootDevice)
	r("POST", "/adb/prop/set", adbH.SetProp)
	r("POST", "/adb/logcat/clear", adbH.ClearLogcat)

	// Benchmark
	benchmarkSvc := service.NewBenchmarkService(db.Conn)
	benchmarkAPIH := NewBenchmarkAPIHandler(benchmarkSvc, adbSvc)
	r("POST", "/benchmark/run", benchmarkAPIH.RunBenchmark)
	r("GET", "/benchmark/history", benchmarkAPIH.GetHistory)

	// Security
	r("POST", "/security/scan", securityH.ScanFiles)
	r("POST", "/security/scan-project/:id", securityH.ScanProject)
	r("POST", "/projects/:id/scan-vulns", securityH.ScanVulnerabilities)
	r("GET", "/projects/:id/vuln-history", securityH.GetVulnHistory)

	// Module import
	r("POST", "/module/import-zip", ImportModuleZip)

	// Market write
	r("POST", "/market/module/:slug/star", marketH.StarModule)
	r("POST", "/market/publish", marketH.Publish)
	r("POST", "/market/module/:slug/review", marketH.AddReview)
	r("POST", "/market/batch/install", marketH.BatchInstall)
	r("POST", "/market/batch/uninstall", marketH.BatchUninstall)
	r("POST", "/market/batch/update", marketH.BatchUpdate)

	// Projects CRUD
	r("GET", "/projects", projectH.List)
	r("POST", "/projects", projectH.Create)
	r("GET", "/projects/:id", projectH.Get)
	r("PUT", "/projects/:id", projectH.Update)
	r("DELETE", "/projects/:id", projectH.Delete)
	r("GET", "/search", projectH.Search)

	// File comments
	fileCommentH := NewFileCommentHandler(db.Conn)
	r("POST", "/projects/:id/files/*/comments", fileCommentH.AddComment)
	r("GET", "/projects/:id/files/*/comments", fileCommentH.GetComments)
	r("DELETE", "/projects/:id/comments/:comment_id", fileCommentH.DeleteComment)
	r("POST", "/projects/:id/comments/:comment_id/reply", fileCommentH.ReplyToComment)
	r("GET", "/projects/:id/file-comments", fileCommentH.ListProjectComments)

	// Project files
	r("GET", "/projects/:id/files", projectH.ListFiles)
	r("GET", "/projects/:id/files/*", projectH.GetFile)
	r("PUT", "/projects/:id/files/*", projectH.SaveFile)
	r("POST", "/projects/:id/files/upload", projectH.UploadFiles)
	r("DELETE", "/projects/:id/files/*", projectH.DeleteFile)
	r("POST", "/projects/:id/validate", projectH.ValidateProject)

	// Builds
	r("POST", "/projects/:id/build", buildH.Create)
	r("GET", "/projects/:id/builds", buildH.ListByProject)
	r("DELETE", "/projects/:id/builds/failed", buildH.DeleteFailed)
	r("DELETE", "/projects/:id/builds/:buildId", buildH.Delete)
	r("DELETE", "/projects/:id/build-cache", buildH.ClearBuildCache)
	r("GET", "/projects/:id/build/cache", buildH.GetBuildCacheStatus)
	r("GET", "/projects/:id/build/architectures", buildH.GetSupportedArchitectures)
	r("POST", "/projects/:id/build-schedules", buildScheduleH.Create)
	r("GET", "/projects/:id/build-schedules", buildScheduleH.List)
	r("PUT", "/projects/:id/build-schedules/:scheduleId", buildScheduleH.Toggle)
	r("DELETE", "/projects/:id/build-schedules/:scheduleId", buildScheduleH.Delete)
	r("GET", "/builds/:id", buildH.Get)
	r("GET", "/builds/:id/logs", buildH.StreamLogs)
	r("GET", "/builds/:id/download", buildH.Download)
	r("POST", "/projects/:id/build/auto", buildH.CreateAuto)
	r("POST", "/builds/:id/cancel", buildH.Cancel)
	r("GET", "/builds/:id/progress", aiH.StreamBuildProgress)

	// Global cache management (admin only for cleanup, all users can view stats)
	r("GET", "/cache/stats", buildH.GetGlobalCacheStats)
	rAdmin("POST", "/cache/cleanup", buildH.TriggerCacheCleanup)

	// Project ZIP export
	r("POST", "/projects/:id/export-zip", zipperH.ExportProjectZip)

	// Git write
	r("POST", "/git/commit", gitH.Commit)
	r("POST", "/git/checkout", gitH.Checkout)
	r("POST", "/git/branch", gitH.CreateBranch)
	r("POST", "/git/checkout-branch", gitH.CheckoutBranch)
	r("POST", "/git/push", gitH.Push)
	r("POST", "/git/pull", gitH.Pull)

	// API keys
	apiKeyH := NewAPIKeyHandler(db.Conn)
	r("GET", "/api-keys", apiKeyH.List)
	r("POST", "/api-keys", apiKeyH.Create)
	r("DELETE", "/api-keys/:id", apiKeyH.Delete)
	r("POST", "/api-keys/:id/rotate", apiKeyH.Rotate)

	// Agent
	agentH := NewAgentHandler(cfg, db)
	r("POST", "/agent/run", agentH.Run)
	r("GET", "/agent/skills", agentH.ListSkills)
	r("GET", "/agent/custom-skills", agentH.ListCustomSkills)
	r("POST", "/agent/custom-skills", agentH.CreateCustomSkill)
	r("PUT", "/agent/custom-skills/:id", agentH.UpdateCustomSkill)
	r("DELETE", "/agent/custom-skills/:id", agentH.DeleteCustomSkill)
	r("POST", "/agent/custom-skills/:id/execute", agentH.ExecuteCustomSkill)
	r("GET", "/agent/custom-skills/:id/evolution", agentH.GetSkillEvolution)
	r("POST", "/agent/custom-skills/:id/evolution", agentH.RecordSkillEvolution)
	r("GET", "/agent/custom-skills/:id/optimize", agentH.GetSkillOptimization)
	// NEW: Statistics and monitoring endpoints
	r("GET", "/agent/stats", agentH.GetToolStats)
	r("GET", "/agent/cache", agentH.GetCacheStats)
	r("GET", "/agent/audit", agentH.GetAuditHistory)
	r("GET", "/agent/denials", agentH.GetPermissionDenials)
	r("GET", "/agent/security/audit", agentH.GetSecurityAuditLog)
	r("GET", "/agent/security/rules", agentH.GetSecurityRules)
	r("POST", "/agent/security/check", agentH.CheckCommandSecurity)
	r("GET", "/agent/session/:sessionId", agentH.GetSessionState)
	// NEW: Agent session management
	r("GET", "/agent/sessions", agentH.ListSessions)
	r("GET", "/agent/sessions/:id", agentH.GetSession)

	// Collaboration
	collabSvc := service.NewCollaborationService(db.Conn)
	collabH := NewCollaborationHandler(collabSvc)
	r("POST", "/projects/:id/collaborators", collabH.AddCollaborator)
	r("GET", "/projects/:id/collaborators", collabH.ListCollaborators)
	r("DELETE", "/projects/:id/collaborators/:userId", collabH.RemoveCollaborator)
	r("POST", "/projects/:id/comments", collabH.AddComment)
	r("GET", "/projects/:id/comments", collabH.ListComments)
	r("POST", "/comments/:commentId/resolve", collabH.ResolveComment)
	r("POST", "/projects/:id/edit-session", collabH.UpsertEditSession)
	r("GET", "/projects/:id/edit-sessions", collabH.ListEditSessions)
	r("DELETE", "/projects/:id/edit-session/:sessionId", collabH.RemoveEditSession)

	// Team members
	r("POST", "/projects/:id/members", collabH.AddTeamMember)
	r("GET", "/projects/:id/members", collabH.ListTeamMembers)
	r("PUT", "/projects/:id/members/:userId", collabH.UpdateMemberRole)
	r("DELETE", "/projects/:id/members/:userId", collabH.RemoveTeamMember)
	r("GET", "/projects/:id/audit-logs", collabH.GetAuditLogs)

	// Collab status
	collabWS := NewCollaborationWS(collabSvc)
	r("GET", "/projects/:id/collab-status", collabWS.GetCollaborationStatus)

	// Webhook
	webhookH := NewWebhookHandler(cfg, buildSvc)
	r("POST", "/webhook/git", webhookH.HandleGitWebhook)

	// Plugin write
	r("POST", "/plugins/hooks/execute", pluginH.ExecuteHook)
	r("POST", "/plugins/install", pluginH.Install)
	r("POST", "/plugins/:id/enable", pluginH.Enable)
	r("POST", "/plugins/:id/disable", pluginH.Disable)
	r("DELETE", "/plugins/:id", pluginH.Uninstall)
	r("POST", "/plugins/:id/hooks", pluginH.RegisterHook)
	r("GET", "/plugins/:id/hooks", pluginH.GetHooks)

	// AI rate-limited
	rA("POST", "/ai/generate", aiH.GenerateModule)
	rA("POST", "/ai/chat", aiH.Chat)
	rA("POST", "/ai/repair", aiH.RepairBuild)
	rA("POST", "/ai/gather", aiH.GatherRequirements)
	rA("POST", "/ai/stream", aiStreamH.StreamChat)
	rA("POST", "/ai/compare", aiH.CompareModels)
	rA("POST", "/ai/auto-build", aiH.AutoBuild)

	// AI conversations
	r("GET", "/ai/conversations", aiH.ListConversations)
	r("POST", "/ai/conversations", aiH.SaveConversation)
	r("GET", "/ai/conversations/:id", aiH.GetConversation)
	r("DELETE", "/ai/conversations/:id", aiH.DeleteConversation)
	r("GET", "/ai/sessions", aiH.ListSessions)
	r("GET", "/ai/sessions/:session_id/messages", aiH.GetSessionMessages)
	r("DELETE", "/ai/sessions/:session_id", aiH.DeleteSession)
	r("GET", "/ai/sessions/:session_id/export", aiH.ExportSession)
	r("GET", "/ai/sessions/search", aiH.SearchSessions)
	r("POST", "/ai/diff", aiH.ComputeDiff)

	// Backup & Restore
	backupSvc := service.NewBackupService(db.Conn, cfg.StoragePath+"/backups")
	backupH := NewBackupHandler(backupSvc)
	r("POST", "/backup/export", backupH.ExportDatabase)
	r("POST", "/backup/import", backupH.ImportDatabase)
	r("POST", "/projects/:id/export", backupH.ExportProject)
	r("POST", "/projects/:id/import", backupH.ImportProject)

	// Notifications
	notifSvc := service.NewNotificationService(db.Conn)
	notifH := NewNotificationHandler(notifSvc)
	r("GET", "/notifications", notifH.List)
	r("GET", "/notifications/unread-count", notifH.UnreadCount)
	r("POST", "/notifications/:id/read", notifH.MarkRead)
	r("POST", "/notifications/read-all", notifH.MarkAllRead)
	r("DELETE", "/notifications/:id", notifH.Delete)

	// Activities
	activitySvc := service.NewActivityService(db.Conn)
	activityH := NewActivityHandler(activitySvc)
	r("GET", "/projects/:id/activities", activityH.GetProjectActivities)
	r("GET", "/activities", activityH.GetUserActivities)
	r("GET", "/activity/export", activityH.Export)

	// Crash (protected read/delete)
	r("GET", "/crash/logs", crashH.ListLogs)
	r("GET", "/crash/stats", crashH.Stats)
	r("DELETE", "/crash/logs/:id", crashH.Delete)
	r("DELETE", "/crash/logs", crashH.ClearAll)

	// Search history
	searchH := NewSearchHistoryHandler(db.Conn)
	r("GET", "/search/history", searchH.GetHistory)
	r("DELETE", "/search/history/:id", searchH.DeleteHistory)
	r("DELETE", "/search/history", searchH.ClearHistory)

	// Favorites
	favoritesH := NewFavoritesHandler(db.Conn)
	r("GET", "/favorites", favoritesH.List)
	r("POST", "/favorites", favoritesH.Add)
	r("DELETE", "/favorites/:type/:id", favoritesH.Remove)
	r("GET", "/favorites/check/:type/:id", favoritesH.Check)

	// Badges (my)
	r("GET", "/badges/my", badgeH.MyBadges)

	// Code formatting
	formatH := NewFormatHandler(db.Conn)
	r("POST", "/projects/:id/format", formatH.FormatProject)
	r("POST", "/projects/:id/format/preview", formatH.PreviewFormat)

	// Webhook deliveries
	webhookH.SetDB(db.Conn)
	r("GET", "/webhooks/:hookId/deliveries", webhookH.ListDeliveries)
	r("POST", "/webhooks/:hookId/test", webhookH.TestWebhook)
	r("DELETE", "/webhooks/deliveries/:id", webhookH.DeleteDelivery)
	r("GET", "/webhooks/deliveries/stats", webhookH.DeliveryStats)

	// Backup schedules
	r("GET", "/backup/schedules", backupH.ListSchedules)
	r("POST", "/backup/schedules", backupH.CreateSchedule)
	r("PUT", "/backup/schedules/:id", backupH.UpdateSchedule)
	r("DELETE", "/backup/schedules/:id", backupH.DeleteSchedule)
	r("POST", "/backup/schedules/:id/run", backupH.RunSchedule)

	// Recycle bin
	recycleH := NewRecycleHandler(db.Conn)
	r("GET", "/recycle-bin", recycleH.List)
	r("POST", "/recycle-bin/:type/:id/restore", recycleH.Restore)
	r("DELETE", "/recycle-bin/:type/:id", recycleH.PermanentlyDelete)
	r("DELETE", "/recycle-bin", recycleH.ClearAll)

	// Module system enhancements
	r("POST", "/projects/:id/versions", moduleVersionH.CreateVersion)
	r("GET", "/projects/:id/versions", moduleVersionH.ListVersions)
	r("POST", "/projects/:id/versions/:version/rollback", moduleVersionH.RollbackVersion)
	r("GET", "/projects/:id/versions/diff", moduleVersionH.VersionDiff)
	r("POST", "/templates/market/:id/use", templateMarketH.UseTemplate)
	r("POST", "/templates/market", templateMarketH.PublishTemplate)
	r("POST", "/templates/market/:id/rate", templateMarketH.RateTemplate)
	r("DELETE", "/templates/market/:id", templateMarketH.DeleteTemplate)
	r("POST", "/projects/:id/analyze-deps", dependencyH.AnalyzeDependencies)
	r("GET", "/projects/:id/dependencies", dependencyH.GetDependencyTree)
	r("POST", "/projects/:id/resolve-deps", dependencyH.ResolveDependencies)

	// Security enhancements
	r("POST", "/projects/:id/sign", signingH.SignModule)
	r("POST", "/projects/:id/verify", signingH.VerifyModule)
	r("GET", "/projects/:id/signature", signingH.GetSignatureInfo)
	r("POST", "/projects/:id/scan-vulns-ai", vulnH.ScanModuleVulnerabilities)
	r("GET", "/projects/:id/vulnerabilities", vulnH.GetModuleVulnerabilities)
	r("POST", "/projects/:id/audit", permAuditH.AuditModulePermissions)
	r("GET", "/projects/:id/permissions", permAuditH.GetModulePermissions)

	// ============================================================================
	// ADMIN ROUTES
	// ============================================================================

	rAdmin("GET", "/admin/email-config", settingsH.GetEmailConfig)
	rAdmin("PUT", "/admin/email-config", settingsH.UpdateEmailConfig)
	rAdmin("POST", "/admin/email-config/test", settingsH.SendTestEmail)
	rAdmin("POST", "/admin/email-config/test-connection", settingsH.TestConnection)

	// Agent Settings (所有用户可读写)
	r("GET", "/settings/agent", settingsH.GetAgentConfig)
	r("PUT", "/settings/agent", settingsH.UpdateAgentConfig)

	rAdmin("POST", "/admin/tags", tagsH.Create)
	rAdmin("DELETE", "/admin/tags/:id", tagsH.Delete)

	rAdmin("GET", "/admin/analytics/build-stats", analyticsH.BuildStats)
	rAdmin("GET", "/admin/analytics/build-trends", analyticsH.BuildTrends)
	rAdmin("GET", "/admin/analytics/system", analyticsH.SystemStats)

	rAdmin("POST", "/admin/cache/clear", func(c fiber.Ctx) error {
		count := cache.Clear()
		return c.JSON(fiber.Map{"status": "ok", "message": "缓存已清除", "entries": count})
	})
	rAdmin("GET", "/admin/cache/status", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"entries": cache.Size(), "ttl": cache.ttl.String()})
	})

	rAdmin("POST", "/admin/market/module/:slug/rollback", marketH.RollbackModule)
	r("POST", "/market/module/:slug/version", marketH.UpdateModuleVersion)
	r("GET", "/market/module/:slug/versions", marketH.GetModuleVersions)
	rAdmin("POST", "/admin/market/module/:slug/changelog", marketH.CreateChangelog)
	rAdmin("DELETE", "/admin/market/changelog/:id", marketH.DeleteChangelog)
	rAdmin("POST", "/admin/market/module/:slug/screenshots", marketH.UploadScreenshots)
	rAdmin("DELETE", "/admin/market/screenshots/:id", marketH.DeleteScreenshot)
	rAdmin("PUT", "/admin/market/screenshots/:id", marketH.UpdateScreenshot)
	rAdmin("POST", "/admin/market/module/:slug/tags", tagsH.SetModuleTags)

	rAdmin("GET", "/admin/health/detailed", healthH.DetailedHealth)

	logAggH := NewLogAggregatorHandler(db.Conn)
	rAdmin("GET", "/admin/logs", logAggH.ListLogs)
	rAdmin("GET", "/admin/logs/stats", logAggH.GetLogStats)
	rAdmin("DELETE", "/admin/logs/cleanup", logAggH.CleanupLogs)

	rAdmin("POST", "/admin/glossary", glossaryH.Create)
	rAdmin("PUT", "/admin/glossary/:id", glossaryH.Update)
	rAdmin("DELETE", "/admin/glossary/:id", glossaryH.Delete)

	// Wire services
	buildH.SetNotifSvc(notifSvc)
	buildH.SetActivitySvc(activitySvc)
	collabH.SetNotifSvc(notifSvc)
	collabH.SetActivitySvc(activitySvc)
}
