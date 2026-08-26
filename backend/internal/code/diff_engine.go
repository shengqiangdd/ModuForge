package code

import (
	"fmt"
	"strings"
)

// DiffEngine 代码版本对比引擎
type DiffEngine struct{}

// NewDiffEngine 创建对比引擎
func NewDiffEngine() *DiffEngine {
	return &DiffEngine{}
}

// DiffRequest 对比请求
type DiffRequest struct {
	OldCode  string `json:"old_code"`
	NewCode  string `json:"new_code"`
	Language string `json:"language"`
}

// DiffResult 对比结果
type DiffResult struct {
	LinesAdded   int          `json:"lines_added"`
	LinesRemoved int          `json:"lines_removed"`
	Changes      []DiffChange `json:"changes"`
	Summary      string       `json:"summary"`
}

// DiffChange 对比变更
type DiffChange struct {
	Type    string `json:"type"` // added, removed, modified
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// Compare 对比两个版本
func (d *DiffEngine) Compare(req DiffRequest) *DiffResult {
	oldLines := strings.Split(req.OldCode, "\n")
	newLines := strings.Split(req.NewCode, "\n")

	result := &DiffResult{
		Changes: make([]DiffChange, 0),
	}

	// 简单的逐行对比
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		oldLine := ""
		newLine := ""

		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine == newLine {
			continue
		}

		if oldLine == "" && newLine != "" {
			result.LinesAdded++
			result.Changes = append(result.Changes, DiffChange{
				Type:    "added",
				Line:    i + 1,
				Content: newLine,
			})
		} else if oldLine != "" && newLine == "" {
			result.LinesRemoved++
			result.Changes = append(result.Changes, DiffChange{
				Type:    "removed",
				Line:    i + 1,
				Content: oldLine,
			})
		} else {
			result.LinesRemoved++
			result.LinesAdded++
			result.Changes = append(result.Changes, DiffChange{
				Type:    "modified",
				Line:    i + 1,
				Content: fmt.Sprintf("- %s\n+ %s", oldLine, newLine),
			})
		}
	}

	result.Summary = fmt.Sprintf("新增 %d 行，删除 %d 行，共 %d 处变更",
		result.LinesAdded, result.LinesRemoved, len(result.Changes))

	return result
}
