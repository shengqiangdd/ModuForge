package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// AutoFixCompileErrorsV2 is the enhanced auto-fix with:
// 1. Full file content sent to LLM (not just context window)
// 2. Error categorization (syntax vs link vs runtime)
// 3. Language-specific repair prompts
// 4. Progressive file-level isolation (fix one file at a time)
// 5. Model-aware repair strategy (free vs paid models)
func (b *Builder) AutoFixCompileErrorsV2(
	ctx context.Context,
	projectDir string,
	compileErr error,
	logFn func(string),
) bool {
	if compileErr == nil {
		return true
	}

	errMsg := compileErr.Error()
	logFn(fmt.Sprintf("\n🔧 Auto-fix v2: Analyzing compilation errors...\n"))

	// Categorize errors
	categorized := categorizeErrors(errMsg)
	if categorized.TotalCount() == 0 {
		logFn("  ⚠️  Could not parse compilation errors\n")
		return false
	}

	logFn(fmt.Sprintf("  📊 Error summary: %d syntax, %d link, %d type, %d other\n",
		categorized.Syntax, categorized.Link, categorized.Type, categorized.Other))

	// Strategy 1: Try post-processing fixes first (fast, no LLM needed)
	postFixed := b.tryPostProcessFixes(projectDir, categorized, logFn)
	if postFixed {
		logFn("  ✅ Post-processing fixes applied\n")
		return true
	}

	// Strategy 2: LLM-based fix, file by file (most effective)
	// Sort files by error count (fix most errors first)
	type fileErrPair struct {
		file   string
		errors []CompileError
	}
	var pairs []fileErrPair
	for file, errs := range categorized.ByFile {
		pairs = append(pairs, fileErrPair{file, errs})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return len(pairs[i].errors) > len(pairs[j].errors)
	})

	fixedAny := false
	for _, pair := range pairs {
		fixed := b.fixFileWithFullContext(ctx, projectDir, pair.file, pair.errors, errMsg, logFn)
		if fixed {
			fixedAny = true
		}
	}

	return fixedAny
}

// ErrorCategories groups errors by type for strategy selection.
type ErrorCategories struct {
	ByFile map[string][]CompileError
	Syntax int
	Link   int
	Type   int
	Other  int
}

func (c *ErrorCategories) TotalCount() int {
	return c.Syntax + c.Link + c.Type + c.Other
}

// categorizeErrors parses compilation output and categorizes errors.
func categorizeErrors(errMsg string) *ErrorCategories {
	cats := &ErrorCategories{
		ByFile: make(map[string][]CompileError),
	}

	lines := strings.Split(errMsg, "\n")
	for _, line := range lines {
		errs := parseLineErrors(line)
		for _, e := range errs {
			cats.ByFile[e.File] = append(cats.ByFile[e.File], e)

			msg := strings.ToLower(e.Message)
			switch {
			case strings.Contains(msg, "undefined reference") ||
				strings.Contains(msg, "undefined symbol") ||
				strings.Contains(msg, "ld:"):
				cats.Link++
			case strings.Contains(msg, "cannot use") ||
				strings.Contains(msg, "cannot convert") ||
				strings.Contains(msg, "type mismatch") ||
				strings.Contains(msg, "incompatible"):
				cats.Type++
			case strings.Contains(msg, "syntax error") ||
				strings.Contains(msg, "expected") ||
				strings.Contains(msg, "unexpected") ||
				strings.Contains(msg, "undeclared") ||
				strings.Contains(msg, "not enough arguments"):
				cats.Syntax++
			default:
				cats.Other++
			}
		}
	}
	return cats
}

// parseLineErrors parses a single line of compilation output.
func parseLineErrors(line string) []CompileError {
	var errors []CompileError

	// Go: ./file.go:line:col: message
	goRe := regexp.MustCompile(`\./([^:]+):(\d+):(\d+):\s*(.+)`)
	for _, m := range goRe.FindAllStringSubmatch(line, -1) {
		var l, c int
		fmt.Sscanf(m[2], "%d", &l)
		fmt.Sscanf(m[3], "%d", &c)
		errors = append(errors, CompileError{File: m[1], Line: l, Column: c, Message: m[4]})
	}

	// C: file.c:line:col: message or file.c:line: message
	cRe := regexp.MustCompile(`([\w]+\.[chp]{1,2}):(\d+)(?::(\d+))?:\s*(.+)`)
	for _, m := range cRe.FindAllStringSubmatch(line, -1) {
		var l, c int
		fmt.Sscanf(m[2], "%d", &l)
		if m[3] != "" {
			fmt.Sscanf(m[3], "%d", &c)
		}
		errors = append(errors, CompileError{File: m[1], Line: l, Column: c, Message: m[4]})
	}

	// Link errors often don't have file:line format
	if strings.Contains(line, "undefined reference") || strings.Contains(line, "undefined symbol") {
		errors = append(errors, CompileError{
			File:    "_linker",
			Line:    0,
			Message: strings.TrimSpace(line),
		})
	}

	return errors
}

