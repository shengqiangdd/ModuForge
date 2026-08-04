package agent

import (
	"fmt"
	"strings"
	"sync"
)

// DependencyGraph tracks tool call dependencies and determines execution order.
// It identifies which tool calls can run in parallel vs sequentially.
type DependencyGraph struct {
	mu     sync.RWMutex
	nodes  map[string]*DependencyNode // toolCallID -> node
	edges  map[string][]string       // toolCallID -> dependent toolCallIDs
}

type DependencyNode struct {
	ToolName  string
	FilePath  string // for read/write tools
	IsWrite   bool
	ToolCallID string
}

// NewDependencyGraph creates a new dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*DependencyNode),
		edges: make(map[string][]string),
	}
}

// AddToolCall adds a tool call to the graph.
func (dg *DependencyGraph) AddToolCall(tcID, toolName, filePath string, isWrite bool) {
	dg.mu.Lock()
	defer dg.mu.Unlock()
	dg.nodes[tcID] = &DependencyNode{
		ToolName:  toolName,
		FilePath:  filePath,
		IsWrite:   isWrite,
		ToolCallID: tcID,
	}
}

// AddDependency records that dependentID depends on sourceID.
func (dg *DependencyGraph) AddDependency(sourceID, dependentID string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()
	dg.edges[sourceID] = append(dg.edges[sourceID], dependentID)
}

// AnalyzeAndLink examines all tool calls and links dependencies.
// Rules:
// 1. Read of file X after write of file X → depends on that write
// 2. Write of file X after write of file X → depends on that write (sequential)
// 3. Read/Write of file X after read of file X → can be parallel (reads)
// 4. Tools with no file dependency → can be parallel
func (dg *DependencyGraph) AnalyzeAndLink() {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	// Track file access by type
	type fileAccess struct {
		id     string
		isWrite bool
	}
	// Map: filePath -> list of accesses (in order)
	fileAccessMap := make(map[string][]fileAccess)
	// Map: toolCallID -> filePath
	tcToFile := make(map[string]string)

	for id, node := range dg.nodes {
		if node.FilePath != "" {
			fileAccessMap[node.FilePath] = append(fileAccessMap[node.FilePath], fileAccess{id: id, isWrite: node.IsWrite})
			tcToFile[id] = node.FilePath
		}
	}

	// For each file, link dependencies
	for _, accesses := range fileAccessMap {
		for i := 1; i < len(accesses); i++ {
			prev := accesses[i-1]
			curr := accesses[i]

			// If previous was a write, current depends on it
			if prev.isWrite {
				dg.edges[prev.id] = append(dg.edges[prev.id], curr.id)
			}
			// If current is a write, it depends on all previous writes
			if curr.isWrite {
				for j := 0; j < i; j++ {
					if accesses[j].isWrite {
						dg.edges[accesses[j].id] = append(dg.edges[accesses[j].id], curr.id)
					}
				}
			}
		}
	}
}

// GetExecutionLayers returns tool calls grouped into layers that can be executed in parallel.
// Layer 0 has no dependencies, layer 1 depends only on layer 0, etc.
func (dg *DependencyGraph) GetExecutionLayers() [][]string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for id := range dg.nodes {
		inDegree[id] = 0
	}
	for _, targets := range dg.edges {
		for _, target := range targets {
			inDegree[target]++
		}
	}

	var layers [][]string
	processed := make(map[string]bool)

	for {
		// Find all nodes with in-degree 0 that haven't been processed
		var currentLayer []string
		for id, degree := range inDegree {
			if degree == 0 && !processed[id] {
				currentLayer = append(currentLayer, id)
			}
		}

		if len(currentLayer) == 0 {
			break
		}

		layers = append(layers, currentLayer)

		// Mark current layer as processed and reduce in-degree of dependents
		for _, id := range currentLayer {
			processed[id] = true
			for _, target := range dg.edges[id] {
				inDegree[target]--
			}
		}
	}

	return layers
}

// GetParallelGroup returns a human-readable description of parallel/sequential grouping.
func (dg *DependencyGraph) GetParallelGroup() string {
	layers := dg.GetExecutionLayers()
	if len(layers) == 0 {
		return "no tool calls"
	}

	var parts []string
	for i, layer := range layers {
		if len(layer) == 1 {
			tcID := layer[0]
			node := dg.nodes[tcID]
			parts = append(parts, fmt.Sprintf("Layer %d: %s (%s)", i, node.ToolName, tcID))
		} else {
			names := make([]string, len(layer))
			for j, tcID := range layer {
				names[j] = dg.nodes[tcID].ToolName
			}
			parts = append(parts, fmt.Sprintf("Layer %d: [%s] (parallel)", i, strings.Join(names, ", ")))
		}
	}
	return strings.Join(parts, " → ")
}

// Reset clears the graph.
func (dg *DependencyGraph) Reset() {
	dg.mu.Lock()
	defer dg.mu.Unlock()
	dg.nodes = make(map[string]*DependencyNode)
	dg.edges = make(map[string][]string)
}
