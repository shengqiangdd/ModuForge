package database

import "encoding/base64"

type ProviderConfig struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Endpoint  string `json:"endpoint,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	IsActive  bool   `json:"is_active"`
	UpdatedAt string `json:"updated_at"`
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

func encodeKey(key string) string {
	return base64.StdEncoding.EncodeToString([]byte(key))
}

func decodeKey(encoded string) string {
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(b)
}

// GetProviderConfigs returns all provider configs for a user.
func (db *DB) GetProviderConfigs(userID string) ([]ProviderConfig, error) {
	rows, err := db.Conn.Query(
		`SELECT id, user_id, COALESCE(endpoint,''), COALESCE(api_key,''), is_active, COALESCE(updated_at,'') FROM provider_configs WHERE user_id=?`,
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
		if err := rows.Scan(&c.ID, &c.UserID, &c.Endpoint, &apiKeyEnc, &c.IsActive, &c.UpdatedAt); err != nil {
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
		`SELECT id, user_id, COALESCE(endpoint,''), COALESCE(api_key,''), is_active, COALESCE(updated_at,'') FROM provider_configs WHERE user_id=? AND id=?`,
		userID, providerID,
	).Scan(&c.ID, &c.UserID, &c.Endpoint, &apiKeyEnc, &c.IsActive, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.APIKey = decodeKey(apiKeyEnc)
	return &c, nil
}

// UpsertProviderConfig inserts or updates a provider config for a user.
func (db *DB) UpsertProviderConfig(userID, providerID, endpoint, apiKey string) error {
	apiKeyEnc := encodeKey(apiKey)
	_, err := db.Conn.Exec(
		`INSERT INTO provider_configs (id, user_id, endpoint, api_key, updated_at) VALUES (?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(id, user_id) DO UPDATE SET endpoint=?, api_key=?, updated_at=datetime('now')`,
		providerID, userID, endpoint, apiKeyEnc, endpoint, apiKeyEnc,
	)
	return err
}

// DeleteProviderConfig deletes a provider config for a user (restores defaults).
func (db *DB) DeleteProviderConfig(userID, providerID string) error {
	_, err := db.Conn.Exec(
		`DELETE FROM provider_configs WHERE user_id=? AND id=?`,
		userID, providerID,
	)
	return err
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
