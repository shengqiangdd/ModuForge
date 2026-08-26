package perf

import (
	"container/list"
	"sync"
)

// LRUCache 带淘汰策略的LRU缓存
type LRUCache[K comparable, V any] struct {
	capacity int
	cache    map[K]*list.Element
	order    *list.List
	lock     sync.RWMutex
	hits     int64
	misses   int64
}

type cacheEntry[K comparable, V any] struct {
	key   K
	value V
}

// NewLRUCache 创建LRU缓存
func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	return &LRUCache[K, V]{
		capacity: capacity,
		cache:    make(map[K]*list.Element),
		order:    list.New(),
	}
}

// Get 获取缓存值
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.order.MoveToFront(elem)
		c.hits++
		return elem.Value.(*cacheEntry[K, V]).value, true
	}
	c.misses++
	var zero V
	return zero, false
}

// Put 设置缓存值
func (c *LRUCache[K, V]) Put(key K, value V) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry[K, V]).value = value
		return
	}

	if c.order.Len() >= c.capacity {
		c.evict()
	}

	entry := &cacheEntry[K, V]{key: key, value: value}
	elem := c.order.PushFront(entry)
	c.cache[key] = elem
}

// evict 淘汰最久未使用的条目
func (c *LRUCache[K, V]) evict() {
	back := c.order.Back()
	if back == nil {
		return
	}
	c.order.Remove(back)
	entry := back.Value.(*cacheEntry[K, V])
	delete(c.cache, entry.key)
}

// Stats 缓存统计
type CacheStats struct {
	Hits      int64
	Misses    int64
	HitRate   float64
	Size      int
	Capacity  int
}

// Stats 获取缓存统计
func (c *LRUCache[K, V]) Stats() CacheStats {
	c.lock.RLock()
	defer c.lock.RUnlock()

	total := c.hits + c.misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}
	return CacheStats{
		Hits:     c.hits,
		Misses:   c.misses,
		HitRate:  hitRate,
		Size:     c.order.Len(),
		Capacity: c.capacity,
	}
}

// Len 返回缓存大小
func (c *LRUCache[K, V]) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.order.Len()
}

// Clear 清空缓存
func (c *LRUCache[K, V]) Clear() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache = make(map[K]*list.Element)
	c.order.Init()
	c.hits = 0
	c.misses = 0
}
