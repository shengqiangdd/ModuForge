package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RepoInfo struct {
	Owner     string    `json:"owner"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Stars     int       `json:"stars"`
	Topics    []string  `json:"topics"`
	License   string    `json:"license"`
	FetchedAt time.Time `json:"fetched_at"`
}

type RepoService struct {
	client *http.Client
}

func NewRepoService() *RepoService {
	return &RepoService{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchRepoInfo 从 GitHub API 获取仓库信息
func (s *RepoService) FetchRepoInfo(ctx context.Context, repoURL string) (*RepoInfo, error) {
	// 提取 owner/name
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repo URL: %s", repoURL)
	}
	owner, name := parts[len(parts)-2], parts[len(parts)-1]

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, name)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	// 用 map 解析 response（避免依赖外部库/生成结构体）
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	info := &RepoInfo{
		Owner:     owner,
		Name:      name,
		URL:       repoURL,
		FetchedAt: time.Now(),
	}

	if stars, ok := result["stargazers_count"].(float64); ok {
		info.Stars = int(stars)
	}
	if license, ok := result["license"].(map[string]interface{}); ok {
		if spdx, ok := license["spdx_id"].(string); ok {
			info.License = spdx
		}
	}
	if topics, ok := result["topics"].([]interface{}); ok {
		for _, t := range topics {
			if topic, ok := t.(string); ok {
				info.Topics = append(info.Topics, topic)
			}
		}
	}

	return info, nil
}

// FetchRepoFiles 获取仓库文件列表（用于参考改造）
func (s *RepoService) FetchRepoFiles(ctx context.Context, repoURL, path string) ([]map[string]interface{}, error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repo URL: %s", repoURL)
	}
	owner, name := parts[len(parts)-2], parts[len(parts)-1]

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, name, path)
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// 可能是单文件
		var single map[string]interface{}
		if err2 := json.Unmarshal(body, &single); err2 != nil {
			return nil, err
		}
		return []map[string]interface{}{single}, nil
	}
	return result, nil
}
