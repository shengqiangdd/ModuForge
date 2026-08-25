package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/llm"
	"github.com/moduforge/backend/internal/saferead"
	"github.com/moduforge/backend/internal/service"
)

const maxMessagesPerRequest = 100

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type AIHandler struct {
	svc         *service.AIService
	cfg         *config.Config
	db          *database.DB
	memoryStore *service.MemoryStore
	memV2       *service.MemoryV2Store
	fr          *service.FileContentRepo // S3-first content access (optional)
	// Runner (optional) used for monthly AI cost-cap checks in chat/generate.
	runner *agent.AgentRunner
}

func NewAIHandler(svc *service.AIService, cfg *config.Config, db *database.DB) *AIHandler {
	return &AIHandler{svc: svc, cfg: cfg, db: db, memV2: service.NewMemoryV2Store(db.Conn)}
}

// SetRunner injects the AgentRunner for monthly cost-cap enforcement.
func (h *AIHandler) SetRunner(r *agent.AgentRunner) {
	h.runner = r
}

// costCapExceeded returns the monthly cost info and whether the cap is hit.
func (h *AIHandler) costCapExceeded(uid, modelID string) (*agent.MonthlyCostInfo, bool) {
	if h.runner == nil {
		return nil, false
	}
	pi, po := agent.ModelPricer(modelID)
	info := h.runner.CalcMonthlyCostInfo(uid, pi, po)
	return &info, info.Exceeded
}

func (h *AIHandler) SetMemoryStore(ms *service.MemoryStore) {
	h.memoryStore = ms
}

// SetFileContentRepo injects the S3-first file content repository.
func (h *AIHandler) SetFileContentRepo(fr *service.FileContentRepo) {
	h.fr = fr
}

// autoLoadProjectContext loads all files from a project and returns them as context
func (h *AIHandler) autoLoadProjectContext(ctx context.Context, projectID, uid string) string {
	if h.db == nil {
		return ""
	}
	db := h.db.Conn

	// Verify project ownership
	var name, ownerID string
	err := db.QueryRow(`SELECT name, user_id FROM projects WHERE id=?`, projectID).Scan(&name, &ownerID)
	if err != nil || ownerID != uid {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s\n", name))
	fileCount := 0
	if h.fr != nil {
		files, err := h.fr.ReadAll(ctx, projectID)
		if err != nil {
			return ""
		}
		for _, f := range files {
			content, err := h.fr.ReadOne(ctx, projectID, f.Path)
			if err != nil {
				continue
			}
			// Skip empty or oversized files
			if content == "" || len(content) > 10240 {
				continue
			}
			sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", f.Path, content))
			fileCount++
			if fileCount >= 50 {
				break
			}
		}
	} else {
		rows, err := db.Query(`SELECT path, content FROM project_files WHERE project_id=? ORDER BY path`, projectID)
		if err != nil {
			return ""
		}
		defer rows.Close()

		for rows.Next() {
			var path, content string
			if err := rows.Scan(&path, &content); err != nil {
				continue
			}
			// Skip empty or oversized files
			if content == "" || len(content) > 10240 {
				continue
			}
			sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", path, content))
			fileCount++
			if fileCount >= 50 {
				break
			}
		}
	}
	if fileCount == 0 {
		return ""
	}
	return sb.String()
}

// resolveProvider resolves LLM provider from request params or fallback to global config.
// Non-destructive: saves and restores the original provider after resolution.
func (h *AIHandler) resolveProvider(providerID, modelID string) {
	h.cfg.Lock()
	defer h.cfg.Unlock()
	if providerID == "" {
		providerID = h.cfg.LLMProvider
	}
	if modelID == "" {
		modelID = h.cfg.LLMModel
	}
	if providerID != "" {
		savedProvider := h.cfg.LLMProvider
		savedModel := h.cfg.LLMModel
		savedEndpoint := h.cfg.LLMEndpoint
		savedKey := h.cfg.LLMApiKey

		h.cfg.LLMProvider = providerID
		if modelID != "" {
			h.cfg.LLMModel = modelID
		}
		if p := llm.FindProvider(providerID); p != nil {
			h.cfg.LLMEndpoint = p.Endpoint
		}
		h.cfg.LLMApiKey = h.cfg.EffectiveLLMKey()
		slog.Info("resolveProvider", "provider", providerID, "model", modelID, "endpoint", h.cfg.LLMEndpoint, "has_key", h.cfg.LLMApiKey != "")

		// Restore original values before returning. The outer Lock (taken at
		// the top of this function and released by the first defer) is still
		// held here, so the restore closure must NOT re-acquire the write lock
		// — Locking again would self-deadlock (the outer Unlock only runs after
		// this closure, so it would wait forever on a lock owned by us).
		defer func() {
			h.cfg.LLMProvider = savedProvider
			h.cfg.LLMModel = savedModel
			h.cfg.LLMEndpoint = savedEndpoint
			h.cfg.LLMApiKey = savedKey
		}()
	}
}

func (h *AIHandler) getUserID(c fiber.Ctx) string {
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		return uid
	}
	if uid, ok := c.Locals("uid").(string); ok && uid != "" {
		return uid
	}
	return ""
}

// callLLMForSummary makes a simple LLM API call for summary generation
func (h *AIHandler) callLLMForSummary(prompt, systemPrompt string) (string, error) {
	body := map[string]interface{}{
		"model":      h.cfg.LLMModel,
		"messages":   []map[string]string{{"role": "system", "content": systemPrompt}, {"role": "user", "content": prompt}},
		"stream":     false,
		"max_tokens": 4096,
	}
	bodyJSON, _ := json.Marshal(body)

	endpoint := h.cfg.LLMEndpoint
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := h.cfg.EffectiveLLMKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := saferead.SafeReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read LLM response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}
	return result.Choices[0].Message.Content, nil
}

// computeLCS finds the longest common subsequence of lines between two slices.
func computeLCS(a, b []string) [][2]int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to find matching pairs
	var result [][2]int
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append([][2]int{{i - 1, j - 1}}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return result
}
