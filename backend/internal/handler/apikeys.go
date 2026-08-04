package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/gofiber/fiber/v3"
)

type APIKeyHandler struct {
	db *sql.DB
}

func NewAPIKeyHandler(db *sql.DB) *APIKeyHandler {
	return &APIKeyHandler{db: db}
}

type APIKeyResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	KeyPrefix   string  `json:"key_prefix"`
	Permissions string  `json:"permissions"`
	LastUsedAt  *string `json:"last_used_at"`
	ExpiresAt   *string `json:"expires_at"`
	CreatedAt   string  `json:"created_at"`
	PlainKey    string  `json:"plain_key,omitempty"`
}

func generateAPIKey() (string, string, string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	key := "mf_"
	for i := 0; i < 40; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", "", "", err
		}
		key += string(chars[n.Int64()])
	}
	prefix := key[:12]
	h := sha256.New()
	h.Write([]byte(key))
	hash := fmt.Sprintf("%x", h.Sum(nil))
	return key, prefix, hash, nil
}

func (h *APIKeyHandler) List(c fiber.Ctx) error {
	userID := safeUserID(c)
	rows, err := h.db.Query(
		`SELECT id, name, key_prefix, permissions, last_used_at, expires_at, created_at
		 FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()
	var keys []APIKeyResponse
	for rows.Next() {
		var k APIKeyResponse
		var lastUsed, expires sql.NullString
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Permissions, &lastUsed, &expires, &k.CreatedAt); err != nil {
			continue
		}
		if lastUsed.Valid { k.LastUsedAt = &lastUsed.String }
		if expires.Valid { k.ExpiresAt = &expires.String }
		keys = append(keys, k)
	}
	if keys == nil {
		keys = []APIKeyResponse{}
	}
	return c.JSON(keys)
}

func (h *APIKeyHandler) Create(c fiber.Ctx) error {
	userID := safeUserID(c)
	var req struct {
		Name        string `json:"name"`
		Permissions string `json:"permissions"`
		ExpiresAt   string `json:"expires_at"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Name == "" {
		return ValidationError(c, "name required")
	}
	if req.Permissions == "" {
		req.Permissions = `["read"]`
	}
	if !json.Valid([]byte(req.Permissions)) {
		return ValidationError(c, "permissions must be valid JSON array")
	}
	// Validate permissions — only "read" and "write" are allowed; "admin" is forbidden
	var perms []string
	if err := json.Unmarshal([]byte(req.Permissions), &perms); err != nil {
		return ValidationError(c, "permissions must be a JSON array of strings")
	}
	for _, p := range perms {
		if p != "read" && p != "write" {
			return ValidationError(c, "invalid permission: "+p+" (allowed: read, write)")
		}
	}

	key, prefix, hash, err := generateAPIKey()
	if err != nil {
		return InternalError(c, "key generation failed")
	}

	var expires interface{}
	if req.ExpiresAt != "" {
		expires = req.ExpiresAt
	}

	res, err := h.db.Exec(
		`INSERT INTO api_keys (user_id, name, key_hash, key_prefix, permissions, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, req.Name, hash, prefix, req.Permissions, expires)
	if err != nil {
		return InternalError(c, err.Error())
	}
	id, _ := res.LastInsertId()

	return c.Status(201).JSON(APIKeyResponse{
		ID:          id,
		Name:        req.Name,
		KeyPrefix:   prefix,
		Permissions: req.Permissions,
		PlainKey:    key,
	})
}

func (h *APIKeyHandler) Delete(c fiber.Ctx) error {
	userID := safeUserID(c)
	id := c.Params("id")
	_, err := h.db.Exec(`DELETE FROM api_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *APIKeyHandler) Rotate(c fiber.Ctx) error {
	userID := safeUserID(c)
	id := c.Params("id")

	key, prefix, hash, err := generateAPIKey()
	if err != nil {
		return InternalError(c, "key generation failed")
	}

	var name string
	err = h.db.QueryRow(`SELECT name FROM api_keys WHERE id = ? AND user_id = ?`, id, userID).Scan(&name)
	if err != nil {
		return NotFound(c, "api key not found")
	}

	_, err = h.db.Exec(`UPDATE api_keys SET key_hash = ?, key_prefix = ? WHERE id = ? AND user_id = ?`,
		hash, prefix, id, userID)
	if err != nil {
		return InternalError(c, err.Error())
	}

	return c.JSON(APIKeyResponse{
		ID:        parseInt64OrZero(id),
		Name:      name,
		KeyPrefix: prefix,
		PlainKey:  key,
	})
}

func parseInt64OrZero(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}
