package handler

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type SecurityHandler struct {
	scanner *service.SecurityScanner
	db      *sql.DB
	fr      *service.FileContentRepo // S3-first content access (optional)
}

func NewSecurityHandler(scanner *service.SecurityScanner, db *sql.DB) *SecurityHandler {
	return &SecurityHandler{scanner: scanner, db: db}
}

// SetFileContentRepo injects the S3-first file content repository.
func (h *SecurityHandler) SetFileContentRepo(fr *service.FileContentRepo) {
	h.fr = fr
}

type ScanRequest struct {
	Files map[string]string `json:"files"`
}

func (h *SecurityHandler) ScanFiles(c fiber.Ctx) error {
	var req ScanRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if len(req.Files) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "no files provided"})
	}

	result := h.scanner.ScanFiles(req.Files)
	return c.JSON(result)
}

func (h *SecurityHandler) ScanProject(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project id required"})
	}

	files, err := h.fr.ReadAllContent(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to read project files"})
	}

	if len(files) == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "no files found in project"})
	}

	result := h.scanner.ScanFiles(files)
	return c.JSON(result)
}

// simulated known vulnerabilities for scanning
var knownVulns = map[string][]struct {
	ID       string
	Severity string
	Fix      string
	Desc     string
}{
	"lodash":          {{ID: "CVE-2024-23338", Severity: "high", Fix: ">=4.17.21", Desc: "Prototype pollution in lodash"}},
	"axios":           {{ID: "CVE-2024-39338", Severity: "critical", Fix: ">=1.7.4", Desc: "SSRF vulnerability in axios"}},
	"express":         {{ID: "CVE-2024-29041", Severity: "medium", Fix: ">=4.19.2", Desc: "Open redirect in Express"}},
	"minimatch":       {{ID: "CVE-2024-4068", Severity: "high", Fix: ">=3.1.2", Desc: "ReDoS in minimatch"}},
	"golang.org/x/net": {{ID: "CVE-2024-24791", Severity: "medium", Fix: ">=0.24.0", Desc: "Memory exhaustion in HTTP/2"}},
	"gopkg.in/yaml.v2": {{ID: "CVE-2024-32000", Severity: "high", Fix: ">=2.4.0", Desc: "Unmarshal infinite loop"}},
	"requests":        {{ID: "CVE-2024-3651", Severity: "medium", Fix: ">=2.31.0", Desc: "Session handling bypass"}},
	"cryptography":    {{ID: "CVE-2024-26131", Severity: "critical", Fix: ">=42.0.0", Desc: "Buffer overflow in OpenSSL bindings"}},
	"react":           {{ID: "CVE-2024-36138", Severity: "low", Fix: ">=18.3.1", Desc: "XSS via dangerouslySetInnerHTML"}},
	"semver":          {{ID: "CVE-2024-4067", Severity: "low", Fix: ">=7.6.2", Desc: "ReDoS in semver range"}},
}

type VulnResult struct {
	Dependency string `json:"dependency"`
	Version    string `json:"version"`
	VulnID     string `json:"vuln_id"`
	Severity   string `json:"severity"`
	FixVersion string `json:"fix_version"`
	Desc       string `json:"desc"`
}

func parseDeps(content string) map[string]string {
	deps := map[string]string{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		// package.json style: "name": "version"
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			name := strings.Trim(strings.TrimSpace(parts[0]), "\"' ")
			ver := strings.Trim(strings.TrimSpace(parts[1]), "\",' ")
			if name != "" && ver != "" {
				deps[name] = ver
			}
		}
	}
	return deps
}

func (h *SecurityHandler) ScanVulnerabilities(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project id required"})
	}

	pid, err := strconv.ParseInt(projectID, 10, 64)
	if err != nil {
		pid = 0
	}

	allFiles, err := h.fr.ReadAllContent(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to read files"})
	}

	var results []VulnResult
	totalDeps := 0
	criticalCount := 0
	highCount := 0
	mediumCount := 0
	lowCount := 0

	for path, content := range allFiles {
		if !strings.HasSuffix(path, "package.json") && !strings.HasSuffix(path, "requirements.txt") && !strings.HasSuffix(path, "go.mod") {
			continue
		}
		deps := parseDeps(content)
		for name, ver := range deps {
			totalDeps++
			if vulns, ok := knownVulns[name]; ok {
				for _, v := range vulns {
					results = append(results, VulnResult{
						Dependency: name,
						Version:    ver,
						VulnID:     v.ID,
						Severity:   v.Severity,
						FixVersion: v.Fix,
						Desc:       v.Desc,
					})
					switch v.Severity {
					case "critical":
						criticalCount++
					case "high":
						highCount++
					case "medium":
						mediumCount++
					case "low":
						lowCount++
					}
				}
			}
		}
	}

	resultsJSON, _ := json.Marshal(results)
	h.db.Exec(
		"INSERT INTO vulnerability_scans (project_id, scanner, total_deps, vulnerable_deps, critical_count, high_count, medium_count, low_count, results) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		pid, "mock-scanner", totalDeps, len(results), criticalCount, highCount, mediumCount, lowCount, string(resultsJSON),
	)

	return c.JSON(fiber.Map{
		"total_deps":       totalDeps,
		"vulnerable_deps":  len(results),
		"critical_count":   criticalCount,
		"high_count":       highCount,
		"medium_count":     mediumCount,
		"low_count":        lowCount,
		"results":          results,
	})
}

func (h *SecurityHandler) GetVulnHistory(c fiber.Ctx) error {
	projectID := c.Params("id")
	rows, err := h.db.Query("SELECT id, scanner, total_deps, vulnerable_deps, critical_count, high_count, medium_count, low_count, scanned_at FROM vulnerability_scans WHERE project_id = ? ORDER BY scanned_at DESC LIMIT 20", projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type ScanRecord struct {
		ID             int64  `json:"id"`
		Scanner        string `json:"scanner"`
		TotalDeps      int    `json:"total_deps"`
		VulnerableDeps int    `json:"vulnerable_deps"`
		CriticalCount  int    `json:"critical_count"`
		HighCount      int    `json:"high_count"`
		MediumCount    int    `json:"medium_count"`
		LowCount       int    `json:"low_count"`
		ScannedAt      string `json:"scanned_at"`
	}
	var records []ScanRecord
	for rows.Next() {
		var r ScanRecord
		if err := rows.Scan(&r.ID, &r.Scanner, &r.TotalDeps, &r.VulnerableDeps, &r.CriticalCount, &r.HighCount, &r.MediumCount, &r.LowCount, &r.ScannedAt); err != nil {
			continue
		}
		records = append(records, r)
	}
	if records == nil {
		records = []ScanRecord{}
	}
	return c.JSON(fiber.Map{"history": records})
}
