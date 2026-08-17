package agent

import (
	"time"

	"github.com/moduforge/backend/internal/llm"
)

// MonthlyCostInfo carries the aggregated cost estimation for the current month.
type MonthlyCostInfo struct {
	Month         string  `json:"month"`          // YYYY-MM
	Tokens        int64   `json:"tokens"`         // total LLM tokens this month
	EstimatedCost float64 `json:"estimated_cost"` // USD, based on current model price
	LimitUSD      float64 `json:"limit_usd"`      // configured limit; 0 = unlimited
	Exceeded      bool    `json:"exceeded"`       // estimated_cost >= limit (when limit > 0)
}

// calcMonthlyCost sums llm_token_usage from ai_usage_daily for the current
// month and estimates cost at the given per-1M-token price.
func (r *AgentRunner) calcMonthlyCost(userID string, inputPrice, outputPrice float64) MonthlyCostInfo {
	info := MonthlyCostInfo{
		Month:    time.Now().Format("2006-01"),
		LimitUSD: r.monthlyCostLimit,
	}
	if r.db == nil {
		return info
	}
	monthPrefix := time.Now().Format("2006-01")
	rows, err := r.db.Query(`SELECT llm_token_usage FROM ai_usage_daily WHERE user_id = ? AND date LIKE ?`, userID, monthPrefix+"%")
	if err != nil {
		return info
	}
	defer rows.Close()
	for rows.Next() {
		var tokens int64
		if err := rows.Scan(&tokens); err == nil {
			info.Tokens += tokens
		}
	}
	avgPrice := (inputPrice + outputPrice) / 2
	if avgPrice > 0 {
		info.EstimatedCost = float64(info.Tokens) / 1_000_000 * avgPrice
	}
	if info.LimitUSD > 0 {
		info.Exceeded = info.EstimatedCost >= info.LimitUSD
	}
	return info
}

// CalcMonthlyCostInfo is the exported accessor for the handler/reporting layer.
func (r *AgentRunner) CalcMonthlyCostInfo(userID string, inputPrice, outputPrice float64) MonthlyCostInfo {
	return r.calcMonthlyCost(userID, inputPrice, outputPrice)
}

// MonthlyCostExceeded is a quick check used at AI entry points.
func (r *AgentRunner) MonthlyCostExceeded(userID string, inputPrice, outputPrice float64) bool {
	if r.monthlyCostLimit <= 0 {
		return false
	}
	info := r.calcMonthlyCost(userID, inputPrice, outputPrice)
	return info.Exceeded
}// SetMonthlyCostLimit configures the monthly AI cost cap (USD, 0 = unlimited).
func (r *AgentRunner) SetMonthlyCostLimit(limit float64) {
	r.monthlyCostLimit = limit
}// ModelPricer resolves input/output USD-per-1M-token prices by model ID.
// It scans the built-in provider catalog; unknown models (e.g. custom
// endpoints) fall back to (0,0) so cost estimation stays conservative
// (free/unspecified models never trip the cap).
func ModelPricer(modelID string) (inputPerM, outputPerM float64) {
	if modelID == "" {
		return 0, 0
	}
	for _, p := range llm.GetProviders() {
		for _, m := range p.Models {
			if m.ID == modelID {
				return m.PriceInput, m.PriceOutput
			}
		}
	}
	return 0, 0
}