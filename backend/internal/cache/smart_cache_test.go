package cache

import (
	"testing"
	"time"
)

func TestSmartCache_SetAndGet(t *testing.T) {
	c := NewSmartCache(100, 5*time.Minute)
	defer c.Stop()

	// 设置值
	c.Set("key1", "value1", 5*time.Minute)
	c.Set("key2", 42, 5*time.Minute)

	// 获取值
	if val, ok := c.Get("key1"); !ok || val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}

	if val, ok := c.Get("key2"); !ok || val != 42 {
		t.Errorf("Expected 42, got %v", val)
	}

	// 不存在的键
	if _, ok := c.Get("nonexistent"); ok {
		t.Error("Expected false for nonexistent key")
	}
}

func TestSmartCache_Expiration(t *testing.T) {
	c := NewSmartCache(100, 100*time.Millisecond)
	defer c.Stop()

	c.Set("expire_key", "value", 50*time.Millisecond)

	// 立即获取应该存在
	if _, ok := c.Get("expire_key"); !ok {
		t.Error("Key should exist immediately")
	}

	// 等待过期
	time.Sleep(100 * time.Millisecond)

	if _, ok := c.Get("expire_key"); ok {
		t.Error("Key should have expired")
	}
}

func TestSmartCache_Eviction(t *testing.T) {
	c := NewSmartCache(2, 5*time.Minute) // 最多2项
	defer c.Stop()

	c.Set("a", 1, 5*time.Minute)
	c.Set("b", 2, 5*time.Minute)
	c.Set("c", 3, 5*time.Minute) // 应该触发淘汰

	if c.GetStats().ItemCount != 2 {
		t.Errorf("Expected 2 items after eviction, got %d", c.GetStats().ItemCount)
	}

	if c.GetStats().Evictions != 1 {
		t.Errorf("Expected 1 eviction, got %d", c.GetStats().Evictions)
	}
}

func TestSmartCache_Stats(t *testing.T) {
	c := NewSmartCache(100, 5*time.Minute)
	defer c.Stop()

	c.Set("key", "value", 5*time.Minute)

	// 命中
	c.Get("key")
	c.Get("key")

	// 未命中
	c.Get("missing")

	stats := c.GetStats()
	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestSmartCache_Delete(t *testing.T) {
	c := NewSmartCache(100, 5*time.Minute)
	defer c.Stop()

	c.Set("key", "value", 5*time.Minute)
	c.Delete("key")

	if _, ok := c.Get("key"); ok {
		t.Error("Key should have been deleted")
	}
}

func TestSmartCache_Concurrent(t *testing.T) {
	c := NewSmartCache(100, 5*time.Minute)
	defer c.Stop()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(n int) {
			key := "key" + string(rune('0'+n))
			c.Set(key, n, 5*time.Minute)
			c.Get(key)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if c.GetStats().ItemCount != 10 {
		t.Errorf("Expected 10 items, got %d", c.GetStats().ItemCount)
	}
}
