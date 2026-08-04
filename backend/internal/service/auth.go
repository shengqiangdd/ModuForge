package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/domain"
	"github.com/pquerna/otp/totp"
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
	user.ID = uuid.New().String()

	// Use transaction to prevent race condition where two concurrent registrations
	// both see COUNT=0 and both become admin
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if this is the first user — if so, make them admin
	var userCount int
	tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	role := "user"
	if userCount == 0 {
		role = "admin"
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (id, username, email, password_hash, role, email_verified) VALUES (?, ?, ?, ?, ?, ?)
		 RETURNING id, username, email, created_at`,
		user.ID, req.Username, req.Email, string(hash), role, 1,
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	token, err := GenerateJWT(s.cfg.JWTSecret, user.ID, user.Username, role)
	if err != nil {
		return nil, err
	}

	// Send verification email asynchronously
	go func() {
		s.SendVerificationEmail(user.ID, user.Email)
	}()

	return &domain.AuthResponse{Token: token, User: &user}, nil
}

func (s *AuthService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.AuthResponse, error) {
	var user domain.User
	var totpEnabled int
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, created_at, COALESCE(role,'user'), totp_enabled FROM users WHERE username = ?`,
		req.Username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &role, &totpEnabled)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if totpEnabled == 1 && req.TOTPCode == "" {
		// Generate a temp token for 2FA verification
		tempToken, err := GenerateJWT(s.cfg.JWTSecret, user.ID, user.Username, "2fa_pending")
		if err != nil {
			return nil, err
		}
		return &domain.AuthResponse{
			Requires2FA: true,
			TempToken:   tempToken,
		}, nil
	}

	if totpEnabled == 1 && req.TOTPCode != "" {
		var secret string
		s.db.QueryRow("SELECT totp_secret FROM users WHERE id = ?", user.ID).Scan(&secret)
		if !totp.Validate(req.TOTPCode, secret) {
			return nil, fmt.Errorf("invalid TOTP code")
		}
	}

	token, err := GenerateJWT(s.cfg.JWTSecret, user.ID, user.Username, role)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: token, User: &user}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, tokenStr string) (*domain.AuthResponse, error) {
	// Allow expired tokens within 7-day grace period for refresh
	claims, err := ParseJWTAllowExpired(tokenStr, s.cfg.JWTSecret, 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	var user domain.User
	var role string
	err = s.db.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, COALESCE(role,'user'), created_at FROM users WHERE id = ?`,
		claims.UID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &role, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	newToken, err := GenerateJWT(s.cfg.JWTSecret, user.ID, user.Username, role)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: newToken, User: &user}, nil
}

type TOTPSetup struct {
	Secret    string `json:"secret"`
	QRURL     string `json:"qr_url"`
}

func (s *AuthService) GenerateTOTPSecret(userID string) (*TOTPSetup, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "ModuForge",
		AccountName: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("generate totp secret: %w", err)
	}

	secret := key.Secret()
	qrURL := key.URL()

	// Store the secret temporarily (not enabled yet)
	_, err = s.db.Exec("UPDATE users SET totp_secret = ? WHERE id = ?", secret, userID)
	if err != nil {
		return nil, fmt.Errorf("save totp secret: %w", err)
	}

	return &TOTPSetup{Secret: secret, QRURL: qrURL}, nil
}

func (s *AuthService) EnableTOTP(userID, code string) error {
	var secret string
	err := s.db.QueryRow("SELECT totp_secret FROM users WHERE id = ?", userID).Scan(&secret)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if secret == "" {
		return fmt.Errorf("TOTP not set up, generate a secret first")
	}

	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid TOTP code")
	}

	_, err = s.db.Exec("UPDATE users SET totp_enabled = 1 WHERE id = ?", userID)
	return err
}

func (s *AuthService) DisableTOTP(userID, code string) error {
	var secret string
	err := s.db.QueryRow("SELECT totp_secret FROM users WHERE id = ?", userID).Scan(&secret)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if secret == "" {
		return fmt.Errorf("TOTP is not enabled")
	}

	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid TOTP code")
	}

	_, err = s.db.Exec("UPDATE users SET totp_secret = '', totp_enabled = 0 WHERE id = ?", userID)
	return err
}

func (s *AuthService) VerifyTOTP(userID, code string) error {
	var secret string
	err := s.db.QueryRow("SELECT totp_secret FROM users WHERE id = ?", userID).Scan(&secret)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if secret == "" {
		return nil
	}
	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid TOTP code")
	}
	return nil
}

func (s *AuthService) GetUser(ctx context.Context, uid string) (*domain.User, error) {
	var user domain.User
	var totpEnabled int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, email, created_at, totp_enabled FROM users WHERE id = ?`,
		uid,
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt, &totpEnabled)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	return &user, nil
}

