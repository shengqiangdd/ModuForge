package cache

import (
	"sync"
	"time"
)

// SmartCache 智能缓存系统
type SmartCache struct {
	mu       sync.RWMutex
	items    map[string]*CacheItem
	stats    CacheStats
	maxSize  int
	ttl      time.Duration
	stopChan chan struct{}
}

// CacheItem 缓存项
type CacheItem struct {
	Key        string
	Value      interface{}
	ExpiresAt  time.Time
	HitCount   int64
	LastAccess time.Time
	Size       int64
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits      int64   `json:"Hits"`
	Misses    int64   `json:"Misses"`
	Evictions int64   `json:"Evictions"`
	TotalSize int64   `json:"TotalSize"`
	ItemCount int64   `json:"ItemCount"`
	HitRate   float64 `json:"HitRate"`
}

// NewSmartCache 创建智能缓存
func NewSmartCache(maxSize int, ttl time.Duration) *SmartCache {
	c := &SmartCache{
		items:    make(map[string]*CacheItem),
		maxSize:  maxSize,
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}

	go c.cleanupLoop()

	return c
}

// Get 获取缓存
func (c *SmartCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		c.stats.Misses++
		c.updateHitRate()
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		c.deleteItem(key)
		c.stats.Misses++
		c.updateHitRate()
		return nil, false
	}

	item.HitCount++
	item.LastAccess = time.Now()
	c.stats.Hits++
	c.updateHitRate()

	return item.Value, true
}

// Set 设置缓存
func (c *SmartCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) >= c.maxSize {
		c.evict()
	}

	item := &CacheItem{
		Key:        key,
		Value:      value,
		ExpiresAt:  time.Now().Add(ttl),
		HitCount:   0,
		LastAccess: time.Now(),
		Size:       estimateSize(value),
	}

	c.items[key] = item
	c.stats.TotalSize += item.Size
	c.stats.ItemCount++
}

// Delete 删除缓存
func (c *SmartCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteItem(key)
}

// GetStats 获取缓存统计
func (c *SmartCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Stop 停止缓存
func (c *SmartCache) Stop() {
	close(c.stopChan)
}

func (c *SmartCache) deleteItem(key string) {
	item, exists := c.items[key]
	if exists {
		c.stats.TotalSize -= item.Size
		c.stats.ItemCount--
		delete(c.items, key)
	}
}

func (c *SmartCache) evict() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.LastAccess
		}
	}

	if oldestKey != "" {
		c.deleteItem(oldestKey)
		c.stats.Evictions++
	}
}

func (c *SmartCache) updateHitRate() {
	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRate = float64(c.stats.Hits) / float64(total) * 100
	}
}

func (c *SmartCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopChan:
			return
		}
	}
}

func (c *SmartCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.ExpiresAt) {
			c.deleteItem(key)
		}
	}
}

func estimateSize(v interface{}) int64 {
	switch val := v.(type) {
	case string:
		return int64(len(val))
	case []byte:
		return int64(len(val))
	default:
		return 64
	}
}
