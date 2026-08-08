package agent

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// Security Policy — 基于模式的权限规则引擎
// 参考: OpenHands sandbox, Claude Code permission system
// ═══════════════════════════════════════════════════════════════════

// SecurityLevel defines risk level for operations
type SecurityLevel int

const (
	SecurityAuto     SecurityLevel = iota // 自动批准
	SecurityConfirm                       // 需要用户确认
	SecurityDeny                          // 禁止执行
)

// SecurityRule defines a pattern-based permission rule
type SecurityRule struct {
	Name        string        // 规则名称
	Pattern     *regexp.Regexp // 正则匹配模式
	Level       SecurityLevel // 安全级别
	RiskScore   int           // 风险评分 0-100
	Description string        // 人类可读描述
}

// DangerousOperation represents a detected dangerous operation
type DangerousOperation struct {
	Rule        string
	Command     string
	RiskScore   int
	Level       SecurityLevel
	Description string
	Timestamp   time.Time
}

// SecurityEngine provides pattern-based security checking
type SecurityEngine struct {
	mu           sync.RWMutex
	rules        []SecurityRule
	auditLog     []DangerousOperation
	maxAuditSize int
}

// NewSecurityEngine creates a security engine with default rules
func NewSecurityEngine() *SecurityEngine {
	se := &SecurityEngine{
		rules:        getDefaultSecurityRules(),
		auditLog:     make([]DangerousOperation, 0),
		maxAuditSize: 10000,
	}
	return se
}