func (s *AuthService) SendVerificationEmail(userID, email string) error {
	code, err := GenerateCode(6)
	if err != nil {
		return err
	}
	expires := time.Now().Add(10 * time.Minute)
	_, err = s.db.Exec(
		`UPDATE users SET verify_token = ?, verify_expires = ? WHERE id = ?`,
		code, expires.Format("2006-01-02 15:04:05"), userID)
	if err != nil {
		return err
	}
	emailSvc := NewEmailService(s.db)
	return emailSvc.SendVerificationCode(email, code)
}

func (s *AuthService) VerifyEmail(token string) error {
	var userID string
	var expires sql.NullString
	err := s.db.QueryRow(
		`SELECT id, verify_expires FROM users WHERE verify_token = ? AND email_verified = 0`, token,
	).Scan(&userID, &expires)
	if err != nil {
		return fmt.Errorf("invalid verification token")
	}
	if expires.Valid && expires.String != "" {
		expTime, err := time.Parse("2006-01-02 15:04:05", expires.String)
		if err == nil && time.Now().After(expTime) {
			return fmt.Errorf("verification code expired")
		}
	}
	_, err = s.db.Exec(`UPDATE users SET email_verified = 1, verify_token = '' WHERE id = ?`, userID)
	return err
}

func (s *AuthService) RequestPasswordReset(email string) error {
	var userID string
	err := s.db.QueryRow(`SELECT id FROM users WHERE email = ?`, email).Scan(&userID)
	if err != nil {
		// Return generic message to prevent user enumeration
		return nil
	}
	code, err := GenerateCode(6)
	if err != nil {
		return err
	}
	expires := time.Now().Add(1 * time.Hour)
	_, err = s.db.Exec(
		`UPDATE users SET verify_token = ?, verify_expires = ? WHERE id = ?`,
		code, expires.Format("2006-01-02 15:04:05"), userID)
	if err != nil {
		return err
	}
	emailSvc := NewEmailService(s.db)
	return emailSvc.SendPasswordReset(email, code)
}

func (s *AuthService) ResetPassword(token, newPassword string) error {
	var userID string
	var expires sql.NullString
	err := s.db.QueryRow(
		`SELECT id, verify_expires FROM users WHERE verify_token = ?`, token,
	).Scan(&userID, &expires)
	if err != nil {
		return fmt.Errorf("invalid reset token")
	}
	if expires.Valid && expires.String != "" {
		expTime, err := time.Parse("2006-01-02 15:04:05", expires.String)
		if err == nil && time.Now().After(expTime) {
			return fmt.Errorf("reset token expired")
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ?, verify_token = '' WHERE id = ?`, string(hash), userID)
	return err
}

type UserProfile struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	DisplayName   string `json:"display_name"`
	Bio           string `json:"bio"`
	Location      string `json:"location"`
	Website       string `json:"website"`
	AvatarURL     string `json:"avatar_url"`
	EmailVerified int    `json:"email_verified"`
	IsAdmin       bool   `json:"is_admin"`
	CreatedAt     string `json:"created_at"`
}

func (s *AuthService) GetProfile(userID string) (*UserProfile, error) {
	var p UserProfile
	var role string
	err := s.db.QueryRow(
		`SELECT id, username, email, COALESCE(display_name,''), COALESCE(bio,''), COALESCE(location,''), COALESCE(website,''), COALESCE(avatar_url,''), COALESCE(email_verified,0), COALESCE(role,'user'), created_at
		 FROM users WHERE id = ?`, userID,
	).Scan(&p.ID, &p.Username, &p.Email, &p.DisplayName, &p.Bio, &p.Location, &p.Website, &p.AvatarURL, &p.EmailVerified, &role, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	p.IsAdmin = (role == "admin")
	return &p, nil
}

func (s *AuthService) UpdateProfile(userID string, updates map[string]string) error {
	allowed := map[string]bool{"display_name": true, "bio": true, "location": true, "website": true}
	// Build SET clause only from allowed keys to avoid SQL injection
	var setClauses []string
	var args []interface{}
	for k, v := range updates {
		if !allowed[k] {
			continue
		}
		setClauses = append(setClauses, k+"=?")
		args = append(args, v)
	}
	if len(setClauses) == 0 {
		return nil
	}
	args = append(args, userID)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id=?", strings.Join(setClauses, ", "))
	_, err := s.db.Exec(query, args...)
	return err
}

func (s *AuthService) UploadAvatar(userID string, avatarPath string) error {
	_, err := s.db.Exec(`UPDATE users SET avatar_url = ? WHERE id = ?`, avatarPath, userID)
	return err
}

// ChangePassword verifies old password and sets new one.
func (s *AuthService) ChangePassword(userID, oldPassword, newPassword string) error {
	var hash string
	if err := s.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		return fmt.Errorf("用户不存在")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("当前密码错误")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(newHash), userID)
	return err
}