// tryPostProcessFixes attempts quick fixes without LLM.
func (b *Builder) tryPostProcessFixes(projectDir string, cats *ErrorCategories, logFn func(string)) bool {
	fixed := false

	// Fix common issues in Go files
	for file, errs := range cats.ByFile {
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		fullPath := filepath.Join(projectDir, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		original := string(content)
		_ = original
		s := original

		for _, e := range errs {
			msg := strings.ToLower(e.Message)

			// Fix: "declared but not used" — remove unused variable
			if strings.Contains(msg, "declared but not used") {
				s = fixUnusedVar(s, e)
				fixed = true
			}

			// Fix: "cannot use _ as value" — replace _ in non-assignment
			if strings.Contains(msg, "cannot use _ as") {
				s = fixBlankIdentifier(s, e)
				fixed = true
			}

			// Fix: "syntax error: unexpected" — likely truncated code
			if strings.Contains(msg, "syntax error") && strings.Contains(msg, "unexpected") {
				s = fixTruncatedCode(s, e)
				fixed = true
			}
		}

		if s != original {
			os.WriteFile(fullPath, []byte(s), 0644)
			logFn(fmt.Sprintf("  🔧 Post-processed %s\n", file))
		}
	}

	return fixed
}

// fixUnusedVar removes or prefixes unused variables.
func fixUnusedVar(code string, err CompileError) string {
	// Find the line and try to remove/comment the unused variable declaration
	lines := strings.Split(code, "\n")
	if err.Line > 0 && err.Line <= len(lines) {
		line := lines[err.Line-1]
		// Common pattern: varName := something — prefix with _ =
		if strings.Contains(line, ":=") {
			parts := strings.SplitN(line, ":=", 2)
			varName := strings.TrimSpace(parts[0])
			// Only fix if it's a simple identifier
			if varName != "" && !strings.ContainsAny(varName, " []{}()") {
				lines[err.Line-1] = "_ = " + strings.TrimSpace(parts[1])
			}
		}
	}
	return strings.Join(lines, "\n")
}

// fixBlankIdentifier replaces blank identifier in invalid positions.
func fixBlankIdentifier(code string, err CompileError) string {
	lines := strings.Split(code, "\n")
	if err.Line > 0 && err.Line <= len(lines) {
		lines[err.Line-1] = strings.ReplaceAll(lines[err.Line-1], "= _", "= nil")
	}
	return strings.Join(lines, "\n")
}

// fixTruncatedCode adds missing closing braces/brackets.
func fixTruncatedCode(code string, err CompileError) string {
	// Count open vs close braces
	openBraces := strings.Count(code, "{")
	closeBraces := strings.Count(code, "}")
	if openBraces > closeBraces {
		missing := openBraces - closeBraces
		for i := 0; i < missing; i++ {
			code += "\n}"
		}
	}
	// Also check for unterminated strings
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `"`) && !strings.HasSuffix(trimmed, `"`) && !strings.HasSuffix(trimmed, `",`) {
			lines[i] = line + `"`
		}
	}
	return strings.Join(lines, "\n")
}

