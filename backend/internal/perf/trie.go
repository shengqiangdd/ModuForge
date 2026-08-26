package perf

import (
	"strings"
	"sync"
)

// Trie 前缀树 - 用于快速关键字匹配
type Trie struct {
	children map[rune]*Trie
	isEnd    bool
	data     interface{}
	lock     sync.RWMutex
}

// NewTrie 创建前缀树
func NewTrie() *Trie {
	return &Trie{
		children: make(map[rune]*Trie),
	}
}

// Insert 插入关键字
func (t *Trie) Insert(key string, data interface{}) {
	t.lock.Lock()
	defer t.lock.Unlock()

	node := t
	for _, ch := range key {
		if _, ok := node.children[ch]; !ok {
			node.children[ch] = &Trie{
				children: make(map[rune]*Trie),
			}
		}
		node = node.children[ch]
	}
	node.isEnd = true
	node.data = data
}

// Search 搜索完全匹配
func (t *Trie) Search(key string) (interface{}, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	node := t
	for _, ch := range key {
		if _, ok := node.children[ch]; !ok {
			return nil, false
		}
		node = node.children[ch]
	}
	if node.isEnd {
		return node.data, true
	}
	return nil, false
}

// StartsWith 检查是否有指定前缀
func (t *Trie) StartsWith(prefix string) bool {
	t.lock.RLock()
	defer t.lock.RUnlock()

	node := t
	for _, ch := range prefix {
		if _, ok := node.children[ch]; !ok {
			return false
		}
		node = node.children[ch]
	}
	return true
}

// SearchByPrefix 按前缀搜索所有匹配项
func (t *Trie) SearchByPrefix(prefix string) []interface{} {
	t.lock.RLock()
	defer t.lock.RUnlock()

	node := t
	for _, ch := range prefix {
		if _, ok := node.children[ch]; !ok {
			return nil
		}
		node = node.children[ch]
	}

	results := make([]interface{}, 0)
	node.collectAll(&results)
	return results
}

// collectAll 收集所有子节点数据
func (t *Trie) collectAll(results *[]interface{}) {
	if t.isEnd {
		*results = append(*results, t.data)
	}
	for _, child := range t.children {
		child.collectAll(results)
	}
}

// Delete 删除关键字
func (t *Trie) Delete(key string) bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.deleteRecursive(t, key, 0)
}

func (t *Trie) deleteRecursive(node *Trie, key string, depth int) bool {
	if depth == len(key) {
		if !node.isEnd {
			return false
		}
		node.isEnd = false
		node.data = nil
		return len(node.children) == 0
	}

	ch := rune(key[depth])
	child, ok := node.children[ch]
	if !ok {
		return false
	}

	shouldDelete := t.deleteRecursive(child, key, depth+1)
	if shouldDelete {
		delete(node.children, ch)
		return !node.isEnd && len(node.children) == 0
	}
	return false
}

// Size 返回关键字数量
func (t *Trie) Size() int {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.countNodes(t)
}

func (t *Trie) countNodes(node *Trie) int {
	count := 0
	if node.isEnd {
		count++
	}
	for _, child := range node.children {
		count += t.countNodes(child)
	}
	return count
}

// KeywordSearcher 关键字搜索引擎（基于Trie）
type KeywordSearcher struct {
	trie *Trie
}

// NewKeywordSearcher 创建关键字搜索引擎
func NewKeywordSearcher() *KeywordSearcher {
	return &KeywordSearcher{trie: NewTrie()}
}

// AddKeywords 添加关键字列表
func (s *KeywordSearcher) AddKeywords(keywords []string, category string) {
	for _, kw := range keywords {
		s.trie.Insert(strings.ToLower(kw), category)
	}
}

// Search 搜索文本中出现的关键字
func (s *KeywordSearcher) Search(text string) map[string][]string {
	textLower := strings.ToLower(text)
	results := make(map[string][]string)

	for _, word := range strings.Fields(textLower) {
		word = strings.Trim(word, ".,;:!?\"'()[]{}")
		if cat, ok := s.trie.Search(word); ok {
			category := cat.(string)
			results[category] = append(results[category], word)
		}
	}
	return results
}
