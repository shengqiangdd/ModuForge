package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type UpdateService struct {
	client     *http.Client
	db         *sql.DB
	checkerURL string
}

func NewUpdateService(db *sql.DB) *UpdateService {
	return &UpdateService{
		client:     &http.Client{Timeout: 15 * time.Second},
		db:         db,
		checkerURL: "https://api.github.com",
	}
}

type UpdateInfo struct {
	ModuleID    string `json:"module_id"`
	CurrentVer  string `json:"current_version"`
	LatestVer   string `json:"latest_version"`
	HasUpdate   bool   `json:"has_update"`
	DownloadURL string `json:"download_url,omitempty"`
	ReleaseNote string `json:"release_note,omitempty"`
}

type RepoRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
	PublishedAt string `json:"published_at"`
}

// CheckModuleUpdate checks if a module has an update available
func (s *UpdateService) CheckModuleUpdate(ctx context.Context, moduleID, currentVersion, repoURL string) (*UpdateInfo, error) {
	info := &UpdateInfo{
		ModuleID:   moduleID,
		CurrentVer: currentVersion,
	}

	if repoURL == "" {
		info.HasUpdate = false
		return info, nil
	}

	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return info, fmt.Errorf("invalid repo URL")
	}
	owner, name := parts[len(parts)-2], parts[len(parts)-1]

	apiURL := fmt.Sprintf("%s/repos/%s/%s/releases/latest", s.checkerURL, owner, name)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.client.Do(req)
	if err != nil {
		info.HasUpdate = false
		return info, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		info.HasUpdate = false
		return info, nil
	}

	var release RepoRelease
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &release); err != nil {
		return info, nil
	}

	info.LatestVer = release.TagName
	info.ReleaseNote = release.Body

	if info.LatestVer != info.CurrentVer {
		info.HasUpdate = true
		for _, asset := range release.Assets {
			if strings.HasSuffix(asset.Name, ".zip") {
				info.DownloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	return info, nil
}

// CheckAllModuleUpdates checks updates for multiple modules
func (s *UpdateService) CheckAllModuleUpdates(ctx context.Context, modules []struct {
	ID, Version, RepoURL string
}) []UpdateInfo {
	results := make([]UpdateInfo, len(modules))
	for i, m := range modules {
		info, _ := s.CheckModuleUpdate(ctx, m.ID, m.Version, m.RepoURL)
		results[i] = *info
	}
	return results
}
