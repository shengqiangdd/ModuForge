package perf

import (
	"sort"
	"strings"
)

// BatchProcessor 批量处理器
type BatchProcessor[T any] struct {
	batchSize int
	handler   func([]T) error
}

// NewBatchProcessor 创建批量处理器
func NewBatchProcessor[T any](batchSize int, handler func([]T) error) *BatchProcessor[T] {
	return &BatchProcessor[T]{
		batchSize: batchSize,
		handler:   handler,
	}
}

// Process 批量处理
func (bp *BatchProcessor[T]) Process(items []T) error {
	for i := 0; i < len(items); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]
		if err := bp.handler(batch); err != nil {
			return err
		}
	}
	return nil
}

// Deduplicator 去重器
type Deduplicator[T comparable] struct {
	seen map[T]bool
}

// NewDeduplicator 创建去重器
func NewDeduplicator[T comparable]() *Deduplicator[T] {
	return &Deduplicator[T]{
		seen: make(map[T]bool),
	}
}

// Add 添加元素（返回true表示是新元素）
func (d *Deduplicator[T]) Add(item T) bool {
	if d.seen[item] {
		return false
	}
	d.seen[item] = true
	return true
}

// Contains 检查是否已存在
func (d *Deduplicator[T]) Contains(item T) bool {
	return d.seen[item]
}

// Reset 重置
func (d *Deduplicator[T]) Reset() {
	d.seen = make(map[T]bool)
}

// Size 返回已见元素数量
func (d *Deduplicator[T]) Size() int {
	return len(d.seen)
}

// SortedInsert 有序插入（二分查找位置）
func SortedInsert(arr []int, val int) []int {
	idx := sort.SearchInts(arr, val)
	if idx < len(arr) && arr[idx] == val {
		return arr // 已存在
	}
	arr = append(arr, 0)
	copy(arr[idx+1:], arr[idx:])
	arr[idx] = val
	return arr
}

// SortedSearch 有序二分搜索
func SortedSearch(arr []int, val int) bool {
	idx := sort.SearchInts(arr, val)
	return idx < len(arr) && arr[idx] == val
}

// LCP 最长公共前缀
func LCP(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

// StringDeduplicator 字符串去重器
type StringDeduplicator struct {
	bloom *BloomFilter
	exact map[string]bool
}

// NewStringDeduplicator 创建字符串去重器（Bloom预过滤 + 精确检查）
func NewStringDeduplicator(expectedItems int) *StringDeduplicator {
	return &StringDeduplicator{
		bloom: NewBloomFilter(expectedItems, 0.01),
		exact: make(map[string]bool),
	}
}

// IsDuplicate 检查是否重复
func (sd *StringDeduplicator) IsDuplicate(s string) bool {
	// 快速路径：Bloom filter 判断不存在
	if !sd.bloom.ContainsString(s) {
		return false
	}
	// 慢路径：精确检查
	return sd.exact[s]
}

// Add 添加字符串
func (sd *StringDeduplicator) Add(s string) {
	sd.bloom.AddString(s)
	sd.exact[s] = true
}
