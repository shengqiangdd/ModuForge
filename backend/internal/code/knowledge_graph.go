package code

import (
	"strings"
)

// KnowledgeGraph 代码知识图谱
type KnowledgeGraph struct {
	Nodes []KGNode `json:"nodes"`
	Edges []KGEdge `json:"edges"`
}

// KGNode 知识图谱节点
type KGNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Type     string `json:"type"` // package, function, struct, interface, variable
	Language string `json:"language"`
	Parent   string `json:"parent,omitempty"`
}

// KGEdge 知识图谱边
type KGEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // contains, imports, calls, implements, extends
	Weight int    `json:"weight"`
}

// NewKnowledgeGraph 创建知识图谱
func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		Nodes: make([]KGNode, 0),
		Edges: make([]KGEdge, 0),
	}
}

// BuildGraph 构建代码知识图谱
func (g *KnowledgeGraph) BuildGraph(files map[string]string, language string) {
	for fileName, code := range files {
		g.addNode(KGNode{
			ID:       fileName,
			Label:    fileName,
			Type:     "package",
			Language: language,
		})

		lines := strings.Split(code, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)

			switch language {
			case "go":
				g.analyzeGoLine(fileName, line)
			case "javascript", "typescript":
				g.analyzeJSLine(fileName, line)
			case "python":
				g.analyzePythonLine(fileName, line)
			}
		}
	}
}

func (g *KnowledgeGraph) addNode(node KGNode) {
	for _, n := range g.Nodes {
		if n.ID == node.ID {
			return
		}
	}
	g.Nodes = append(g.Nodes, node)
}

func (g *KnowledgeGraph) addEdge(edge KGEdge) {
	for i, e := range g.Edges {
		if e.Source == edge.Source && e.Target == edge.Target && e.Type == edge.Type {
			g.Edges[i].Weight++
			return
		}
	}
	edge.Weight = 1
	g.Edges = append(g.Edges, edge)
}

func (g *KnowledgeGraph) analyzeGoLine(fileName string, line string) {
	if strings.HasPrefix(line, "func ") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := strings.Split(parts[1], "(")[0]
			nodeID := fileName + ":" + name
			g.addNode(KGNode{
				ID:       nodeID,
				Label:    name,
				Type:     "function",
				Language: "go",
				Parent:   fileName,
			})
			g.addEdge(KGEdge{
				Source: fileName,
				Target: nodeID,
				Type:   "contains",
			})
		}
	}

	if strings.HasPrefix(line, "type ") && strings.Contains(line, "struct") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[1]
			nodeID := fileName + ":" + name
			g.addNode(KGNode{
				ID:       nodeID,
				Label:    name,
				Type:     "struct",
				Language: "go",
				Parent:   fileName,
			})
			g.addEdge(KGEdge{
				Source: fileName,
				Target: nodeID,
				Type:   "contains",
			})
		}
	}

	if strings.HasPrefix(line, "type ") && strings.Contains(line, "interface") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[1]
			nodeID := fileName + ":" + name
			g.addNode(KGNode{
				ID:       nodeID,
				Label:    name,
				Type:     "interface",
				Language: "go",
				Parent:   fileName,
			})
			g.addEdge(KGEdge{
				Source: fileName,
				Target: nodeID,
				Type:   "contains",
			})
		}
	}
}

func (g *KnowledgeGraph) analyzeJSLine(fileName string, line string) {
	if strings.Contains(line, "function ") {
		parts := strings.Split(line, "function ")
		if len(parts) == 2 {
			name := strings.Split(parts[1], "(")[0]
			if name != "" && !strings.Contains(name, "=") {
				nodeID := fileName + ":" + name
				g.addNode(KGNode{
					ID:       nodeID,
					Label:    name,
					Type:     "function",
					Language: "javascript",
					Parent:   fileName,
				})
				g.addEdge(KGEdge{
					Source: fileName,
					Target: nodeID,
					Type:   "contains",
				})
			}
		}
	}

	if strings.HasPrefix(line, "class ") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[1]
			nodeID := fileName + ":" + name
			g.addNode(KGNode{
				ID:       nodeID,
				Label:    name,
				Type:     "struct",
				Language: "javascript",
				Parent:   fileName,
			})
			g.addEdge(KGEdge{
				Source: fileName,
				Target: nodeID,
				Type:   "contains",
			})
		}
	}
}

func (g *KnowledgeGraph) analyzePythonLine(fileName string, line string) {
	if strings.HasPrefix(line, "def ") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := strings.Split(parts[1], "(")[0]
			nodeID := fileName + ":" + name
			g.addNode(KGNode{
				ID:       nodeID,
				Label:    name,
				Type:     "function",
				Language: "python",
				Parent:   fileName,
			})
			g.addEdge(KGEdge{
				Source: fileName,
				Target: nodeID,
				Type:   "contains",
			})
		}
	}

	if strings.HasPrefix(line, "class ") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := strings.Split(parts[1], "(")[0]
			nodeID := fileName + ":" + name
			g.addNode(KGNode{
				ID:       nodeID,
				Label:    name,
				Type:     "struct",
				Language: "python",
				Parent:   fileName,
			})
			g.addEdge(KGEdge{
				Source: fileName,
				Target: nodeID,
				Type:   "contains",
			})
		}
	}
}

// GetStats 获取图谱统计
func (g *KnowledgeGraph) GetStats() map[string]int {
	stats := map[string]int{
		"total_nodes": len(g.Nodes),
		"total_edges": len(g.Edges),
		"packages":    0,
		"functions":   0,
		"structs":     0,
		"interfaces":  0,
		"contains":    0,
		"imports":     0,
		"calls":       0,
	}

	for _, node := range g.Nodes {
		switch node.Type {
		case "package":
			stats["packages"]++
		case "function":
			stats["functions"]++
		case "struct":
			stats["structs"]++
		case "interface":
			stats["interfaces"]++
		}
	}

	for _, edge := range g.Edges {
		switch edge.Type {
		case "contains":
			stats["contains"]++
		case "imports":
			stats["imports"]++
		case "calls":
			stats["calls"]++
		}
	}

	return stats
}
