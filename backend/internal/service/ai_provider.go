package service

import (
	"encoding/base64"

	"github.com/moduforge/backend/internal/llm"
)

// resolveUserProviderConfig 查询用户自定义的 endpoint 和 api_key
func (s *AIService) resolveUserProviderConfig(userID, providerID string) (endpoint, apiKey string) {
	if s.db == nil || userID == "" {
		return "", ""
	}
	var dbEndpoint, dbAPIKey string
	err := s.db.QueryRow(
		`SELECT COALESCE(endpoint,''), COALESCE(api_key,'') FROM provider_configs WHERE user_id=? AND id=?`, userID, providerID,
	).Scan(&dbEndpoint, &dbAPIKey)
	if err == nil {
		if dbAPIKey != "" {
			if b, err := base64.StdEncoding.DecodeString(dbAPIKey); err == nil {
				dbAPIKey = string(b)
			}
		}
		return dbEndpoint, dbAPIKey
	}
	return "", ""
}

// resolveLLMConfig resolves the effective LLM endpoint, API key, and model
// for a given user. It merges global config with per-user provider overrides.
func (s *AIService) resolveLLMConfig(userID string) (endpoint, apiKey, model, providerID string) {
	endpoint = s.cfg.LLMEndpoint
	apiKey = s.cfg.EffectiveLLMKey()
	model = s.cfg.LLMModel
	providerID = s.cfg.LLMProvider

	if userID != "" && providerID != "" {
		userEndpoint, userKey := s.resolveUserProviderConfig(userID, providerID)
		if userEndpoint != "" {
			endpoint = userEndpoint
		}
		if userKey != "" {
			apiKey = userKey
		}
	}
	return
}

// providerRequiresKey checks whether the given provider requires an API key.
func providerRequiresKey(providerID string) bool {
	if providerID == "" {
		return true
	}
	p := llm.FindProvider(providerID)
	if p != nil {
		return p.RequiresKey
	}
	return true
}

// historyForLLM builds the chronological list of past user/assistant turns to
// include in the next LLM request. It walks backwards from the newest turn,
// keeps as many as fit within budgetChars, and drops the trailing current
// user turn (== userPrompt) that the frontend includes in messages.
func historyForLLM(history []Message, userPrompt string, budgetChars int) []Message {
	if len(history) == 0 || budgetChars <= 0 {
		return nil
	}
	limit := len(history)
	if limit > 0 && history[limit-1].Role == "user" && history[limit-1].Content == userPrompt {
		limit--
	}
	keep := make([]Message, 0, limit)
	used := 0
	for i := limit - 1; i >= 0; i-- {
		m := history[i]
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		sz := len(m.Content) + 8
		if used+sz > budgetChars {
			break
		}
		used += sz
		keep = append(keep, m)
	}
	// Reverse back to chronological order.
	for i, j := 0, len(keep)-1; i < j; i, j = i+1, j-1 {
		keep[i], keep[j] = keep[j], keep[i]
	}
	return keep
}

// queryProviderConfigs returns all provider configurations for a user.
func (s *AIService) queryProviderConfigs(userID string) ([]map[string]string, error) {
	if s.db == nil || userID == "" {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT id, endpoint, COALESCE(api_key,'') FROM provider_configs WHERE user_id=?`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []map[string]string
	for rows.Next() {
		var id, endpoint, apiKey string
		if err := rows.Scan(&id, &endpoint, &apiKey); err != nil {
			continue
		}
		if apiKey != "" {
			if b, err := base64.StdEncoding.DecodeString(apiKey); err == nil {
				apiKey = string(b)
			}
		}
		configs = append(configs, map[string]string{
			"id":       id,
			"endpoint": endpoint,
			"api_key":  apiKey,
		})
	}
	return configs, nil
}

// saveProviderConfig upserts a provider configuration for a user.
func (s *AIService) saveProviderConfig(userID, providerID, endpoint, apiKey string) error {
	if s.db == nil {
		return nil
	}
	if userID == "" || providerID == "" {
		return nil
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(apiKey))
	_, err := s.db.Exec(
		`INSERT INTO provider_configs (user_id, id, endpoint, api_key, updated_at)
		 VALUES (?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(user_id, id) DO UPDATE SET endpoint=?, api_key=?, updated_at=datetime('now')`,
		userID, providerID, endpoint, encoded, endpoint, encoded,
	)
	return err
}

// deleteProviderConfig removes a provider configuration for a user.
func (s *AIService) deleteProviderConfig(userID, providerID string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(
		`DELETE FROM provider_configs WHERE user_id=? AND id=?`,
		userID, providerID,
	)
	return err
}
