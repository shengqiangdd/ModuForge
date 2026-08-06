package skills

import (
	"context"
	"strings"
	"testing"
)

func TestSecurityScanSkill_Name(t *testing.T) {
	s := NewSecurityScanSkill()
	if s.Name() != "security_scan" {
		t.Errorf("expected 'security_scan', got %q", s.Name())
	}
}

func TestSecurityScanSkill_EmptyContent(t *testing.T) {
	s := NewSecurityScanSkill()
	_, err := s.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestSecurityScanSkill_NoIssues(t *testing.T) {
	s := NewSecurityScanSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
		"language": "go",
		"path":     "main.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No issues found") {
		t.Errorf("expected 'No issues found', got: %s", result)
	}
}

func TestSecurityScanSkill_DetectsHardcodedSecret(t *testing.T) {
	s := NewSecurityScanSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "password = \"supersecret123\"\napi_key = \"sk-1234567890abcdef\"\n",
		"language": "python",
		"path":     "config.py",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "critical") {
		t.Errorf("expected critical severity, got: %s", result)
	}
}

func TestSecurityScanSkill_GoSQLInjection(t *testing.T) {
	s := NewSecurityScanSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  `query := fmt.Sprintf("SELECT * FROM users WHERE id=%s", id)`,
		"language": "go",
		"path":     "db.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "sql_injection") && !strings.Contains(result, "SQL injection") {
		t.Errorf("expected SQL injection detection, got: %s", result)
	}
}

func TestSecurityScanSkill_PythonEval(t *testing.T) {
	s := NewSecurityScanSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "result = eval(user_input)\n",
		"language": "python",
		"path":     "unsafe.py",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "eval") {
		t.Errorf("expected eval detection, got: %s", result)
	}
}

func TestSecurityScanSkill_CppGets(t *testing.T) {
	s := NewSecurityScanSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "char buf[100];\ngets(buf);\n",
		"language": "c++",
		"path:":     "unsafe.c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "gets") {
		t.Errorf("expected gets detection, got: %s", result)
	}
}

func TestSecurityScanSkill_AutoDetectLanguage(t *testing.T) {
	s := NewSecurityScanSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content": "package main\nfunc main() {}\n",
		"path":    "main.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No issues found") {
		t.Errorf("expected auto-detection to work, got: %s", result)
	}
}

func TestCodeQualitySkill_Name(t *testing.T) {
	s := NewCodeQualitySkill()
	if s.Name() != "code_quality" {
		t.Errorf("expected 'code_quality', got %q", s.Name())
	}
}

func TestCodeQualitySkill_EmptyContent(t *testing.T) {
	s := NewCodeQualitySkill()
	_, err := s.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestCodeQualitySkill_SmallFile(t *testing.T) {
	s := NewCodeQualitySkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
		"language": "go",
		"path":     "main.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Grade:") {
		t.Errorf("expected grade in report, got: %s", result)
	}
}

func TestCodeQualitySkill_LongFunction(t *testing.T) {
	s := NewCodeQualitySkill()
	// Create a file with a 100-line function
	var sb strings.Builder
	sb.WriteString("package main\n\nfunc longFunc() {\n")
	for i := 0; i < 100; i++ {
		sb.WriteString("\tx := 1\n")
	}
	sb.WriteString("}\n")

	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  sb.String(),
		"language": "go",
		"path":     "long.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "long function") && !strings.Contains(result, "100 lines") {
		t.Errorf("expected long function warning, got: %s", result)
	}
}

func TestCodeQualitySkill_RustNaming(t *testing.T) {
	s := NewCodeQualitySkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "fn CamelCase() {\n}\n",
		"language": "rust",
		"path:":     "lib.rs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "snake_case") {
		t.Errorf("expected snake_case naming warning for Rust, got: %s", result)
	}
}

func TestProfilingSkill_Name(t *testing.T) {
	s := NewProfilingSkill()
	if s.Name() != "profiling" {
		t.Errorf("expected 'profiling', got %q", s.Name())
	}
}

func TestProfilingSkill_EmptyContent(t *testing.T) {
	s := NewProfilingSkill()
	_, err := s.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestProfilingSkill_GoPerformance(t *testing.T) {
	s := NewProfilingSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "result += \"hello\" // string concat\nfor i := 0; i < 1000; i++ {\n\tgo func() { fmt.Println(i) }()\n}\n",
		"language": "go",
		"path":     "perf.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Profiling Report") {
		t.Errorf("expected profiling report, got: %s", result)
	}
}

func TestProfilingSkill_RustCloneAbuse(t *testing.T) {
	s := NewProfilingSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "let x = data.clone();\nlet y = data.clone();\nlet z = data.clone();\n",
		"language": "rust",
		"path":     "lib.rs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "clone_abuse") && !strings.Contains(result, ".clone()") {
		t.Errorf("expected clone detection, got: %s", result)
	}
}

func TestProfilingSkill_CPUFilter(t *testing.T) {
	s := NewProfilingSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content":  "result += \"hello\"\nfor i := 0; i < 1000; i++ {\n\tgo func() {}()\n}\n",
		"language": "go",
		"path":     "perf.go",
		"target":   "cpu",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Profiling Report") {
		t.Errorf("expected profiling report, got: %s", result)
	}
}

func TestProfilingSkill_AutoDetectLanguage(t *testing.T) {
	s := NewProfilingSkill()
	result, err := s.Execute(context.Background(), map[string]interface{}{
		"content": "fn main() { println!(\"hello\"); }\n",
		"path":    "main.rs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Profiling Report") {
		t.Errorf("expected profiling report with auto-detected language, got: %s", result)
	}
}

func TestProfilingSkill_Metadata(t *testing.T) {
	s := NewProfilingSkill()
	meta := s.Metadata()
	if !meta.ReadOnly {
		t.Error("expected ReadOnly=true")
	}
	if meta.NeedsDB {
		t.Error("expected NeedsDB=false")
	}
}