// fixFileWithFullContext sends the ENTIRE file content + all errors to LLM.
// This gives the model full understanding of the codebase for better fixes.
func (b *Builder) fixFileWithFullContext(
	ctx context.Context,
	projectDir string,
	file string,
	errors []CompileError,
	compileOutput string,
	logFn func(string),
) bool {
	fullPath := filepath.Join(projectDir, file)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		logFn(fmt.Sprintf("  ⚠️  Cannot read %s: %v\n", file, err))
		return false
	}

	// Skip very large files (>50KB) — too much for LLM context
	if len(content) > 50000 {
		logFn(fmt.Sprintf("  ⚠️  %s too large (%d bytes) for LLM fix\n", file, len(content)))
		return false
	}

	// Detect language
	lang := detectLanguage(file, string(content))

	// Build language-specific context
	projectContext := gatherProjectContext(projectDir, file)

	// Build comprehensive error list
	var errList strings.Builder
	for _, e := range errors {
		if e.Line > 0 {
			errList.WriteString(fmt.Sprintf("  Line %d: %s\n", e.Line, e.Message))
		} else {
			errList.WriteString(fmt.Sprintf("  %s\n", e.Message))
		}
	}

	// Build link errors section
	var linkErrors []string
	for _, e := range errors {
		if e.File == "_linker" {
			linkErrors = append(linkErrors, e.Message)
		}
	}

	prompt := buildAutoFixPrompt(lang, file, string(content), errList.String(), linkErrors, projectContext, compileOutput)

	logFn(fmt.Sprintf("  📤 Sending %s (%d bytes, %d errors) to LLM...\n", file, len(content), len(errors)))

	endpoint, apiKey, model := b.resolveLLMForFix()
	if endpoint == "" || apiKey == "" {
		logFn("  ⚠️  No LLM configured for auto-fix\n")
		return false
	}

	fixedCode, fixErr := callLLMForFix(ctx, endpoint, apiKey, model, prompt)
	if fixErr != nil {
		logFn(fmt.Sprintf("  ⚠️  LLM call failed: %v\n", fixErr))
		return false
	}

	// Extract clean code from response
	fixedCode = extractCodeFromResponseV2(fixedCode, lang)
	if fixedCode == "" || fixedCode == string(content) {
		logFn(fmt.Sprintf("  ⚠️  LLM returned unchanged code for %s\n", file))
		return false
	}

	if err := os.WriteFile(fullPath, []byte(fixedCode), 0644); err != nil {
		logFn(fmt.Sprintf("  ⚠️  Write failed: %v\n", err))
		return false
	}

	logFn(fmt.Sprintf("  ✅ Fixed %s (%d bytes → %d bytes)\n", file, len(content), len(fixedCode)))
	return true
}

// buildAutoFixPrompt creates a language-specific repair prompt.
func buildAutoFixPrompt(lang, file, code string, errList string, linkErrors []string, projectContext, compileOutput string) string {
	var sb strings.Builder

	// System context
	switch lang {
	case "go":
		sb.WriteString("You are an expert Go developer. Fix ALL compilation errors in this Go file.\n")
		sb.WriteString("Return the COMPLETE fixed file. Do NOT add comments about fixes.\n")
		sb.WriteString("Critical Go rules:\n")
		sb.WriteString("- Every imported package MUST be used\n")
		sb.WriteString("- Every variable MUST be used\n")
		sb.WriteString("- Use only: int, int64, string, bool, float64, []byte, error, map, slice\n")
		sb.WriteString("- Initialize variables: var x int = 0 or x := 0\n")
		sb.WriteString("- Fix: 'declared but not used' by removing unused vars or using _\n")
		sb.WriteString("- Fix: 'cannot use _ as value' by replacing with nil/default\n")
		sb.WriteString("- Close all open braces and strings properly\n")
		sb.WriteString("- DO NOT use cgo or unsafe unless explicitly needed\n")
		sb.WriteString("- For empty structs/arrays: use var x = make([]Type, 0) or var x []Type\n")
	case "c", "cpp":
		sb.WriteString("You are an expert C/C++ developer. Fix ALL compilation errors.\n")
		sb.WriteString("Return the COMPLETE fixed file. Do NOT add comments about fixes.\n")
		sb.WriteString("Critical C rules:\n")
		sb.WriteString("- Declare ALL variables before use (C89 style)\n")
		sb.WriteString("- Initialize ALL variables: int x = 0; not int x;\n")
		sb.WriteString("- For arrays: use fixed sizes or malloc, NOT VLA\n")
		sb.WriteString("- Include required headers: <stdio.h>, <stdlib.h>, <string.h>, <unistd.h>\n")
		sb.WriteString("- Use POSIX API only (no Android-specific headers)\n")
		sb.WriteString("- Properly close braces, parentheses, and strings\n")
	case "sh", "bash":
		sb.WriteString("You are an expert Shell script developer for Android/Magisk.\n")
		sb.WriteString("Return the COMPLETE fixed script. Do NOT add comments about fixes.\n")
		sb.WriteString("Critical Shell rules:\n")
		sb.WriteString("- Use ${VAR} syntax, NEVER $VAR alone in strings\n")
		sb.WriteString("- All variables in double quotes: \"${VAR}\"\n")
		sb.WriteString("- sleep takes integer seconds only: sleep 5 (not sleep 1.5)\n")
		sb.WriteString("- Test syntax: [ condition ] with spaces inside brackets\n")
		sb.WriteString("- Use #!/system/bin/sh not #!/bin/sh\n")
	default:
		sb.WriteString("Fix ALL compilation errors in this file. Return COMPLETE fixed code.\n")
	}

	sb.WriteString("\n")

	// Project context
	if projectContext != "" {
		sb.WriteString("## Other files in project:\n")
		sb.WriteString(projectContext)
		sb.WriteString("\n")
	}

	// Errors
	sb.WriteString("## Compilation errors:\n")
	sb.WriteString(errList)
	sb.WriteString("\n")

	// Link errors (special treatment)
	if len(linkErrors) > 0 {
		sb.WriteString("## Link errors (missing symbols — add declarations or implementations):\n")
		for _, le := range linkErrors {
			sb.WriteString(fmt.Sprintf("  %s\n", le))
		}
		sb.WriteString("\n")
	}

	// Raw compile output for additional context
	if compileOutput != "" {
		// Trim to last 2000 chars to avoid prompt overflow
		if len(compileOutput) > 2000 {
			compileOutput = compileOutput[len(compileOutput)-2000:]
		}
		sb.WriteString("## Full compile output (last 2000 chars):\n")
		sb.WriteString(compileOutput)
		sb.WriteString("\n")
	}

	// File content
	sb.WriteString(fmt.Sprintf("## File to fix (%s):\n", file))
	sb.WriteString("```\n")
	sb.WriteString(code)
	sb.WriteString("\n```\n")

	// Output format instruction
	sb.WriteString("\nReturn ONLY the complete fixed file code, nothing else. No explanations.\n")

	return sb.String()
}

