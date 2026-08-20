package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
)

type ProviderConfig struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Endpoint   string `json:"endpoint,omitempty"`
	APIKey     string `json:"api_key,omitempty"`
	ModelsJSON string `json:"models_json,omitempty"`
	IsActive   bool   `json:"is_active"`
	UpdatedAt  string `json:"updated_at"`
}

type CustomProvider struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	APIKey     string `json:"api_key,omitempty"`
	ModelsJSON string `json:"models_json,omitempty"`
	IsActive   bool   `json:"is_active"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// encryptionKey derives a 32-byte AES key from MODUFORGE_ENCRYPT_KEY env var.
// If not set, falls back to base64 encoding (legacy behavior, insecure).
var encryptionKey []byte

func init() {
	key := os.Getenv("MODUFORGE_ENCRYPT_KEY")
	if key != "" {
		h := sha256.Sum256([]byte(key))
		encryptionKey = h[:]
	}
}

func encodeKey(key string) string {
	if len(encryptionKey) == 0 || key == "" {
		return base64.StdEncoding.EncodeToString([]byte(key))
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return base64.StdEncoding.EncodeToString([]byte(key))
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return base64.StdEncoding.EncodeToString([]byte(key))
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return base64.StdEncoding.EncodeToString([]byte(key))
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(key), nil)
	// Prefix with "enc:" to distinguish from base64-encoded legacy values
	return "enc:" + base64.StdEncoding.EncodeToString(ciphertext)
}

func decodeKey(encoded string) string {
	if encoded == "" {
		return ""
	}
	// Try AES-GCM decryption (new format: "enc:...")
	if len(encryptionKey) > 0 && len(encoded) > 4 && encoded[:4] == "enc:" {
		data, err := base64.StdEncoding.DecodeString(encoded[4:])
		if err != nil {
			return ""
		}
		block, err := aes.NewCipher(encryptionKey)
		if err != nil {
			return ""
		}
		aesGCM, err := cipher.NewGCM(block)
		if err != nil {
			return ""
		}
		nonceSize := aesGCM.NonceSize()
		if len(data) < nonceSize {
			return ""
		}
		nonce, ciphertext := data[:nonceSize], data[nonceSize:]
		plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return ""
		}
		return string(plaintext)
	}
	// Legacy base64 format (no encryption)
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(b)
}

// GetProviderConfigs returns all provider configs for a user.
func (db *DB) GetProviderConfigs(userID string) ([]ProviderConfig, error) {
	rows, err := db.Conn.Query(
		`SELECT id, user_id, COALESCE(endpoint,''), COALESCE(api_key,''), COALESCE(models_json,''), is_active, COALESCE(updated_at,'') FROM provider_configs WHERE user_id=?`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []ProviderConfig
	for rows.Next() {
		var c ProviderConfig
		var apiKeyEnc string
		if err := rows.Scan(&c.ID, &c.UserID, &c.Endpoint, &apiKeyEnc, &c.ModelsJSON, &c.IsActive, &c.UpdatedAt); err != nil {
			continue
		}
		c.APIKey = decodeKey(apiKeyEnc)
		configs = append(configs, c)
	}
	if configs == nil {
		configs = []ProviderConfig{}
	}
	return configs, nil
}

// GetProviderConfig returns a single provider config for a user.
func (db *DB) GetProviderConfig(userID, providerID string) (*ProviderConfig, error) {
	var c ProviderConfig
	var apiKeyEnc string
	err := db.Conn.QueryRow(
		`SELECT id, user_id, COALESCE(endpoint,''), COALESCE(api_key,''), COALESCE(models_json,''), is_active, COALESCE(updated_at,'') FROM provider_configs WHERE user_id=? AND id=?`,
		userID, providerID,
	).Scan(&c.ID, &c.UserID, &c.Endpoint, &apiKeyEnc, &c.ModelsJSON, &c.IsActive, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.APIKey = decodeKey(apiKeyEnc)
	return &c, nil
}

// UpsertProviderConfig inserts or updates a provider config for a user.
func (db *DB) UpsertProviderConfig(userID, providerID, endpoint, apiKey, modelsJSON string) error {
	apiKeyEnc := encodeKey(apiKey)
	_, err := db.Conn.Exec(
		`INSERT INTO provider_configs (id, user_id, endpoint, api_key, models_json, updated_at) VALUES (?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(id, user_id) DO UPDATE SET endpoint=?, api_key=?, models_json=?, updated_at=datetime('now')`,
		providerID, userID, endpoint, apiKeyEnc, modelsJSON, endpoint, apiKeyEnc, modelsJSON,
	)
	if err != nil {
		return fmt.Errorf("save provider config: %w", err)
	}
	return nil
}

// DeleteProviderConfig deletes a provider config for a user (restores defaults).
func (db *DB) DeleteProviderConfig(userID, providerID string) error {
	_, err := db.Conn.Exec(
		`DELETE FROM provider_configs WHERE user_id=? AND id=?`,
		userID, providerID,
	)
	if err != nil {
		return fmt.Errorf("delete provider config: %w", err)
	}
	return nil
}

// GetCustomProviders returns all custom providers for a user.
func (db *DB) GetCustomProviders(userID string) ([]CustomProvider, error) {
	rows, err := db.Conn.Query(
		`SELECT id, user_id, name, endpoint, COALESCE(api_key,''), COALESCE(models_json,''), is_active, COALESCE(created_at,''), COALESCE(updated_at,'') FROM custom_providers WHERE user_id=? ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []CustomProvider
	for rows.Next() {
		var p CustomProvider
		var apiKeyEnc string
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Endpoint, &apiKeyEnc, &p.ModelsJSON, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		p.APIKey = decodeKey(apiKeyEnc)
		providers = append(providers, p)
	}
	if providers == nil {
		providers = []CustomProvider{}
	}
	return providers, nil
}

