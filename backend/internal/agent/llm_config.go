package agent

import (
	"encoding/base64"
	"log"
)

// resolveLLMConfig resolves the LLM endpoint, API key, and model from various sources.
func (r *AgentRunner) resolveLLMConfig(userID, reqProviderID, reqModel string, cfg ...RunConfig) (string, string, string) {
	// If RunConfig has pre-resolved values, use them directly
	if len(cfg) > 0 && cfg[0].LLMEndpoint != "" {
		endpoint := cfg[0].LLMEndpoint
		apiKey := cfg[0].LLMApiKey
		model := cfg[0].LLMModel
		if reqModel != "" {
			model = reqModel
		}
		// If API key is empty, try to load from provider_configs or custom_providers table
		if apiKey == "" && reqProviderID != "" && r.db != nil {
			var encKey string
			err := r.db.QueryRow(
				"SELECT api_key FROM provider_configs WHERE id=? AND user_id=?",
				reqProviderID, userID,
			).Scan(&encKey)
			if err == nil && encKey != "" {
				if b, dErr := base64.StdEncoding.DecodeString(encKey); dErr == nil {
					apiKey = string(b)
					log.Printf("[Agent] resolveLLMConfig: loaded API key from provider_configs for provider=%s", reqProviderID)
				}
			}
			// Also try custom_providers table (try by name first, then by UUID id)
			if apiKey == "" {
				var cpKey, cpEp string
				cpErr := r.db.QueryRow(
					"SELECT api_key, endpoint FROM custom_providers WHERE name=? AND user_id=?",
					reqProviderID, userID,
				).Scan(&cpKey, &cpEp)
				if cpErr != nil {
					cpErr = r.db.QueryRow(
						"SELECT api_key, endpoint FROM custom_providers WHERE id=? AND user_id=?",
						reqProviderID, userID,
					).Scan(&cpKey, &cpEp)
				}
				if cpErr == nil && cpKey != "" {
					if b, dErr := base64.StdEncoding.DecodeString(cpKey); dErr == nil {
						apiKey = string(b)
					} else {
						apiKey = cpKey
					}
					// Also override endpoint from custom provider
					if cpEp != "" {
						endpoint = cpEp
					}
					log.Printf("[Agent] resolveLLMConfig: loaded API key+endpoint from custom_providers for provider=%s", reqProviderID)
				}
			}
		}
		log.Printf("[Agent] resolveLLMConfig: using RunConfig endpoint=%s model=%s apiKey_len=%d providerID=%s", endpoint, model, len(apiKey), reqProviderID)
		return endpoint, apiKey, model
	}

	endpoint := r.endpoint
	apiKey := r.apiKey
	model := r.model

	if reqProviderID != "" && r.db != nil {
		// First try llm_providers (preset providers)
		var ep, key, mdl string
		err := r.db.QueryRow(
			"SELECT endpoint, api_key, model_id FROM llm_providers WHERE id=? AND user_id=?",
			reqProviderID, userID,
		).Scan(&ep, &key, &mdl)
		if err == nil && ep != "" {
			endpoint = ep
			if key != "" {
				apiKey = key
			}
			if reqModel != "" {
				model = reqModel
			} else if mdl != "" {
				model = mdl
			}
			log.Printf("[Agent] resolveLLMConfig: provider=%s endpoint=%s model=%s", reqProviderID, endpoint, model)
			return endpoint, apiKey, model
		}
		// Then try custom_providers table (try by name first, then by UUID id)
		var cpEp, cpKey, cpModel string
		cpErr := r.db.QueryRow(
			"SELECT endpoint, api_key, COALESCE(model_id,'') FROM custom_providers WHERE name=? AND user_id=?",
			reqProviderID, userID,
		).Scan(&cpEp, &cpKey, &cpModel)
		if cpErr != nil {
			cpErr = r.db.QueryRow(
				"SELECT endpoint, api_key, COALESCE(model_id,'') FROM custom_providers WHERE id=? AND user_id=?",
				reqProviderID, userID,
			).Scan(&cpEp, &cpKey, &cpModel)
		}
		if cpErr == nil && cpEp != "" {
			endpoint = cpEp
			if cpKey != "" {
				// Decode base64-encoded API key if needed
				if decoded, dErr := base64.StdEncoding.DecodeString(cpKey); dErr == nil {
					apiKey = string(decoded)
				} else {
					apiKey = cpKey
				}
			}
			if reqModel != "" {
				model = reqModel
			} else if cpModel != "" {
				model = cpModel
			}
			log.Printf("[Agent] resolveLLMConfig: custom provider=%s endpoint=%s model=%s", reqProviderID, endpoint, model)
			return endpoint, apiKey, model
		}
		log.Printf("[Agent] resolveLLMConfig: provider=%s not found in db, fallback to default", reqProviderID)
	}

	if r.db != nil {
		var cfgModel, cfgEndpoint, cfgKey string
		err := r.db.QueryRow("SELECT model_id, endpoint, api_key FROM llm_config WHERE id='default'").Scan(&cfgModel, &cfgEndpoint, &cfgKey)
		if err == nil {
			if cfgModel != "" {
				model = cfgModel
			}
			if cfgEndpoint != "" {
				endpoint = cfgEndpoint
			}
			if cfgKey != "" {
				apiKey = cfgKey
			}
		}

		// P0-Fix: If llm_config loaded a preset provider with no API key,
		// also check custom_providers for this user. This ensures callLLMSummary
		// (compact/plan) uses the correct custom provider instead of falling back
		// to the free preset (which triggers FreeUsageLimitError 429).
		if apiKey == "" && userID != "" {
			var cpEp, cpKey, cpModel string
			cpErr := r.db.QueryRow(
				"SELECT endpoint, api_key, COALESCE(model_id,'') FROM custom_providers WHERE user_id=? ORDER BY updated_at DESC LIMIT 1",
				userID,
			).Scan(&cpEp, &cpKey, &cpModel)
			if cpErr == nil && cpEp != "" {
				endpoint = cpEp
				if cpKey != "" {
					if decoded, dErr := base64.StdEncoding.DecodeString(cpKey); dErr == nil {
						apiKey = string(decoded)
					} else {
						apiKey = cpKey
					}
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
