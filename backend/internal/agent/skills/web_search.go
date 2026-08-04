package skills

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type WebSearchSkill struct {
	client *http.Client
}

func NewWebSearchSkill() *WebSearchSkill {
	return &WebSearchSkill{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *WebSearchSkill) Name() string {
	return "web_search"
}

func (s *WebSearchSkill) Description() string {
	return "Search the web for latest information. Input: {\"query\": \"...\"}"
}

func (s *WebSearchSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Try engines in order: DuckDuckGo -> Bing -> Google
	type searchFunc func(ctx context.Context, query string) (string, error)
	engines := []searchFunc{
		s.searchDuckDuckGo,
		s.searchBing,
		s.searchGoogle,
	}

	for _, engine := range engines {
		result, err := engine(ctx, query)
		if err == nil && result != "" && !strings.Contains(result, "未找到") {
			return result, nil
		}
	}
	return "搜索功能暂时不可用，请稍后重试", nil
}

func (s *WebSearchSkill) searchDuckDuckGo(ctx context.Context, query string) (string, error) {
	u := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ModuForgeAgent/1.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return extractDuckDuckGoResults(string(body)), nil
}

func extractDuckDuckGoResults(html string) string {
	var results []string
	lines := strings.Split(html, "\n")
	inResult := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `class="result__a"`) {
			inResult = true
			start := strings.Index(trimmed, ">")
			end := strings.LastIndex(trimmed, "</a>")
			if start >= 0 && end > start {
				title := trimmed[start+1 : end]
				title = stripHTMLTags(title)
				results = append(results, "• "+title)
			}
			continue
		}
		if inResult && strings.Contains(trimmed, `class="result__snippet"`) {
			start := strings.Index(trimmed, ">")
			end := strings.LastIndex(trimmed, "</a>")
			if start < 0 || end <= start {
				end = strings.LastIndex(trimmed, "</span>")
			}
			if start >= 0 && end > start {
				snippet := trimmed[start+1 : end]
				snippet = stripHTMLTags(snippet)
				if len(results) > 0 {
					results[len(results)-1] += "\n  " + snippet
				}
			}
			inResult = false
		}
	}

	if len(results) == 0 {
		return "未找到相关搜索结果"
	}

	if len(results) > 5 {
		results = results[:5]
	}
	return strings.Join(results, "\n")
}

func (s *WebSearchSkill) searchBing(ctx context.Context, query string) (string, error) {
	u := fmt.Sprintf("https://www.bing.com/search?q=%s&setlang=en", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return extractBingResults(string(body)), nil
}

func extractBingResults(html string) string {
	var results []string
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Bing wraps result titles in <li class="b_algo"><h2><a ...>Title</a></h2></li>
		if strings.Contains(trimmed, `class="b_algo"`) || strings.Contains(trimmed, `<h2><a`) {
			start := strings.Index(trimmed, ">")
			end := strings.Index(trimmed, "</a>")
			if start >= 0 && end > start {
				title := trimmed[start+1 : end]
				title = stripHTMLTags(title)
				title = strings.TrimSpace(title)
				if title != "" {
					results = append(results, "• "+title)
				}
			}
		}
	}

	if len(results) == 0 {
		return "未找到相关搜索结果"
	}
	if len(results) > 5 {
		results = results[:5]
	}
	return strings.Join(results, "\n")
}

func (s *WebSearchSkill) searchGoogle(ctx context.Context, query string) (string, error) {
	u := fmt.Sprintf("https://www.google.com/search?q=%s&hl=en", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return extractGoogleResults(string(body)), nil
}

func extractGoogleResults(html string) string {
	var results []string
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Google wraps result titles in <h3> tags within <a> links
		if strings.Contains(trimmed, `<h3`) {
			start := strings.Index(trimmed, ">")
			end := strings.Index(trimmed, "</h3>")
			if start >= 0 && end > start {
				title := trimmed[start+1 : end]
				title = stripHTMLTags(title)
				title = strings.TrimSpace(title)
				if title != "" {
					results = append(results, "• "+title)
				}
			}
		}
	}

	if len(results) == 0 {
		return "未找到相关搜索结果"
	}
	if len(results) > 5 {
		results = results[:5]
	}
	return strings.Join(results, "\n")
}

func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

func (s *WebSearchSkill) Metadata() SkillMeta {
	return SkillMeta{
		ReadOnly:  true,
		Essential: true,
		NeedsDB:   false,
		NeedsLLM:  false,
	}
}
