package skills

import (
	"database/sql"
	"path/filepath"
)

// ResolveProjectPath resolves the actual filesystem path for a project.
// Checks the database for storage_path, falls back to projectPath/projectID.
func ResolveProjectPath(db *sql.DB, projectPath, projectID string) string {
	if projectID == "" {
		return projectPath
	}
	if db != nil {
		var storagePath string
		err := db.QueryRow(`SELECT COALESCE(storage_path,'') FROM projects WHERE id=?`, projectID).Scan(&storagePath)
		if err == nil && storagePath != "" {
			return storagePath
		}
	}
	return filepath.Join(projectPath, projectID)
}