// GetCustomProvider returns a single custom provider by ID.
func (db *DB) GetCustomProvider(userID, providerID string) (*CustomProvider, error) {
	var p CustomProvider
	var apiKeyEnc string
	err := db.Conn.QueryRow(
		`SELECT id, user_id, name, endpoint, COALESCE(api_key,''), COALESCE(models_json,''), is_active, COALESCE(created_at,''), COALESCE(updated_at,'') FROM custom_providers WHERE user_id=? AND id=?`,
		userID, providerID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Endpoint, &apiKeyEnc, &p.ModelsJSON, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.APIKey = decodeKey(apiKeyEnc)
	return &p, nil
}

// CreateCustomProvider inserts a new custom provider.
func (db *DB) CreateCustomProvider(p *CustomProvider) error {
	apiKeyEnc := encodeKey(p.APIKey)
	_, err := db.Conn.Exec(
		`INSERT INTO custom_providers (id, user_id, name, endpoint, api_key, models_json) VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.UserID, p.Name, p.Endpoint, apiKeyEnc, p.ModelsJSON,
	)
	return err
}

// UpdateCustomProvider updates an existing custom provider.
func (db *DB) UpdateCustomProvider(p *CustomProvider) error {
	apiKeyEnc := encodeKey(p.APIKey)
	_, err := db.Conn.Exec(
		`UPDATE custom_providers SET name=?, endpoint=?, api_key=?, models_json=?, updated_at=datetime('now') WHERE user_id=? AND id=?`,
		p.Name, p.Endpoint, apiKeyEnc, p.ModelsJSON, p.UserID, p.ID,
	)
	return err
}

// DeleteCustomProvider deletes a custom provider.
func (db *DB) DeleteCustomProvider(userID, providerID string) error {
	_, err := db.Conn.Exec(
		`DELETE FROM custom_providers WHERE user_id=? AND id=?`,
		userID, providerID,
	)
	return err
}
