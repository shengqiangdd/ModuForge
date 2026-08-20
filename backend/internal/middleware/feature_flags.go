package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// FeatureFlagChecker provides a middleware that blocks requests
// to disabled features. It uses a simple substring-based routing map.
type FeatureFlagChecker struct {
	isEnabled func(key string) bool
}

// NewFeatureFlagChecker creates a checker with the given flag lookup function.
func NewFeatureFlagChecker(isEnabled func(key string) bool) *FeatureFlagChecker {
	return &FeatureFlagChecker{isEnabled: isEnabled}
}

// routeFeatureMap maps URL path substrings to feature flag keys.
// More specific patterns come first to avoid false positives.
var routeFeatureMap = []struct {
	substring string
	feature  string
}{
	// Crash reporting — /crash/report, /crash/logs, /crash/stats
	{"/crash/", "crash_reporting"},

	// Collaboration — /collaborators, /collab-status, /comments/:id/resolve, /edit-session
	{"/collaborators", "collaboration"},
	{"/collab-status", "collaboration"},
	{"/collab-", "collaboration"},
	{"/comments/", "collaboration"},
	{"/edit-session", "collaboration"},

	// File comments — /files/*/comments, /file-comments
	{"/file-comments", "file_comments"},
	{"/files/", "file_comments"},

	// Favorites
	{"/favorites", "favorites"},

	// Backup schedules — /backup/schedules
	{"/backup/schedules", "backup_schedules"},

	// Security — /security/scan, /scan-vulns, /vuln-history
	{"/security/", "security_scanning"},
	{"/scan-vulns", "security_scanning"},
	{"/vuln-history", "security_scanning"},
	{"/vulnerabilities", "security_scanning"},

	// Module signing — /sign, /verify, /signature
	{"/sign", "module_signing"},
	{"/verify", "module_signing"},
	{"/signature", "module_signing"},

	// Badges
	{"/badges/", "badges"},

	// Marketplace — /market/, /templates/market
	{"/market/", "module_marketplace"},
	{"/templates/market", "template_marketplace"},

	// Email config — /admin/email-config
	{"/admin/email-config", "email_config"},

	// Tags
	{"/tags", "tags"},

	// Build schedules
	{"/build-schedules", "build_schedules"},

	// Benchmarks
	{"/benchmark/", "benchmarks"},

	// Analytics — /analytics/, /admin/analytics/
	{"/analytics/", "analytics"},

	// Code formatting
	{"/format", "code_formatting"},

	// Dependency analysis
	{"/analyze-deps", "dependency_analysis"},
	{"/dependencies", "dependency_analysis"},
	{"/resolve-deps", "dependency_analysis"},

	// Module versions — /versions
	{"/versions", "module_versions"},
}

// Middleware returns a Fiber middleware that checks feature flags.
// Routes that don't match any rule pass through (fail-open).
func (ff *FeatureFlagChecker) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()

		for _, rule := range routeFeatureMap {
			if strings.Contains(path, rule.substring) {
				if !ff.isEnabled(rule.feature) {
					return c.Status(501).JSON(fiber.Map{
						"error":   "功能未启用",
						"feature": rule.feature,
						"code":    "FEATURE_DISABLED",
						"message": "此功能已被管理员禁用，如需使用请联系管理员开启",
					})
				}
				break
			}
		}

		return c.Next()
	}
}
