package service

import (
	"database/sql"
	"strings"

	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/llm"
)

// AICapabilityScore 表示当前配置的 AI 能力评分
type AICapabilityScore struct {
	Grade           string   `json:"grade"`            // S/A/B/C/D
	TotalScore      int      `json:"total_score"`      // 0-100
	ModelScore      int      `json:"model_score"`      // 模型能力分
	SpeedScore      int      `json:"speed_score"`      // 响应速度分
	CostScore       int      `json:"cost_score"`       // 成本效率分
	FeatureScore    int      `json:"feature_score"`    // 功能完整度分
	CurrentModel    string   `json:"current_model"`    // 当前模型名称
	CurrentProvider string   `json:"current_provider"` // 当前提供商
	Suggestions     []string `json:"suggestions"`      // 优化建议
}

// EvaluateAICapability 评估当前配置的 AI 能力等级
func EvaluateAICapability(cfg *config.Config, db *sql.DB, userID string) *AICapabilityScore {
	score := &AICapabilityScore{
		Suggestions: make([]string, 0),
	}

	// 解析当前使用的 provider 和 model
	providerID := cfg.LLMProvider
	modelID := cfg.LLMModelID
	if modelID == "" {
		modelID = cfg.LLMModel
	}

	// 查找 provider 和 model 信息
	var currentModel *llm.Model
	var currentProvider *llm.Provider
	providers := llm.GetProviders()
	for _, p := range providers {
		if p.ID == providerID {
			pCopy := p
			currentProvider = &pCopy
			for _, m := range p.Models {
				if m.ID == modelID {
					mCopy := m
					currentModel = &mCopy
					break
				}
			}
			break
		}
	}

	// 如果没找到，尝试从用户自定义 provider 查找
	if currentProvider == nil && db != nil && userID != "" {
		var endpoint, apiKey string
		// 查 provider_configs
		err := db.QueryRow(
			`SELECT COALESCE(endpoint,''), COALESCE(api_key,'') FROM provider_configs WHERE user_id=? AND id=?`, userID, providerID,
		).Scan(&endpoint, &apiKey)
		if err == nil && endpoint != "" {
			currentProvider = &llm.Provider{
				Name:     providerID,
				ID:       providerID,
				Endpoint: endpoint,
			}
		}
	}

	if currentProvider != nil {
		score.CurrentProvider = currentProvider.Name
	} else if providerID != "" {
		// Fallback: use provider ID as display name
		score.CurrentProvider = providerID
	}
	if currentModel != nil {
		score.CurrentModel = currentModel.Name
	} else if modelID != "" {
		score.CurrentModel = modelID
	} else if cfg.LLMModel != "" {
		score.CurrentModel = cfg.LLMModel
	}

	// 1. 模型能力评分 (0-40)
	score.ModelScore = evaluateModelCapability(currentModel, currentProvider)

	// 2. 响应速度评分 (0-20)
	score.SpeedScore = evaluateSpeedScore(currentModel, currentProvider)

	// 3. 成本效率评分 (0-20)
	score.CostScore = evaluateCostScore(currentModel)

	// 4. 功能完整度评分 (0-20)
	score.FeatureScore = evaluateFeatureScore(cfg, db, userID)

	// 计算总分
	score.TotalScore = score.ModelScore + score.SpeedScore + score.CostScore + score.FeatureScore

	// 评定等级
	score.Grade = calculateGrade(score.TotalScore)

	// 生成建议
	score.Suggestions = generateSuggestions(score, currentModel, currentProvider, cfg)

	return score
}

// evaluateModelCapability 评估模型能力
func evaluateModelCapability(model *llm.Model, provider *llm.Provider) int {
	if model == nil {
		return 5 // 无模型信息，给基础分
	}

	score := 10 // 基础分

	// 基于 max tokens 评估
	if model.MaxTokens >= 200000 {
		score += 10
	} else if model.MaxTokens >= 100000 {
		score += 7
	} else if model.MaxTokens >= 32000 {
		score += 5
	} else if model.MaxTokens >= 16000 {
		score += 3
	}

	// 基于流式支持
	if model.SupportsStream {
		score += 5
	}

	// 基于价格区间评估质量（高价通常意味着更高质量）
	if model.PriceInput >= 10.0 {
		score += 15 // 旗舰模型
	} else if model.PriceInput >= 3.0 {
		score += 12 // 高端模型
	} else if model.PriceInput >= 1.0 {
		score += 9 // 中端模型
	} else if model.PriceInput > 0 {
		score += 6 // 经济模型
	} else {
		score += 4 // 免费模型
	}

	// 模型名称中的关键词评估
	modelID := strings.ToLower(model.ID)
	switch {
	case strings.Contains(modelID, "opus") || strings.Contains(modelID, "fable"):
		score += 10 // 顶级模型
	case strings.Contains(modelID, "sonnet") || strings.Contains(modelID, "pro"):
		score += 7
	case strings.Contains(modelID, "haiku") || strings.Contains(modelID, "mini") || strings.Contains(modelID, "nano"):
		score += 4
	case strings.Contains(modelID, "flash") || strings.Contains(modelID, "lite"):
		score += 3
	}

	if score > 40 {
		score = 40
	}
	return score
}

