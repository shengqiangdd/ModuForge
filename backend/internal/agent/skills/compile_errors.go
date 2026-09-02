package skills

import (
	"fmt"
	"regexp"
	"strings"
)

// BuildResult holds structured build information for the runner to consume.
type BuildResult struct {
	BuildReady    bool              `json:"build_ready"`
	Success       bool              `json:"success"`
	SourceResults map[string]string `json:"source_results"` // "rust"|"cpp"|"go" -> result message
	Errors        []CompileError    `json:"errors"`
	Warnings      []string          `json:"warnings"`
}

// CompileError holds parsed compilation error details.
type CompileError struct {
	SourceType string `json:"source_type"` // "rust", "cpp", "go"
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Message    string `json:"message"`
	ErrorType  string `json:"error_type"` // "syntax", "undefined", "type_mismatch", "linker", "missing_import", "timeout", "unknown"
}

// sourceInfo holds the results of a single-pass source detection walk.
type sourceInfo struct {
	hasCargo bool
	cargoDir string
	hasCpp   bool
	hasGo    bool
	goModDir string
	hasShell bool
}

// compileResult holds aggregated compilation results.
type compileResult struct {
	buildSuccess  bool
	sourceResults map[string]string // "rust"|"cpp"|"go" -> result message
	errors        []CompileError
	warnings      []string
	log           string // human-readable compilation log
}

// parseCompileErrors extracts structured error information from compiler output.
func parseCompileErrors(sourceType, output string) []CompileError {
	var errors []CompileError
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		ce := CompileError{SourceType: sourceType}

		switch sourceType {
		case "rust":
			// Rust error format: error[E0XXX]: message\n  --> file:line:col
			if m := regexp.MustCompile(`error\[(E\d+)\]:\s*(.+)`).FindStringSubmatch(line); len(m) > 2 {
				ce.ErrorType = classifyRustError(m[1])
				ce.Message = m[2]
			} else if m := regexp.MustCompile(`--> (.+):(\d+):(\d+)`).FindStringSubmatch(line); len(m) > 3 {
				ce.File = m[1]
				ce.Line = atoi(m[2])
				ce.Column = atoi(m[3])
			} else if strings.HasPrefix(line, "error") {
				ce.Message = strings.TrimPrefix(line, "error: ")
				ce.ErrorType = classifyRustError("")
			}

		case "go":
			// Go error format: file:line:col: message
			if m := regexp.MustCompile(`(.+):(\d+):(\d+):\s*(.+)`).FindStringSubmatch(line); len(m) > 4 {
				ce.File = m[1]
				ce.Line = atoi(m[2])
				ce.Column = atoi(m[3])
				ce.Message = m[4]
				ce.ErrorType = classifyGoError(m[4])
			} else if strings.Contains(line, "undefined") || strings.Contains(line, "undeclared") {
				ce.Message = line
				ce.ErrorType = "undefined"
			}

		case "cpp":
			// C++ error format: file:line:col: error: message
			if m := regexp.MustCompile(`(.+):(\d+):(\d+):\s*(?:error|warning):\s*(.+)`).FindStringSubmatch(line); len(m) > 4 {
				ce.File = m[1]
				ce.Line = atoi(m[2])
				ce.Column = atoi(m[3])
				ce.Message = m[4]
				ce.ErrorType = classifyCppError(m[4])
			} else if strings.Contains(line, "error:") {
				ce.Message = line
				ce.ErrorType = "unknown"
			}
		}

		if ce.Message != "" {
			errors = append(errors, ce)
		}
	}
	return errors
}

// atoi is a simple string-to-int converter for error parsing.
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

// classifyRustError categorizes Rust compiler error codes.
func classifyRustError(code string) string {
	switch {
	case strings.HasPrefix(code, "E0"):
		if strings.Contains(code, "0412") || strings.Contains(code, "0432") || strings.Contains(code, "0433") {
			return "missing_import"
		}
		if strings.Contains(code, "0596") || strings.Contains(code, "0599") || strings.Contains(code, "0609") || strings.Contains(code, "0603") {
			return "undefined"
		}
		if strings.Contains(code, "0308") || strings.Contains(code, "0305") || strings.Contains(code, "0382") {
			return "type_mismatch"
		}
		if strings.Contains(code, "0001") || strings.Contains(code, "0002") || strings.Contains(code, "0003") || strings.Contains(code, "0004") {
			return "syntax"
		}
		if strings.Contains(code, "0277") || strings.Contains(code, "0282") {
			return "type_mismatch"
		}
		if strings.Contains(code, "0583") || strings.Contains(code, "0584") {
			return "undefined"
		}
	case strings.Contains(code, "linker"):
		return "linker"
	}
	return "unknown"
}

// classifyGoError categorizes Go compiler error messages.
func classifyGoError(msg string) string {
	switch {
	case strings.Contains(msg, "undefined") || strings.Contains(msg, "undeclared"):
		return "undefined"
	case strings.Contains(msg, "cannot use") || strings.Contains(msg, "type mismatch"):
		return "type_mismatch"
	case strings.Contains(msg, "syntax error"):
		return "syntax"
	case strings.Contains(msg, "cannot find package") || strings.Contains(msg, "no required module"):
		return "missing_import"
	case strings.Contains(msg, "imported and not used"):
		return "unused_import"
	case strings.Contains(msg, "not enough arguments") || strings.Contains(msg, "too many arguments"):
		return "type_mismatch"
	case strings.Contains(msg, "cannot call") || strings.Contains(msg, "has no field") || strings.Contains(msg, "has no method"):
		return "undefined"
	case strings.Contains(msg, "missing return"):
		return "syntax"
	case strings.Contains(msg, "expected") || strings.Contains(msg, "unexpected"):
		return "syntax"
	case strings.Contains(msg, "go.mod"):
		return "missing_import"
	case strings.Contains(msg, "module declared multiple times") || strings.Contains(msg, "non-Go module"):
		return "missing_import"
	}
	return "unknown"
}

