package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// DocGenerator generates documentation for modules
type DocGenerator struct{}

func NewDocGenerator() *DocGenerator {
	return &DocGenerator{}
}

// DocOptions contains options for documentation generation
type DocOptions struct {
	ProjectName   string    `json:"project_name"`
	Description   string    `json:"description"`
	Author        string    `json:"author"`
	Version       string    `json:"version"`
	ModuleType    string    `json:"module_type"` // magisk, ksu, apatch, universal
	License       string    `json:"license"`
	Tags          []string  `json:"tags"`
	Dependencies  []string  `json:"dependencies"`
	Files         []FileDoc `json:"files"`
	HasDaemon     bool      `json:"has_daemon"`
	HasWebUI      bool      `json:"has_webui"`
	HasService    bool      `json:"has_service"`
	MinAPI        int       `json:"min_api"`
	Architectures []string  `json:"architectures"` // arm64, arm, x86_64
}

type FileDoc struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}

// GeneratedDoc represents a generated document
type GeneratedDoc struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Type     string `json:"type"` // readme, usage, api, changelog
}

// shieldBadge returns a shields.io badge URL with the given label and message.
func shieldBadge(label, message, color string) string {
	return fmt.Sprintf("![%s](https://img.shields.io/badge/%s-%s-%s)",
		label, label, message, color)
}

// moduleTypeBadge returns the coloured badge for a module type.
func moduleTypeBadge(mt string) string {
	colors := map[string]string{
		"magisk":    "4CAF50",
		"ksu":       "2196F3",
		"apatch":    "FF9800",
		"universal": "9C27B0",
	}
	c := colors[mt]
	if c == "" {
		c = "757575"
	}
	return shieldBadge("Module", strings.ToUpper(mt), c)
}

// architectureString joins the architecture list into a human-readable string.
func architectureString(archs []string) string {
	if len(archs) == 0 {
		return "arm64-v8a"
	}
	return strings.Join(archs, ", ")
}

// hasConfig returns true when any file looks like a configuration file.
func hasConfig(files []FileDoc) bool {
	for _, f := range files {
		lower := strings.ToLower(f.Path)
		if strings.HasSuffix(lower, ".conf") || strings.HasSuffix(lower, ".json") ||
			strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") ||
			strings.HasSuffix(lower, ".ini") || strings.HasSuffix(lower, ".properties") ||
			strings.HasSuffix(lower, ".cfg") {
			return true
		}
	}
	return false
}

// fileTree builds a markdown file-tree overview from FileDoc entries.
func fileTree(files []FileDoc) string {
	var sb strings.Builder
	if len(files) == 0 {
		sb.WriteString("```\n")
		sb.WriteString(fmt.Sprintf("%s/\n", "module"))
		sb.WriteString("├── module.prop\n")
		sb.WriteString("├── system/\n")
		sb.WriteString("│   └── modules/\n")
		sb.WriteString("│       └── <module-name>/\n")
		sb.WriteString("└── META-INF/\n")
		sb.WriteString("    └── com/google/android/\n")
		sb.WriteString("        ├── update-binary\n")
		sb.WriteString("        └── updater-script\n")
		sb.WriteString("```\n")
		return sb.String()
	}

	sb.WriteString("```\n")
	sb.WriteString(fmt.Sprintf("%s/\n", filepath.Base(files[0].Path)))
	for i, f := range files {
		parts := strings.Split(f.Path, "/")
		prefix := "├── "
		if i == len(files)-1 {
			prefix = "└── "
		}
		if len(parts) > 1 {
			for j := 0; j < len(parts)-1; j++ {
				sb.WriteString("│   ")
			}
			sb.WriteString(fmt.Sprintf("%s%s\n", prefix, parts[len(parts)-1]))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s\n", prefix, parts[0]))
		}
	}
	sb.WriteString("```\n")
	return sb.String()
}

// --- GenerateReadme ----------------------------------------------------------

