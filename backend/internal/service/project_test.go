package service

import (
	"testing"
)

func TestNewProjectService(t *testing.T) {
	svc := NewProjectService(nil)
	if svc == nil {
		t.Fatal("NewProjectService returned nil")
	}
	if svc.db != nil {
		t.Fatal("expected nil db")
	}
}
