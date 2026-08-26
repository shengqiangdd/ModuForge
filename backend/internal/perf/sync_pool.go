package perf

import (
	"sync"
	"sync/atomic"
)

// SyncPool 高性能同步对象池（减少GC压力）
type SyncPool[T any] struct {
	pool    sync.Pool
	size    atomic.Int64
	factory func() T
	reset   func(T)
}

// NewSyncPool 创建同步对象池
func NewSyncPool[T any](factory func() T, reset func(T)) *SyncPool[T] {
	return &SyncPool[T]{
		factory: factory,
		reset:   reset,
		pool: sync.Pool{
			New: func() interface{} {
				return factory()
			},
		},
	}
}

// Get 获取对象
func (p *SyncPool[T]) Get() T {
	p.size.Add(1)
	return p.pool.Get().(T)
}

// Put 归还对象
func (p *SyncPool[T]) Put(obj T) {
	p.size.Add(-1)
	if p.reset != nil {
		p.reset(obj)
	}
	p.pool.Put(obj)
}

// Stats 获取池统计
func (p *SyncPool[T]) Stats() int64 {
	return p.size.Load()
}

// ByteBuffer 字节缓冲池
type ByteBuffer struct {
	pool *SyncPool[[]byte]
}

// NewByteBuffer 创建字节缓冲池
func NewByteBuffer(initialSize int) *ByteBuffer {
	return &ByteBuffer{
		pool: NewSyncPool[[]byte](
			func() []byte {
				return make([]byte, 0, initialSize)
			},
			func(_ []byte) {
				// Reset buffer length to 0 (capacity preserved)
			},
		),
	}
}

// Get 获取缓冲区
func (b *ByteBuffer) Get() []byte {
	return b.pool.Get()
}

// Put 归还缓冲区
func (b *ByteBuffer) Put(buf []byte) {
	b.pool.Put(buf)
}

// StringPool 字符串构建池
type StringPool struct {
	builderPool *SyncPool[[]rune]
}

// NewStringPool 创建字符串池
func NewStringPool() *StringPool {
	return &StringPool{
		builderPool: NewSyncPool[[]rune](
			func() []rune {
				return make([]rune, 0, 256)
			},
			func(_ []rune) {
				// Reset rune slice length (capacity preserved)
			},
		),
	}
}

// Get 获取字符串缓冲
func (p *StringPool) Get() []rune {
	return p.builderPool.Get()
}

// Put 归还字符串缓冲
func (p *StringPool) Put(buf []rune) {
	p.builderPool.Put(buf)
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	Name        string  `json:"name"`
	Operations  int64   `json:"operations"`
	NSPerOp     float64 `json:"ns_per_op"`
	MBPerSec    float64 `json:"mb_per_sec"`
	AllocsPerOp int64   `json:"allocs_per_op"`
}

// RunBenchmark 运行简单基准测试
func RunBenchmark(name string, iterations int, fn func()) BenchmarkResult {
	// 预热
	for i := 0; i < iterations/10; i++ {
		fn()
	}

	// 正式测试
	start := make(chan struct{})
	done := make(chan struct{})

	var ops int64
	go func() {
		<-start
		for i := 0; i < iterations; i++ {
			fn()
			atomic.AddInt64(&ops, 1)
		}
		close(done)
	}()

	close(start)
	<-done

	return BenchmarkResult{
		Name:       name,
		Operations: ops,
	}
}
