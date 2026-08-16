package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)


// ─── Shell & Exec + File Operations ───

// ─── Shell & Exec ───

func sanitizePath(p string) string {
	p = strings.ReplaceAll(p, "'", "")
	p = strings.ReplaceAll(p, "\"", "")
	p = strings.ReplaceAll(p, ";", "")
	p = strings.ReplaceAll(p, "|", "")
	p = strings.ReplaceAll(p, "&", "")
	p = strings.ReplaceAll(p, "$", "")
	p = strings.ReplaceAll(p, "`", "")
	p = strings.ReplaceAll(p, "(", "")
	p = strings.ReplaceAll(p, ")", "")
	p = strings.ReplaceAll(p, "{", "")
	p = strings.ReplaceAll(p, "}", "")
	p = strings.ReplaceAll(p, "\n", "")
	p = strings.ReplaceAll(p, "\r", "")
	// Remove path traversal components
	var cleaned []string
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "." {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}

func (s *ADBService) RunShell(ctx context.Context, serial, shellCmd string) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", shellCmd)
	out, err := s.run(ctx, args...)
	if err != nil {
		return "", err
	}
	// Truncate very long output
	if len(out) > 50000 {
		out = out[:50000] + "\n... (truncated)"
	}
	return out, nil
}

func (s *ADBService) RunExec(ctx context.Context, serial, shellCmd string) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	// Parse the command to determine if it's a host-level or shell command
	trimmed := strings.TrimSpace(shellCmd)
	hostCmds := []string{"connect", "disconnect", "devices", "start-server", "kill-server", "forward", "reverse", "usb", "tcpip", "wait-for-device", "get-state", "get-serialno", "remount", "unlock", "oem"}
	isHostCmd := false
	for _, hc := range hostCmds {
		if strings.HasPrefix(trimmed, hc+" ") || trimmed == hc {
			isHostCmd = true
			break
		}
	}
	if isHostCmd {
		args = append(args, strings.Fields(trimmed)...)
	} else {
		args = append(args, "shell", shellCmd)
	}
	out, err := s.RunWithTimeout(ctx, 30*time.Second, args...)
	if err != nil {
		return "", err
	}
	return out, nil
}

// ─── File Management ───

type cacheEntry struct {
	files     []FileInfo
	expiresAt time.Time
}

var (
	fileCache            = make(map[string]cacheEntry)
	fileCacheMu          sync.RWMutex
	fileCacheCleanupOnce sync.Once
)

// startFileCacheCleanup launches a background ticker that removes expired
// fileCache entries. Without this, the map grows forever (one key per
// device:path visited), leaking memory over long-running sessions.
func startFileCacheCleanup() {
	fileCacheCleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				fileCacheMu.Lock()
				now := time.Now()
				for k, e := range fileCache {
					if now.After(e.expiresAt) {
						delete(fileCache, k)
					}
				}
				fileCacheMu.Unlock()
			}
		}()
	})
}

func (s *ADBService) ListFiles(ctx context.Context, serial, remotePath string) ([]FileInfo, error) {
	if remotePath == "" {
		remotePath = "/sdcard/"
	}
	remotePath = sanitizePath(remotePath)
	if remotePath != "/" && !strings.HasSuffix(remotePath, "/") {
		remotePath += "/"
	}

	cacheKey := serial + ":" + remotePath
	fileCacheMu.RLock()
	if entry, ok := fileCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		fileCacheMu.RUnlock()
		return entry.files, nil
	}
	fileCacheMu.RUnlock()

	out, err := s.RunShell(ctx, serial, "ls -la "+remotePath+" 2>/dev/null")
	if err != nil || strings.TrimSpace(out) == "" {
		out, err = s.RunShell(ctx, serial, "su -c 'ls -la "+remotePath+"' 2>/dev/null")
	}
	if err != nil {
		return nil, fmt.Errorf("list files failed: %w", err)
	}
	var files []FileInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 7 {
			continue
		}
		fi := FileInfo{
			Mode:  parts[0],
			Name:  sanitizePath(parts[len(parts)-1]),
			Path:  strings.TrimRight(remotePath, "/") + "/" + sanitizePath(parts[len(parts)-1]),
			IsDir: strings.HasPrefix(parts[0], "d"),
		}
		if !fi.IsDir && len(parts) >= 5 {
			fmt.Sscanf(parts[4], "%d", &fi.Size)
		}
		if fi.Name == "." || fi.Name == ".." {
			continue
		}
		files = append(files, fi)
	}

	// Sort: directories first, then by name (case-insensitive)
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	fileCacheMu.Lock()
	fileCache[cacheKey] = cacheEntry{files: files, expiresAt: time.Now().Add(30 * time.Second)}
	fileCacheMu.Unlock()
	startFileCacheCleanup()

	return files, nil
}

