package code

import (
	"fmt"
	"strings"
	"time"
)

// SnippetLibrary 代码片段库
type SnippetLibrary struct {
	snippets map[string]*CodeSnippet
}

// NewSnippetLibrary 创建片段库
func NewSnippetLibrary() *SnippetLibrary {
	lib := &SnippetLibrary{
		snippets: make(map[string]*CodeSnippet),
	}
	lib.loadDefaultSnippets()
	return lib
}

// CodeSnippet 代码片段
type CodeSnippet struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Language    string    `json:"language"`
	Category    string    `json:"category"`
	Code        string    `json:"code"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UsageCount  int       `json:"usage_count"`
}

// ListSnippets 列出所有片段
func (l *SnippetLibrary) ListSnippets() []*CodeSnippet {
	snippets := make([]*CodeSnippet, 0)
	for _, s := range l.snippets {
		snippets = append(snippets, s)
	}
	return snippets
}

// GetSnippet 获取片段
func (l *SnippetLibrary) GetSnippet(id string) (*CodeSnippet, bool) {
	s, ok := l.snippets[id]
	return s, ok
}

// SearchSnippets 搜索片段
func (l *SnippetLibrary) SearchSnippets(query string, language string) []*CodeSnippet {
	results := make([]*CodeSnippet, 0)
	for _, s := range l.snippets {
		if language != "" && s.Language != language {
			continue
		}
		if query == "" || containsIgnoreCase(s.Title, query) || containsIgnoreCase(s.Description, query) || containsIgnoreCase(s.Code, query) {
			results = append(results, s)
		}
	}
	return results
}

// CreateSnippet 创建片段
func (l *SnippetLibrary) CreateSnippet(s *CodeSnippet) {
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	l.snippets[s.ID] = s
}

// UseSnippet 使用片段（增加计数）
func (l *SnippetLibrary) UseSnippet(id string) error {
	s, ok := l.snippets[id]
	if !ok {
		return fmt.Errorf("snippet not found: %s", id)
	}
	s.UsageCount++
	s.UpdatedAt = time.Now()
	return nil
}

func containsIgnoreCase(s, substr string) bool {
	if substr == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func (l *SnippetLibrary) loadDefaultSnippets() {
	l.CreateSnippet(&CodeSnippet{
		ID:          "go-http-server",
		Title:       "HTTP 服务器",
		Description: "创建一个简单的 HTTP 服务器",
		Language:    "go",
		Category:    "web",
		Code: `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})

	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}`,
		Tags: []string{"http", "server", "web"},
	})

	l.CreateSnippet(&CodeSnippet{
		ID:          "go-json-marshal",
		Title:       "JSON 序列化",
		Description: "将结构体转换为 JSON",
		Language:    "go",
		Category:    "data",
		Code: `package main

import (
	"encoding/json"
	"fmt"
)

type User struct {
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
	Age   int    ` + "`json:\"age\"`" + `
}

func main() {
	user := User{
		Name:  "John",
		Email: "john@example.com",
		Age:   30,
	}

	jsonData, err := json.Marshal(user)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(string(jsonData))
}`,
		Tags: []string{"json", "marshal", "struct"},
	})

	l.CreateSnippet(&CodeSnippet{
		ID:          "python-fastapi",
		Title:       "FastAPI 服务器",
		Description: "创建 FastAPI Web 服务器",
		Language:    "python",
		Category:    "web",
		Code: `from fastapi import FastAPI

app = FastAPI()

@app.get("/")
def read_root():
    return {"Hello": "World"}

@app.get("/items/{item_id}")
def read_item(item_id: int, q: str = None):
    return {"item_id": item_id, "q": q}`,
		Tags: []string{"fastapi", "web", "api"},
	})

	l.CreateSnippet(&CodeSnippet{
		ID:          "python-file-read",
		Title:       "文件读取",
		Description: "安全地读取文件内容",
		Language:    "python",
		Category:    "io",
		Code: `def read_file(filepath: str) -> str:
    """安全地读取文件内容"""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return f.read()
    except FileNotFoundError:
        print(f"File not found: {filepath}")
        return ""
    except Exception as e:
        print(f"Error reading file: {e}")
        return ""`,
		Tags: []string{"file", "read", "io"},
	})

	l.CreateSnippet(&CodeSnippet{
		ID:          "js-fetch-api",
		Title:       "Fetch API 调用",
		Description: "使用 Fetch API 调用 REST API",
		Language:    "javascript",
		Category:    "web",
		Code: `async function fetchData(url) {
  try {
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error('Network response was not ok');
    }
    const data = await response.json();
    return data;
  } catch (error) {
    console.error('Fetch error:', error);
    throw error;
  }
}`,
		Tags: []string{"fetch", "api", "async"},
	})

	l.CreateSnippet(&CodeSnippet{
		ID:          "js-debounce",
		Title:       "防抖函数",
		Description: "实现防抖功能",
		Language:    "javascript",
		Category:    "utility",
		Code: `function debounce(func, wait) {
  let timeout;
  return function executedFunction(...args) {
    const later = () => {
      clearTimeout(timeout);
      func(...args);
    };
    clearTimeout(timeout);
    timeout = setTimeout(later, wait);
  };
}

// 使用示例
const debouncedSearch = debounce((query) => {
  console.log('Searching:', query);
}, 300);`,
		Tags: []string{"debounce", "utility", "performance"},
	})
}
