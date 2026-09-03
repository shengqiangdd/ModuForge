package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/moduforge/backend/internal/config"
)

// safeID sanitizes a project or build ID to prevent path traversal.
func safeID(id string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	return re.ReplaceAllString(id, "")
}

type BuildService struct {
	db  *sql.DB
	cfg *config.Config
	fr  *FileContentRepo // S3-first content access (optional)
}

func NewBuildService(db *sql.DB, cfg *config.Config) *BuildService {
	return &BuildService{db: db, cfg: cfg}
}

// SetFileContentRepo injects the S3-first file content repository.
func (s *BuildService) SetFileContentRepo(fr *FileContentRepo) {
	s.fr = fr
}

// readProjectFile returns a single file's content via FileContentRepo (S3 first),
// falling back to the raw DB content column when no repo is configured.
func (s *BuildService) readProjectFile(ctx context.Context, projectID, path string) (string, error) {
	if s.fr != nil {
		return s.fr.ReadOne(ctx, projectID, path)
	}
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(content,'') FROM project_files WHERE project_id=? AND path=?`, projectID, path,
	).Scan(&content)
	return content, err
}

// GetStoragePath returns the storage path for cache management.
func (s *BuildService) GetStoragePath() string {
	return s.cfg.StoragePath
}

// DB returns the underlying database connection for direct queries.
func (s *BuildService) DB() *sql.DB {
	return s.db
}

func (s *BuildService) getCacheKey(projectID, filesHash string) string {
	projectID = safeID(projectID)
	return filepath.Join(s.cfg.StoragePath, "build-cache", projectID, filesHash+".zip")
}

func (s *BuildService) computeFilesHash(ctx context.Context, projectID string) (string, error) {
	h := sha256.New()
	if s.fr != nil {
		// S3-first mode: use the cached per-file sha256 from DB metadata to
		// avoid downloading every file from S3 on each build. Only files
		// missing a hash (legacy rows) fall back to reading content.
		rows, err := s.db.QueryContext(ctx,
			`SELECT path, COALESCE(sha256,'') FROM project_files WHERE project_id=? ORDER BY path`, projectID)
		if err != nil {
			return "", err
		}
		defer rows.Close()

		legacyFetch := 0
		for rows.Next() {
			var path, sha string
			if err := rows.Scan(&path, &sha); err != nil {
				continue
			}
			if sha == "" {
				// No cached hash — fetch content (S3 first, DB fallback).
				content, err := s.fr.ReadOne(ctx, projectID, path)
				if err != nil {
					continue
				}
				h.Write([]byte(path))
				h.Write([]byte(content))
				legacyFetch++
				continue
			}
			h.Write([]byte(path))
			h.Write([]byte(sha))
		}
		if legacyFetch > 0 {
			log.Printf("[Build] computeFilesHash: %d legacy rows fetched via content (missing sha256)", legacyFetch)
		}
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT path, content FROM project_files WHERE project_id=? ORDER BY path`, projectID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		h.Write([]byte(path))
		h.Write([]byte(content))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// buildTimeout is the maximum time a build can run before being killed.
const buildTimeout = 15 * time.Minute

// ── SSE Build Progress ────────────────────────────────────────────────

var (
	// sseChannels maps projectID → list of subscriber channels.
	sseChannels = make(map[string][]chan string)
	sseMu       sync.RWMutex
)

// RegisterSSEChannel opens a buffered channel for SSE subscribers.
func RegisterSSEChannel(projectID string) chan string {
	ch := make(chan string, 20)
	sseMu.Lock()
	defer sseMu.Unlock()
	sseChannels[projectID] = append(sseChannels[projectID], ch)
	return ch
}

// UnregisterSSEChannel closes and removes a subscriber channel.
func UnregisterSSEChannel(projectID string, ch chan string) {
	sseMu.Lock()
	defer sseMu.Unlock()
	out := make([]chan string, 0, len(sseChannels[projectID]))
	for _, c := range sseChannels[projectID] {
		if c != ch {
			out = append(out, c)
		} else {
			close(c)
		}
	}
	sseChannels[projectID] = out
}

// BroadcastProgress sends an SSE event to every subscriber of projectID.
func BroadcastProgress(projectID string, phase string, detail string) {
	data := fmt.Sprintf(`{"phase":"%s","detail":"%s"}`, phase, detail)
	sseMu.RLock()
	defer sseMu.RUnlock()
	for _, ch := range sseChannels[projectID] {
		select {
		case ch <- data:
		default:
			// drop if subscriber is slow
		}
	}
}
