package agent

import (
	"fmt"
	"strings"
	"sync"
)

// PermissionLevel defines the access level for tool operations.
type PermissionLevel int

const (
	PermRead      PermissionLevel = iota // Read-only operations
	PermWrite                            // Write operations (non-destructive)
	PermDelete                           // Delete operations (destructive)
	PermSystem                           // System-level operations (bash, docker)
)

// ToolPermission defines what a tool can do.
type ToolPermission struct {
	ToolName       string
	Level          PermissionLevel
	RequiresConfirm bool   // Whether user confirmation is needed
	Description    string // Human-readable description for confirmation
}

// PermissionChecker manages tool call permissions.
type PermissionChecker struct {
	mu          sync.RWMutex
	permissions map[string]ToolPermission
	// Per-project overrides
	projectPerms map[string]map[string]bool // projectID -> toolName -> allowed
	// Audit trail
	deniedCalls  []DenialRecord
	maxDenials   int
}

type DenialRecord struct {
	SessionID string
	ToolName  string
	Reason    string
	Timestamp int64
}

// NewPermissionChecker creates a permission checker with defaults.
func NewPermissionChecker() *PermissionChecker {
	pc := &PermissionChecker{
		permissions: make(map[string]ToolPermission),
		projectPerms: make(map[string]map[string]bool),
		deniedCalls:  make([]DenialRecord, 0),
		maxDenials:   1000,
	}

	// Register default permissions
	pc.Register(ToolPermission{"read_file", PermRead, false, "读取文件"})
	pc.Register(ToolPermission{"grep_search", PermRead, false, "搜索文件内容"})
	pc.Register(ToolPermission{"glob_search", PermRead, false, "搜索文件名"})
	pc.Register(ToolPermission{"write_file", PermWrite, false, "写入文件"})
	pc.Register(ToolPermission{"edit_file", PermWrite, false, "编辑文件"})
	pc.Register(ToolPermission{"write_file_batch", PermWrite, false, "批量写入文件"})
	pc.Register(ToolPermission{"bash", PermSystem, true, "执行Shell命令"})
	pc.Register(ToolPermission{"build_module", PermSystem, true, "构建模块"})
	pc.Register(ToolPermission{"create_module", PermWrite, true, "创建新模块"})

	return pc
}

// Register adds or updates a tool permission.
func (pc *PermissionChecker) Register(perm ToolPermission) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.permissions[perm.ToolName] = perm
}

// CheckPermission checks if a tool call is allowed.
func (pc *PermissionChecker) CheckPermission(toolName, sessionID string) (allowed bool, needsConfirm bool, reason string) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	perm, exists := pc.permissions[toolName]
	if !exists {
		// Default: allow read, block unknown writes
		if strings.HasPrefix(toolName, "read") || strings.HasPrefix(toolName, "search") || strings.HasPrefix(toolName, "grep") || strings.HasPrefix(toolName, "glob") {
			return true, false, ""
		}
		return false, false, fmt.Sprintf("未知工具: %s", toolName)
	}

	return true, perm.RequiresConfirm, perm.Description
}

// GetPermissionLevel returns the permission level for a tool.
func (pc *PermissionChecker) GetPermissionLevel(toolName string) PermissionLevel {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	perm, exists := pc.permissions[toolName]
	if !exists {
		return PermRead
	}
	return perm.Level
}

// NeedsConfirmation returns whether a tool needs user confirmation.
func (pc *PermissionChecker) NeedsConfirmation(toolName string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	perm, exists := pc.permissions[toolName]
	if !exists {
		return false
	}
	return perm.RequiresConfirm
}

// GetConfirmationMessage returns a human-readable confirmation message.
func (pc *PermissionChecker) GetConfirmationMessage(toolName string, params map[string]interface{}) string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	perm, exists := pc.permissions[toolName]
	if !exists {
		return fmt.Sprintf("确认执行 %s?", toolName)
	}

	msg := fmt.Sprintf("⚠️ Agent 请求执行: %s", perm.Description)
	if path, ok := params["path"].(string); ok {
		msg += fmt.Sprintf("\n目标: %s", path)
	}
	if command, ok := params["command"].(string); ok {
		msg += fmt.Sprintf("\n命令: %s", command)
	}
	return msg
}

// LogDenial records a denied tool call.
func (pc *PermissionChecker) LogDenial(sessionID, toolName, reason string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if len(pc.deniedCalls) >= pc.maxDenials {
		pc.deniedCalls = pc.deniedCalls[1:]
	}
	pc.deniedCalls = append(pc.deniedCalls, DenialRecord{
		SessionID: sessionID,
		ToolName:  toolName,
		Reason:    reason,
	})
}

// GetDenials returns recent denials.
func (pc *PermissionChecker) GetDenials(limit int) []DenialRecord {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	start := len(pc.deniedCalls) - limit
	if start < 0 {
		start = 0
	}
	return append([]DenialRecord{}, pc.deniedCalls[start:]...)
}

// IsDestructiveTool returns whether a tool performs destructive operations.
func IsDestructiveTool(toolName string) bool {
	switch toolName {
	case "bash", "delete_file", "rm", "rmdir":
		return true
	}
	return false
}

// GetSensitivePatterns returns patterns that indicate sensitive operations.
func GetSensitivePatterns(command string) []string {
	var patterns []string
	sensitive := []struct {
		pattern string
		desc    string
	}{
		{"rm -rf", "递归删除"},
		{"rm -r", "递归删除"},
		{"sudo", "需要sudo权限"},
		{"chmod 777", "设置不安全权限"},
		{"> /dev/", "写入设备文件"},
		{"dd if=", "磁盘操作"},
		{"mkfs", "格式化磁盘"},
		{"fdisk", "分区操作"},
		{"curl | sh", "管道执行脚本"},
		{"wget | sh", "管道执行脚本"},
		{"eval", "动态执行"},
		{"exec", "执行命令"},
	}

	for _, s := range sensitive {
		if strings.Contains(command, s.pattern) {
			patterns = append(patterns, s.desc)
		}
	}
	return patterns
}