// getDefaultSecurityRules returns the default security rules
func getDefaultSecurityRules() []SecurityRule {
	return []SecurityRule{
		// ═══ 绝对禁止 (Risk: 100) ═══
		{
			Name:      "rm_rf_root",
			Pattern:   regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s+(/|/\*|\.\.)`),
			Level:     SecurityDeny,
			RiskScore: 100,
			Description: "禁止删除根目录或上级目录",
		},
		{
			Name:      "rm_rf_system",
			Pattern:   regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s+/((usr|etc|var|sys|proc|dev|boot|sbin|bin)/)`),
			Level:     SecurityDeny,
			RiskScore: 100,
			Description: "禁止删除系统目录",
		},
		{
			Name:      "format_disk",
			Pattern:   regexp.MustCompile(`(mkfs|format)\s+(/dev/|//\\\\.\\\\)`),
			Level:     SecurityDeny,
			RiskScore: 100,
			Description: "禁止格式化磁盘",
		},
		{
			Name:      "dd_if",
			Pattern:   regexp.MustCompile(`dd\s+if=/dev/(zero|random|urandom)\s+of=/dev/`),
			Level:     SecurityDeny,
			RiskScore: 100,
			Description: "禁止覆写磁盘",
		},
		{
			Name:      "chmod_777",
			Pattern:   regexp.MustCompile(`chmod\s+(-R\s+)?777\s+/`),
			Level:     SecurityDeny,
			RiskScore: 95,
			Description: "禁止对系统目录设置777权限",
		},

		// ═══ 需要确认 (Risk: 60-90) ═══
		{
			Name:      "rm_rf_any",
			Pattern:   regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r)\s+`),
			Level:     SecurityConfirm,
			RiskScore: 85,
			Description: "递归强制删除操作",
		},
		{
			Name:      "rm_force",
			Pattern:   regexp.MustCompile(`rm\s+(-[a-zA-Z]*f[a-zA-Z]*)\s+`),
			Level:     SecurityConfirm,
			RiskScore: 70,
			Description: "强制删除操作",
		},
		{
			Name:      "git_push_force",
			Pattern:   regexp.MustCompile(`git\s+push\s+.*--force`),
			Level:     SecurityConfirm,
			RiskScore: 80,
			Description: "Git强制推送（可能覆盖他人代码）",
		},
		{
			Name:      "git_push_main",
			Pattern:   regexp.MustCompile(`git\s+push\s+(origin\s+)?(main|master)`),
			Level:     SecurityConfirm,
			RiskScore: 75,
			Description: "推送到主分支",
		},
		{
			Name:      "docker_rm",
			Pattern:   regexp.MustCompile(`docker\s+(rm|rmi|kill|stop)\s+(-f\s+)?`),
			Level:     SecurityConfirm,
			RiskScore: 70,
			Description: "删除/停止Docker容器或镜像",
		},
		{
			Name:      "docker_compose_down",
			Pattern:   regexp.MustCompile(`docker\s+compose\s+down`),
			Level:     SecurityConfirm,
			RiskScore: 65,
			Description: "停止所有Docker容器",
		},
		{
			Name:      "systemctl_stop",
			Pattern:   regexp.MustCompile(`systemctl\s+(stop|disable|mask)\s+`),
			Level:     SecurityConfirm,
			RiskScore: 75,
			Description: "停止/禁用系统服务",
		},
		{
			Name:      "kill_process",
			Pattern:   regexp.MustCompile(`kill\s+(-9\s+|-SIGKILL\s+)?\d+`),
			Level:     SecurityConfirm,
			RiskScore: 65,
			Description: "终止进程",
		},
		{
			Name:      "sudo_rm",
			Pattern:   regexp.MustCompile(`sudo\s+rm`),
			Level:     SecurityConfirm,
			RiskScore: 90,
			Description: "使用sudo执行删除操作",
		},
		{
			Name:      "sudo_chown",
			Pattern:   regexp.MustCompile(`sudo\s+chown\s+.*(/|/usr|/etc|/var)`),
			Level:     SecurityConfirm,
			RiskScore: 70,
			Description: "使用sudo修改系统目录所有者",
		},

		// ═══ 安全操作 (Risk: 0-30) ═══
		{
			Name:      "git_status",
			Pattern:   regexp.MustCompile(`git\s+(status|log|diff|show|branch)`),
			Level:     SecurityAuto,
			RiskScore: 5,
			Description: "Git只读操作",
		},
		{
			Name:      "ls_dir",
			Pattern:   regexp.MustCompile(`(ls|dir|tree)\s+`),
			Level:     SecurityAuto,
			RiskScore: 5,
			Description: "列出目录内容",
		},
		{
			Name:      "cat_file",
			Pattern:   regexp.MustCompile(`(cat|head|tail|less|more)\s+`),
			Level:     SecurityAuto,
			RiskScore: 5,
			Description: "查看文件内容",
		},
		{
			Name:      "grep_file",
			Pattern:   regexp.MustCompile(`(grep|rg|ag|fg)\s+`),
			Level:     SecurityAuto,
			RiskScore: 5,
			Description: "搜索文件内容",
		},
		{
			Name:      "go_build",
			Pattern:   regexp.MustCompile(`go\s+(build|test|vet|fmt|mod)`),
			Level:     SecurityAuto,
			RiskScore: 10,
			Description: "Go构建/测试命令",
		},
		{
			Name:      "cargo_build",
			Pattern:   regexp.MustCompile(`cargo\s+(build|test|clippy|fmt)`),
			Level:     SecurityAuto,
			RiskScore: 10,
			Description: "Rust构建/测试命令",
		},
		{
			Name:      "npm_yarn",
			Pattern:   regexp.MustCompile(`(npm|yarn|pnpm)\s+(install|build|test|run)`),
			Level:     SecurityAuto,
			RiskScore: 15,
			Description: "Node.js包管理命令",
		},
	}
}

// CheckCommand checks a bash command against security rules
func (se *SecurityEngine) CheckCommand(command string) (level SecurityLevel, riskScore int, matchedRules []SecurityRule) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	level = SecurityAuto
	riskScore = 0
	matchedRules = make([]SecurityRule, 0)

	for _, rule := range se.rules {
		if rule.Pattern.MatchString(command) {
			matchedRules = append(matchedRules, rule)
			if rule.RiskScore > riskScore {
				riskScore = rule.RiskScore
			}
			if rule.Level > level {
				level = rule.Level
			}
		}
	}

	// 未知命令默认需要确认
	if len(matchedRules) == 0 && riskScore == 0 {
		level = SecurityConfirm
		riskScore = 50
	}

	return
}

// AuditAndCheck checks command and logs if dangerous
func (se *SecurityEngine) AuditAndCheck(command, sessionID string) (allowed bool, needsConfirm bool, riskScore int, message string) {
	level, score, rules := se.CheckCommand(command)

	// Log dangerous operations
	if level > SecurityAuto {
		op := DangerousOperation{
			Command:   command,
			RiskScore: score,
			Level:     level,
			Timestamp: time.Now(),
		}
		if len(rules) > 0 {
			op.Rule = rules[0].Name
			op.Description = rules[0].Description
		}
		se.logAudit(op)
	}

	switch level {
	case SecurityAuto:
		return true, false, score, ""
	case SecurityConfirm:
		msg := fmt.Sprintf("⚠️ 需要确认: %s (风险评分: %d/100)", 
			se.buildDescription(rules), score)
		return true, true, score, msg
	case SecurityDeny:
		msg := fmt.Sprintf("🚫 禁止执行: %s (风险评分: %d/100)", 
			se.buildDescription(rules), score)
		log.Printf("[Security] DENIED session=%s cmd=%q rules=%v", 
			sessionID, truncate(command, 200), ruleNames(rules))
		return false, false, score, msg
	}

	return false, false, 0, "未知安全级别"
}

func (se *SecurityEngine) buildDescription(rules []SecurityRule) string {
	if len(rules) == 0 {
		return "未识别的危险操作"
	}
	descs := make([]string, 0, len(rules))
	for _, r := range rules {
		descs = append(descs, r.Description)
	}
	return strings.Join(descs, "; ")
}

func (se *SecurityEngine) logAudit(op DangerousOperation) {
	se.mu.Lock()
	defer se.mu.Unlock()

	if len(se.auditLog) >= se.maxAuditSize {
		se.auditLog = se.auditLog[1:]
	}
	se.auditLog = append(se.auditLog, op)
}

// GetAuditLog returns recent audit entries
func (se *SecurityEngine) GetAuditLog(limit int) []DangerousOperation {
	se.mu.RLock()
	defer se.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}
	start := len(se.auditLog) - limit
	if start < 0 {
		start = 0
	}
	return append([]DangerousOperation{}, se.auditLog[start:]...)
}

// AddRule adds a custom security rule
func (se *SecurityEngine) AddRule(rule SecurityRule) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.rules = append(se.rules, rule)
}

// GetRules returns all rules (for API/UI display)
func (se *SecurityEngine) GetRules() []SecurityRule {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return append([]SecurityRule{}, se.rules...)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func ruleNames(rules []SecurityRule) []string {
	names := make([]string, 0, len(rules))
	for _, r := range rules {
		names = append(names, r.Name)
	}
	return names
}
