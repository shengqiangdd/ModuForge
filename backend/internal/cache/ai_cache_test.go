package cache

import (
	"testing"
	"time"
)

func TestAICache_SetAndGet(t *testing.T) {
	ac := NewAICache()

	prompt := "Write a hello world program"
	model := "gpt-4"
	response := "Here is a hello world program..."

	ac.CacheResponse(prompt, model, response, 5*time.Minute)

	// 获取缓存
	cached, ok := ac.GetCachedResponse(prompt, model)
	if !ok || cached != response {
		t.Errorf("Expected cached response, got %v, %v", cached, ok)
	}

	// 不同的prompt应该未命中
	cached, ok = ac.GetCachedResponse("Different prompt", model)
	if ok {
		t.Error("Expected cache miss for different prompt")
	}

	// 不同的model应该未命中
	cached, ok = ac.GetCachedResponse(prompt, "different-model")
	if ok {
		t.Error("Expected cache miss for different model")
	}
}

func TestAICache_Stats(t *testing.T) {
	ac := NewAICache()

	ac.CacheResponse("prompt1", "model1", "response1", 5*time.Minute)
	ac.GetCachedResponse("prompt1", "model1") // hit
	ac.GetCachedResponse("missing", "model")  // miss

	stats := ac.GetStats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}
