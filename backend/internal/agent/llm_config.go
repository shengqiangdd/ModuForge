package agent

import (
	"database/sql"
	"encoding/base64"
	"log"
	"strings"
)

// decodeAPIKey decodes a base64-encoded API key if possible, otherwise returns raw value.
func decodeAPIKey(key string) string {
	if key == "" {
		return key
	}
	if decoded, err := base64.StdEncoding.DecodeString(key); err == nil {
		return string(decoded)
	}
	return key
}

// resolveLLMConfig resolves the LLM endpoint, API key, and model from various sources.
// Consolidated to minimize DB queries: at most 2-3 queries instead of 5+.
func (r *AgentRunner) resolveLLMConfig(userID, reqProviderID, reqModel string, cfg ...RunConfig) (string, string, string) {
	// If RunConfig has pre-resolved values, use them directly
	if len(cfg) > 0 && cfg[0].LLMEndpoint != "" {
		endpoint := cfg[0].LLMEndpoint
		apiKey := cfg[0].LLMApiKey
		model := cfg[0].LLMModel
		if reqModel != "" {
			model = reqModel
		}
		// If API key is empty, try to load from DB (single query)
		if apiKey == "" && reqProviderID != "" && r.db != nil {
			apiKey, endpoint = r.loadProviderKeyAndEndpoint(userID, reqProviderID, endpoint)
		}
		log.Printf("[Agent] resolveLLMConfig: using RunConfig endpoint=%s model=%s apiKey_len=%d providerID=%s", endpoint, model, len(apiKey), reqProviderID)
		return endpoint, apiKey, model
	}

	endpoint := r.endpoint
	apiKey := r.apiKey
	model := r.model

	if r.db != nil {
		if reqProviderID != "" {
			// Single query: try llm_providers first, then custom_providers (by name, then by id)
			ep, key, mdl, found := r.resolveProviderByID(userID, reqProviderID)
			if found && ep != "" {
				endpoint = ep
				if key != "" {
					apiKey = decodeAPIKey(key)
				}
				if reqModel != "" {
					model = reqModel
				} else if mdl != "" {
					model = mdl
				}
				log.Printf("[Agent] resolveLLMConfig: provider=%s endpoint=%s model=%s", reqProviderID, endpoint, model)
				return endpoint, apiKey, model
			}
			log.Printf("[Agent] resolveLLMConfig: provider=%s not found, falling back to defaults", reqProviderID)
		}

		// Load default config (1 query)
		cfgModel, cfgEndpoint, cfgKey := r.loadDefaultLLMConfig()
		if cfgModel != "" {
			model = cfgModel
		}
		if cfgEndpoint != "" {
			endpoint = cfgEndpoint
		}
		if cfgKey != "" {
			apiKey = decodeAPIKey(cfgKey)
		}

		// If still no API key, try latest custom provider for this user (1 query)
		if apiKey == "" && userID != "" {
			cpEp, cpKey, cpModel, found := r.loadLatestCustomProvider(userID)
			if found && cpEp != "" {
				endpoint = cpEp
				if cpKey != "" {
					apiKey = decodeAPIKey(cpKey)
				}
				if cpModel != "" {
					model = cpModel
				}
				log.Printf("[Agent] resolveLLMConfig: fallback loaded custom provider for user=%s endpoint=%s model=%s", userID, endpoint, model)
			}
		}
	}

	if reqModel != "" {
		model = reqModel
	}
	log.Printf("[Agent] resolveLLMConfig: fallback endpoint=%s model=%s", endpoint, model)
	return endpoint, apiKey, model
}

// resolveProviderByID tries llm_providers then custom_providers in a single query via UNION ALL.
func (r *AgentRunner) resolveProviderByID(userID, providerID string) (endpoint, apiKey, model string, found bool) {
	query := `
		SELECT endpoint, api_key, COALESCE(model_id, ''), 'llm' as src FROM llm_providers WHERE id=? AND user_id=?
		UNION ALL
		SELECT endpoint, api_key, COALESCE(model_id, ''), 'custom' FROM custom_providers WHERE name=? AND user_id=?
		UNION ALL
		SELECT endpoint, api_key, COALESCE(model_id, ''), 'custom' FROM custom_providers WHERE id=? AND user_id=?
		LIMIT 1`
	err := r.db.QueryRow(query, providerID, userID, providerID, userID, providerID, userID).
		Scan(&endpoint, &apiKey, &model, new(string))
	if err != nil {
		return "", "", "", false
	}
	return endpoint, apiKey, model, true
}

