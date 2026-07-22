package service

import (
	"testing"

	"github.com/moduforge/backend/internal/config"
)

func TestNewBuildService(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	svc := NewBuildService(nil, cfg)
	if svc == nil {
		t.Fatal("NewBuildService returned nil")
	}
	if svc.cfg != cfg {
		t.Fatal("config not assigned correctly")
	}
}
