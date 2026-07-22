package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/service"
)

func RegisterRoutes(api fiber.Router, db *database.DB, cfg *config.Config) {
	// Services
	authSvc := service.NewAuthService(db.Conn, cfg)
	projectSvc := service.NewProjectService(db.Conn)
	buildSvc := service.NewBuildService(db.Conn, cfg)
	aiSvc := service.NewAIService(cfg)
	repoSvc := service.NewRepoService()
	templateSvc := service.NewTemplateService()
	translateSvc := service.NewTranslateService()
	aiStreamSvc := service.NewAIStreamService()
	buildLogSvc := service.NewBuildLogService(cfg.StoragePath + "/logs")

	// Handlers
	authH := NewAuthHandler(authSvc)
	projectH := NewProjectHandler(projectSvc)
	buildH := NewBuildHandler(buildSvc)
	aiH := NewAIHandler(aiSvc, cfg)
	repoH := NewRepoHandler(repoSvc)
	templateH := NewTemplateHandler(templateSvc)
	translateH := NewTranslateHandler(translateSvc)
	aiStreamH := NewAIStreamHandler(aiStreamSvc)
	buildLogH := NewBuildLogHandler(buildLogSvc)

	// Public routes (no auth required)
	api.Post("/auth/register", authH.Register)
	api.Post("/auth/login", authH.Login)
	api.Get("/templates", projectH.ListTemplates)
	api.Get("/templates/list", templateH.List)
	api.Get("/templates/:name", templateH.Get)
	api.Post("/templates/recommend", templateH.Recommend)

	// AI (public for now)
	api.Post("/ai/generate", aiH.GenerateModule)
	api.Post("/ai/chat", aiH.Chat)
	api.Post("/ai/repair", aiH.RepairBuild)
	api.Post("/ai/stream", aiStreamH.StreamChat)

	// LLM Provider routes
	api.Get("/llm/providers", aiH.ListProviders)
	api.Get("/llm/refresh", aiH.RefreshModels)

	// Repo tracking (public)
	api.Post("/repo/fetch", repoH.Fetch)
	api.Post("/repo/files", repoH.FetchFiles)

	// Translate (public)
	api.Post("/translate", translateH.Translate)
	api.Post("/translate/props", translateH.TranslateProps)

	// Validator (public)
	validatorSvc := service.NewValidatorService()
	validatorH := NewValidatorHandler(validatorSvc)
	api.Post("/validate", validatorH.ValidateFiles)
	api.Post("/validate/file", validatorH.ValidateFile)

	// Zipper (public)
	zipperSvc := service.NewZipperService(cfg.StoragePath + "/downloads")
	zipperH := NewZipperHandler(zipperSvc)
	api.Post("/build/zip", zipperH.BuildZip)
	api.Get("/build/downloads", zipperH.ListDownloads)

	// Signer (public)
	signerSvc := service.NewSignerService("data/keys")
	signerH := NewSignerHandler(signerSvc)
	api.Post("/sign", signerH.Sign)
	api.Post("/verify", signerH.Verify)

	// ADB (public - device management)
	adbSvc := service.NewADBService()
	adbH := NewADBHandler(adbSvc)
	api.Get("/adb/devices", adbH.ListDevices)
	api.Post("/adb/push", adbH.PushFile)
	api.Post("/adb/install", adbH.InstallModule)
	api.Post("/adb/shell", adbH.RunShell)
	api.Post("/adb/reboot", adbH.RebootDevice)
	api.Get("/adb/check", adbH.CheckADB)

	// ADB benchmark (public)
	benchH := NewBenchmarkHandler(adbSvc)
	api.Post("/adb/benchmark", benchH.Benchmark)

	// Mirror (public for streaming)
	mirrorH := NewMirrorHandler(adbSvc)
	api.Get("/adb/mirror", mirrorH.Mirror)

	// Module update checking (public)
	updateSvc := service.NewUpdateService(db.Conn)
	updateH := NewUpdateHandler(updateSvc)
	api.Post("/update/check", updateH.CheckUpdate)
	api.Post("/update/check-all", updateH.CheckAllUpdates)

	// Git (public)
	gitSvc := service.NewGitManagerService(cfg.StoragePath + "/projects")
	gitH := NewGitHandler(gitSvc)
	api.Get("/git/commits", gitH.ListCommits)
	api.Get("/git/diff", gitH.GetDiff)
	api.Get("/git/head", gitH.GetCurrentHash)

	// Market browse (public)
	marketSvc := service.NewSQLiteMarketService(db)
	marketH := NewMarketHandler(marketSvc)
	api.Get("/market/modules", marketH.ListModules)
	api.Get("/market/trending", marketH.Trending)
	api.Get("/market/categories", marketH.Categories)
	api.Get("/market/module/:slug", marketH.GetModule)
	api.Get("/market/module/:slug/reviews", marketH.GetReviews)
	api.Post("/market/module/:slug/star", marketH.StarModule)

	// Build log streaming (public)
	api.Get("/build/log", buildLogH.GetBuildLog)

	// Analytics (public)
	analyticsSvc := service.NewAnalyticsService(db.Conn)
	analyticsH := NewAnalyticsHandler(analyticsSvc)
	api.Get("/analytics/build-stats", analyticsH.BuildStats)
	api.Get("/analytics/build-trends", analyticsH.BuildTrends)
	api.Get("/analytics/module-stats", analyticsH.ModuleStats)
	api.Get("/analytics/system", analyticsH.SystemStats)

	// ===== Protected routes (JWT required) =====
	protected := api.Group("")
	protected.Use(AuthMiddleware(cfg.JWTSecret))

	// LLM config (protected)
	protected.Post("/llm/config", aiH.UpdateLLMConfig)
	protected.Get("/llm/config", aiH.GetLLMConfig)

	// Projects (CRUD)
	protected.Get("/projects", projectH.List)
	protected.Post("/projects", projectH.Create)
	protected.Get("/projects/:id", projectH.Get)
	protected.Put("/projects/:id", projectH.Update)
	protected.Delete("/projects/:id", projectH.Delete)

	// Project files
	protected.Get("/projects/:id/files", projectH.ListFiles)
	protected.Get("/projects/:id/files/*", projectH.GetFile)
	protected.Put("/projects/:id/files/*", projectH.SaveFile)

	// Builds
	protected.Post("/projects/:id/build", buildH.Create)
	protected.Get("/builds/:id", buildH.Get)
	protected.Get("/builds/:id/logs", buildH.StreamLogs)
	protected.Get("/builds/:id/download", buildH.Download)

	// Market (write operations)
	protected.Post("/market/publish", marketH.Publish)
	protected.Post("/market/module/:slug/review", marketH.AddReview)

	// Git (write operations)
	protected.Post("/git/commit", gitH.Commit)
	protected.Post("/git/checkout", gitH.Checkout)

	// ADB screenshot (protected)
	screenshotH := NewScreenshotHandler(adbSvc)
	protected.Get("/adb/screenshot", screenshotH.Screenshot)
	protected.Get("/adb/screenshot/stream", screenshotH.StreamScreenshots)

	// Benchmark with history (protected)
	benchmarkSvc := service.NewBenchmarkService(db.Conn)
	benchmarkAPIH := NewBenchmarkAPIHandler(benchmarkSvc, adbSvc)
	protected.Post("/benchmark/run", benchmarkAPIH.RunBenchmark)
	protected.Get("/benchmark/history", benchmarkAPIH.GetHistory)

	// ===== Wave 2: Collaboration =====
	collabSvc := service.NewCollaborationService(db.Conn)
	collabH := NewCollaborationHandler(collabSvc)

	protected.Post("/projects/:id/collaborators", collabH.AddCollaborator)
	protected.Get("/projects/:id/collaborators", collabH.ListCollaborators)
	protected.Delete("/projects/:id/collaborators/:userId", collabH.RemoveCollaborator)
	protected.Post("/projects/:id/comments", collabH.AddComment)
	protected.Get("/projects/:id/comments", collabH.ListComments)
	protected.Post("/comments/:commentId/resolve", collabH.ResolveComment)
	protected.Post("/projects/:id/edit-session", collabH.UpsertEditSession)
	protected.Get("/projects/:id/edit-sessions", collabH.ListEditSessions)
	protected.Delete("/projects/:id/edit-session/:sessionId", collabH.RemoveEditSession)

	// ===== Wave 2: Plugin System =====
	pluginSvc := service.NewPluginService(db.Conn)
	pluginH := NewPluginHandler(pluginSvc)

	// Public read routes
	api.Get("/plugins", pluginH.List)
	api.Post("/plugins/hooks/execute", pluginH.ExecuteHook)

	// Protected write routes
	protected.Post("/plugins/install", pluginH.Install)
	protected.Post("/plugins/:id/enable", pluginH.Enable)
	protected.Post("/plugins/:id/disable", pluginH.Disable)
	protected.Delete("/plugins/:id", pluginH.Uninstall)
	protected.Post("/plugins/:id/hooks", pluginH.RegisterHook)
	protected.Get("/plugins/:id/hooks", pluginH.GetHooks)
}
