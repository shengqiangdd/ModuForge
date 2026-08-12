# ModuForge API Reference

## Overview

This document provides a comprehensive reference for the ModuForge API, including all endpoints, request/response formats, and usage examples.

## Authentication

All API endpoints require authentication via JWT token. Include the token in the `Authorization` header:

```
Authorization: Bearer <jwt_token>
```

Some endpoints also support token via query parameter for download functionality:

```
?token=<jwt_token>
```

## Git Push Optimization Endpoints

### 1. Optimized Git Push

Push files to a remote repository with advanced filtering options.

**Endpoint:** `POST /api/v1/git/push-optimized`

**Request Body:**
```json
{
  "project_id": "uuid",
  "remote": "origin",
  "branch": "main",
  "token": "github_token",
  "include_patterns": ["src/**", "*.go"],
  "exclude_patterns": ["test/**"],
  "commit_message": "Custom commit message",
  "dry_run": false
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| project_id | string | Yes | Project UUID |
| remote | string | No | Git remote name (default: "origin") |
| branch | string | No | Branch name (default: "main") |
| token | string | No | GitHub token for authentication |
| include_patterns | array | No | File patterns to include (glob syntax) |
| exclude_patterns | array | No | File patterns to exclude (glob syntax) |
| commit_message | string | No | Custom commit message |
| dry_run | boolean | No | Preview changes without pushing (default: false) |

**Response:**
```json
{
  "success": true,
  "message": "Pushed 5 files to origin/main",
  "commit_hash": "abc123def456",
  "files_pushed": 5,
  "dry_run": false
}
```

**Default Exclusion Patterns:**
- `*.log`, `*.tmp`, `*.cache`
- `node_modules/`, `__pycache__/`
- `.env`, `.env.local`
- `build/`, `dist/`
- `*.zip`, `*.tar.gz`
- `.DS_Store`, `Thumbs.db`
- `.git/`
- `*.exe`, `*.dll`, `*.so`, `*.dylib`

---

### 2. Preview Files to Push

Preview which files will be pushed before actually pushing.

**Endpoint:** `POST /api/v1/git/preview-files`

**Request Body:**
```json
{
  "project_id": "uuid",
  "include_patterns": ["src/**"],
  "exclude_patterns": ["test/**"]
}
```

**Response:**
```json
{
  "files": [
    "src/main.go",
    "src/utils.go",
    "module.prop",
    "META-INF/com/google/android/update-binary"
  ],
  "count": 4
}
```

---

## Build Artifact Release Endpoints

### 3. Publish Build to GitHub Release

Publish a build artifact to a GitHub Release.

**Endpoint:** `POST /api/v1/projects/:id/builds/:buildId/release`

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | string | Project UUID |
| buildId | string | Build task UUID |

**Request Body:**
```json
{
  "token": "github_token"
}
```

**Response:**
```json
{
  "tag_name": "v1.0.0",
  "name": "MyModule v1.0.0",
  "body": "## MyModule v1.0.0\n\nAutomated release from ModuForge build.\n\n**Build ID:** abc123\n**Architecture:** arm64\n**Build Time:** 2026-08-11 10:30:00",
  "draft": false,
  "prerelease": false,
  "html_url": "https://github.com/owner/repo/releases/tag/v1.0.0",
  "upload_url": "https://uploads.github.com/repos/owner/repo/releases/123/assets{?name,label}"
}
```

**Notes:**
- Version is automatically extracted from `module.prop` if it exists
- Default version is `1.0.0` if not found
- Artifact is automatically uploaded to the release
- Git remote URL is parsed to determine GitHub owner/repo

---

## File Management Endpoints

### 4. Get File Tree

Get a hierarchical file tree structure for a project.

**Endpoint:** `GET /api/v1/projects/:id/tree`

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| id | string | Project UUID |

**Response:**
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
      "children": [
        {
          "name": "main.go",
          "path": "src/main.go",
          "type": "file",
          "size": 1024,
          "modified": "2026-08-11T10:30:00Z"
        }
      ]
    },
    {
      "name": "module.prop",
      "path": "module.prop",
      "type": "file",
      "size": 256,
      "modified": "2026-08-11T09:15:00Z"
    }
  ]
}
```

---

## Removed Endpoints (Webhook)

The following endpoints have been removed as part of the webhook removal:

- `POST /api/v1/webhook/git`
- `GET /api/v1/webhooks/:hookId/deliveries`
- `POST /api/v1/webhooks/:hookId/test`
- `DELETE /api/v1/webhooks/deliveries/:id`
- `GET /api/v1/webhooks/deliveries/stats`

**Note:** Build schedule service remains functional for cron-based scheduling.

---

## Usage Examples

### Example 1: Push with File Filtering

```bash
curl -X POST http://localhost:8086/api/v1/git/push-optimized \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "project_id": "abc-123",
    "remote": "origin",
    "branch": "main",
    "exclude_patterns": ["*.log", "temp/**"],
    "commit_message": "feat: add new feature"
  }'
```

### Example 2: Preview Files Before Push

```bash
curl -X POST http://localhost:8086/api/v1/git/preview-files \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "project_id": "abc-123",
    "exclude_patterns": ["test/**"]
  }'
```

### Example 3: Publish Build to Release

```bash
curl -X POST http://localhost:8086/api/v1/projects/abc-123/builds/build-456/release \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "token": "github_token"
  }'
```

### Example 4: Get File Tree

```bash
curl http://localhost:8086/api/v1/projects/abc-123/tree \
  -H "Authorization: Bearer <token>"
```

### Example 5: Dry Run Push

```bash
curl -X POST http://localhost:8086/api/v1/git/push-optimized \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "project_id": "abc-123",
    "remote": "origin",
    "branch": "main",
    "dry_run": true
  }'
```

---

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid request parameters",
  "message": "project_id is required"
}
```

### 401 Unauthorized
```json
{
  "error": "Unauthorized",
  "message": "Invalid or expired token"
}
```

### 403 Forbidden
```json
{
  "error": "Forbidden",
  "message": "Insufficient permissions"
}
```

### 404 Not Found
```json
{
  "error": "Not Found",
  "message": "Project or build not found"
}
```

### 500 Internal Server Error
```json
{
  "error": "Internal Server Error",
  "message": "Failed to execute operation"
}
```

---

## Rate Limiting

API endpoints are rate-limited to prevent abuse. The current limits are:

- **Authenticated requests:** 100 requests per minute
- **Unauthenticated requests:** 10 requests per minute

Rate limit headers are included in responses:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 99
X-RateLimit-Reset: 1691740800
```

---

## Changelog

### v1.1.0 (2026-08-11)
- Added optimized Git push with file filtering
- Added file preview before push
- Added GitHub Release publishing
- Added file tree endpoint
- Removed webhook functionality

### v1.0.0 (2026-08-01)
- Initial API release
- Basic project management
- Build system
- Authentication
