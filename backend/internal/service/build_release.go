package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/saferead"
)

// ReleaseInfo contains information about a GitHub Release
type ReleaseInfo struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	HTMLURL    string `json:"html_url"`
	UploadURL  string `json:"upload_url"`
}

// PublishToRelease publishes a build artifact to GitHub Release
func (s *BuildService) PublishToRelease(ctx context.Context, projectID, buildID, token string) (*ReleaseInfo, error) {
	// Get build task
	task, err := s.Get(ctx, buildID)
	if err != nil {
		return nil, fmt.Errorf("build not found: %w", err)
	}

	// Verify build belongs to project
	if task.ProjectID != projectID {
		return nil, fmt.Errorf("build does not belong to project")
	}

	// Check if build succeeded
	if task.Status != domain.BuildSuccess {
		return nil, fmt.Errorf("build did not succeed, status: %s", task.Status)
	}

	// Get artifact path
	if task.ArtifactPath == nil || *task.ArtifactPath == "" {
		return nil, fmt.Errorf("no artifact path found")
	}

	// Get project info for module name
	var projectName string
	err = s.db.QueryRowContext(ctx,
		`SELECT name FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&projectName)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// Get version from module.prop if exists
	version := "1.0.0"
	modulePropContent, err := s.readProjectFile(ctx, projectID, "module.prop")
	if err == nil {
		// Parse version from module.prop
		for _, line := range strings.Split(modulePropContent, "\n") {
			if strings.HasPrefix(line, "version=") {
				version = strings.TrimPrefix(line, "version=")
				version = strings.TrimSpace(version)
				break
			}
		}
	}

	// Create release tag
	tagName := fmt.Sprintf("v%s", version)
	releaseName := fmt.Sprintf("%s v%s", projectName, version)
	releaseBody := fmt.Sprintf("## %s v%s\n\nAutomated release from ModuForge build.\n\n**Build ID:** %s\n**Architecture:** arm64\n**Build Time:** %s",
		projectName, version, buildID, task.CreatedAt)

	// GitHub API call to create release
	releaseInfo, err := s.createGitHubRelease(ctx, token, tagName, releaseName, releaseBody, *task.ArtifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub release: %w", err)
	}

	return releaseInfo, nil
}

// createGitHubRelease makes the actual GitHub API call
func (s *BuildService) createGitHubRelease(ctx context.Context, token, tagName, name, body, artifactPath string) (*ReleaseInfo, error) {
	// Parse git remote URL to get owner/repo
	owner, repo, err := s.parseGitRemote()
	if err != nil {
		return nil, fmt.Errorf("failed to parse git remote: %w", err)
	}

	// Create release payload
	releasePayload := map[string]interface{}{
		"tag_name":   tagName,
		"name":       name,
		"body":       body,
		"draft":      false,
		"prerelease": false,
	}

	payloadBytes, err := json.Marshal(releasePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal release payload: %w", err)
	}

	// Create release via GitHub API
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "POST", releaseURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, err := saferead.SafeReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read GitHub API error response: %w", err)
		}
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var releaseResp struct {
		ID       int64  `json:"id"`
		TagName  string `json:"tag_name"`
		Name     string `json:"name"`
		HTMLURL  string `json:"html_url"`
		UploadURL string `json:"upload_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&releaseResp); err != nil {
		return nil, fmt.Errorf("failed to decode release response: %w", err)
	}

	// Upload artifact if it exists
	if artifactPath != "" {
		assetName := filepath.Base(artifactPath)
		if err := s.uploadReleaseAsset(ctx, token, releaseResp.UploadURL, assetName, artifactPath); err != nil {
			// Log warning but don't fail - release was created
			log.Printf("Warning: failed to upload artifact: %v", err)
		}
	}

	return &ReleaseInfo{
		TagName:   releaseResp.TagName,
		Name:      releaseResp.Name,
		Body:      body,
		Draft:     false,
		Prerelease: false,
		HTMLURL:   releaseResp.HTMLURL,
		UploadURL: releaseResp.UploadURL,
	}, nil
}

// parseGitRemote parses the git remote URL to extract owner and repo
func (s *BuildService) parseGitRemote() (owner, repo string, err error) {
	// Try to get remote URL from git command
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get git remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(output))

	// Parse SSH URL: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		// Remove git@ prefix and .git suffix
		remoteURL = strings.TrimPrefix(remoteURL, "git@")
		remoteURL = strings.TrimSuffix(remoteURL, ".git")
		// Replace : with /
		remoteURL = strings.Replace(remoteURL, ":", "/", 1)
	} else if strings.HasPrefix(remoteURL, "https://") {
		// Parse HTTPS URL: https://github.com/owner/repo.git
		remoteURL = strings.TrimPrefix(remoteURL, "https://")
		remoteURL = strings.TrimSuffix(remoteURL, ".git")
		// Remove any authentication info
		if idx := strings.Index(remoteURL, "@"); idx != -1 {
			remoteURL = remoteURL[idx+1:]
		}
	} else {
		return "", "", fmt.Errorf("unsupported remote URL format: %s", remoteURL)
	}

	// Split into owner/repo
	parts := strings.Split(remoteURL, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid remote URL format: %s", remoteURL)
	}

	// Handle case where github.com might be included
	if len(parts) >= 3 && parts[0] == "github.com" {
		owner = parts[1]
		repo = parts[2]
	} else if len(parts) == 2 {
		owner = parts[0]
		repo = parts[1]
	} else {
		return "", "", fmt.Errorf("cannot parse owner/repo from: %s", remoteURL)
	}

	return owner, repo, nil
}

// uploadReleaseAsset uploads a file to a GitHub release
func (s *BuildService) uploadReleaseAsset(ctx context.Context, token, uploadURL, fileName, filePath string) error {
	// Read the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Replace {name} placeholder in upload URL
	uploadURL = strings.Replace(uploadURL, "{name}", fileName, 1)
	uploadURL = strings.Replace(uploadURL, "{label}", "", 1)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, file)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Execute request
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, err := saferead.SafeReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read GitHub API error response: %w", err)
		}
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
