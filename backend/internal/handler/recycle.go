package handler

import (
	"database/sql"
	"encoding/json"
	"os"

	"github.com/gofiber/fiber/v3"
)

type RecycleHandler struct {
	db *sql.DB
}

func NewRecycleHandler(db *sql.DB) *RecycleHandler {
	return &RecycleHandler{db: db}
}

func (h *RecycleHandler) userID(c fiber.Ctx) string {
	return currentUserID(c)
}

func (h *RecycleHandler) List(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	rows, err := h.db.Query("SELECT id, user_id, item_type, item_id, item_name, item_data, deleted_at, expires_at FROM recycle_bin WHERE user_id = ? ORDER BY deleted_at DESC", uid)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()
	type RecycleItem struct {
		ID        int64  `json:"id"`
		UserID    string `json:"user_id"`
		ItemType  string `json:"item_type"`
		ItemID    int64  `json:"item_id"`
		ItemName  string `json:"item_name"`
		ItemData  string `json:"item_data"`
		DeletedAt string `json:"deleted_at"`
		ExpiresAt string `json:"expires_at"`
	}
	var items []RecycleItem
	for rows.Next() {
		var item RecycleItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.ItemType, &item.ItemID, &item.ItemName, &item.ItemData, &item.DeletedAt, &item.ExpiresAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	if items == nil {
		items = []RecycleItem{}
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *RecycleHandler) Restore(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	itemType := c.Params("type")
	itemID := c.Params("id")
	var itemData string
	var itemName string
	err := h.db.QueryRow("SELECT item_name, item_data FROM recycle_bin WHERE user_id = ? AND item_type = ? AND item_id = ?", uid, itemType, itemID).Scan(&itemName, &itemData)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	if itemType == "project" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(itemData), &data); err == nil {
			if _, err := h.db.Exec("UPDATE projects SET deleted_at = NULL WHERE id = ? AND user_id = ?", itemID, uid); err != nil {
				return InternalError(c, err.Error())
			}
		}
	} else if itemType == "module" {
		// Restore module logic
	}
	if _, err := h.db.Exec("DELETE FROM recycle_bin WHERE user_id = ? AND item_type = ? AND item_id = ?", uid, itemType, itemID); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "name": itemName})
}

func (h *RecycleHandler) PermanentlyDelete(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}
	itemType := c.Params("type")
	itemID := c.Params("id")

	// For projects, item_id in recycle_bin is stored via ParseInt (often 0 for UUIDs),
	// so we read item_data to get the real project ID.
	if itemType == "project" {
		var itemData string
		err := h.db.QueryRow("SELECT item_data FROM recycle_bin WHERE user_id = ? AND item_type = ? AND item_id = ?", uid, itemType, itemID).Scan(&itemData)
		if err == nil {
			var data map[string]interface{}
			if json.Unmarshal([]byte(itemData), &data) == nil {
				if pid, ok := data["id"].(string); ok && pid != "" {
					h.cascadeDeleteProject(pid)
				}
			}
		}
	}

	if _, err := h.db.Exec("DELETE FROM recycle_bin WHERE user_id = ? AND item_type = ? AND item_id = ?", uid, itemType, itemID); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *RecycleHandler) ClearAll(c fiber.Ctx) error {
	uid := h.userID(c)
	if uid == "" {
		return Unauthorized(c, "unauthorized")
	}

	// Find and clean up all projects in recycle bin before deleting entries
	rows, _ := h.db.Query("SELECT item_data, item_type FROM recycle_bin WHERE user_id = ? AND item_type = 'project'", uid)
	if rows != nil {
		for rows.Next() {
			var itemData, itemType string
			if rows.Scan(&itemData, &itemType) == nil {
				var data map[string]interface{}
				if json.Unmarshal([]byte(itemData), &data) == nil {
					if pid, ok := data["id"].(string); ok && pid != "" {
						h.cascadeDeleteProject(pid)
					}
				}
			}
		}
		rows.Close()
	}

	if _, err := h.db.Exec("DELETE FROM recycle_bin WHERE user_id = ?", uid); err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// cascadeDeleteProject performs hard delete of all project-related data
func (h *RecycleHandler) cascadeDeleteProject(projectID string) {
	// Collect artifact paths
	rows, _ := h.db.Query("SELECT artifact_path FROM build_tasks WHERE project_id=?", projectID)
	if rows != nil {
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				os.Remove(p)
			}
		}
		rows.Close()
	}

	// Delete from all project-related tables (explicit statements to avoid SQL injection)
	projectTables := []string{
		"build_tasks", "project_files", "file_comments", "comments",
		"collaborators", "edit_sessions", "team_members", "activities",
		"vulnerability_scans", "project_versions", "git_branches",
		"collaboration_sessions", "module_vuln_scans", "permission_audits",
	}
	for _, table := range projectTables {
		// Validate table name against whitelist to prevent injection
		valid := false
		for _, validTable := range projectTables {
			if table == validTable {
				valid = true
				break
			}
		}
		if !valid {
			continue
		}
		h.db.Exec("DELETE FROM "+table+" WHERE project_id=?", projectID)
	}

	// Hard delete the project itself
	h.db.Exec("DELETE FROM projects WHERE id=?", projectID)
}

func (h *RecycleHandler) CleanupExpiredItems() {
	h.db.Exec("DELETE FROM recycle_bin WHERE expires_at <= datetime('now')")
}
