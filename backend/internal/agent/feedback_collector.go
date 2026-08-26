package agent

import (
	"sync"
	"time"
)

// FeedbackCollector 收集用户对AI生成代码的反馈
type FeedbackCollector struct {
	mu        sync.Mutex
	feedbacks []FeedbackRecord
	runner    *AgentRunner
}

// FeedbackRecord 反馈记录
type FeedbackRecord struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	MessageID  string    `json:"message_id"`
	Rating     int       `json:"rating"` // 1-5
	Comment    string    `json:"comment"`
	Accepted   bool      `json:"accepted"`   // 是否接受AI生成的代码
	Modified   bool      `json:"modified"`   // 是否修改了代码
	Language   string    `json:"language"`
	TokensUsed int       `json:"tokens_used"`
	Latency    int64     `json:"latency"`
	CreatedAt  time.Time `json:"created_at"`
}

// NewFeedbackCollector 创建反馈收集器
func NewFeedbackCollector(runner *AgentRunner) *FeedbackCollector {
	return &FeedbackCollector{
		feedbacks: make([]FeedbackRecord, 0),
		runner:    runner,
	}
}

// RecordFeedback 记录用户反馈
func (fc *FeedbackCollector) RecordFeedback(record FeedbackRecord) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	record.CreatedAt = time.Now()
	fc.feedbacks = append(fc.feedbacks, record)
}

// GetFeedbackStats 获取反馈统计
func (fc *FeedbackCollector) GetFeedbackStats() map[string]interface{} {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if len(fc.feedbacks) == 0 {
		return map[string]interface{}{
			"total":           0,
			"avg_rating":      0,
			"acceptance_rate": 0,
		}
	}

	totalRating := 0
	accepted := 0
	modified := 0

	for _, f := range fc.feedbacks {
		totalRating += f.Rating
		if f.Accepted {
			accepted++
		}
		if f.Modified {
			modified++
		}
	}

	return map[string]interface{}{
		"total":             len(fc.feedbacks),
		"avg_rating":        float64(totalRating) / float64(len(fc.feedbacks)),
		"acceptance_rate":   float64(accepted) / float64(len(fc.feedbacks)) * 100,
		"modification_rate": float64(modified) / float64(len(fc.feedbacks)) * 100,
	}
}

// GetRecentFeedbacks 获取最近的反馈
func (fc *FeedbackCollector) GetRecentFeedbacks(limit int) []FeedbackRecord {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if limit <= 0 || limit > len(fc.feedbacks) {
		limit = len(fc.feedbacks)
	}

	result := make([]FeedbackRecord, limit)
	copy(result, fc.feedbacks[len(fc.feedbacks)-limit:])
	return result
}
