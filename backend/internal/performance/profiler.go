package performance

import (
	"runtime"
	"sync"
	"time"
)

// Profiler 性能分析器
type Profiler struct {
	mu       sync.RWMutex
	metrics  map[string]*Metric
	stopChan chan struct{}
}

// Metric 指标
type Metric struct {
	Name      string
	Count     int64
	Total     float64
	Min       float64
	Max       float64
	Avg       float64
	LastValue float64
	UpdatedAt time.Time
}

// MemoryProfile 内存概况
type MemoryProfile struct {
	Alloc      uint32 `json:"alloc"`
	TotalAlloc uint32 `json:"total_alloc"`
	Sys        uint32 `json:"sys"`
	NumGC      int32  `json:"num_gc"`
	HeapAlloc  uint32 `json:"heap_alloc"`
	HeapSys    uint32 `json:"heap_sys"`
	HeapIdle   uint32 `json:"heap_idle"`
	HeapInuse  uint32 `json:"heap_inuse"`
}

// NewProfiler 创建性能分析器
func NewProfiler() *Profiler {
	p := &Profiler{
		metrics:  make(map[string]*Metric),
		stopChan: make(chan struct{}),
	}

	go p.cleanupLoop()

	return p
}

// Record 记录指标
func (p *Profiler) Record(name string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	metric, exists := p.metrics[name]
	if !exists {
		metric = &Metric{Name: name, Min: value, Max: value}
		p.metrics[name] = metric
	}

	metric.Count++
	metric.Total += value
	metric.LastValue = value
	metric.UpdatedAt = time.Now()

	if value < metric.Min {
		metric.Min = value
	}
	if value > metric.Max {
		metric.Max = value
	}

	metric.Avg = metric.Total / float64(metric.Count)
}

// GetMetric 获取指标
func (p *Profiler) GetMetric(name string) *Metric {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metrics[name]
}

// GetAllMetrics 获取所有指标
func (p *Profiler) GetAllMetrics() map[string]*Metric {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*Metric)
	for k, v := range p.metrics {
		result[k] = v
	}
	return result
}

// GetMemoryProfile 获取内存概况
func (p *Profiler) GetMemoryProfile() MemoryProfile {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryProfile{
		Alloc:      uint32(m.Alloc),
		TotalAlloc: uint32(m.TotalAlloc),
		Sys:        uint32(m.Sys),
		NumGC:      int32(m.NumGC),
		HeapAlloc:  uint32(m.HeapAlloc),
		HeapSys:    uint32(m.HeapSys),
		HeapIdle:   uint32(m.HeapIdle),
		HeapInuse:  uint32(m.HeapInuse),
	}
}

// Reset 重置指标
func (p *Profiler) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metrics = make(map[string]*Metric)
}

func (p *Profiler) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanup()
		case <-p.stopChan:
			return
		}
	}
}

func (p *Profiler) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 清理超过1小时未更新的指标
	cutoff := time.Now().Add(-1 * time.Hour)
	for name, metric := range p.metrics {
		if metric.UpdatedAt.Before(cutoff) {
			delete(p.metrics, name)
		}
	}
}

func (p *Profiler) Stop() {
	close(p.stopChan)
}
