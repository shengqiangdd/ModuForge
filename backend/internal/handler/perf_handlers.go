package handler

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/perf"
)

// Phase 28: 性能优化性能分析 Handler

// lruCacheGlobal 全局LRU缓存实例
var lruCacheGlobal = perf.NewLRUCache[string, string](1000)

// CachePut 设置缓存
func (h *AIHandler) CachePut(c fiber.Ctx) error {
	type request struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		TTL   int    `json:"ttl"` // seconds, 0 = no expiry
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}
	if req.Key == "" {
		return BadRequest(c, "Key is required")
	}

	lruCacheGlobal.Put(req.Key, req.Value)

	return c.Status(200).JSON(fiber.Map{
		"valid": true,
		"stats": lruCacheGlobal.Stats(),
	})
}

// CacheGet 获取缓存
func (h *AIHandler) CacheGet(c fiber.Ctx) error {
	key := c.Query("key")
	if key == "" {
		return BadRequest(c, "Key is required")
	}

	value, ok := lruCacheGlobal.Get(key)

	return c.Status(200).JSON(fiber.Map{
		"valid": true,
		"found": ok,
		"value": value,
		"stats": lruCacheGlobal.Stats(),
	})
}

// CacheStats 获取缓存统计
func (h *AIHandler) CacheStats(c fiber.Ctx) error {
	return c.Status(200).JSON(fiber.Map{
		"valid": true,
		"stats": lruCacheGlobal.Stats(),
	})
}

// HandleBloomFilterDemo 演示布隆过滤器
func (h *AIHandler) HandleBloomFilterDemo(c fiber.Ctx) error {
	type request struct {
		Action string   `json:"action"` // add, check, batch-add
		Items  []string `json:"items"`
		Item   string   `json:"item"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	bf := perf.NewBloomFilter(10000, 0.01)

	switch req.Action {
	case "add":
		bf.AddString(req.Item)
		return c.Status(200).JSON(fiber.Map{"valid": true})
	case "check":
		found := bf.ContainsString(req.Item)
		return c.Status(200).JSON(fiber.Map{"valid": true, "found": found, "item": req.Item})
	case "batch-add":
		for _, item := range req.Items {
			bf.AddString(item)
		}
		return c.Status(200).JSON(fiber.Map{
			"valid":            true,
			"approximateCount": bf.ApproximateCount(),
		})
	default:
		return BadRequest(c, "Unknown action")
	}
}

// HandleTrieSearch 演示前缀树搜索
func (h *AIHandler) HandleTrieSearch(c fiber.Ctx) error {
	type request struct {
		Action   string            `json:"action"` // insert, search, prefix-search, batch
		Key      string            `json:"key"`
		Prefix   string            `json:"prefix"`
		Keywords map[string]string `json:"keywords"` // keyword -> category
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	trie := perf.NewTrie()

	switch req.Action {
	case "batch":
		for k, v := range req.Keywords {
			trie.Insert(k, v)
		}
		results := trie.SearchByPrefix(req.Prefix)
		return c.Status(200).JSON(fiber.Map{
			"valid":   true,
			"results": results,
			"count":   len(results),
			"size":    trie.Size(),
		})
	default:
		return BadRequest(c, "Use batch action for demo")
	}
}

// HandleGoroutinePoolDemo 演示协程池
func (h *AIHandler) HandleGoroutinePoolDemo(c fiber.Ctx) error {
	type request struct {
		Tasks int `json:"tasks"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		req.Tasks = 10
	}
	if req.Tasks <= 0 || req.Tasks > 1000 {
		req.Tasks = 10
	}

	pool := perf.NewGoroutinePool(8, 100)
	pool.Start()

	var counter int64
	var mu sync.Mutex

	for i := 0; i < req.Tasks; i++ {
		pool.SubmitAndWait(func() {
			mu.Lock()
			counter++
			mu.Unlock()
		})
	}

	pool.Stop()

	return c.Status(200).JSON(fiber.Map{
		"valid":    true,
		"tasks":    req.Tasks,
		"completed": counter,
		"pool":     pool.Stats(),
	})
}
