package service

import (
	"testing"

	"github.com/moduforge/backend/internal/domain"
)

func TestMarketService_ListModules(t *testing.T) {
	svc := NewMarketService()
	modules, total := svc.ListModules("", "", "stars", 1, 20)
	if total == 0 {
		t.Error("expected at least one module")
	}
	if len(modules) == 0 {
		t.Error("expected at least one module in result")
	}
}

func TestMarketService_ListModules_FilterByCategory(t *testing.T) {
	svc := NewMarketService()
	modules, total := svc.ListModules("", "system", "stars", 1, 20)
	if total == 0 {
		t.Error("expected at least one system module")
	}
	for _, m := range modules {
		if m.Category != "system" {
			t.Errorf("expected category 'system', got '%s'", m.Category)
		}
	}
}

func TestMarketService_ListModules_Search(t *testing.T) {
	svc := NewMarketService()
	modules, _ := svc.ListModules("audio", "", "stars", 1, 20)
	if len(modules) == 0 {
		t.Error("expected at least one module matching 'audio'")
	}
}

func TestMarketService_ListModules_Pagination(t *testing.T) {
	svc := NewMarketService()
	_, total := svc.ListModules("", "", "stars", 1, 100)
	modules1, t1 := svc.ListModules("", "", "stars", 1, 3)

	if total != t1 {
		t.Errorf("total mismatch: %d vs %d", total, t1)
	}
	if len(modules1) != 3 {
		t.Errorf("expected 3 modules on page 1, got %d", len(modules1))
	}
	if total < 3 {
		t.Errorf("expected at least 3 total modules, got %d", total)
	}
}

func TestMarketService_GetModule(t *testing.T) {
	svc := NewMarketService()
	// Use the seeded slug
	mod, err := svc.GetModule("system-prop-tweaks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Slug != "system-prop-tweaks" {
		t.Errorf("expected slug 'system-prop-tweaks', got '%s'", mod.Slug)
	}
}

func TestMarketService_GetModule_NotFound(t *testing.T) {
	svc := NewMarketService()
	_, err := svc.GetModule("nonexistent-slug")
	if err == nil {
		t.Error("expected error for nonexistent module")
	}
}

func TestMarketService_StarModule(t *testing.T) {
	svc := NewMarketService()
	mod, _ := svc.GetModule("system-prop-tweaks")
	originalStars := mod.Stars

	stars, err := svc.StarModule("system-prop-tweaks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stars != originalStars+1 {
		t.Errorf("expected %d stars, got %d", originalStars+1, stars)
	}
}

func TestMarketService_StarModule_NotFound(t *testing.T) {
	svc := NewMarketService()
	_, err := svc.StarModule("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent module")
	}
}

func TestMarketService_AddReview(t *testing.T) {
	svc := NewMarketService()
	err := svc.AddReview("system-prop-tweaks", "uid1", "user1", 5, "Great module!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reviews := svc.GetReviews("system-prop-tweaks")
	if len(reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(reviews))
	}
	if reviews[0].Rating != 5 {
		t.Errorf("expected rating 5, got %d", reviews[0].Rating)
	}
}

func TestMarketService_AddReview_InvalidRating(t *testing.T) {
	svc := NewMarketService()
	err := svc.AddReview("system-prop-tweaks", "uid1", "user1", 0, "bad")
	if err == nil {
		t.Error("expected error for rating 0")
	}
	err = svc.AddReview("system-prop-tweaks", "uid1", "user1", 6, "bad")
	if err == nil {
		t.Error("expected error for rating 6")
	}
}

func TestMarketService_PublishModule(t *testing.T) {
	svc := NewMarketService()
	mod := &domain.MarketModule{
		Title:       "Test Module",
		Description: "A test module",
		Category:    "utility",
		Version:     "v1.0",
		Author:      "tester",
	}
	result, err := svc.PublishModule(mod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID == "" {
		t.Error("expected generated ID")
	}
	if result.Slug == "" {
		t.Error("expected generated slug")
	}
}

func TestMarketService_Categories(t *testing.T) {
	svc := NewMarketService()
	cats := svc.Categories()
	if len(cats) == 0 {
		t.Error("expected at least one category")
	}
}

func TestMarketService_TrendingModules(t *testing.T) {
	svc := NewMarketService()
	trending := svc.TrendingModules(3)
	if len(trending) > 3 {
		t.Errorf("expected at most 3 trending, got %d", len(trending))
	}
	for _, m := range trending {
		if m.Stars <= 100 {
			t.Errorf("expected stars > 100, got %d for %s", m.Stars, m.Slug)
		}
	}
}
