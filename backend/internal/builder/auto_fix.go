package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AutoFixCompileErrors attempts to fix compilation errors by sending them
// back to the LLM for correction. This is the "AI self-repair" feature.
func (b *Builder) AutoFixCompileErrors(
	ctx context.Context,
	projectDir string,
	compileErr error,
	logFn func(string),
) bool {
	if compileErr == nil {
		return true
	}

	errMsg := compileErr.Error()
	logFn(fmt.Sprintf("\n🔧 Auto-fix: Attempting to fix compilation errors...\n"))

	// Parse errors from compilation output
	errors := parseCompileErrors(errMsg)
	if len(errors) == 0 {
		logFn("  ⚠️  Could not parse compilation errors\n")
		return false
	}

	logFn(fmt.Sprintf("  Found %d error(s) to fix\n", len(errors)))

	// Group errors by file
	fileErrors := groupErrorsByFile(errors)

	// Try to fix each file
	fixedAny := false
	for file, errs := range fileErrors {
		fullPath := filepath.Join(projectDir, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		fixedCode, fixErr := b.fixCodeWithLLM(ctx, string(content), errs, logFn)
		if fixErr != nil {
			logFn(fmt.Sprintf("  ⚠️  LLM fix failed for %s: %v\n", file, fixErr))
			continue
		}

		if fixedCode != string(content) {
			if err := os.WriteFile(fullPath, []byte(fixedCode), 0644); err == nil {
				logFn(fmt.Sprintf("  ✅ Fixed %s (%d errors)\n", file, len(errs)))
				fixedAny = true
			}
		}
	}

	return fixedAny
}

// CompileError represents a parsed compilation error.
type CompileError struct {
	File    string
	Line    int
	Column  int
	Message string
}

// parseCompileErrors parses Go/C compilation error output.
func parseCompileErrors(errMsg string) []CompileError {
	var errors []CompileError

	// Go error pattern: ./file.go:line:col: message
	goPattern := regexp.MustCompile(`\./([^:]+):(\d+):(\d+):\s*(.+)`)
	for _, match := range goPattern.FindAllStringSubmatch(errMsg, -1) {
		line := 0
		fmt.Sscanf(match[2], "%d", &line)
		col := 0
		fmt.Sscanf(match[3], "%d", &col)
		errors = append(errors, CompileError{
			File:    match[1],
			Line:    line,
			Column:  col,
			Message: match[4],
		})
	}

	// C error pattern: file.c:line: message
	cPattern := regexp.MustCompile(`([a-zA-Z_][\w]*\.[ch]):(\d+):\s*(.+)`)
	for _, match := range cPattern.FindAllStringSubmatch(errMsg, -1) {
		line := 0
		fmt.Sscanf(match[2], "%d", &line)
		errors = append(errors, CompileError{
			File:    match[1],
			Line:    line,
			Message: match[3],
		})
	}

	return errors
}

// groupErrorsByFile groups errors by filename.
func groupErrorsByFile(errors []CompileError) map[string][]CompileError {
	result := make(map[string][]CompileError)
	for _, e := range errors {
		result[e.File] = append(result[e.File], e)
	}
	return result
}

// fixCodeWithLLM sends code and errors to LLM for fixing.
func (b *Builder) fixCodeWithLLM(
	ctx context.Context,
	code string,
	errors []CompileError,
	logFn func(string),
) (string, error) {
	// Build error description
	var errDesc strings.Builder
	for _, e := range errors {
		errDesc.WriteString(fmt.Sprintf("Line %d: %s\n", e.Line, e.Message))
	}

	// Extract relevant code context (around errors)
	contextLines := extractCodeContext(code, errors, 5)

	// Build prompt
	prompt := fmt.Sprintf(`Fix the following Go code that has compilation errors.

ERRORS:
%s

CODE WITH ERRORS (lines around errors):
%s

Please return the COMPLETE fixed code. Do not add comments about the fixes.
Return only the fixed code, no explanations.`, errDesc.String(), contextLines)

	// Call LLM
	logFn("  📤 Sending errors to LLM for fix...\n")

	endpoint, apiKey, model := resolveLLMForFix()
	if endpoint == "" || apiKey == "" {
		return "", fmt.Errorf("no LLM configured for auto-fix")
	}

	fixedCode, err := callLLMForFix(ctx, endpoint, apiKey, model, prompt)
	if err != nil {
		return "", err
	}

	// Extract code from response
	fixedCode = extractCodeFromResponse(fixedCode)

	return fixedCode, nil
}

// extractCodeContext extracts code around error lines.
func extractCodeContext(code string, errors []CompileError, contextRange int) string {
	lines := strings.Split(code, "\n")
	var result strings.Builder

	errorLines := make(map[int]bool)
	for _, e := range errors {
		errorLines[e.Line] = true
	}

	start := 1
	end := len(lines)
	for i := range lines {
		lineNum := i + 1
		if errorLines[lineNum] {
			if start < lineNum-contextRange {
				start = lineNum - contextRange
			}
			if end > lineNum+contextRange {
				end = lineNum + contextRange
			}
		}
	}

	for i := start - 1; i < end && i < len(lines); i++ {
		lineNum := i + 1
		prefix := "  "
		if errorLines[lineNum] {
			prefix = ">>"
		}
		result.WriteString(fmt.Sprintf("%s %d: %s\n", prefix, lineNum, lines[i]))
	}

	return result.String()
}

// callLLMForFix calls LLM to fix code.
func callLLMForFix(ctx context.Context, endpoint, apiKey, model, prompt string) (string, error) {
	url := endpoint + "/chat/completions"

	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a Go expert. Fix compilation errors and return complete fixed code. Return only code, no explanations."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
	}

	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call LLM: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return result.Choices[0].Message.Content, nil
}

// extractCodeFromResponse extracts code from LLM response.
func extractCodeFromResponse(response string) string {
	patterns := []string{
		"```go\n(.*?)```",
		"```\n(.*?)```",
		"```(.*?)```",
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?s)` + pattern)
		matches := re.FindStringSubmatch(response)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return response
}

// resolveLLMForFix resolves LLM configuration for auto-fix.
func resolveLLMForFix() (endpoint, apiKey, model string) {
	endpoint = os.Getenv("LLM_ENDPOINT")
	apiKey = os.Getenv("LLM_API_KEY")
	model = os.Getenv("LLM_MODEL")

	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	return
}