// detectLanguage detects programming language from file extension and content.
func detectLanguage(filename, content string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go":
		return "go"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "c"
	case ".sh":
		return "sh"
	case ".rs":
		return "rust"
	case ".py":
		return "python"
	default:
		// Heuristic
		if strings.Contains(content, "package main") || strings.Contains(content, "package ") {
			return "go"
		}
		if strings.Contains(content, "#include") || strings.Contains(content, "printf(") {
			return "c"
		}
		if strings.Contains(content, "#!/system/bin/sh") || strings.Contains(content, "#!/bin/sh") {
			return "sh"
		}
		return "go"
	}
}

// gatherProjectContext gathers info about other files in the project.
func gatherProjectContext(projectDir, currentFile string) string {
	var sb strings.Builder
	_ = sb

	// Read module.prop if exists
	propPath := filepath.Join(projectDir, "module.prop")
	if data, err := os.ReadFile(propPath); err == nil {
		sb.WriteString("module.prop:\n")
		sb.WriteString(string(data))
		sb.WriteString("\n")
	}

	// List all source files
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return sb.String()
	}

	for _, e := range entries {
		if e.IsDir() || e.Name() == currentFile {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".go" || ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".sh" || ext == ".rs" {
			fullPath := filepath.Join(projectDir, e.Name())
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			// Only include first 50 lines of other files for context
			content := string(data)
			lines := strings.SplitN(content, "\n", 50)
			sb.WriteString(fmt.Sprintf("%s (first 50 lines):\n", e.Name()))
			sb.WriteString(strings.Join(lines, "\n"))
			if len(strings.Split(content, "\n")) > 50 {
				sb.WriteString("\n... (truncated)\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// extractCodeFromResponseV2 extracts code with better language-aware parsing.
func extractCodeFromResponseV2(response string, lang string) string {
	// Try language-specific fence first
	langFences := []string{
		"```" + lang + "\n",
		"```" + lang + "\r\n",
		"```\n",
		"```",
	}

	for _, fence := range langFences {
		start := strings.Index(response, fence)
		if start < 0 {
			continue
		}
		content := response[start+len(fence):]
		endFence := "```"
		end := strings.Index(content, endFence)
		if end > 0 {
			code := strings.TrimSpace(content[:end])
			if len(code) > 50 { // Sanity check
				return code
			}
		}
	}

	// Fallback: find the largest code-like block
	lines := strings.Split(response, "\n")
	var codeLines []string
	inCode := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			codeLines = append(codeLines, line)
		}
	}

	if len(codeLines) > 3 {
		return strings.Join(codeLines, "\n")
	}

	// Last resort: return everything after first code fence or blank line
	lines2 := strings.Split(response, "\n")
	for i, line := range lines2 {
		if strings.TrimSpace(line) == "" && i > 2 {
			return strings.Join(lines2[i+1:], "\n")
		}
	}

	return response
}