// evaluateSpeedScore 评估响应速度
func evaluateSpeedScore(model *llm.Model, provider *llm.Provider) int {
	score := 10 // 基础分

	if provider == nil {
		return score
	}

	// 免费/本地 provider 通常速度较慢
	if provider.IsFree {
		score += 3
	} else if provider.Tier == "subscription" {
		score += 8
	} else {
		score += 6
	}

	// 基于模型名称判断速度
	if model != nil {
		modelID := strings.ToLower(model.ID)
		switch {
		case strings.Contains(modelID, "flash") || strings.Contains(modelID, "fast") || strings.Contains(modelID, "lite"):
			score += 5 // 快速模型
		case strings.Contains(modelID, "mini") || strings.Contains(modelID, "nano"):
			score += 4
		case strings.Contains(modelID, "pro") || strings.Contains(modelID, "max"):
			score += 2 // 较慢但更强
		case strings.Contains(modelID, "opus") || strings.Contains(modelID, "fable"):
			score += 1 // 最慢但最强
		default:
			score += 3
		}
	}

	if score > 20 {
		score = 20
	}
	return score
}

// evaluateCostScore 评估成本效率
func evaluateCostScore(model *llm.Model) int {
	score := 10

	if model == nil {
		return score
	}

	// 免费模型满分
	if model.PriceInput == 0 && model.PriceOutput == 0 {
		return 20
	}

	// 基于总成本评估（越低成本效率越高）
	totalCost := model.PriceInput + model.PriceOutput
	switch {
	case totalCost <= 0.5:
		score += 10 // 极低成本
	case totalCost <= 2.0:
		score += 8 // 低成本
	case totalCost <= 5.0:
		score += 6 // 中等成本
	case totalCost <= 15.0:
		score += 4 // 较高成本
	case totalCost <= 50.0:
		score += 2 // 高成本
	default:
		score += 0 // 极高成本
	}

	if score > 20 {
		score = 20
	}
	return score
}

// evaluateFeatureScore 评估功能完整度
func evaluateFeatureScore(cfg *config.Config, db *sql.DB, userID string) int {
	score := 0

	// API Key 已配置
	if cfg.EffectiveLLMKey() != "" {
		score += 5
	}

	// 有 provider 配置
	if cfg.LLMProvider != "" {
		score += 3
	}

	// 有 model 配置
	if cfg.LLMModelID != "" || cfg.LLMModel != "" {
		score += 2
	}

	// 用户有自定义 provider 配置
	if db != nil && userID != "" {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM provider_configs WHERE user_id=?`, userID).Scan(&count)
		if count > 0 {
			score += 5
		}

		// 自定义 provider
		var customCount int
		db.QueryRow(`SELECT COUNT(*) FROM custom_providers WHERE user_id=?`, userID).Scan(&customCount)
		if customCount > 0 {
			score += 3
		}

		// 自定义技能
		var skillCount int
		db.QueryRow(`SELECT COUNT(*) FROM custom_skills WHERE user_id=?`, userID).Scan(&skillCount)
		if skillCount > 0 {
			score += 2
		}
	}

	if score > 20 {
		score = 20
	}
	return score
}

// calculateGrade 根据总分计算等级
func calculateGrade(totalScore int) string {
	switch {
	case totalScore >= 90:
		return "S"
	case totalScore >= 75:
		return "A"
	case totalScore >= 60:
		return "B"
	case totalScore >= 40:
		return "C"
	default:
		return "D"
	}
}

// generateSuggestions 生成优化建议
func generateSuggestions(score *AICapabilityScore, model *llm.Model, provider *llm.Provider, cfg *config.Config) []string {
	suggestions := make([]string, 0)

	// 模型建议
	if score.ModelScore < 25 {
		suggestions = append(suggestions, "建议升级到更强大的模型（如 Claude Opus 或 GPT 5.5）以获得更好的代码生成质量")
	}

	// 速度建议
	if score.SpeedScore < 12 {
		suggestions = append(suggestions, "当前模型响应较慢，建议切换到 Flash/Lite 系列模型提升开发效率")
	}

	// 成本建议
	if score.CostScore < 10 && model != nil && model.PriceInput > 5.0 {
		suggestions = append(suggestions, "当前模型成本较高，建议在非关键任务中使用 DeepSeek V4 Flash 或免费模型以降低成本")
	}

	// 功能建议
	if score.FeatureScore < 12 {
		suggestions = append(suggestions, "建议配置用户级 Provider API Key 以获得更稳定的 AI 服务")
	}

	// Provider 建议
	if provider != nil && provider.IsFree {
		suggestions = append(suggestions, "当前使用免费模型，建议配置付费 API Key 以获得更高质量和更快速度")
	}

	// 无 API Key 建议
	if cfg.EffectiveLLMKey() == "" {
		suggestions = append(suggestions, "尚未配置 API Key，建议在设置中配置以启用完整 AI 功能")
	}

	// 等级特定建议
	switch score.Grade {
	case "D":
		suggestions = append(suggestions, "当前 AI 能力等级较低，建议优先配置至少一个付费 Provider")
	case "C":
		suggestions = append(suggestions, "AI 能力基础可用，建议添加自定义 Provider 配置以提升能力")
	case "B":
		suggestions = append(suggestions, "AI 能力良好，可通过添加更多自定义技能进一步提升")
	case "A":
		suggestions = append(suggestions, "AI 能力优秀，可考虑添加自进化技能实现自动化优化")
	case "S":
		suggestions = append(suggestions, "AI 能力已达顶级水平，建议充分利用 Agent 模式的多技能协作能力")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前配置良好，无需优化")
	}

	return suggestions
}