func (d *DocGenerator) GenerateReadme(opts DocOptions) *GeneratedDoc {
	var sb strings.Builder

	title := opts.ProjectName
	if title == "" {
		title = "Module"
	}

	// --- Badges
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	license := opts.License
	if license == "" {
		license = "MIT"
	}
	version := opts.Version
	if version == "" {
		version = "1.0.0"
	}
	moduleType := opts.ModuleType
	if moduleType == "" {
		moduleType = "universal"
	}

	sb.WriteString(fmt.Sprintf("%s %s %s %s\n\n",
		shieldBadge("License", license, "blue"),
		shieldBadge("Version", version, "green"),
		moduleTypeBadge(moduleType),
		shieldBadge("Platform", "Android", "brightgreen"),
	))

	// --- Description
	desc := opts.Description
	if desc == "" {
		desc = "A powerful Android module for system-level customisation and automation."
	}
	sb.WriteString(fmt.Sprintf("%s\n\n", desc))

	// --- Features
	sb.WriteString("## Features\n\n")
	features := d.buildFeatureList(opts)
	for _, f := range features {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}
	sb.WriteString("\n")

	// --- Installation
	sb.WriteString("## Installation\n\n")
	switch moduleType {
	case "magisk":
		sb.WriteString("### Via Magisk Manager\n\n")
		sb.WriteString("1. Download the latest `.zip` release.\n")
		sb.WriteString("2. Open **Magisk Manager** → **Modules** → **Install from storage**.\n")
		sb.WriteString("3. Select the downloaded `.zip` and wait for the installation to finish.\n")
		sb.WriteString("4. **Reboot** your device.\n\n")
	case "ksu":
		sb.WriteString("### Via KernelSU Manager\n\n")
		sb.WriteString("1. Download the latest `.zip` release.\n")
		sb.WriteString("2. Open **KernelSU Manager** → **Modules** → **Install**.\n")
		sb.WriteString("3. Select the downloaded `.zip` and confirm.\n")
		sb.WriteString("4. **Reboot** your device.\n\n")
	case "apatch":
		sb.WriteString("### Via APatch\n\n")
		sb.WriteString("1. Download the latest `.zip` release.\n")
		sb.WriteString("2. Open **APatch** → **Modules** → **Install module**.\n")
		sb.WriteString("3. Select the downloaded `.zip` and wait.\n")
		sb.WriteString("4. **Reboot** your device.\n\n")
	default:
		sb.WriteString("### Via Magisk / KernelSU / APatch\n\n")
		sb.WriteString("1. Download the latest `.zip` release.\n")
		sb.WriteString("2. Open your module manager → **Modules** → **Install**.\n")
		sb.WriteString("3. Select the downloaded `.zip` and confirm.\n")
		sb.WriteString("4. **Reboot** your device.\n\n")
	}

	// --- Usage
	sb.WriteString("## Usage\n\n")
	if opts.HasDaemon {
		sb.WriteString("Once installed, the module daemon starts automatically. You can interact with it via the provided CLI or ADB:\n\n")
		sb.WriteString("```bash\n")
		sb.WriteString(fmt.Sprintf("adb shell %s-cli --help\n", title))
		sb.WriteString("```\n\n")
	} else if opts.HasService {
		sb.WriteString("The background service runs automatically after installation. Check its status with:\n\n")
		sb.WriteString("```bash\n")
		sb.WriteString("adb shell su -c 'getprop init.svc.module_service'\n")
		sb.WriteString("```\n\n")
	} else {
		sb.WriteString("After rebooting, the module is active. Refer to the [Usage Guide](USAGE.md) for detailed instructions.\n\n")
	}

	// --- Configuration
	if hasConfig(opts.Files) {
		sb.WriteString("## Configuration\n\n")
		sb.WriteString("This module is configurable. Edit the configuration file located at:\n\n")
		sb.WriteString("```\n")
		sb.WriteString("/data/adb/modules/<module-name>/config/\n")
		sb.WriteString("```\n\n")
		sb.WriteString("After making changes, restart the module service:\n\n")
		sb.WriteString("```bash\n")
		sb.WriteString("adb shell su -c 'module-ctl restart'\n")
		sb.WriteString("```\n\n")
	}

	// --- File structure
	sb.WriteString("## File Structure\n\n")
	sb.WriteString(fileTree(opts.Files))

	// --- Dependencies
	if len(opts.Dependencies) > 0 {
		sb.WriteString("## Dependencies\n\n")
		sb.WriteString("| Dependency | Description |\n")
		sb.WriteString("|------------|-------------|\n")
		for _, dep := range opts.Dependencies {
			sb.WriteString(fmt.Sprintf("| %s | System package |\n", dep))
		}
		sb.WriteString("\n")
	}

	// --- Compatibility
	sb.WriteString("## Compatibility\n\n")
	sb.WriteString("| Module Manager | Supported | Min Version |\n")
	sb.WriteString("|----------------|-----------|-------------|\n")
	sb.WriteString("| Magisk         | ✅        | 24.0+       |\n")
	sb.WriteString("| KernelSU       | ✅        | 0.6.0+      |\n")
	sb.WriteString("| APatch         | ✅        | 10000+      |\n")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("**Android versions**: %d+\n\n", opts.MinAPI))
	sb.WriteString(fmt.Sprintf("**Architectures**: %s\n\n", architectureString(opts.Architectures)))

	// --- Build
	sb.WriteString("## Build\n\n")
	sb.WriteString("### Prerequisites\n\n")
	sb.WriteString("- Linux or macOS build environment\n")
	sb.WriteString("- Android NDK (r25+ recommended)\n")
	sb.WriteString("- `zip` command-line utility\n")
	sb.WriteString("- Rooted test device or emulator\n\n")
	sb.WriteString("### Steps\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# Clone the repository\n")
	sb.WriteString(fmt.Sprintf("git clone https://github.com/your-username/%s.git\n", title))
	sb.WriteString(fmt.Sprintf("cd %s\n\n", title))
	sb.WriteString("# Build the module\n")
	sb.WriteString("./build.sh\n\n")
	sb.WriteString("# Output will be in dist/\n")
	sb.WriteString("ls dist/\n")
	sb.WriteString("```\n\n")

	// --- License
	sb.WriteString("## License\n\n")
	sb.WriteString(fmt.Sprintf("This project is licensed under the **%s** License.\n\n", license))

	// --- Author
	if opts.Author != "" {
		sb.WriteString("## Author\n\n")
		sb.WriteString(fmt.Sprintf("**%s**\n\n", opts.Author))
		sb.WriteString("---\n\n")
		sb.WriteString("*Built with [ModuForge](https://github.com/moduforge) — Android module development toolkit*\n")
	}

	return &GeneratedDoc{
		Filename: "README.md",
		Content:  sb.String(),
		Type:     "readme",
	}
}

