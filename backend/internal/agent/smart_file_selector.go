package agent

import (
	"sort"
	"strings"
)

// FileRelevance represents a file's relevance score to a user request.
type FileRelevance struct {
	Path    string
	Score   float64
	Reasons []string
}

// SelectRelevantFiles chooses the most relevant project files for a given request.
// Uses filename matching, function name matching, and domain-specific heuristics.
func SelectRelevantFiles(projectIndex *ProjectIndex, request string, maxFiles int) []FileRelevance {
	if projectIndex == nil || len(projectIndex.FileTree) == 0 {
		return nil
	}

	keywords := extractKeywords(request)
	if len(keywords) == 0 {
		return nil
	}

	var results []FileRelevance

	for _, file := range projectIndex.FileTree {
		score := 0.0
		var reasons []string

		// Filename matching
		for _, kw := range keywords {
			if containsIgnoreCase(file, kw) {
				score += 0.3
				reasons = append(reasons, "filename match: "+kw)
			}
		}

		// Function name matching
		if funcs, ok := projectIndex.GoFunctions[file]; ok {
			for _, f := range funcs {
				for _, kw := range keywords {
					if containsIgnoreCase(f, kw) {
						score += 0.4
						reasons = append(reasons, "function match: "+f)
					}
				}
			}
		}

		// Type/interface matching
		if types, ok := projectIndex.GoTypes[file]; ok {
			for _, t := range types {
				for _, kw := range keywords {
					if containsIgnoreCase(t, kw) {
						score += 0.4
						reasons = append(reasons, "type match: "+t)
					}
				}
			}
		}

		// Fingerprint matching
		if fingerprint, ok := projectIndex.FileFingerprints[file]; ok {
			for _, kw := range keywords {
				if containsIgnoreCase(fingerprint, kw) {
					score += 0.3
					reasons = append(reasons, "fingerprint match: "+kw)
				}
			}
		}

		// Performance-related keywords
		if containsAny(request, []string{"性能", "performance", "速度", "speed", "优化", "optimize", "fast", "slow", "latency", "瓶颈"}) {
			if containsAny(file, []string{"runner", "llm", "cache", "pool", "worker", "config", "token", "compact", "optimizer", "budget"}) {
				score += 0.5
				reasons = append(reasons, "performance-related file")
			}
		}

		// Security-related keywords
		if containsAny(request, []string{"安全", "security", "漏洞", "vulnerability", "权限", "permission", "auth", "encrypt"}) {
			if containsAny(file, []string{"auth", "permission", "security", "bash", "csp", "jwt", "token", "skill", "checker"}) {
				score += 0.5
				reasons = append(reasons, "security-related file")
			}
		}

		// Build/test related keywords
		if containsAny(request, []string{"构建", "build", "测试", "test", "编译", "compile", "错误", "error", "fix"}) {
			if containsAny(file, []string{"build", "test", "healer", "error", "fix", "recovery", "syntax"}) {
				score += 0.5
				reasons = append(reasons, "build/test-related file")
			}
		}

		// Memory/context related keywords
		if containsAny(request, []string{"记忆", "memory", "上下文", "context", "会话", "session", "历史", "history"}) {
			if containsAny(file, []string{"memory", "context", "session", "history", "compact", "persist", "recall", "condenser"}) {
				score += 0.5
				reasons = append(reasons, "memory/context-related file")
			}
		}

		// Planning/task related keywords
		if containsAny(request, []string{"计划", "plan", "任务", "task", "步骤", "step", "workflow"}) {
			if containsAny(file, []string{"plan", "task", "planner", "workflow", "enhanced"}) {
				score += 0.5
				reasons = append(reasons, "planning-related file")
			}
		}

		if score > 0 {
			results = append(results, FileRelevance{Path: file, Score: score, Reasons: reasons})
		}
	}

	// Expand with dependencies
	expanded := expandWithDependencies(projectIndex, results, 5)
	results = append(results, expanded...)

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > maxFiles {
		results = results[:maxFiles]
	}
	return results
}

// expandWithDependencies adds upstream/downstream files from the dependency graph.
func expandWithDependencies(idx *ProjectIndex, selected []FileRelevance, maxExtra int) []FileRelevance {
	seen := map[string]bool{}
	for _, fr := range selected {
		seen[fr.Path] = true
	}

	var expanded []FileRelevance
	for _, fr := range selected {
		if deps, ok := idx.DepGraph[fr.Path]; ok {
			for _, dep := range deps {
				if !seen[dep] && len(expanded) < maxExtra {
					seen[dep] = true
					expanded = append(expanded, FileRelevance{
						Path:    dep,
						Score:   fr.Score * 0.5,
						Reasons: []string{"dependency of " + fr.Path},
					})
				}
			}
		}
	}
	return expanded
}

func extractKeywords(text string) []string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '，' || r == '。' || r == ',' || r == '.' ||
			r == '!' || r == '?' || r == '：' || r == ':' || r == '；' ||
			r == '(' || r == ')' || r == '[' || r == ']' || r == '{' || r == '}'
	})
	var keywords []string
	seen := map[string]bool{}
	for _, w := range words {
		w = strings.ToLower(w)
		if len(w) > 1 && !seen[w] {
			keywords = append(keywords, w)
			seen[w] = true
		}
	}
	return keywords
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func containsAny(s string, keywords []string) bool {
	lower := strings.ToLower(s)
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
