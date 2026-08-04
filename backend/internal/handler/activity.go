package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type ActivityHandler struct {
	svc *service.ActivityService
}

func NewActivityHandler(svc *service.ActivityService) *ActivityHandler {
	return &ActivityHandler{svc: svc}
}

func (h *ActivityHandler) GetProjectActivities(c fiber.Ctx) error {
	projectID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid project id"})
	}
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	list, err := h.svc.GetProjectActivities(projectID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"activities": list})
}

func (h *ActivityHandler) GetUserActivities(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "invalid user"})
	}
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	list, err := h.svc.GetUserActivities(userID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"activities": list})
}

func (h *ActivityHandler) Export(c fiber.Ctx) error {
	userID := safeUserID(c)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "invalid user"})
	}
	format := c.Query("format", "json")
	start := c.Query("start", "")
	end := c.Query("end", "")

	activities, err := h.svc.GetUserActivities(userID, 10000, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Filter by date range
	if start != "" || end != "" {
		var filtered []service.Activity
		for _, a := range activities {
			if start != "" {
				t, _ := time.Parse("2006-01-02", start)
				if a.CreatedAt.Before(t) {
					continue
				}
			}
			if end != "" {
				t, _ := time.Parse("2006-01-02", end)
				if a.CreatedAt.After(t.Add(24*time.Hour - time.Second)) {
					continue
				}
			}
			filtered = append(filtered, a)
		}
		activities = filtered
	}

	switch format {
	case "csv":
		var sb strings.Builder
		sb.WriteString("ID,UserID,ProjectID,Type,Description,CreatedAt\n")
		for _, a := range activities {
			pid := ""
			if a.ProjectID > 0 {
				pid = strconv.FormatInt(a.ProjectID, 10)
			}
			sb.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%s\n", a.ID, a.UserID, pid, a.ActivityType, a.Description, a.CreatedAt.Format(time.RFC3339)))
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="activities_%d.csv"`, time.Now().Unix()))
		return c.Send([]byte(sb.String()))
	default:
		c.Set("Content-Type", "application/json; charset=utf-8")
		c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="activities_%d.json"`, time.Now().Unix()))
		data, _ := json.Marshal(activities)
		return c.Send(data)
	}
}
