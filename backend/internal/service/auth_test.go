package service

import (
	"context"
	"testing"

	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/domain"
)

func TestNewAuthService(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	svc := NewAuthService(nil, cfg)
	if svc == nil {
		t.Fatal("NewAuthService returned nil")
	}
	if svc.cfg != cfg {
		t.Fatal("config not assigned correctly")
	}
}

func TestNewAuthService_NilDB(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	svc := NewAuthService(nil, cfg)
	if svc == nil {
		t.Fatal("NewAuthService returned nil with nil db")
	}
	if svc.db != nil {
		t.Fatal("expected nil db")
	}
}

func TestRegister_EmptyFields(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	svc := NewAuthService(nil, cfg)

	tests := []struct {
		name string
		req  domain.RegisterRequest
	}{
		{"all empty", domain.RegisterRequest{}},
		{"missing username", domain.RegisterRequest{Email: "a@b.com", Password: "pass"}},
		{"missing email", domain.RegisterRequest{Username: "user", Password: "pass"}},
		{"missing password", domain.RegisterRequest{Username: "user", Email: "a@b.com"}},
		{"only username", domain.RegisterRequest{Username: "user"}},
		{"only email", domain.RegisterRequest{Email: "a@b.com"}},
		{"only password", domain.RegisterRequest{Password: "pass"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(context.Background(), &tt.req)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestRegister_ValidationError_Messages(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	svc := NewAuthService(nil, cfg)

	_, err := svc.Register(context.Background(), &domain.RegisterRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "all fields required" {
		t.Errorf("expected 'all fields required', got '%s'", err.Error())
	}
}

func TestRegister_UsernameRequired(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	svc := NewAuthService(nil, cfg)

	_, err := svc.Register(context.Background(), &domain.RegisterRequest{Email: "a@b.com", Password: "pass"})
	if err == nil {
		t.Error("expected error for missing username")
	}
}

func TestRegister_EmailRequired(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	svc := NewAuthService(nil, cfg)

	_, err := svc.Register(context.Background(), &domain.RegisterRequest{Username: "user", Password: "pass"})
	if err == nil {
		t.Error("expected error for missing email")
	}
}

func TestRegister_PasswordRequired(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	svc := NewAuthService(nil, cfg)

	_, err := svc.Register(context.Background(), &domain.RegisterRequest{Username: "user", Email: "a@b.com"})
	if err == nil {
		t.Error("expected error for missing password")
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateJWT("test-secret", "user-123", "testuser", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestGenerateToken_DifferentSecrets(t *testing.T) {
	token1, _ := GenerateJWT("secret-1", "user-123", "testuser", "user")
	token2, _ := GenerateJWT("secret-2", "user-123", "testuser", "user")

	if token1 == token2 {
		t.Error("different secrets should produce different tokens")
	}
}

func TestGenerateToken_ParsableClaims(t *testing.T) {
	token, _ := GenerateJWT("test-secret", "user-456", "testuser", "user")
	if len(token) < 10 {
		t.Error("token too short")
	}
	parts := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("expected 2 dots in JWT token, got %d", parts)
	}
}
