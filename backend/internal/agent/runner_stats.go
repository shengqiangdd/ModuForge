package agent

import (
	"log"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// Statistics and monitoring methods
// ═══════════════════════════════════════════════════════════════════

// GetToolStats returns tool usage statistics from audit log.
func (r *AgentRunner) GetToolStats() map[string]ToolStats {
	return r.auditLog.GetToolStats()
}

// PrefixCache exposes the agent's shared prefix cache for metrics reporting.
func (r *AgentRunner) PrefixCache() *PrefixCache {
	if r == nil {
		return nil
	}
	return r.prefixCache
}

// GetPerfMetrics returns the aggregated process-lifetime performance metrics
// (LLM calls/tokens, tool calls, errors, retries) for observability UIs.
func (r *AgentRunner) GetPerfMetrics() map[string]interface{} {
	return r.perfMetrics.GetSummary()
}

// GetDailyUsage returns per-day aggregated AI usage for trend charts,
// scoped to the given user. Rows are ordered ascending by date (oldest first).
func (r *AgentRunner) GetDailyUsage(limit int, userID string) []map[string]interface{} {
	if r.db == nil {
		return []map[string]interface{}{}
	}
	rows, err := r.db.Query(`SELECT date, llm_call_count, llm_token_usage, tool_call_count, error_count, retry_count
		FROM ai_usage_daily WHERE user_id = ? ORDER BY date DESC LIMIT ?`, userID, limit)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close()
	days := []map[string]interface{}{}
	for rows.Next() {
		var date string
		var calls, tokens, tools, errs, retries int64
		if err := rows.Scan(&date, &calls, &tokens, &tools, &errs, &retries); err != nil {
			continue
		}
		days = append(days, map[string]interface{}{
			"date":             date,
			"llm_call_count":   calls,
			"llm_token_usage":  tokens,
			"tool_call_count":  tools,
			"error_count":      errs,
			"retry_count":      retries,
		})
	}
	// Reverse to ascending order for charting
	for i, j := 0, len(days)-1; i < j; i, j = i+1, j-1 {
		days[i], days[j] = days[j], days[i]
	}
	return days
}

// persistDailyUsage writes the delta since the last snapshot into ai_usage_daily
// for the current day. Called once per Run so restart losses are bounded by
// one in-flight task.
var usagePersistMu sync.Mutex

func (r *AgentRunner) persistDailyUsage(userID string) {
	if r.db == nil {
		return
	}
	usagePersistMu.Lock()
	defer usagePersistMu.Unlock()

	pm := r.perfMetrics
	pm.mu.Lock()
	calls := pm.LLMCallCount - r.lastUsageSnapshot.Calls
	tokens := pm.LLMTokenUsage - r.lastUsageSnapshot.Tokens
	tools := pm.ToolCallCount - r.lastUsageSnapshot.Tools
	errs := pm.ErrorCount - r.lastUsageSnapshot.Errors
	retries := pm.RetryCount - r.lastUsageSnapshot.Retries
	pm.mu.Unlock()

	if calls <= 0 && tokens <= 0 && tools <= 0 && errs <= 0 && retries <= 0 {
		return
	}
	today := time.Now().Format("2006-01-02")
	_, err := r.db.Exec(`INSERT INTO ai_usage_daily (date, user_id, llm_call_count, llm_token_usage, tool_call_count, error_count, retry_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date, user_id) DO UPDATE SET
			llm_call_count = llm_call_count + excluded.llm_call_count,
			llm_token_usage = llm_token_usage + excluded.llm_token_usage,
			tool_call_count = tool_call_count + excluded.tool_call_count,
			error_count = error_count + excluded.error_count,
			retry_count = retry_count + excluded.retry_count,
			updated_at = CURRENT_TIMESTAMP`,
		today, userID, calls, tokens, tools, errs, retries)
	if err != nil {
		log.Printf("[Usage] persist daily usage: %v", err)
		return
	}
	r.lastUsageSnapshot = usageSnapshot{Calls: pm.LLMCallCount, Tokens: pm.LLMTokenUsage, Tools: pm.ToolCallCount, Errors: pm.ErrorCount, Retries: pm.RetryCount}
	log.Printf("[Usage] persisted %d calls / %d tokens for %s", calls, tokens, today)
}

type usageSnapshot struct {
	Calls   int64
	Tokens  int64
	Tools   int64
	Errors  int64
	Retries int64
}

func (r *AgentRunner) GetAuditHistory(toolName string, limit int) []AuditEntry {
	return r.auditLog.GetHistory(toolName, limit)
}

func (r *AgentRunner) GetPermissionDenials(limit int) []DenialRecord {
	return r.permChecker.GetDenials(limit)
}

func (r *AgentRunner) GetSecurityAuditLog(limit int) []DangerousOperation {
	return r.securityEngine.GetAuditLog(limit)
}

func (r *AgentRunner) GetSecurityRules() []SecurityRule {
	return r.securityEngine.GetRules()
}

func (r *AgentRunner) CheckCommandSecurity(command string) (level int, riskScore int, rules []SecurityRule) {
	sl, rs, rl := r.securityEngine.CheckCommand(command)
	return int(sl), rs, rl
}

func (r *AgentRunner) GetSessionState(sessionID string) *SessionState {
	return r.sessionPersist.GetOrCreate(sessionID)
}
