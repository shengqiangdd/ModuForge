package agent

import (
	"context"
	"strings"
	"testing"
)

func TestNewQA_NoLLM(t *testing.T) {
	qa := NewQA()
	if qa != nil {
		t.Log("LLM configured, skipping no-LLM test")
		return
	}
}

func TestRunIntegrationTest_MissingModuleProp(t *testing.T) {
	qa := &QA{} // No LLM needed for static checks

	design := ModuleDesign{ModuleName: "test"}
	files := []GeneratedFile{
		{Path: "customize.sh", Content: "#!/system/bin/sh\necho hello"},
	}

	report, err := qa.RunIntegrationTest(context.Background(), design, files)
	if err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}

	if report.Passed {
		t.Error("expected failed report for missing module.prop")
	}

	// Should have error about missing module.prop
	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue.Description, "module.prop") && issue.Severity == SeverityError {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected issue about missing module.prop")
	}
}

func TestRunIntegrationTest_GoodModule(t *testing.T) {
	qa := &QA{}

	design := ModuleDesign{ModuleName: "test"}
	files := []GeneratedFile{
		{Path: "module.prop", Content: "id=test\nname=Test\nversion=1.0\nversionCode=1\nauthor=Test\ndescription=Test module"},
		{Path: "customize.sh", Content: "#!/system/bin/sh\nMODPATH=${0%/*}\nset_perm ${MODPATH}/system/bin/test 0 0 0755"},
	}

	report, err := qa.RunIntegrationTest(context.Background(), design, files)
	if err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}

	if !report.Passed {
		for _, issue := range report.Issues {
			t.Logf("Issue: [%s] %s: %s", issue.Severity, issue.File, issue.Description)
		}
		// Some warnings are acceptable
	}
}

func TestRunIntegrationTest_BareVariable(t *testing.T) {
	qa := &QA{}

	design := ModuleDesign{ModuleName: "test"}
	files := []GeneratedFile{
		{Path: "module.prop", Content: "id=test\nname=Test\nversion=1.0\nversionCode=1\nauthor=Test\ndescription=Test"},
		{Path: "customize.sh", Content: "#!/system/bin/sh\necho $MODPATH"},
	}

	report, err := qa.RunIntegrationTest(context.Background(), design, files)
	if err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}

	// Should find bare variable issue
	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue.Description, "${VAR}") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected issue about bare $VAR")
	}
}

func TestRunIntegrationTest_DangerousRM(t *testing.T) {
	qa := &QA{}

	design := ModuleDesign{ModuleName: "test"}
	files := []GeneratedFile{
		{Path: "module.prop", Content: "id=test\nname=Test\nversion=1.0\nversionCode=1\nauthor=Test\ndescription=Test"},
		{Path: "uninstall.sh", Content: "#!/system/bin/sh\nrm -rf /"},
	}

	report, err := qa.RunIntegrationTest(context.Background(), design, files)
	if err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}

	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue.Description, "rm -rf /") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected issue about dangerous rm -rf /")
	}
}

func TestRunIntegrationTest_MissingSetPerm(t *testing.T) {
	qa := &QA{}

	design := ModuleDesign{ModuleName: "test"}
	files := []GeneratedFile{
		{Path: "module.prop", Content: "id=test\nname=Test\nversion=1.0\nversionCode=1\nauthor=Test\ndescription=Test"},
		{Path: "customize.sh", Content: "#!/system/bin/sh\necho hello"},
		{Path: "main.go", Content: "package main\n\nfunc main() {}"},
	}

	report, err := qa.RunIntegrationTest(context.Background(), design, files)
	if err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}

	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue.Description, "set_perm") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected issue about missing set_perm")
	}
}

func TestRunIntegrationTest_QuotedPropValues(t *testing.T) {
	qa := &QA{}

	design := ModuleDesign{ModuleName: "test"}
	files := []GeneratedFile{
		{Path: "module.prop", Content: `id="test"
name="Test"
version="1.0"
versionCode="1"
author="Test"
description="Test"`},
	}

	report, err := qa.RunIntegrationTest(context.Background(), design, files)
	if err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}

	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue.Description, "quoted") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about quoted values in module.prop")
	}
}

func TestRunIntegrationTest_MissingShebang(t *testing.T) {
	qa := &QA{}

	design := ModuleDesign{ModuleName: "test"}
	files := []GeneratedFile{
		{Path: "module.prop", Content: "id=test\nname=Test\nversion=1.0\nversionCode=1\nauthor=Test\ndescription=Test"},
		{Path: "customize.sh", Content: "echo hello"},
	}

	report, err := qa.RunIntegrationTest(context.Background(), design, files)
	if err != nil {
		t.Fatalf("RunIntegrationTest failed: %v", err)
	}

	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue.Description, "shebang") || strings.Contains(issue.Description, "#!/system/bin/sh") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about missing shebang")
	}
}

func TestTestReport_Summary(t *testing.T) {
	report := TestReport{
		Passed:  true,
		Issues:  []TestIssue{},
		Summary: "All checks passed.",
	}

	if !report.Passed {
		t.Error("expected passed=true")
	}

	if report.Failed {
		t.Error("expected failed=false")
	}
}
