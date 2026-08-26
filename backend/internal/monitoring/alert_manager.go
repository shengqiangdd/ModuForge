package monitoring

import (
	"fmt"
	"sync"
	"time"
)

// AlertManager 告警管理器
type AlertManager struct {
	mu          sync.RWMutex
	rules       []AlertRule
	alerts      []Alert
	subscribers []chan Alert
	stopChan    chan struct{}
}

// AlertRule 告警规则
type AlertRule struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Metric    string        `json:"metric"`
	Threshold float64       `json:"threshold"`
	Operator  string        `json:"operator"` // gt, lt, eq, gte, lte
	Duration  time.Duration `json:"duration"`
	Severity  string        `json:"severity"` // critical, warning, info
	Enabled   bool          `json:"enabled"`
}

// Alert 告警
type Alert struct {
	ID         string     `json:"id"`
	RuleID     string     `json:"rule_id"`
	Message    string     `json:"message"`
	Severity   string     `json:"severity"`
	Value      float64    `json:"value"`
	Threshold  float64    `json:"threshold"`
	Status     string     `json:"status"` // firing, resolved
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// NewAlertManager 创建告警管理器
func NewAlertManager() *AlertManager {
	am := &AlertManager{
		rules:       make([]AlertRule, 0),
		alerts:      make([]Alert, 0),
		subscribers: make([]chan Alert, 0),
		stopChan:    make(chan struct{}),
	}

	// 默认告警规则
	am.rules = []AlertRule{
		{ID: "high_error_rate", Name: "高错误率", Metric: "error_rate", Threshold: 10, Operator: "gt", Severity: "critical", Enabled: true},
		{ID: "low_cache_hit", Name: "低缓存命中率", Metric: "cache_hit_rate", Threshold: 50, Operator: "lt", Severity: "warning", Enabled: true},
		{ID: "high_latency", Name: "高延迟", Metric: "avg_latency", Threshold: 5000, Operator: "gt", Severity: "warning", Enabled: true},
		{ID: "high_memory", Name: "高内存使用", Metric: "memory_usage", Threshold: 80, Operator: "gt", Severity: "critical", Enabled: true},
	}

	go am.cleanupLoop()

	return am
}

// EvaluateMetric 评估指标
func (am *AlertManager) EvaluateMetric(metric string, value float64) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	for _, rule := range am.rules {
		if !rule.Enabled || rule.Metric != metric {
			continue
		}

		triggered := false
		switch rule.Operator {
		case "gt":
			triggered = value > rule.Threshold
		case "lt":
			triggered = value < rule.Threshold
		case "gte":
			triggered = value >= rule.Threshold
		case "lte":
			triggered = value <= rule.Threshold
		case "eq":
			triggered = value == rule.Threshold
		}

		if triggered {
			alert := Alert{
				ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
				RuleID:    rule.ID,
				Message:   fmt.Sprintf("%s: %.2f (threshold: %.2f)", rule.Name, value, rule.Threshold),
				Severity:  rule.Severity,
				Value:     value,
				Threshold: rule.Threshold,
				Status:    "firing",
				CreatedAt: time.Now(),
			}
			am.alerts = append(am.alerts, alert)
			am.notifySubscribers(alert)
		}
	}
}

// Subscribe 订阅告警
func (am *AlertManager) Subscribe() chan Alert {
	ch := make(chan Alert, 10)
	am.subscribers = append(am.subscribers, ch)
	return ch
}

// GetActiveAlerts 获取活跃告警
func (am *AlertManager) GetActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	active := make([]Alert, 0)
	for _, alert := range am.alerts {
		if alert.Status == "firing" {
			active = append(active, alert)
		}
	}
	return active
}

// GetAlertHistory 获取告警历史
func (am *AlertManager) GetAlertHistory(limit int) []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if limit <= 0 || limit > len(am.alerts) {
		limit = len(am.alerts)
	}

	result := make([]Alert, limit)
	copy(result, am.alerts[len(am.alerts)-limit:])
	return result
}

// ResolveAlert 解决告警
func (am *AlertManager) ResolveAlert(alertID string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i := range am.alerts {
		if am.alerts[i].ID == alertID && am.alerts[i].Status == "firing" {
			now := time.Now()
			am.alerts[i].Status = "resolved"
			am.alerts[i].ResolvedAt = &now
			return true
		}
	}
	return false
}

func (am *AlertManager) notifySubscribers(alert Alert) {
	for _, ch := range am.subscribers {
		select {
		case ch <- alert:
		default:
			// 非阻塞发送
		}
	}
}

func (am *AlertManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			am.cleanup()
		case <-am.stopChan:
			return
		}
	}
}

func (am *AlertManager) cleanup() {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 保留最近1000条告警
	if len(am.alerts) > 1000 {
		am.alerts = am.alerts[len(am.alerts)-1000:]
	}
}

// Stop 停止清理循环
func (am *AlertManager) Stop() {
	close(am.stopChan)
}
