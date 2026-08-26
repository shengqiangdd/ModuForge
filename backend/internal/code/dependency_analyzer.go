package code

import (
	"strings"
)

// DependencyAnalyzer 依赖分析器
type DependencyAnalyzer struct{}

// NewDependencyAnalyzer 创建依赖分析器
func NewDependencyAnalyzer() *DependencyAnalyzer {
	return &DependencyAnalyzer{}
}

// DependencyGraph 依赖图
type DependencyGraph struct {
	Nodes  []DependencyNode `json:"nodes"`
	Edges  []DependencyEdge `json:"edges"`
	Cycles []Cycle          `json:"cycles"`
}

// DependencyNode 依赖节点
type DependencyNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	ImportCount int    `json:"import_count"`
}

// DependencyEdge 依赖边
type DependencyEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight"`
}

// Cycle 循环依赖
type Cycle struct {
	Path []string `json:"path"`
}

// AnalyzeDependencies 分析依赖关系
func (d *DependencyAnalyzer) AnalyzeDependencies(files map[string]string, language string) *DependencyGraph {
	graph := &DependencyGraph{
		Nodes:  make([]DependencyNode, 0),
		Edges:  make([]DependencyEdge, 0),
		Cycles: make([]Cycle, 0),
	}

	importMap := make(map[string]map[string]int)

	for fileName, code := range files {
		graph.Nodes = append(graph.Nodes, DependencyNode{
			ID:   fileName,
			Name: fileName,
			Type: "file",
		})

		imports := d.extractImports(code, language)
		importMap[fileName] = make(map[string]int)

		for _, imp := range imports {
			importMap[fileName][imp]++

			if !d.nodeExists(graph.Nodes, imp) {
				graph.Nodes = append(graph.Nodes, DependencyNode{
					ID:   imp,
					Name: imp,
					Type: "package",
				})
			}

			graph.Edges = append(graph.Edges, DependencyEdge{
				From:   fileName,
				To:     imp,
				Weight: importMap[fileName][imp],
			})
		}
	}

	graph.Cycles = d.detectCycles(graph)

	return graph
}

func (d *DependencyAnalyzer) extractImports(code string, language string) []string {
	imports := make([]string, 0)

	switch strings.ToLower(language) {
	case "go":
		lines := strings.Split(code, "\n")
		inImport := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "import (" {
				inImport = true
				continue
			}
			if inImport && line == ")" {
				inImport = false
				continue
			}
			if inImport || strings.HasPrefix(line, "import ") {
				imp := strings.Trim(line, "\" \t")
				if imp != "" {
					imports = append(imports, imp)
				}
			}
		}

	case "javascript", "js", "typescript", "ts":
		lines := strings.Split(code, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "from '") || strings.Contains(line, "from \"") {
				imp := d.extractStringAfter(line, "from ")
				if imp != "" {
					imports = append(imports, imp)
				}
			} else if strings.Contains(line, "import '") || strings.Contains(line, "import \"") {
				imp := d.extractStringAfter(line, "import ")
				if imp != "" {
					imports = append(imports, imp)
				}
			} else if strings.Contains(line, "require('") || strings.Contains(line, "require(\"") {
				imp := d.extractStringAfter(line, "require(")
				if imp != "" {
					imports = append(imports, imp)
				}
			}
		}

	case "python", "py":
		lines := strings.Split(code, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					imp := parts[1]
					if imp != "" {
						imports = append(imports, imp)
					}
				}
			}
		}
	}

	return imports
}

func (d *DependencyAnalyzer) extractStringAfter(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return ""
	}
	s = s[idx+len(prefix):]
	s = strings.TrimSpace(s)

	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') {
		s = s[1:]
		if i := strings.IndexAny(s, "'\""); i != -1 {
			s = s[:i]
		}
	}

	return s
}

func (d *DependencyAnalyzer) nodeExists(nodes []DependencyNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func (d *DependencyAnalyzer) detectCycles(graph *DependencyGraph) []Cycle {
	cycles := make([]Cycle, 0)

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(node string, path []string)
	dfs = func(node string, path []string) {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, edge := range graph.Edges {
			if edge.From == node {
				if !visited[edge.To] {
					dfs(edge.To, path)
				} else if recStack[edge.To] {
					cyclePath := make([]string, 0)
					for _, p := range path {
						cyclePath = append(cyclePath, p)
						if p == edge.To {
							break
						}
					}
					cycles = append(cycles, Cycle{Path: cyclePath})
				}
			}
		}

		recStack[node] = false
	}

	for _, node := range graph.Nodes {
		if !visited[node.ID] {
			dfs(node.ID, nil)
		}
	}

	return cycles
}