// buildFeatureList derives a feature list from the DocOptions.
func (d *DocGenerator) buildFeatureList(opts DocOptions) []string {
	features := []string{
		"System-level customisation without modifying the system partition",
		"Seamless updates via module manager",
		"No lasting filesystem modifications (systemless approach)",
	}
	if opts.HasDaemon {
		features = append(features, "Background daemon for real-time monitoring and control")
	}
	if opts.HasWebUI {
		features = append(features, "Web-based user interface for configuration and monitoring")
	}
	if opts.HasService {
		features = append(features, "Persistent background service with automatic restart")
	}
	if len(opts.Architectures) > 1 {
		features = append(features, fmt.Sprintf("Multi-architecture support: %s", architectureString(opts.Architectures)))
	} else if len(opts.Architectures) == 1 {
		features = append(features, fmt.Sprintf("Optimised for %s", opts.Architectures[0]))
	}
	if len(opts.Tags) > 0 {
		features = append(features, fmt.Sprintf("Topics: %s", strings.Join(opts.Tags, ", ")))
	}
	return features
}

// --- GenerateUsageGuide ------------------------------------------------------

func (d *DocGenerator) GenerateUsageGuide(opts DocOptions) *GeneratedDoc {
	var sb strings.Builder
	title := opts.ProjectName
	if title == "" {
		title = "Module"
	}

	sb.WriteString(fmt.Sprintf("# %s — Usage Guide\n\n", title))

	// Prerequisites
	sb.WriteString("## Prerequisites\n\n")
	sb.WriteString("Before using this module, ensure you have:\n\n")
	sb.WriteString("- A rooted Android device (Android 5.0+)\n")
	sb.WriteString("- One of the supported module managers installed:\n")
	sb.WriteString("  - Magisk 24.0 or later\n")
	sb.WriteString("  - KernelSU 0.6.0 or later\n")
	sb.WriteString("  - APatch 10000 or later\n")
	sb.WriteString("- Sufficient storage space\n")
	sb.WriteString("- A backup of your current configuration (recommended)\n\n")

	// Installation
	sb.WriteString("## Installation\n\n")
	sb.WriteString("![Step 1: Open Module Manager](screenshots/step1_open_manager.png)\n\n")
	sb.WriteString("**Step 1:** Open your module manager application.\n\n")
	sb.WriteString("![Step 2: Navigate to Install](screenshots/step2_navigate_install.png)\n\n")
	sb.WriteString("**Step 2:** Navigate to the **Modules** section and tap **Install from storage**.\n\n")
	sb.WriteString("![Step 3: Select ZIP](screenshots/step3_select_zip.png)\n\n")
	sb.WriteString("**Step 3:** Locate and select the downloaded `.zip` file.\n\n")
	sb.WriteString("![Step 4: Confirm](screenshots/step4_confirm.png)\n\n")
	sb.WriteString("**Step 4:** Confirm the installation and wait for it to complete.\n\n")
	sb.WriteString("![Step 5: Reboot](screenshots/step5_reboot.png)\n\n")
	sb.WriteString("**Step 5:** Tap **Reboot** when prompted.\n\n")

	// Configuration
	sb.WriteString("## Configuration\n\n")
	sb.WriteString("After installation, you can configure the module by editing files in:\n\n")
	sb.WriteString("```\n/data/adb/modules/<module-name>/config/\n```\n\n")
	sb.WriteString("### Common Settings\n\n")
	sb.WriteString("| Setting | Type | Default | Description |\n")
	sb.WriteString("|---------|------|---------|-------------|\n")
	sb.WriteString("| enabled | bool | true | Enable/disable the module |\n")
	sb.WriteString("| log_level | string | info | Logging verbosity (debug, info, warn, error) |\n")
	sb.WriteString("| interval | int | 30 | Polling interval in seconds |\n\n")

	if opts.HasDaemon {
		sb.WriteString("### Daemon Configuration\n\n")
		sb.WriteString("The daemon can be controlled via the command line:\n\n")
		sb.WriteString("```bash\n")
		sb.WriteString("# Start daemon\n")
		sb.WriteString(fmt.Sprintf("adb shell su -c '%s-cli daemon start'\n\n", title))
		sb.WriteString("# Stop daemon\n")
		sb.WriteString(fmt.Sprintf("adb shell su -c '%s-cli daemon stop'\n\n", title))
		sb.WriteString("# View status\n")
		sb.WriteString(fmt.Sprintf("adb shell su -c '%s-cli daemon status'\n", title))
		sb.WriteString("```\n\n")
	}

	// Troubleshooting
	sb.WriteString("## Troubleshooting\n\n")
	sb.WriteString("| Symptom | Likely Cause | Solution |\n")
	sb.WriteString("|---------|--------------|----------|\n")
	sb.WriteString("| Module not loading | Incompatible manager version | Update to latest version |\n")
	sb.WriteString("| Service crashes | Config syntax error | Check logs: `adb logcat -s ModuleTag` |\n")
	sb.WriteString("| Performance issues | Polling interval too low | Increase interval in config |\n")
	sb.WriteString("| Permission denied | Missing root access | Grant root to module manager |\n")
	sb.WriteString("| Boot loop | Incompatible module | Disable module via recovery/manager |\n\n")

	// FAQ
	sb.WriteString("## FAQ\n\n")
	sb.WriteString("### Will this module pass SafetyNet/Play Integrity?\n\n")
	sb.WriteString("The module is designed to be systemless and should not affect SafetyNet or Play Integrity checks. However, no guarantee can be made as detection methods evolve.\n\n")
	sb.WriteString("### Can I use this module alongside other modules?\n\n")
	sb.WriteString("Yes. The module uses standard module conventions and should be compatible with other modules. Ensure there are no conflicts in the files they modify.\n\n")
	sb.WriteString("### How do I completely remove the module?\n\n")
	sb.WriteString("See the [Uninstallation](#uninstallation) section below.\n\n")

	// Uninstallation
	sb.WriteString("## Uninstallation\n\n")
	sb.WriteString("To completely remove the module:\n\n")
	sb.WriteString("1. Open your module manager.\n")
	sb.WriteString("2. Navigate to **Modules** and find this module.\n")
	sb.WriteString("3. Tap the **three-dot menu** → **Remove** (or **Uninstall**).\n")
	sb.WriteString("4. **Reboot** your device.\n\n")
	sb.WriteString("Alternatively, via ADB:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("adb shell su -c 'magisk --remove-module <module-name>'\n")
	sb.WriteString("adb shell su -c 'rm -rf /data/adb/modules/<module-name>'\n")
	sb.WriteString("adb reboot\n")
	sb.WriteString("```\n\n")
	sb.WriteString("> **Note:** Removing the module does not uninstall any changes made to other modules or system files.\n")

	return &GeneratedDoc{
		Filename: "USAGE.md",
		Content:  sb.String(),
		Type:     "usage",
	}
}

// --- GenerateAPIDoc ----------------------------------------------------------

func (d *DocGenerator) GenerateAPIDoc(opts DocOptions) *GeneratedDoc {
	var sb strings.Builder
	title := opts.ProjectName
	if title == "" {
		title = "Module"
	}

	sb.WriteString(fmt.Sprintf("# %s — API Documentation\n\n", title))
	sb.WriteString("This document describes the HTTP/CLI API exposed by the module daemon.\n\n")

	// Overview
	sb.WriteString("## Overview\n\n")
	sb.WriteString("The daemon listens on a local socket and exposes a RESTful HTTP API for control and data access.\n\n")
	sb.WriteString("| Property | Value |\n")
	sb.WriteString("|----------|-------|\n")
	sb.WriteString("| Base URL | `http://127.0.0.1:<port>/api/v1` |\n")
	sb.WriteString("| Protocol | HTTP 1.1 / JSON |\n")
	sb.WriteString("| Auth     | Local socket (no external network) |\n\n")

	// Endpoints
	sb.WriteString("## Endpoints\n\n")

	sb.WriteString("### `GET /status`\n\n")
	sb.WriteString("Returns the current status of the module.\n\n")
	sb.WriteString("**Response**\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"running\": true,\n")
	sb.WriteString("  \"uptime\": 3600,\n")
	sb.WriteString("  \"version\": \"")
	sb.WriteString(opts.Version)
	sb.WriteString("\",\n")
	sb.WriteString("  \"module_type\": \"")
	sb.WriteString(opts.ModuleType)
	sb.WriteString("\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### `GET /config`\n\n")
	sb.WriteString("Returns the current configuration.\n\n")
	sb.WriteString("**Response**\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"enabled\": true,\n")
	sb.WriteString("  \"log_level\": \"info\",\n")
	sb.WriteString("  \"interval\": 30\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### `POST /config`\n\n")
	sb.WriteString("Updates the configuration. Requires restart to take effect.\n\n")
	sb.WriteString("**Request Body**\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"log_level\": \"debug\",\n")
	sb.WriteString("  \"interval\": 60\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### `POST /service/restart`\n\n")
	sb.WriteString("Restarts the background service. Returns `202 Accepted`.\n\n")

	sb.WriteString("### `GET /logs`\n\n")
	sb.WriteString("Returns recent log entries.\n\n")
	sb.WriteString("**Query Parameters**\n\n")
	sb.WriteString("| Parameter | Type | Default | Description |\n")
	sb.WriteString("|-----------|------|---------|-------------|\n")
	sb.WriteString("| limit | int | 100 | Max entries to return |\n")
	sb.WriteString("| level | string | all | Filter by level (debug, info, warn, error) |\n\n")

	sb.WriteString("**Response**\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"logs\": [\n")
	sb.WriteString("    {\n")
	sb.WriteString("      \"timestamp\": \"2025-01-15T10:30:00Z\",\n")
	sb.WriteString("      \"level\": \"info\",\n")
	sb.WriteString("      \"message\": \"Module initialized\"\n")
	sb.WriteString("    }\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"total\": 1\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	// Error codes
	sb.WriteString("## Error Codes\n\n")
	sb.WriteString("| Code | Description |\n")
	sb.WriteString("|------|-------------|\n")
	sb.WriteString("| 200 | Success |\n")
	sb.WriteString("| 202 | Request accepted (async operation) |\n")
	sb.WriteString("| 400 | Bad request — invalid parameters |\n")
	sb.WriteString("| 401 | Unauthorized — authentication required |\n")
	sb.WriteString("| 404 | Not found — endpoint does not exist |\n")
	sb.WriteString("| 409 | Conflict — operation already in progress |\n")
	sb.WriteString("| 500 | Internal server error |\n")
	sb.WriteString("| 503 | Service unavailable — daemon not running |\n\n")

	// Authentication
	sb.WriteString("## Authentication\n\n")
	sb.WriteString("The API is exposed on a local Unix socket or loopback interface. External network access is not permitted by default.\n\n")
	sb.WriteString("If your module requires external network access (not recommended), enable authentication in the daemon config:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"auth_enabled\": true,\n")
	sb.WriteString("  \"auth_token\": \"<your-secret-token>\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Include the token in requests:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("curl -H 'Authorization: Bearer <token>' http://127.0.0.1:8080/api/v1/status\n")
	sb.WriteString("```\n")

	return &GeneratedDoc{
		Filename: "API.md",
		Content:  sb.String(),
		Type:     "api",
	}
}

// --- GenerateChangelog -------------------------------------------------------

func (d *DocGenerator) GenerateChangelog(opts DocOptions) *GeneratedDoc {
	var sb strings.Builder
	title := opts.ProjectName
	if title == "" {
		title = "Module"
	}

	version := opts.Version
	if version == "" {
		version = "1.0.0"
	}
	date := time.Now().Format("2006-01-02")

	sb.WriteString("# Changelog\n\n")
	sb.WriteString(fmt.Sprintf("All notable changes to **%s** will be documented in this file.\n\n", title))
	sb.WriteString("Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).\n\n")

	sb.WriteString("## [Unreleased]\n\n")
	sb.WriteString("- None yet.\n\n")

	sb.WriteString(fmt.Sprintf("## [%s] - %s\n\n", version, date))
	sb.WriteString("### Added\n\n")
	sb.WriteString(fmt.Sprintf("- Initial release of %s\n", title))
	if opts.HasDaemon {
		sb.WriteString("- Background daemon with REST API\n")
	}
	if opts.HasWebUI {
		sb.WriteString("- Web-based configuration interface\n")
	}
	if opts.HasService {
		sb.WriteString("- Persistent background service\n")
	}
	if len(opts.Architectures) > 0 {
		sb.WriteString(fmt.Sprintf("- Multi-architecture support: %s\n", architectureString(opts.Architectures)))
	}
	if len(opts.Dependencies) > 0 {
		sb.WriteString(fmt.Sprintf("- Dependencies: %s\n", strings.Join(opts.Dependencies, ", ")))
	}
	sb.WriteString("- Module installer for Magisk, KernelSU, and APatch\n")
	sb.WriteString("- Comprehensive documentation (README, USAGE, API)\n")

	return &GeneratedDoc{
		Filename: "CHANGELOG.md",
		Content:  sb.String(),
		Type:     "changelog",
	}
}

// --- GenerateAll -------------------------------------------------------------

func (d *DocGenerator) GenerateAll(opts DocOptions) []*GeneratedDoc {
	return []*GeneratedDoc{
		d.GenerateReadme(opts),
		d.GenerateUsageGuide(opts),
		d.GenerateAPIDoc(opts),
		d.GenerateChangelog(opts),
	}
}