func (s *ADBService) PullFile(ctx context.Context, serial, remotePath, localPath string) (string, error) {
	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}
	out, err := s.run(ctx, "-s", serial, "pull", remotePath, localPath)
	if err != nil {
		return "", fmt.Errorf("pull failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) PushFile(ctx context.Context, serial, localPath, remotePath string) (string, error) {
	if remotePath == "" {
		remotePath = "/sdcard/"
	}
	out, err := s.run(ctx, "-s", serial, "push", localPath, remotePath)
	if err != nil {
		return "", fmt.Errorf("push failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) DeleteFile(ctx context.Context, serial, remotePath string) (string, error) {
	remotePath = sanitizePath(remotePath)
	out, err := s.RunShell(ctx, serial, "rm -rf "+remotePath+" && echo 'deleted'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) MakeDir(ctx context.Context, serial, remotePath string) (string, error) {
	remotePath = sanitizePath(remotePath)
	out, err := s.RunShell(ctx, serial, "mkdir -p "+remotePath+" && echo 'created'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) RenameFile(ctx context.Context, serial, oldPath, newPath string) (string, error) {
	oldPath = sanitizePath(oldPath)
	newPath = sanitizePath(newPath)
	out, err := s.RunShell(ctx, serial, fmt.Sprintf("mv %q %q && echo 'renamed'", oldPath, newPath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) GetFileSize(ctx context.Context, serial, remotePath string) (string, error) {
	remotePath = sanitizePath(remotePath)
	out, err := s.RunShell(ctx, serial, "du -sh "+remotePath+" 2>/dev/null | awk '{print $1}'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) ReadFile(ctx context.Context, serial, remotePath string) (string, error) {
	remotePath = sanitizePath(remotePath)

	// Check file size first (avoids pulling huge files)
	sizeOut, _ := s.RunShell(ctx, serial, "wc -c < '"+remotePath+"' 2>/dev/null")
	fileSize := int64(0)
	if sizeStr := strings.TrimSpace(sizeOut); sizeStr != "" {
		if sz, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			fileSize = sz
			if fileSize > 1024*100 {
				return "(file too large to preview: " + fmt.Sprintf("%.1f", float64(fileSize)/1024) + " KB)", nil
			}
		}
	}

	// Detect binary files via `file` command (broad pattern matching)
	fileOut, _ := s.RunShell(ctx, serial, "file '"+remotePath+"' 2>/dev/null")
	fileLower := strings.ToLower(fileOut)
	binaryKeywords := []string{
		"binary", "image", "executable", "archive", "compressed",
		"data", "media", "audio", "video", "font", "pdf",
		"elf", "mach-o", "java archive", "zip", "gzip",
	}
	for _, kw := range binaryKeywords {
		if strings.Contains(fileLower, kw) {
			return "(binary file, can't preview)", nil
		}
	}

	// Fallback: check for null bytes in first 8KB (definitive binary indicator)
	// Uses od instead of grep -P for Android compatibility
	if fileSize > 0 {
		nullCheck, _ := s.RunShell(ctx, serial, "dd if='"+remotePath+"' bs=4096 count=2 2>/dev/null | od -An -tx1 2>/dev/null | grep 00 | wc -l")
		nullCount := strings.TrimSpace(nullCheck)
		if nullCount != "" && nullCount != "0" {
			return "(binary file, can't preview)", nil
		}
	}

	// Read as text
	out, err := s.RunShell(ctx, serial, "cat '"+remotePath+"' 2>/dev/null")
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}

	// Strip common non-printable control characters that cause garbled display
	// Keep newlines (\n), tabs (\t), and carriage returns (\r)
	out = strings.ReplaceAll(out, "\x00", "")
	out = strings.ReplaceAll(out, "\x1b", "") // ESC sequences
	return out, nil
}

func (s *ADBService) WriteFile(ctx context.Context, serial, remotePath, content string) (string, error) {
	remotePath = sanitizePath(remotePath)
	// Use base64 to avoid shell escaping issues
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	cmd := fmt.Sprintf("echo '%s' | base64 -d > '%s' && echo 'written'", encoded, remotePath)
	out, err := s.RunShell(ctx, serial, cmd)
	if err != nil {
		return "", fmt.Errorf("write file failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) CopyFile(ctx context.Context, serial, src, dst string) (string, error) {
	src = sanitizePath(src)
	dst = sanitizePath(dst)
	out, err := s.RunShell(ctx, serial, fmt.Sprintf("cp -r %q %q && echo 'copied'", src, dst))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *ADBService) GetFileInfo(ctx context.Context, serial, remotePath string) (map[string]interface{}, error) {
	remotePath = sanitizePath(remotePath)
	statOut, err := s.RunShell(ctx, serial, "stat "+remotePath+" 2>/dev/null")
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	info := make(map[string]interface{})
	info["path"] = remotePath
	for _, line := range strings.Split(statOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Size:") {
			var sz int
			fmt.Sscanf(line, "Size: %d", &sz)
			info["size"] = sz
		} else if strings.Contains(line, "File:") {
			parts := strings.SplitN(line, "->", 2)
			if len(parts) > 1 {
				info["link_target"] = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Access:") && !strings.HasPrefix(line, "Access: (") {
			info["access_time"] = strings.TrimPrefix(line, "Access: ")
		} else if strings.HasPrefix(line, "Modify:") {
			info["modify_time"] = strings.TrimPrefix(line, "Modify: ")
		} else if strings.HasPrefix(line, "Change:") {
			info["change_time"] = strings.TrimPrefix(line, "Change: ")
		}
	}
	// Get file size human-readable
	sizeStr, _ := s.GetFileSize(ctx, serial, remotePath)
	info["size_human"] = sizeStr
	// Get mode
	modeOut, _ := s.RunShell(ctx, serial, "stat -c '%A %a' "+remotePath+" 2>/dev/null")
	if modeOut != "" {
		parts := strings.Fields(modeOut)
		if len(parts) >= 2 {
			info["mode"] = parts[0]
			info["mode_numeric"] = parts[1]
		}
	}
	return info, nil
}
