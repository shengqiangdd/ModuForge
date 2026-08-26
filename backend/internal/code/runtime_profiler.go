package code

import (
	"runtime"
	"time"
)

// RuntimeProfiler 运行时性能分析器
type RuntimeProfiler struct {
	snapshots []ProfSnapshot
}

// NewRuntimeProfiler 创建运行时分析器
func NewRuntimeProfiler() *RuntimeProfiler {
	return &RuntimeProfiler{
		snapshots: make([]ProfSnapshot, 0),
	}
}

// ProfSnapshot 性能快照
type ProfSnapshot struct {
	Timestamp  time.Time   `json:"timestamp"`
	Memory     MemoryStats `json:"memory"`
	Goroutines int         `json:"goroutines"`
	GC         GCStats     `json:"gc"`
}

// MemoryStats 内存统计
type MemoryStats struct {
	Alloc     uint64 `json:"alloc"`
	TotalAlloc uint64 `json:"total_alloc"`
	Sys       uint64 `json:"sys"`
	HeapAlloc uint64 `json:"heap_alloc"`
	HeapSys   uint64 `json:"heap_sys"`
	HeapInuse uint64 `json:"heap_inuse"`
	HeapIdle  uint64 `json:"heap_idle"`
}

// GCStats GC统计
type GCStats struct {
	NumGC         uint32   `json:"num_gc"`
	PauseTotalNs  uint64   `json:"pause_total_ns"`
	LastPause     uint64   `json:"last_pause_ns"`
	GCCPUFraction float64  `json:"gc_cpu_fraction"`
}

// ProfileResult 分析结果
type ProfileResult struct {
	Snapshots []ProfSnapshot `json:"snapshots"`
	Summary   ProfSummary    `json:"summary"`
}

// ProfSummary 分析摘要
type ProfSummary struct {
	TotalSnapshots int     `json:"total_snapshots"`
	AvgMemoryMB    float64 `json:"avg_memory_mb"`
	MaxMemoryMB    float64 `json:"max_memory_mb"`
	AvgGoroutines  float64 `json:"avg_goroutines"`
	MaxGoroutines  int     `json:"max_goroutines"`
	GCTotalCount   uint32  `json:"gc_total_count"`
}

// TakeSnapshot 获取性能快照
func (p *RuntimeProfiler) TakeSnapshot() ProfSnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	snapshot := ProfSnapshot{
		Timestamp: time.Now(),
		Memory: MemoryStats{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			HeapAlloc:  m.HeapAlloc,
			HeapSys:    m.HeapSys,
			HeapInuse:  m.HeapInuse,
			HeapIdle:   m.HeapIdle,
		},
		Goroutines: runtime.NumGoroutine(),
		GC: GCStats{
			NumGC:         m.NumGC,
			PauseTotalNs:  m.PauseTotalNs,
			LastPause:     m.PauseNs[(m.NumGC+255)%256],
			GCCPUFraction: m.GCCPUFraction,
		},
	}

	p.snapshots = append(p.snapshots, snapshot)
	return snapshot
}

// GetProfile 获取分析结果
func (p *RuntimeProfiler) GetProfile() *ProfileResult {
	if len(p.snapshots) == 0 {
		return &ProfileResult{
			Snapshots: make([]ProfSnapshot, 0),
			Summary:   ProfSummary{},
		}
	}

	summary := ProfSummary{
		TotalSnapshots: len(p.snapshots),
		MaxGoroutines:  0,
	}

	var totalMemory float64
	var totalGoroutines float64

	for _, s := range p.snapshots {
		memMB := float64(s.Memory.Alloc) / 1024 / 1024
		totalMemory += memMB
		totalGoroutines += float64(s.Goroutines)

		if memMB > summary.MaxMemoryMB {
			summary.MaxMemoryMB = memMB
		}
		if s.Goroutines > summary.MaxGoroutines {
			summary.MaxGoroutines = s.Goroutines
		}
		summary.GCTotalCount = s.GC.NumGC
	}

	summary.AvgMemoryMB = totalMemory / float64(len(p.snapshots))
	summary.AvgGoroutines = totalGoroutines / float64(len(p.snapshots))

	return &ProfileResult{
		Snapshots: p.snapshots,
		Summary:   summary,
	}
}

// Reset 重置快照
func (p *RuntimeProfiler) Reset() {
	p.snapshots = make([]ProfSnapshot, 0)
}
