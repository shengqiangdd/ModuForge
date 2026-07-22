package service

import (
	"testing"
)

func TestTemplateService_ListTemplates(t *testing.T) {
	svc := NewTemplateService()
	templates := svc.ListTemplates()
	if len(templates) == 0 {
		t.Error("expected at least one template")
	}
}

func TestTemplateService_GetTemplate(t *testing.T) {
	svc := NewTemplateService()
	tmpl, err := svc.GetTemplate("system.prop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Name == "" {
		t.Error("template name is empty")
	}
}

func TestTemplateService_GetTemplate_NotFound(t *testing.T) {
	svc := NewTemplateService()
	_, err := svc.GetTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestTemplateService_RecommendByDescription(t *testing.T) {
	svc := NewTemplateService()
	results := svc.RecommendByDescription("system prop")
	if len(results) == 0 {
		t.Error("expected at least one recommendation for 'system prop'")
	}
}

func TestTemplateService_RecommendByDescription_NoMatch(t *testing.T) {
	svc := NewTemplateService()
	results := svc.RecommendByDescription("xyzzy_no_match")
	if len(results) != 0 {
		t.Errorf("expected no recommendations, got %d", len(results))
	}
}

func TestTemplateService_RecommendByDescription_TagMatch(t *testing.T) {
	svc := NewTemplateService()
	results := svc.RecommendByDescription("audio")
	if len(results) == 0 {
		t.Error("expected at least one recommendation for 'audio'")
	}
}
