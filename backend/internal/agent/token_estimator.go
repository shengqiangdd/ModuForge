package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sync"
)

func (te *TokenEstimator) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Fast path: check cache
	hash := sha256.Sum256([]byte(text))
	key := hex.EncodeToString(hash[:8])
	if cached, ok := te.cache.Load(key); ok {
		return cached.(int)
	}

	// Classify characters by type
	var chinese, ascii, code, whitespace int

	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\n' || c == '\r' || c == '\t' || c == ' ' {
			whitespace++
			continue
		}
		if c >= 0x80 {
			chinese++
		} else if c == '{' || c == '}' || c == '(' || c == ')' || c == ';' || c == ',' || c == '.' || c == ':' || c == '=' || c == '>' || c == '<' || c == '!' || c == '&' || c == '|' || c == '*' || c == '/' || c == '-' || c == '+' {
			code++
		} else {
			ascii++
		}
	}

	// Weighted estimation
	tokens := float64(chinese)/1.5 + float64(ascii)/4.0 + float64(code)/3.0 + float64(whitespace)/4.0
	result := int(math.Round(tokens))

	// Cache (limit size by only caching if under 100KB text)
	if len(text) < 100000 {
		te.cache.Store(key, result)
	}

	return result
}

// EstimateConversationTokens estimates total tokens in a conversation.
func (te *TokenEstimator) EstimateConversationTokens(messages []map[string]interface{}) int {
	total := 0
	for _, msg := range messages {
		// Message overhead: ~4 tokens per message (role + formatting)
		total += 4
		if content, ok := msg["content"].(string); ok {
			total += te.EstimateTokens(content)
		}
	}
	return total
}

