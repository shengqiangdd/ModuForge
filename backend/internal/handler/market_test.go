package handler

import (
	"testing"

	"github.com/moduforge/backend/internal/service"
)

func TestMarketHandler_ListModules(t *testing.T) {
	svc := service.NewMarketService()
	handler := NewMarketHandler(svc)
	if handler == nil {
		t.Fatal("NewMarketHandler returned nil")
	}
}

func TestMarketHandler_Categories(t *testing.T) {
	svc := service.NewMarketService()
	handler := NewMarketHandler(svc)

	cats := handler.market.Categories()
	if len(cats) == 0 {
		t.Error("expected at least one category")
	}
}

func TestMarketHandler_TrendingModules(t *testing.T) {
	svc := service.NewMarketService()
	handler := NewMarketHandler(svc)

	modules := handler.market.TrendingModules(5)
	if len(modules) == 0 {
		t.Error("expected at least one trending module")
	}
}

func TestMarketHandler_GetModule(t *testing.T) {
	svc := service.NewMarketService()
	handler := NewMarketHandler(svc)

	mod, err := handler.market.GetModule("system-prop-tweaks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.Slug != "system-prop-tweaks" {
		t.Errorf("expected slug 'system-prop-tweaks', got '%s'", mod.Slug)
	}
}

func TestMarketHandler_StarModule(t *testing.T) {
	svc := service.NewMarketService()
	handler := NewMarketHandler(svc)

	stars, err := handler.market.StarModule("system-prop-tweaks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stars < 1 {
		t.Error("expected at least 1 star")
	}
}
