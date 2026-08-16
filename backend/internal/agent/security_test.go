package agent

import (
	"testing"
)

func TestSecurityEngine_DangerousCommands(t *testing.T) {
	se := NewSecurityEngine()

	tests := []struct {
		name     string
		command  string
		expected SecurityLevel
		minRisk  int
	}{
		// 绝对禁止
		{"rm_rf_root", "rm -rf /", SecurityDeny, 85},
		{"rm_rf_dotdot", "rm -rf ..", SecurityDeny, 85},
		{"rm_rf_usr", "rm -rf /usr/", SecurityDeny, 85},
		{"dd_zero", "dd if=/dev/zero of=/dev/sda", SecurityDeny, 95},
		{"chmod_777_root", "chmod -R 777 /", SecurityDeny, 90},

		// 需要确认
		{"rm_rf_local", "rm -rf ./build", SecurityConfirm, 80},
		{"git_push_force", "git push --force origin main", SecurityConfirm, 75},
		{"docker_rm", "docker rm -f container1", SecurityConfirm, 65},
		{"sudo_rm", "sudo rm /tmp/file", SecurityConfirm, 85},

		// 安全操作
		{"git_status", "git status", SecurityAuto, 0},
		{"ls_dir", "ls -la", SecurityAuto, 0},
		{"cat_file", "cat README.md", SecurityAuto, 0},
		{"go_build", "go build ./...", SecurityAuto, 0},
		{"cargo_test", "cargo test", SecurityAuto, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, riskScore, _ := se.CheckCommand(tt.command)
			if level != tt.expected {
				t.Errorf("CheckCommand(%q) level = %v, want %v", tt.command, level, tt.expected)
			}
			if riskScore < tt.minRisk {
				t.Errorf("CheckCommand(%q) riskScore = %d, want >= %d", tt.command, riskScore, tt.minRisk)
			}
		})
	}
}

func TestSecurityEngine_AuditAndCheck(t *testing.T) {
	se := NewSecurityEngine()

	// Test denied command
	allowed, needsConfirm, riskScore, msg := se.AuditAndCheck("rm -rf /", "session1")
	if allowed {
		t.Error("Expected denied command to be blocked")
	}
	if needsConfirm {
		t.Error("Expected denied command to not require confirmation")
	}
	if riskScore < 90 {
		t.Errorf("Expected high risk score, got %d", riskScore)
	}
	if msg == "" {
		t.Error("Expected denial message")
	}

	// Test confirm command
	allowed, needsConfirm, riskScore, msg = se.AuditAndCheck("git push --force", "session2")
	if !allowed {
		t.Error("Expected confirm command to be allowed")
	}
	if !needsConfirm {
		t.Error("Expected confirmation needed")
	}
	if riskScore < 50 {
		t.Errorf("Expected elevated risk score for force push, got %d", riskScore)
	}
	if msg == "" {
		t.Error("Expected confirmation message")
	}

	// Test auto command
	allowed, needsConfirm, riskScore, msg = se.AuditAndCheck("git status", "session3")
	if !allowed {
		t.Error("Expected auto command to be allowed")
	}
	if needsConfirm {
		t.Error("Expected no confirmation needed")
	}
	if riskScore > 30 {
		t.Errorf("Expected low risk score for git status, got %d", riskScore)
	}
	if msg != "" {
		t.Errorf("Expected no message for auto command, got %q", msg)
	}

	// Check audit log
	entries := se.GetAuditLog(100)
	if len(entries) < 2 {
		t.Errorf("Expected at least 2 audit entries, got %d", len(entries))
	}
}

func TestSecurityEngine_GetRules(t *testing.T) {
	se := NewSecurityEngine()
	rules := se.GetRules()
	if len(rules) < 10 {
		t.Errorf("Expected at least 10 rules, got %d", len(rules))
	}
}

func TestSecurityEngine_AddRule(t *testing.T) {
	se := NewSecurityEngine()
	
	// Add custom rule
	se.AddRule(SecurityRule{
		Name:        "custom_block",
		Pattern:     nil, // Will use regex
		Level:       SecurityDeny,
		RiskScore:   100,
		Description: "Custom blocked pattern",
	})
	
	rules := se.GetRules()
	if len(rules) < 12 { // 11 default + 1 custom
		t.Errorf("Expected at least 12 rules after add, got %d", len(rules))
	}
}
