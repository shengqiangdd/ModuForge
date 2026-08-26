package perf

import (
	"sync"
	"testing"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache[string, int](3)

	// Test basic put/get
	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Errorf("Expected a=1, got %v", v)
	}

	// Test eviction
	cache.Put("d", 4) // Should evict "b"
	if _, ok := cache.Get("b"); ok {
		t.Error("Expected 'b' to be evicted")
	}

	// Test stats
	stats := cache.Stats()
	if stats.Size != 3 {
		t.Errorf("Expected size 3, got %d", stats.Size)
	}
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}

	// Test clear
	cache.Clear()
	if cache.Len() != 0 {
		t.Error("Expected empty cache after clear")
	}
}

func TestBloomFilter(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)

	// Test add and contains
	bf.AddString("hello")
	bf.AddString("world")

	if !bf.ContainsString("hello") {
		t.Error("Expected 'hello' to exist")
	}
	if !bf.ContainsString("world") {
		t.Error("Expected 'world' to exist")
	}
	if bf.ContainsString("missing") {
		t.Error("Expected 'missing' to not exist (or false positive)")
	}

	// Test approximate count
	count := bf.ApproximateCount()
	if count < 1.5 || count > 3 {
		t.Errorf("Expected count ~2, got %f", count)
	}
}

func TestTrie(t *testing.T) {
	trie := NewTrie()

	trie.Insert("hello", 1)
	trie.Insert("help", 2)
	trie.Insert("world", 3)

	// Test search
	if v, ok := trie.Search("hello"); !ok || v.(int) != 1 {
		t.Errorf("Expected hello=1, got %v", v)
	}

	// Test prefix
	if !trie.StartsWith("hel") {
		t.Error("Expected prefix 'hel' to exist")
	}
	if trie.StartsWith("xyz") {
		t.Error("Expected prefix 'xyz' to not exist")
	}

	// Test prefix search
	results := trie.SearchByPrefix("hel")
	if len(results) != 2 {
		t.Errorf("Expected 2 results for prefix 'hel', got %d", len(results))
	}

	// Test delete
	trie.Delete("hello")
	if _, ok := trie.Search("hello"); ok {
		t.Error("Expected 'hello' to be deleted")
	}

	if trie.Size() != 2 {
		t.Errorf("Expected size 2, got %d", trie.Size())
	}
}

func TestKeywordSearcher(t *testing.T) {
	searcher := NewKeywordSearcher()
	searcher.AddKeywords([]string{"sql", "injection", "xss", "vulnerability"}, "security")
	searcher.AddKeywords([]string{"function", "class", "method"}, "code")

	results := searcher.Search("This function has a sql injection vulnerability")
	if len(results["security"]) < 1 {
		t.Error("Expected security keywords to be found")
	}
	if len(results["code"]) < 1 {
		t.Error("Expected code keywords to be found")
	}
}

func TestGoroutinePool(t *testing.T) {
	pool := NewGoroutinePool(4, 10)
	pool.Start()

	counter := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		pool.SubmitAndWait(func() {
			mu.Lock()
			counter++
			mu.Unlock()
		})
	}

	pool.Stop()

	if counter != 100 {
		t.Errorf("Expected counter=100, got %d", counter)
	}

	stats := pool.Stats()
	if stats.IsRunning {
		t.Error("Expected pool to be stopped")
	}
}

func TestSyncPool(t *testing.T) {
	pool := NewSyncPool[[]byte](
		func() []byte { return make([]byte, 0, 1024) },
		func(buf []byte) { buf = buf[:0] },
	)

	buf := pool.Get()
	if cap(buf) != 1024 {
		t.Errorf("Expected capacity 1024, got %d", cap(buf))
	}

	// Get() increments size to 1
	if pool.Stats() != 1 {
		t.Errorf("Expected stats 1 after Get, got %d", pool.Stats())
	}

	buf = append(buf, "test"...)
	pool.Put(buf)

	// Put() decrements size back to 0
	if pool.Stats() != 0 {
		t.Errorf("Expected stats 0 after Put, got %d", pool.Stats())
	}
}

func TestDeduplicator(t *testing.T) {
	dedup := NewDeduplicator[int]()

	if !dedup.Add(1) {
		t.Error("Expected 1 to be new")
	}
	if dedup.Add(1) {
		t.Error("Expected 1 to be duplicate")
	}
	if dedup.Size() != 1 {
		t.Errorf("Expected size 1, got %d", dedup.Size())
	}
}

func TestBatchProcessor(t *testing.T) {
	processed := 0
	bp := NewBatchProcessor[int](3, func(batch []int) error {
		processed += len(batch)
		return nil
	})

	items := []int{1, 2, 3, 4, 5, 6, 7}
	bp.Process(items)

	if processed != 7 {
		t.Errorf("Expected 7 processed, got %d", processed)
	}
}

func TestLCP(t *testing.T) {
	result := LCP([]string{"flower", "flow", "flight"})
	if result != "fl" {
		t.Errorf("Expected 'fl', got '%s'", result)
	}

	result = LCP([]string{"dog", "racecar", "car"})
	if result != "" {
		t.Errorf("Expected '', got '%s'", result)
	}
}
