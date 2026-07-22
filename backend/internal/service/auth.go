package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	db  *sql.DB
	cfg *config.Config
}

func NewAuthService(db *sql.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

func (s *AuthService) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.AuthResponse, error) {
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("all fields required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var user domain.User
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)
		 RETURNING id, username, email, created_at`,
		req.Username, req.Email, string(hash),
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("username or email already exists")
	}

	token, err := GenerateJWT(s.cfg.JWTSecret, user.ID, user.Username, "user")
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: token, User: &user}, nil
}

func (s *AuthService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.AuthResponse, error) {
	var user domain.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, created_at FROM users WHERE username = ?`,
		req.Username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	token, err := GenerateJWT(s.cfg.JWTSecret, user.ID, user.Username, "user")
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: token, User: &user}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, tokenStr string) (*domain.AuthResponse, error) {
	claims, err := ParseJWT(tokenStr, s.cfg.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	var user domain.User
	err = s.db.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, created_at FROM users WHERE id = ?`,
		claims.UID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	newToken, err := GenerateJWT(s.cfg.JWTSecret, user.ID, user.Username, "user")
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: newToken, User: &user}, nil
}

func (s *AuthService) GetUser(ctx context.Context, uid string) (*domain.User, error) {
	var user domain.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, email, created_at FROM users WHERE id = ?`,
		uid,
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	return &user, nil
}