// classifyCppError categorizes C++ compiler error messages.
func classifyCppError(msg string) string {
	switch {
	case strings.Contains(msg, "undeclared") || strings.Contains(msg, "was not declared"):
		return "undefined"
	case strings.Contains(msg, "no matching function") || strings.Contains(msg, "cannot convert"):
		return "type_mismatch"
	case strings.Contains(msg, "expected") || strings.Contains(msg, "unexpected"):
		return "syntax"
	case strings.Contains(msg, "fatal error: ") || strings.Contains(msg, "No such file"):
		return "missing_import"
	case strings.Contains(msg, "undefined reference"):
		return "linker"
	}
	return "unknown"
}

// classifyNDKError describes the NDK/cross-compile issue in human-readable form.
func classifyNDKError(output string) string {
	switch {
	case strings.Contains(output, "linker") && strings.Contains(output, "not found"):
		return "NDK linker not found - NDK may not be installed at /opt/android-ndk"
	case strings.Contains(output, "cannot find -l"):
		return "NDK linker cannot find system libraries - NDK sysroot may be incomplete"
	case strings.Contains(output, "aarch64-linux-android"):
		return "Android cross-compile toolchain not fully configured"
	default:
		return "NDK cross-compilation environment issue"
	}
}

// formatCompileErrors formats a slice of CompileErrors into a readable string with fix hints.
func formatCompileErrors(errors []CompileError) string {
	var b strings.Builder
	for _, e := range errors {
		loc := ""
		if e.File != "" {
			loc = fmt.Sprintf("%s:%d", e.File, e.Line)
			if e.Column > 0 {
				loc = fmt.Sprintf("%s:%d", loc, e.Column)
			}
		}
		if loc != "" {
			b.WriteString(fmt.Sprintf("    [%s] %s: %s\n", e.ErrorType, loc, e.Message))
		} else {
			b.WriteString(fmt.Sprintf("    [%s] %s\n", e.ErrorType, e.Message))
		}
		// Append fix hint for each error
		if hint := generateBuildFixHint(e); hint != "" {
			b.WriteString(fmt.Sprintf("    💡 %s\n", hint))
		}
	}
	return b.String()
}

// generateBuildFixHint creates an actionable fix suggestion for a compile error.
func generateBuildFixHint(e CompileError) string {
	msgLower := strings.ToLower(e.Message)
	switch e.SourceType {
	case "go":
		switch e.ErrorType {
		case "undefined":
			return fmt.Sprintf("Check if the identifier is declared. If it's from another package, add the correct import. File: %s:%d", e.File, e.Line)
		case "missing_import":
			if strings.Contains(msgLower, "cannot find package") {
				return "Run 'go mod tidy' to add missing dependencies, or check the package path spelling."
			}
			if strings.Contains(msgLower, "go.mod") {
				return "Ensure go.mod exists in the project root with 'module <name>' declaration."
			}
			return "Check import paths and go.mod dependencies."
		case "type_mismatch":
			if strings.Contains(msgLower, "not enough arguments") || strings.Contains(msgLower, "too many arguments") {
				return "Check function signature and provide the correct number of arguments."
			}
			return "Check types match at the assignment or function call site."
		case "unused_import":
			return "Remove the unused import, or use the imported package in your code."
		case "syntax":
			if strings.Contains(msgLower, "missing return") {
				return "Add a return statement to the function. All non-void functions must return a value."
			}
			return "Fix syntax: check brackets, semicolons, and Go syntax rules."
		}
	case "rust":
		switch e.ErrorType {
		case "missing_import":
			return "Add the missing crate to Cargo.toml [dependencies] and add 'use' statement."
		case "undefined":
			return "Check if the identifier is declared and in scope. Check import paths."
		case "type_mismatch":
			return "Check expected vs actual types. Use .into() or explicit type conversion if needed."
		case "syntax":
			return "Fix Rust syntax: check semicolons, braces, and match arms."
		}
	case "cpp":
		switch e.ErrorType {
		case "missing_import":
			if strings.Contains(msgLower, "fatal error:") || strings.Contains(msgLower, "no such file") {
				return "Add the missing #include directive or check header file paths."
			}
			return "Check #include paths and header file existence."
		case "undefined":
			return "Check if the identifier is declared. Add missing #include or forward declaration."
		case "type_mismatch":
			return "Check argument types match the function parameter types."
		case "syntax":
			return "Fix C++ syntax: check semicolons, braces, and statement structure."
		case "linker":
			return "Undefined reference: ensure all declared functions have implementations."
		}
	}
	return ""
}

// generateBuildErrorSummary creates a concise summary of all build errors for the agent.
func generateBuildErrorSummary(errors []CompileError) string {
	if len(errors) == 0 {
		return ""
	}

	// Count by type
	typeCounts := make(map[string]int)
	for _, e := range errors {
		typeCounts[e.ErrorType]++
	}

	var parts []string
	for errType, count := range typeCounts {
		parts = append(parts, fmt.Sprintf("%d %s error(s)", count, errType))
	}

	return fmt.Sprintf("Build failed with %d error(s): %s", len(errors), strings.Join(parts, ", "))
}
