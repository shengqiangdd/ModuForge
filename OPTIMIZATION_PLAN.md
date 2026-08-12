# ModuForge Git Push Optimization Plan

## Overview

This document outlines the optimizations for ModuForge's Git push functionality, webhook removal, build artifact publishing, and folder display support.

## Current State Analysis

### 1. Git Push (gitmgr.go)
- **Current**: Pushes all files with token authentication
- **Issues**: 
  - No file filtering or exclusion rules
  - Pushes unnecessary files (logs, build artifacts, etc.)
  - No folder structure awareness in push

### 2. Webhook (webhook.go)
- **Current**: Handles GitHub push webhooks to trigger builds
- **User Request**: Remove webhook completely

### 3. File Listing (project.go ListFiles)
- **Current**: Returns flat list of files with paths
- **Issues**: 
  - No folder hierarchy display
  - User wants folder structure view

### 4. Build System (build.go)
- **Current**: Creates zip artifacts
- **User Request**: Publish to GitHub Release

## Optimization Phases

### Phase 1: Remove Webhook (Priority: High)

**Files to modify:**
1. `backend/internal/handler/webhook.go` - Remove or deprecate
2. `backend/internal/handler/routes.go` - Remove webhook routes
3. `backend/internal/handler/build_schedule.go` - Remove webhook-based triggers
4. Database migration - Remove webhook_deliveries table

**Implementation:**
```go
// Remove these routes from routes.go:
// r("POST", "/webhook/git", webhookH.HandleGitWebhook)
// r("GET", "/webhooks/:hookId/deliveries", webhookH.ListDeliveries)
// r("POST", "/webhooks/:hookId/test", webhookH.TestWebhook)
// r("DELETE", "/webhooks/deliveries/:id", webhookH.DeleteDelivery)
// r("GET", "/webhooks/deliveries/stats", webhookH.DeliveryStats)
```

### Phase 2: Optimize Git Push (Priority: High)

**Files to modify:**
1. `backend/internal/service/gitmgr.go` - Add file filtering
2. `backend/internal/handler/gitmgr.go` - Add exclusion parameters

**New features:**
- Add `.gitignore` support
- Implement smart file filtering
- Add commit message templates
- Support partial pushes (selective files)

**Implementation:**
```go
// Add to GitManagerService
type PushOptions struct {
    IncludePatterns []string `json:"include_patterns"`
    ExcludePatterns []string `json:"exclude_patterns"`
    CommitMessage   string   `json:"commit_message"`
    DryRun          bool     `json:"dry_run"`
}

func (s *GitManagerService) PushWithOptions(ctx context.Context, projectID, remote, branch, token string, opts PushOptions) (string, error) {
    // 1. Filter files based on patterns
    // 2. Create .gitignore if needed
    // 3. Add only matching files
    // 4. Commit with custom message
    // 5. Push to remote
}
```

**Default exclusion patterns:**
```go
var defaultExcludePatterns = []string{
    "*.log",
    "*.tmp",
    "*.cache",
    "node_modules/",
    "__pycache__/",
    ".env",
    ".env.local",
    "build/",
    "dist/",
    "*.zip",
    "*.tar.gz",
    ".DS_Store",
    "Thumbs.db",
}
```

### Phase 3: Build Artifacts to GitHub Release (Priority: Medium)

**Files to modify:**
1. `backend/internal/service/build.go` - Add Release publishing
2. `backend/internal/handler/build.go` - Add Release API endpoints

**New features:**
- Auto-publish zip to GitHub Release after build
- Version tagging based on module.prop
- Release notes generation

**Implementation:**
```go
// Add to BuildService
type ReleaseInfo struct {
    TagName    string `json:"tag_name"`
    Name       string `json:"name"`
    Body       string `json:"body"`
    Draft      bool   `json:"draft"`
    Prerelease bool   `json:"prerelease"`
}

func (s *BuildService) PublishToRelease(ctx context.Context, projectID, buildID, token string) (*ReleaseInfo, error) {
    // 1. Get build artifact path
    // 2. Read module.prop for version
    // 3. Create GitHub Release via API
    // 4. Upload zip asset
    // 5. Return release info
}
```

### Phase 4: Folder Display (Priority: Medium)

**Files to modify:**
1. `backend/internal/handler/project.go` - Add tree endpoint
2. `backend/internal/service/project.go` - Implement tree building
3. `frontend/src/lib/components/FileTree.svelte` - New component

**New features:**
- Directory tree API endpoint
- Expandable/collapsible folders
- File icons based on type
- Drag-and-drop support (future)

**Implementation:**
```go
// Add to ProjectHandler
type FileTreeNode struct {
    Name     string         `json:"name"`
    Path     string         `json:"path"`
    Type     string         `json:"type"` // "file" or "directory"
    Children []*FileTreeNode `json:"children,omitempty"`
    Size     int64          `json:"size,omitempty"`
    Modified string         `json:"modified,omitempty"`
}

func (h *ProjectHandler) GetFileTree(c fiber.Ctx) error {
    projectID := c.Params("id")
    userID := c.Locals("uid")
    
    // 1. Get all files for project
    // 2. Build tree structure
    // 3. Return nested JSON
}
```

## Implementation Timeline

### Week 1: Phase 1 & 2
- Day 1-2: Remove webhook functionality
- Day 3-5: Implement Git push optimization

### Week 2: Phase 3 & 4
- Day 1-3: Implement GitHub Release publishing
- Day 4-5: Implement folder display

## Testing Strategy

1. **Unit Tests**: Test each new function
2. **Integration Tests**: Test API endpoints
3. **Manual Testing**: Verify UI interactions
4. **Performance Testing**: Check push speed with large projects

## Migration Notes

1. **Database**: Add migration to remove webhook_deliveries table
2. **API**: Version API changes (v1 → v2 for breaking changes)
3. **Documentation**: Update API docs and user guide

## Success Metrics

1. **Git Push**: 50% faster with file filtering
2. **Webhook**: Complete removal, no regression
3. **Release**: Auto-publish within 30 seconds of build
4. **Folder Display**: Load time < 500ms for 1000+ files

## Risks & Mitigations

1. **Risk**: Breaking existing webhook integrations
   **Mitigation**: Deprecation period, migration guide

2. **Risk**: GitHub API rate limits
   **Mitigation**: Implement caching, retry logic

3. **Risk**: Large project folder display performance
   **Mitigation**: Lazy loading, pagination

---

**Next Steps:**
1. Review and approve plan
2. Start Phase 1 implementation
3. Daily standups during implementation
4. Code review after each phase