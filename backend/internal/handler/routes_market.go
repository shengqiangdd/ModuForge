package handler

import (
	"github.com/moduforge/backend/internal/service"
)

func registerMarketRoutes(ctx *routeContext) {
	db := ctx.db
	cfg := ctx.cfg

	marketSvc := service.NewSQLiteMarketService(db)
	marketH := NewMarketHandlerWithDB(marketSvc, cfg.StoragePath, db.Conn)
	analyticsSvc := service.NewAnalyticsService(db.Conn)
	analyticsH := NewAnalyticsHandler(analyticsSvc)
	tagsH := NewTagsHandler(db.Conn)
	badgeSvc := service.NewBadgeService(db.Conn)
	badgeH := NewBadgeHandler(badgeSvc)
	dashboardH := NewDashboardHandler(db.Conn)
	healthH := NewHealthHandler(db.Conn)
	healthH.SetLLMURL(cfg.LLMEndpoint)
	healthH.SetADBAddr(cfg.ADBAddress)

	// ── Public ──
	ctx.api.Get("/market/modules", CacheMiddleware(ctx.cache), marketH.ListModules)
	ctx.api.Get("/market/trending", CacheMiddleware(ctx.cache), marketH.Trending)
	ctx.api.Get("/market/categories", CacheMiddleware(ctx.cache), marketH.Categories)
	ctx.api.Get("/market/module/:slug", marketH.GetModule)
	ctx.api.Get("/market/module/:slug/reviews", marketH.GetReviews)
	ctx.api.Get("/market/module/:slug/dependencies", marketH.GetModuleDependencies)
	ctx.api.Post("/market/module/:slug/check-deps", marketH.CheckDependencyConflicts)
	ctx.api.Post("/market/compare", marketH.Compare)
	ctx.api.Get("/market/module/:slug/demo", marketH.GetDemo)
	ctx.api.Get("/market/module/:slug/changelogs", marketH.GetChangelogs)
	ctx.api.Get("/market/module/:slug/tags", tagsH.GetModuleTags)
	ctx.api.Get("/market/module/:slug/health", marketH.GetModuleHealth)
	ctx.api.Get("/market/module/:slug/install-stats", marketH.GetInstallStats)
	ctx.api.Get("/market/module/:slug/download", marketH.DownloadModule)
	ctx.api.Get("/market/stats/trending", marketH.GetTrending)

	ctx.api.Get("/analytics/module-stats", analyticsH.ModuleStats)
	ctx.api.Get("/tags", tagsH.List)

	// Badges (public)
	ctx.api.Get("/badges/definitions", badgeH.Definitions)
	ctx.api.Get("/badges/user/:id", badgeH.UserBadges)

	// Dashboard
	ctx.api.Get("/dashboard/widget-types", dashboardH.GetWidgetTypes)

	// ── Protected ──
	ctx.r("POST", "/market/module/:slug/star", marketH.StarModule)
	ctx.r("POST", "/market/publish", marketH.Publish)
	ctx.r("POST", "/market/module/:slug/review", marketH.AddReview)
	ctx.r("POST", "/market/batch/install", marketH.BatchInstall)
	ctx.r("POST", "/market/batch/uninstall", marketH.BatchUninstall)
	ctx.r("POST", "/market/batch/update", marketH.BatchUpdate)
	ctx.r("POST", "/market/module/:slug/version", marketH.UpdateModuleVersion)
	ctx.r("GET", "/market/module/:slug/versions", marketH.GetModuleVersions)
	ctx.r("GET", "/badges/my", badgeH.MyBadges)

	// ── Admin ──
	ctx.rAdmin("POST", "/admin/tags", tagsH.Create)
	ctx.rAdmin("DELETE", "/admin/tags/:id", tagsH.Delete)
	ctx.rAdmin("GET", "/admin/analytics/build-stats", analyticsH.BuildStats)
	ctx.rAdmin("GET", "/admin/analytics/build-trends", analyticsH.BuildTrends)
	ctx.rAdmin("GET", "/admin/analytics/system", analyticsH.SystemStats)
	ctx.rAdmin("POST", "/admin/market/module/:slug/rollback", marketH.RollbackModule)
	ctx.rAdmin("POST", "/admin/market/module/:slug/changelog", marketH.CreateChangelog)
	ctx.rAdmin("DELETE", "/admin/market/changelog/:id", marketH.DeleteChangelog)
	ctx.rAdmin("POST", "/admin/market/module/:slug/screenshots", marketH.UploadScreenshots)
	ctx.rAdmin("DELETE", "/admin/market/screenshots/:id", marketH.DeleteScreenshot)
	ctx.rAdmin("PUT", "/admin/market/screenshots/:id", marketH.UpdateScreenshot)
	ctx.rAdmin("POST", "/admin/market/module/:slug/tags", tagsH.SetModuleTags)
	ctx.rAdmin("GET", "/admin/health/detailed", healthH.DetailedHealth)
}
