package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
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

// FetchFileContent 拉取仓库中单个文件并解码为 UTF-8 文本（用于智能参考喂给 AI）。
func (s *RepoService) FetchFileContent(ctx context.Context, repoURL, filePath string) (map[string]interface{}, error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repo URL: %s", repoURL)
	}
	owner, name := parts[len(parts)-2], parts[len(parts)-1]

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, name, filePath)
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

	var single map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&single); err != nil {
		return nil, err
	}

	content, _ := single["content"].(string)
	encoding, _ := single["encoding"].(string)
	text := content
	if encoding == "base64" && content != "" {
		if dec, err := base64.StdEncoding.DecodeString(content); err == nil {
			text = string(dec)
		}
	}

	return map[string]interface{}{
		"name":     single["name"],
		"path":     single["path"],
		"size":     single["size"],
		"content":  text,
		"encoding": encoding,
	}, nil
}

// magiskKeyFiles 是 Magisk 模块的核心关键文件，按优先级排列（越靠前越重要）。
var magiskKeyFiles = []string{
	"module.prop",
	"config.sh",
	"service.sh",
	"post-fs-data.sh",
	"customize.sh",
	"install.sh",
	"update.json",
	"uninstall.sh",
	"sepolicy.rule",
	"README.md",
}

// smartExtPriority 记录对 AI 改造最有参考价值的扩展名及其优先级（越大越重要）。
var smartExtPriority = map[string]int{
	".sh":  5,
	".prop": 4,
	".json": 3,
	".conf": 3,
	".toml": 3,
	".yaml": 3,
	".yml":  3,
	".md":   3,
	".c":    2,
	".h":    2,
	".go":   2,
	".rs":   2,
	".py":   2,
	".js":   2,
	".ts":   2,
}

// maxSmartFiles 智能选择单次喂给 AI 的关键文件数量上限。
const maxSmartFiles = 8

// SmartSelectFiles 从仓库文件列表中智能挑选对 AI 改造最有价值的文件。
// 优先 Magisk 模块关键文件（module.prop/config.sh/service.sh 等），
// 再按扩展名优先级补齐到上限；目录与超大文件自动剔除。
func (s *RepoService) SmartSelectFiles(files []map[string]interface{}) ([]string, error) {
	var keyHits []string          // 命中 Magisk 关键文件名
	type extPick struct {
		path     string
		priority int
		depth    int
	}
	var extHits []extPick

	for _, f := range files {
		fp, _ := f["path"].(string)
		ftype, _ := f["type"].(string)
		if fp == "" || ftype == "dir" {
			continue
		}
		base := path.Base(fp)
		ext := strings.ToLower(path.Ext(fp))
		size, _ := f["size"].(float64)

		isKey := false
		for _, k := range magiskKeyFiles {
			if strings.EqualFold(base, k) {
				keyHits = append(keyHits, fp)
				isKey = true
				break
			}
		}
		if isKey {
			continue
		}
		if prio, ok := smartExtPriority[ext]; ok && size < 200*1024 {
			extHits = append(extHits, extPick{fp, prio, strings.Count(fp, "/")})
		}
	}

	// 扩展名命中排序：优先级高优先 → 路径浅（更可能是核心入口）优先
	sort.SliceStable(extHits, func(i, j int) bool {
		if extHits[i].priority != extHits[j].priority {
			return extHits[i].priority > extHits[j].priority
		}
		return extHits[i].depth < extHits[j].depth
	})

	// 合并：关键文件优先，再按扩展名排序补齐到上限
	selected := make([]string, 0, len(keyHits)+len(extHits))
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" || seen[p] || len(selected) >= maxSmartFiles {
			return
		}
		selected = append(selected, p)
		seen[p] = true
	}
	for _, k := range keyHits {
		add(k)
	}
	for _, p := range extHits {
		add(p.path)
	}

	return selected, nil
}