// loadProviderKeyAndEndpoint loads API key from provider_configs or custom_providers (single query).
func (r *AgentRunner) loadProviderKeyAndEndpoint(userID, providerID, currentEndpoint string) (apiKey, endpoint string) {
	// Try provider_configs first
	var encKey string
	err := r.db.QueryRow(
		"SELECT api_key FROM provider_configs WHERE id=? AND user_id=?",
		providerID, userID,
	).Scan(&encKey)
	if err == nil && encKey != "" {
		return decodeAPIKey(encKey), currentEndpoint
	}

	// Try custom_providers (by name then by id in one query)
	var cpKey, cpEp string
	err = r.db.QueryRow(`
		SELECT api_key, endpoint FROM (
			SELECT api_key, endpoint FROM custom_providers WHERE name=? AND user_id=?
			UNION ALL
			SELECT api_key, endpoint FROM custom_providers WHERE id=? AND user_id=?
		) LIMIT 1`,
		providerID, userID, providerID, userID,
	).Scan(&cpKey, &cpEp)
	if err == nil && cpKey != "" {
		if cpEp != "" {
			endpoint = cpEp
		} else {
			endpoint = currentEndpoint
		}
		return decodeAPIKey(cpKey), endpoint
	}

	return "", currentEndpoint
}

// loadDefaultLLMConfig loads the default LLM configuration (1 query).
func (r *AgentRunner) loadDefaultLLMConfig() (modelID, endpoint, apiKey string) {
	err := r.db.QueryRow(
		"SELECT COALESCE(model_id, ''), COALESCE(endpoint, ''), COALESCE(api_key, '') FROM llm_config WHERE id='default'",
	).Scan(&modelID, &endpoint, &apiKey)
	if err != nil {
		return "", "", ""
	}
	return modelID, endpoint, apiKey
}

// loadLatestCustomProvider loads the most recently updated custom provider for a user (1 query).
func (r *AgentRunner) loadLatestCustomProvider(userID string) (endpoint, apiKey, model string, found bool) {
	err := r.db.QueryRow(
		"SELECT endpoint, COALESCE(api_key, ''), COALESCE(model_id, '') FROM custom_providers WHERE user_id=? ORDER BY updated_at DESC LIMIT 1",
		userID,
	).Scan(&endpoint, &apiKey, &model)
	if err != nil || endpoint == "" {
		return "", "", "", false
	}
	return endpoint, apiKey, model, true
}

// resolveFallbackConfig finds a paid model to use as fallback when the primary is free.
// It queries the DB for user-configured paid providers, falling back to built-in defaults.
func (r *AgentRunner) resolveFallbackConfig(userID string, currentEndpoint string) (endpoint, apiKey, model string, found bool) {
	if r.db != nil && userID != "" {
		// Try to find a paid provider from the user's llm_providers config
		rows, err := r.db.Query(
			`SELECT endpoint, api_key, model_id FROM llm_providers
			 WHERE user_id=? AND model_id != ''
			 ORDER BY created_at DESC LIMIT 20`,
			userID,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var ep, key, mdl string
				if err := rows.Scan(&ep, &key, &mdl); err != nil {
					continue
				}
				tier := resolveModelTier(mdl)
				if tier != TierFree && ep != "" {
					log.Printf("[Agent] fallback: found paid provider model=%s tier=%d", mdl, tier)
					return ep, decodeAPIKey(key), mdl, true
				}
			}
		}

		// Try custom_providers for paid models
		rows2, err := r.db.Query(
			`SELECT endpoint, api_key, model_id FROM custom_providers
			 WHERE user_id=? AND model_id != ''
			 ORDER BY updated_at DESC LIMIT 20`,
			userID,
		)
		if err == nil {
			defer rows2.Close()
			for rows2.Next() {
				var ep, key, mdl string
				if err := rows2.Scan(&ep, &key, &mdl); err != nil {
					continue
				}
				tier := resolveModelTier(mdl)
				if tier != TierFree && ep != "" {
					log.Printf("[Agent] fallback: found paid custom provider model=%s tier=%d", mdl, tier)
					return ep, decodeAPIKey(key), mdl, true
				}
			}
		}
	}

	// Built-in fallback: use OpenCode Zen's paid GPT-5.4 Mini as universal fallback
	// This requires an API key, so only return if we have one configured globally
	if r.apiKey != "" && r.endpoint != "" {
		// Check if the current endpoint is already the fallback (avoid loop)
		if strings.Contains(currentEndpoint, "opencode.ai") {
			return "", "", "", false
		}
		log.Printf("[Agent] fallback: using built-in paid model gpt-5.4-mini")
		return "https://opencode.ai/zen/v1", r.apiKey, "gpt-5.4-mini", true
	}

	return "", "", "", false
}

// Ensure unused import is consumed
var _ = sql.ErrNoRows
