package skills

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type ListDirSkill struct {
	db *sql.DB
}

func NewListDirSkill(db *sql.DB) *ListDirSkill {
	return &ListDirSkill{db: db}
}

func (s *ListDirSkill) Name() string {
	return "list_dir"
}

func (s *ListDirSkill) Description() string {
	return "List files in a directory. Input: {\"path\": \"...\" (optional, default=root), \"project_id\": \"...\", \"recursive\" (optional, default=false)}. Shows file tree."
}

func (s *ListDirSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, _ := input["path"].(string)
	projectID, _ := input["project_id"].(string)
	recursive, _ := input["recursive"].(bool)

	if path == "" {
		path = "."
	}
	if projectID == "" {
		return "", fmt.Errorf("project_id is required")
	}
	if s.db == nil {
		return "", fmt.Errorf("database not available")
	}

	// Build the LIKE pattern
	likePattern := path + "/%"
	if path == "." {
		likePattern = "%"
	}

	// Query files
	rows, err := s.db.Query(
		`SELECT path FROM project_files WHERE project_id=? AND path LIKE ? ORDER BY path`,
		projectID, likePattern,
	)
	if err != nil {
		return "", fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	var allPaths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			allPaths = append(allPaths, p)
		}
	}

	if len(allPaths) == 0 {
		return fmt.Sprintf("📁 Directory '%s' is empty", path), nil
	}

	if recursive {
		return s.formatTree(path, allPaths), nil
	}

	// Non-recursive: show only immediate children
	children := make(map[string]bool) // true = directory, false = file
	for _, p := range allPaths {
		rel := strings.TrimPrefix(p, path)
		rel = strings.TrimPrefix(rel, "/")
		if idx := strings.Index(rel, "/"); idx >= 0 {
			children[rel[:idx]+"/"] = true
		} else {
			children[rel] = false
		}
	}

	// Sort entries: directories first, then files
	type entry struct {
		name  string
		isDir bool
	}
	var entries []entry
	for name, isDir := range children {
		entries = append(entries, entry{name: name, isDir: isDir})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return entries[i].name < entries[j].name
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📁 %s/\n", path))
	dirCount := 0
	fileCount := 0
	for _, e := range entries {
		if e.isDir {
			sb.WriteString(fmt.Sprintf("  📁 %s\n", e.name))
			dirCount++
		} else {
			sb.WriteString(fmt.Sprintf("  📄 %s\n", e.name))
			fileCount++
		}
	}
	sb.WriteString(fmt.Sprintf("\n(%d directories, %d files)", dirCount, fileCount))

	return sb.String(), nil
}

func (s *ListDirSkill) formatTree(rootPath string, paths []string) string {
	// Build a tree structure
	type node struct {
		name     string
		isDir    bool
		children []*node
	}

	root := &node{name: rootPath, isDir: true}
	nodeMap := map[string]*node{"": root}

	for _, p := range paths {
		parts := strings.Split(strings.TrimPrefix(p, rootPath), "/")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}

		current := root
		currentPath := ""
		for i, part := range parts {
			currentPath += part
			if i < len(parts)-1 {
				currentPath += "/"
			}

			child, exists := nodeMap[currentPath]
			if !exists {
				child = &node{name: part, isDir: i < len(parts)-1}
				if i == len(parts)-1 {
					child.isDir = false // last component is a file
				}
				current.children = append(current.children, child)
				nodeMap[currentPath] = child
			}
			current = child
		}
	}

	// Sort children at each level
	var sortNodes func(n *node)
	sortNodes = func(n *node) {
		sort.Slice(n.children, func(i, j int) bool {
			if n.children[i].isDir != n.children[j].isDir {
				return n.children[i].isDir
			}
			return n.children[i].name < n.children[j].name
		})
		for _, child := range n.children {
			sortNodes(child)
		}
	}
	sortNodes(root)

	// Render tree
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📁 %s/\n", rootPath))

	var render func(n *node, prefix string)
	render = func(n *node, prefix string) {
		for i, child := range n.children {
			isLast := i == len(n.children)-1
			connector := "├── "
			if isLast {
				connector = "└── "
			}
			if child.isDir {
				sb.WriteString(fmt.Sprintf("%s%s📁 %s/\n", prefix, connector, child.name))
				newPrefix := prefix + "│   "
				if isLast {
					newPrefix = prefix + "    "
				}
				render(child, newPrefix)
			} else {
				sb.WriteString(fmt.Sprintf("%s%s📄 %s\n", prefix, connector, child.name))
			}
		}
	}

	render(root, "")
	sb.WriteString(fmt.Sprintf("\n(%d total files)", len(paths)))
	return sb.String()
}

func (s *ListDirSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: true,
		NeedsDB:   true,
		NeedsLLM:  false,
	}
}
