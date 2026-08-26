package code

import (
	"fmt"
	"strings"
)

// DuplicationDetector 代码重复检测器
type DuplicationDetector struct{}

// NewDuplicationDetector 创建重复检测器
func NewDuplicationDetector() *DuplicationDetector {
	return &DuplicationDetector{}
}

// DuplicationRequest 重复检测请求
type DuplicationRequest struct {
	Files map[string]string `json:"files"` // filename -> code
}

// DuplicationResult 重复检测结果
type DuplicationResult struct {
	Duplicates    []DuplicationGroup `json:"duplicates"`
	TotalLines    int                `json:"total_lines"`
	DuplicateLines int               `json:"duplicate_lines"`
	Score         float64            `json:"score"` // 0-100
	Summary       string             `json:"summary"`
}

// DuplicationGroup 重复组
type DuplicationGroup struct {
	ID          int                  `json:"id"`
	Lines       int                  `json:"lines"`
	Locations   []DuplicationLocation `json:"locations"`
	Similarity  float64              `json:"similarity"`
}

// DuplicationLocation 重复位置
type DuplicationLocation struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Code      string `json:"code"`
}

// Detect 检测代码重复
func (d *DuplicationDetector) Detect(req DuplicationRequest) *DuplicationResult {
	result := &DuplicationResult{
		Duplicates: make([]DuplicationGroup, 0),
	}

	// 将所有文件的代码分割成行
	fileLines := make(map[string][]string)
	totalLines := 0
	for filename, code := range req.Files {
		lines := strings.Split(code, "\n")
		fileLines[filename] = lines
		totalLines += len(lines)
	}

	result.TotalLines = totalLines

	// 使用滑动窗口检测重复
	groupID := 0
	minLines := 3 // 最小重复行数

	for file1, lines1 := range fileLines {
		for file2, lines2 := range fileLines {
			if file1 >= file2 {
				continue
			}

			// 检测两个文件之间的重复
			duplicates := d.findDuplicatesBetweenFiles(file1, lines1, file2, lines2, minLines)
			for _, dup := range duplicates {
				dup.ID = groupID
				groupID++
				result.Duplicates = append(result.Duplicates, dup)
				result.DuplicateLines += dup.Lines
			}
		}
	}

	// 计算重复率
	if totalLines > 0 {
		result.Score = float64(result.DuplicateLines) / float64(totalLines) * 100
	}

	// 生成摘要
	if len(result.Duplicates) == 0 {
		result.Summary = "未发现代码重复"
	} else {
		result.Summary = fmt.Sprintf("发现 %d 处重复，共 %d 行（%.1f%%）",
			len(result.Duplicates), result.DuplicateLines, result.Score)
	}

	return result
}

// findDuplicatesBetweenFiles 在两个文件之间查找重复
func (d *DuplicationDetector) findDuplicatesBetweenFiles(file1 string, lines1 []string, file2 string, lines2 []string, minLines int) []DuplicationGroup {
	groups := make([]DuplicationGroup, 0)

	// 使用简单的滑动窗口算法
	for i := 0; i < len(lines1)-minLines+1; i++ {
		for j := 0; j < len(lines2)-minLines+1; j++ {
			matchLen := d.countMatchingLines(lines1, i, lines2, j)
			if matchLen >= minLines {
				// 检查是否与现有组重叠
				if !d.overlapsWithExisting(groups, file1, i, i+matchLen-1) {
					group := DuplicationGroup{
						Lines: matchLen,
						Locations: []DuplicationLocation{
							{
								File:      file1,
								StartLine: i + 1,
								EndLine:   i + matchLen,
								Code:      strings.Join(lines1[i:i+matchLen], "\n"),
							},
							{
								File:      file2,
								StartLine: j + 1,
								EndLine:   j + matchLen,
								Code:      strings.Join(lines2[j:j+matchLen], "\n"),
							},
						},
						Similarity: 100.0,
					}
					groups = append(groups, group)
				}
			}
		}
	}

	return groups
}

// countMatchingLines 计算匹配行数
func (d *DuplicationDetector) countMatchingLines(lines1 []string, start1 int, lines2 []string, start2 int) int {
	count := 0
	maxLen := min(len(lines1)-start1, len(lines2)-start2)

	for i := 0; i < maxLen; i++ {
		if strings.TrimSpace(lines1[start1+i]) == strings.TrimSpace(lines2[start2+i]) {
			count++
		} else {
			break
		}
	}

	return count
}

// overlapsWithExisting 检查是否与现有组重叠
func (d *DuplicationDetector) overlapsWithExisting(groups []DuplicationGroup, file string, start, end int) bool {
	for _, group := range groups {
		for _, loc := range group.Locations {
			if loc.File == file {
				if !(end < loc.StartLine || start > loc.EndLine) {
					return true
				}
			}
		}
	}
	return false
}
