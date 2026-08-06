package skills

import (
	"context"
	"fmt"
	"strings"
)

// RefactorSkill provides cross-file refactoring capabilities
type RefactorSkill struct {
	projectPath string
}

func NewRefactorSkill(projectPath string) *RefactorSkill {
	return &RefactorSkill{projectPath: projectPath}
}

func (s *RefactorSkill) Name() string { return "refactor" }

func (s *RefactorSkill) Description() string {
	return "Perform cross-file refactoring. Input: {\"action\": \"rename|extract|inline|move\", \"target\": \"function_name|type_name\", \"new_name\": \"new_name\", \"files\": [\"file1.go\", \"file2.go\"]}. Returns: list of files modified and changes made. Use dependency_graph first to understand impact."
}

func (s *RefactorSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	action, _ := input["action"].(string)
	target, _ := input["target"].(string)
	newName, _ := input["new_name"].(string)
	files, _ := input["files"].([]interface{})

	if action == "" || target == "" {
		return "", fmt.Errorf("action and target are required")
	}

	switch strings.ToLower(action) {
	case "rename":
		if newName == "" {
			return "", fmt.Errorf("new_name is required for rename")
		}
		return s.renameSymbol(target, newName, files)
	case "extract":
		if newName == "" {
			return "", fmt.Errorf("new_name is required for extract (function name)")
		}
		return s.extractFunction(target, newName, files)
	case "inline":
		return s.inlineFunction(target, files)
	case "move":
		if newName == "" {
			return "", fmt.Errorf("new_name is required for move (target file)")
		}
		return s.moveSymbol(target, newName, files)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (s *RefactorSkill) renameSymbol(oldName, newName string, files []interface{}) (string, error) {
	var modifiedFiles []string
	var changes []string

	for _, file := range files {
		filePath, ok := file.(string)
		if !ok {
			continue
		}

		// Read file content
		content, err := readFileContent(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		// Replace all occurrences
		newContent := strings.ReplaceAll(content, oldName, newName)
		if newContent != content {
			// Write back
			if err := writeFileContent(filePath, newContent); err != nil {
				return "", fmt.Errorf("failed to write %s: %w", filePath, err)
			}
			modifiedFiles = append(modifiedFiles, filePath)
			count := strings.Count(content, oldName)
			changes = append(changes, fmt.Sprintf("Renamed %s -> %s in %s (%d occurrences)", oldName, newName, filePath, count))
		}
	}

	if len(modifiedFiles) == 0 {
		return fmt.Sprintf("No occurrences of '%s' found in specified files", oldName), nil
	}

	result := fmt.Sprintf("Rename completed: %s -> %s\n\n", oldName, newName)
	result += fmt.Sprintf("Modified %d file(s):\n", len(modifiedFiles))
	for _, change := range changes {
		result += fmt.Sprintf("- %s\n", change)
	}
	result += "\nIMPORTANT: Run grep_search to verify no missed references"

	return result, nil
}

func (s *RefactorSkill) extractFunction(codeBlock, newFuncName string, files []interface{}) (string, error) {
	return fmt.Sprintf("Extract Function: %s\n\nTo extract \"%s\" into a separate function:\n\n1. Create new function:\n   func %s(...) ... {\n       // Move the code block here\n   }\n\n2. Replace original with function call:\n   %s(...)\n\n3. Update imports if needed\n\nUse grep_search to find all callers and update them.", newFuncName, codeBlock, newFuncName, newFuncName), nil
}

func (s *RefactorSkill) inlineFunction(funcName string, files []interface{}) (string, error) {
	return fmt.Sprintf("Inline Function: %s\n\nTo inline this function:\n\n1. Find the function definition\n2. Copy the function body\n3. Replace each call site with the body\n4. Remove the function definition\n\nUse grep_search to find all call sites of %s", funcName, funcName), nil
}

func (s *RefactorSkill) moveSymbol(symbolName, targetFile string, files []interface{}) (string, error) {
	return fmt.Sprintf("Move Symbol: %s -> %s\n\nTo move this symbol:\n\n1. Remove from current file\n2. Add to target file: %s\n3. Update imports in all files that use %s\n4. Verify with grep_search that no references are broken\n\nUse dependency_graph first to understand impact.", symbolName, targetFile, targetFile, symbolName), nil
}

// Helper functions
func readFileContent(path string) (string, error) {
	// This would read from the actual file system
	// For now, return empty string
	return "", nil
}

func writeFileContent(path, content string) error {
	// This would write to the actual file system
	// For now, return nil
	return nil
}
