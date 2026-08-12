# ModuForge Git Push Optimization - Quick Reference

## Overview

This document provides a quick reference for the new Git push optimization features in ModuForge.

## New Features

### 1. Optimized Git Push
Push files with advanced filtering and exclusion patterns.

### 2. File Preview
Preview which files will be pushed before actually pushing.

### 3. GitHub Release Publishing
Automatically publish build artifacts to GitHub Releases.

### 4. File Tree Display
View project files in a hierarchical folder structure.

## Quick Start

### Push with Filtering
```bash
curl -X POST http://localhost:8086/api/v1/git/push-optimized \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "project_id": "abc-123",
    "exclude_patterns": ["*.log", "temp/**"],
    "commit_message": "feat: add feature"
  }'
```

### Preview Files
```bash
curl -X POST http://localhost:8086/api/v1/git/preview-files \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "project_id": "abc-123",
    "exclude_patterns": ["test/**"]
  }'
```

### Publish to Release
```bash
curl -X POST http://localhost:8086/api/v1/projects/abc-123/builds/build-456/release \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "token": "github_token"
  }'
```

### Get File Tree
```bash
curl http://localhost:8086/api/v1/projects/abc-123/tree \
  -H "Authorization: Bearer <token>"
```

## Default Exclusion Patterns

The following patterns are automatically excluded from pushes:

| Pattern | Description |
|---------|-------------|
| `*.log` | Log files |
| `*.tmp` | Temporary files |
| `*.cache` | Cache files |
| `node_modules/` | Node.js dependencies |
| `__pycache__/` | Python cache |
| `.env` | Environment files |
| `.env.local` | Local environment files |
| `build/` | Build output |
| `dist/` | Distribution files |
| `*.zip` | Zip archives |
| `*.tar.gz` | Tar archives |
| `.DS_Store` | macOS system files |
| `Thumbs.db` | Windows thumbnail cache |
| `.git/` | Git directory |
| `*.exe` | Windows executables |
| `*.dll` | Windows libraries |
| `*.so` | Linux shared objects |
| `*.dylib` | macOS dynamic libraries |

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/git/push-optimized` | Push with file filtering |
| POST | `/api/v1/git/preview-files` | Preview files before push |
| POST | `/api/v1/projects/:id/builds/:buildId/release` | Publish to GitHub Release |
| GET | `/api/v1/projects/:id/tree` | Get file tree structure |

## Request Examples

### Push with Custom Patterns
```json
{
  "project_id": "uuid",
  "remote": "origin",
  "branch": "main",
  "token": "github_token",
  "include_patterns": ["src/**", "*.go"],
  "exclude_patterns": ["test/**", "*.log"],
  "commit_message": "feat: add new feature",
  "dry_run": false
}
```

### Preview with Filtering
```json
{
  "project_id": "uuid",
  "include_patterns": ["src/**"],
  "exclude_patterns": ["*.test.go"]
}
```

## Response Examples

### Push Response
```json
{
  "success": true,
  "message": "Pushed 5 files to origin/main",
  "commit_hash": "abc123def456",
  "files_pushed": 5,
  "dry_run": false
}
```

### Preview Response
```json
{
  "files": [
    "src/main.go",
    "src/utils.go",
    "module.prop"
  ],
  "count": 3
}
```

### File Tree Response
```json
{
  "name": "root",
  "path": "",
  "type": "directory",
  "children": [
    {
      "name": "src",
      "path": "src",
      "type": "directory",
      "children": [...]
    }
  ]
}
```

## Error Handling

| Status Code | Description | Solution |
|-------------|-------------|----------|
| 400 | Bad Request | Check request parameters |
| 401 | Unauthorized | Verify token is valid |
| 403 | Forbidden | Check user permissions |
| 404 | Not Found | Verify project/build ID exists |
| 500 | Server Error | Check server logs |

## Best Practices

1. **Always preview before push** - Use `/preview-files` to verify what will be pushed
2. **Use exclusion patterns** - Prevent accidental commits of sensitive files
3. **Store GitHub tokens securely** - Use the token persistence feature
4. **Check build status** - Ensure build succeeded before publishing to release
5. **Use file tree** - Understand project structure before making changes

## Troubleshooting

### Push Fails with Authentication Error
- Verify GitHub token is valid and not expired
- Check token has required permissions (repo scope)
- Ensure remote URL is correct

### Release Publishing Fails
- Verify build succeeded (status: "success")
- Check artifact path exists
- Ensure GitHub token has release permissions

### File Tree Shows Empty
- Verify project has files in database
- Check project ID is correct
- Ensure user has read permissions

## Related Documentation

- [API_REFERENCE.md](./API_REFERENCE.md) - Complete API documentation
- [IMPLEMENTATION_SUMMARY.md](./IMPLEMENTATION_SUMMARY.md) - Implementation details
- [OPTIMIZATION_PLAN.md](./OPTIMIZATION_PLAN.md) - Original optimization plan
