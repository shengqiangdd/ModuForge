package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

func registerCoreRoutes(ctx *routeContext, authH *AuthHandler) {
	api := ctx.api
	db := ctx.db
	cfg := ctx.cfg
	fileRepo := ctx.fileRepo
	cache := ctx.cache

	// Services
	projectSvc := service.NewProjectService(db.Conn, cfg.StoragePath, ctx.s3adapter)
	buildSvc := service.NewBuildService(db.Conn, cfg)
	buildSvc.SetFileContentRepo(fileRepo)
	templateSvc := service.NewTemplateService()
	translateSvc := service.NewTranslateService()
	buildLogSvc := service.NewBuildLogService(cfg.StoragePath + "/logs")
	emailSvc := service.NewEmailService(db.Conn)

	// Handlers
	projectH := NewProjectHandlerWithDB(projectSvc, db.Conn)
	buildH := NewBuildHandler(buildSvc)
	buildScheduleSvc := service.NewBuildScheduleService(db.Conn, buildSvc)
	buildScheduleH := NewBuildScheduleHandler(buildScheduleSvc)
	buildScheduleSvc.StartScheduler(context.Background())
	templateH := NewTemplateHandler(templateSvc)
	translateH := NewTranslateHandler(translateSvc)
	buildLogH := NewBuildLogHandler(buildLogSvc)
	settingsH := NewSettingsHandler(emailSvc, db.Conn)
	moduleVersionH := NewModuleVersionHandler(db.Conn)
	templateMarketH := NewTemplateMarketHandler(db.Conn)
	dependencyH := NewDependencyHandler(db.Conn)
	dependencyH.SetFileContentRepo(fileRepo)
	signingH := NewSigningHandler(db.Conn, cfg.StoragePath+"/keys")
	signingH.SetFileContentRepo(fileRepo)
	vulnH := NewVulnerabilityHandler(db.Conn)
	vulnH.SetFileContentRepo(fileRepo)
	securitySvc := service.NewSecurityScanner()
	securityH := NewSecurityHandler(securitySvc, db.Conn)
	securityH.SetFileContentRepo(fileRepo)
	zipperSvc := service.NewZipperService(cfg.StoragePath+"/downloads", db.Conn)
	zipperSvc.SetFileContentRepo(fileRepo)
	zipperH := NewZipperHandler(zipperSvc)
	gitSvc := service.NewGitManagerService(cfg.StoragePath + "/projects")
	gitH := NewGitHandler(gitSvc)
	gitH.SetAuthService(service.NewAuthService(db.Conn, cfg))
	healthH := NewHealthHandler(db.Conn)
	healthH.SetLLMURL(cfg.LLMEndpoint)
	healthH.SetADBAddr(cfg.ADBAddress)
	glossaryH := NewGlossaryHandler(db.Conn)
	validatorSvc := service.NewValidatorService()
	validatorH := NewValidatorHandler(validatorSvc)
	signerSvc := service.NewSignerService("data/keys")
	signerH := NewSignerHandler(signerSvc)
	crashH := NewCrashHandler(db.Conn)

	repoSvc := service.NewRepoService()
	repoH := NewRepoHandler(repoSvc)

	// Templates
	api.Get("/templates", projectH.ListTemplates)
	api.Get("/templates/list", templateH.List)
	api.Get("/templates/market", templateMarketH.ListTemplates)
	api.Get("/templates/market/trending", templateMarketH.GetTrending)
	api.Get("/templates/market/categories", templateMarketH.GetCategories)
	api.Get("/templates/:name", templateH.Get)
	api.Post("/templates/recommend", templateH.Recommend)

	// Repo
	api.Post("/repo/fetch", ctx.rateRepo, repoH.Fetch)
	api.Post("/repo/files", ctx.rateRepo, repoH.FetchFiles)
	api.Post("/repo/tree", ctx.rateRepo, repoH.FetchTree)
	api.Post("/repo/file", ctx.rateRepo, repoH.FetchFileContent)
	api.Post("/repo/smart-select", repoH.SmartSelect)

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

	// Signer
	ctx.r("POST", "/sign", signerH.Sign)
	api.Post("/verify", signerH.Verify)

	// Build log
	api.Get("/build/log", buildLogH.GetBuildLog)

	// Health
	api.Get("/health/system", healthH.Check)
	api.Get("/health/cache", healthH.CacheStats)

	// Glossary
	api.Get("/glossary", glossaryH.List)
	api.Get("/glossary/popular", glossaryH.Popular)
	api.Get("/glossary/:id", glossaryH.Get)

	// Crash report
	api.Post("/crash/report", crashH.Report)

	// Git (read)
	ctx.r("GET", "/git/commits", gitH.ListCommits)
	ctx.r("GET", "/git/diff", gitH.GetDiff)
	ctx.r("GET", "/git/head", gitH.GetCurrentHash)
	ctx.r("GET", "/git/branches", gitH.ListBranches)
	ctx.r("GET", "/git/branch", gitH.GetCurrentBranch)

	// ── PROTECTED ──

	// Auth protected
	ctx.r("POST", "/auth/change-password", authH.ChangePassword)
	ctx.r("POST", "/auth/2fa/setup", authH.Setup2FA)
	ctx.r("POST", "/auth/2fa/enable", authH.Enable2FA)
	ctx.r("POST", "/auth/2fa/disable", authH.Disable2FA)
	ctx.r("GET", "/auth/profile", authH.GetProfile)
	ctx.r("PUT", "/auth/profile", authH.UpdateProfile)
	ctx.r("POST", "/auth/avatar", authH.UploadAvatar)
	ctx.r("POST", "/auth/resend-verification", authH.ResendVerification)
	ctx.r("GET", "/auth/github-token", authH.GetGitHubToken)
	ctx.r("PUT", "/auth/github-token", authH.SetGitHubToken)

	// Security
	ctx.r("POST", "/security/scan", securityH.ScanFiles)
	ctx.r("POST", "/security/scan-project/:id", securityH.ScanProject)
	ctx.r("POST", "/projects/:id/scan-vulns", securityH.ScanVulnerabilities)
	ctx.r("GET", "/projects/:id/vuln-history", securityH.GetVulnHistory)

	ctx.r("POST", "/module/import-zip", ImportModuleZip)

	// Projects CRUD
	ctx.r("GET", "/projects", projectH.List)
	ctx.r("POST", "/projects", projectH.Create)
	ctx.r("GET", "/projects/:id", projectH.Get)
	ctx.r("PUT", "/projects/:id", projectH.Update)
	ctx.r("DELETE", "/projects/:id", projectH.Delete)
	ctx.r("GET", "/search", projectH.Search)

	// File comments
	fileCommentH := NewFileCommentHandler(db.Conn)
	ctx.r("POST", "/projects/:id/files/*/comments", fileCommentH.AddComment)
	ctx.r("GET", "/projects/:id/files/*/comments", fileCommentH.GetComments)
	ctx.r("DELETE", "/projects/:id/comments/:comment_id", fileCommentH.DeleteComment)
	ctx.r("POST", "/projects/:id/comments/:comment_id/reply", fileCommentH.ReplyToComment)
	ctx.r("GET", "/projects/:id/file-comments", fileCommentH.ListProjectComments)

	// Project files
	ctx.r("GET", "/projects/:id/files", projectH.ListFiles)
	ctx.r("GET", "/projects/:id/files/*", projectH.GetFile)
	ctx.r("PUT", "/projects/:id/files/*", projectH.SaveFile)
	ctx.r("POST", "/projects/:id/files/upload", projectH.UploadFiles)
	ctx.r("DELETE", "/projects/:id/files/*", projectH.DeleteFile)
	ctx.r("POST", "/projects/:id/validate", projectH.ValidateProject)
	ctx.r("GET", "/projects/:id/tree", projectH.GetFileTree)

	// Builds
	ctx.r("POST", "/projects/:id/build", buildH.Create)
	ctx.r("GET", "/projects/:id/builds", buildH.ListByProject)
	ctx.r("DELETE", "/projects/:id/builds/failed", buildH.DeleteFailed)
	ctx.r("DELETE", "/projects/:id/builds/:buildId", buildH.Delete)
	ctx.r("DELETE", "/projects/:id/build-cache", buildH.ClearBuildCache)
	ctx.r("GET", "/projects/:id/build/cache", buildH.GetBuildCacheStatus)
	ctx.r("GET", "/projects/:id/build/architectures", buildH.GetSupportedArchitectures)
	ctx.r("POST", "/projects/:id/build-schedules", buildScheduleH.Create)
	ctx.r("GET", "/projects/:id/build-schedules", buildScheduleH.List)
	ctx.r("PUT", "/projects/:id/build-schedules/:scheduleId", buildScheduleH.Toggle)
	ctx.r("DELETE", "/projects/:id/build-schedules/:scheduleId", buildScheduleH.Delete)
	ctx.r("GET", "/builds/:id", buildH.Get)
	ctx.r("GET", "/builds/:id/logs", buildH.StreamLogs)
	ctx.r("GET", "/builds/:id/download", buildH.Download)
	ctx.r("POST", "/projects/:id/build/auto", buildH.CreateAuto)
	ctx.r("POST", "/builds/:id/cancel", buildH.Cancel)
	ctx.r("POST", "/projects/:id/builds/:buildId/release", buildH.PublishToRelease)

	// Cache
	ctx.r("GET", "/cache/stats", buildH.GetGlobalCacheStats)
	ctx.rAdmin("POST", "/cache/cleanup", buildH.TriggerCacheCleanup)
	ctx.r("POST", "/projects/:id/export-zip", zipperH.ExportProjectZip)

	// Git write
	ctx.r("POST", "/git/commit", gitH.Commit)
	ctx.r("POST", "/git/checkout", gitH.Checkout)
	ctx.r("POST", "/git/branch", gitH.CreateBranch)
	ctx.r("POST", "/git/checkout-branch", gitH.CheckoutBranch)
	ctx.r("POST", "/git/push", gitH.Push)
	ctx.r("POST", "/git/pull", gitH.Pull)
	ctx.r("POST", "/git/push-optimized", gitH.PushOptimized)
	ctx.r("POST", "/git/preview-files", gitH.PreviewFilesToPush)

	// API keys
	apiKeyH := NewAPIKeyHandler(db.Conn)
	ctx.r("GET", "/api-keys", apiKeyH.List)
	ctx.r("POST", "/api-keys", apiKeyH.Create)
	ctx.r("DELETE", "/api-keys/:id", apiKeyH.Delete)
	ctx.r("POST", "/api-keys/:id/rotate", apiKeyH.Rotate)

	// Collaboration
	collabSvc := service.NewCollaborationService(db.Conn)
	collabH := NewCollaborationHandler(collabSvc)
	ctx.r("POST", "/projects/:id/collaborators", collabH.AddCollaborator)
	ctx.r("GET", "/projects/:id/collaborators", collabH.ListCollaborators)
	ctx.r("DELETE", "/projects/:id/collaborators/:userId", collabH.RemoveCollaborator)
	ctx.r("POST", "/projects/:id/comments", collabH.AddComment)
	ctx.r("GET", "/projects/:id/comments", collabH.ListComments)
	ctx.r("POST", "/comments/:commentId/resolve", collabH.ResolveComment)
	ctx.r("POST", "/projects/:id/edit-session", collabH.UpsertEditSession)
	ctx.r("GET", "/projects/:id/edit-sessions", collabH.ListEditSessions)
	ctx.r("DELETE", "/projects/:id/edit-session/:sessionId", collabH.RemoveEditSession)
	ctx.r("POST", "/projects/:id/members", collabH.AddTeamMember)
	ctx.r("GET", "/projects/:id/members", collabH.ListTeamMembers)
	ctx.r("PUT", "/projects/:id/members/:userId", collabH.UpdateMemberRole)
	ctx.r("DELETE", "/projects/:id/members/:userId", collabH.RemoveTeamMember)
	ctx.r("GET", "/projects/:id/audit-logs", collabH.GetAuditLogs)
	collabWS := NewCollaborationWS(collabSvc)
	ctx.r("GET", "/projects/:id/collab-status", collabWS.GetCollaborationStatus)

	// Backup & Restore
	backupSvc := service.NewBackupService(db.Conn, cfg.StoragePath+"/backups")
	backupH := NewBackupHandler(backupSvc)
	ctx.r("POST", "/backup/export", backupH.ExportDatabase)
	ctx.r("POST", "/backup/import", backupH.ImportDatabase)
	ctx.r("POST", "/projects/:id/export", backupH.ExportProject)
	ctx.r("POST", "/projects/:id/import", backupH.ImportProject)

	// Notifications
	notifSvc := service.NewNotificationService(db.Conn)
	notifH := NewNotificationHandler(notifSvc)
	ctx.r("GET", "/notifications", notifH.List)
	ctx.r("GET", "/notifications/unread-count", notifH.UnreadCount)
	ctx.r("POST", "/notifications/:id/read", notifH.MarkRead)
	ctx.r("POST", "/notifications/read-all", notifH.MarkAllRead)
	ctx.r("DELETE", "/notifications/:id", notifH.Delete)

	// Activities
	activitySvc := service.NewActivityService(db.Conn)
	activityH := NewActivityHandler(activitySvc)
	ctx.r("GET", "/projects/:id/activities", activityH.GetProjectActivities)
	ctx.r("GET", "/activities", activityH.GetUserActivities)
	ctx.r("GET", "/activity/export", activityH.Export)

	// Crash protected
	ctx.r("GET", "/crash/logs", crashH.ListLogs)
	ctx.r("GET", "/crash/stats", crashH.Stats)
	ctx.r("DELETE", "/crash/logs/:id", crashH.Delete)
	ctx.r("DELETE", "/crash/logs", crashH.ClearAll)

	// Search history
	searchH := NewSearchHistoryHandler(db.Conn)
	ctx.r("GET", "/search/history", searchH.GetHistory)
	ctx.r("DELETE", "/search/history/:id", searchH.DeleteHistory)
	ctx.r("DELETE", "/search/history", searchH.ClearHistory)

	// Favorites
	favoritesH := NewFavoritesHandler(db.Conn)
	ctx.r("GET", "/favorites", favoritesH.List)
	ctx.r("POST", "/favorites", favoritesH.Add)
	ctx.r("DELETE", "/favorites/:type/:id", favoritesH.Remove)
	ctx.r("GET", "/favorites/check/:type/:id", favoritesH.Check)

	// Code formatting
	formatH := NewFormatHandler(db.Conn)
	formatH.SetFileContentRepo(fileRepo)
	ctx.r("POST", "/projects/:id/format", formatH.FormatProject)
	ctx.r("POST", "/projects/:id/format/preview", formatH.PreviewFormat)

	// Backup schedules
	ctx.r("GET", "/backup/schedules", backupH.ListSchedules)
	ctx.r("POST", "/backup/schedules", backupH.CreateSchedule)
	ctx.r("PUT", "/backup/schedules/:id", backupH.UpdateSchedule)
	ctx.r("DELETE", "/backup/schedules/:id", backupH.DeleteSchedule)
	ctx.r("POST", "/backup/schedules/:id/run", backupH.RunSchedule)

	// Recycle bin
	recycleH := NewRecycleHandler(db.Conn)
	ctx.r("GET", "/recycle-bin", recycleH.List)
	ctx.r("POST", "/recycle-bin/:type/:id/restore", recycleH.Restore)
	ctx.r("DELETE", "/recycle-bin/:type/:id", recycleH.PermanentlyDelete)
	ctx.r("DELETE", "/recycle-bin", recycleH.ClearAll)

	// Module system
	ctx.r("POST", "/projects/:id/versions", moduleVersionH.CreateVersion)
	ctx.r("GET", "/projects/:id/versions", moduleVersionH.ListVersions)
	ctx.r("POST", "/projects/:id/versions/:version/rollback", moduleVersionH.RollbackVersion)
	ctx.r("GET", "/projects/:id/versions/diff", moduleVersionH.VersionDiff)
	ctx.r("POST", "/templates/market/:id/use", templateMarketH.UseTemplate)
	ctx.r("POST", "/templates/market", templateMarketH.PublishTemplate)
	ctx.r("POST", "/templates/market/:id/rate", templateMarketH.RateTemplate)
	ctx.r("DELETE", "/templates/market/:id", templateMarketH.DeleteTemplate)
	ctx.r("POST", "/projects/:id/analyze-deps", dependencyH.AnalyzeDependencies)
	ctx.r("GET", "/projects/:id/dependencies", dependencyH.GetDependencyTree)
	ctx.r("POST", "/projects/:id/resolve-deps", dependencyH.ResolveDependencies)

	// Security enhancements
	ctx.r("POST", "/projects/:id/sign", signingH.SignModule)
	ctx.r("POST", "/projects/:id/verify", signingH.VerifyModule)
	ctx.r("GET", "/projects/:id/signature", signingH.GetSignatureInfo)
	ctx.r("POST", "/projects/:id/scan-vulns-ai", vulnH.ScanModuleVulnerabilities)
	ctx.r("GET", "/projects/:id/vulnerabilities", vulnH.GetModuleVulnerabilities)

	// ── ADMIN ──
	ctx.rAdmin("GET", "/admin/email-config", settingsH.GetEmailConfig)
	ctx.rAdmin("PUT", "/admin/email-config", settingsH.UpdateEmailConfig)
	ctx.rAdmin("POST", "/admin/email-config/test", settingsH.SendTestEmail)
	ctx.rAdmin("POST", "/admin/email-config/test-connection", settingsH.TestConnection)
	ctx.r("GET", "/settings/agent", settingsH.GetAgentConfig)
	ctx.r("PUT", "/settings/agent", settingsH.UpdateAgentConfig)

	tagsH := NewTagsHandler(db.Conn)
	analyticsSvc := service.NewAnalyticsService(db.Conn)
	analyticsH := NewAnalyticsHandler(analyticsSvc)
	ctx.rAdmin("POST", "/admin/tags", tagsH.Create)
	ctx.rAdmin("DELETE", "/admin/tags/:id", tagsH.Delete)
	ctx.rAdmin("GET", "/admin/analytics/build-stats", analyticsH.BuildStats)
	ctx.rAdmin("GET", "/admin/analytics/build-trends", analyticsH.BuildTrends)
	ctx.rAdmin("GET", "/admin/analytics/system", analyticsH.SystemStats)

	ctx.rAdmin("POST", "/admin/cache/clear", func(c fiber.Ctx) error {
		count := cache.Clear()
		return c.JSON(fiber.Map{"status": "ok", "message": "缓存已清除", "entries": count})
	})
	ctx.rAdmin("GET", "/admin/cache/status", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"entries": cache.Size(), "ttl": cache.ttl.String()})
	})

	logAggH := NewLogAggregatorHandler(db.Conn)
	ctx.rAdmin("GET", "/admin/logs", logAggH.ListLogs)
	ctx.rAdmin("GET", "/admin/logs/stats", logAggH.GetLogStats)
	ctx.rAdmin("DELETE", "/admin/logs/cleanup", logAggH.CleanupLogs)

	ctx.rAdmin("POST", "/admin/glossary", glossaryH.Create)
	ctx.rAdmin("PUT", "/admin/glossary/:id", glossaryH.Update)
	ctx.rAdmin("DELETE", "/admin/glossary/:id", glossaryH.Delete)

	// Wire services
	buildH.SetNotifSvc(notifSvc)
	buildH.SetActivitySvc(activitySvc)
	collabH.SetNotifSvc(notifSvc)
	collabH.SetActivitySvc(activitySvc)
}
