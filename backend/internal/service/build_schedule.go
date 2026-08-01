package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type BuildSchedule struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	UserID      string  `json:"user_id"`
	CronExpr    string  `json:"cron_expr"`
	Target      string  `json:"target"`
	Arch        string  `json:"arch"`
	IsActive    bool    `json:"is_active"`
	LastBuildAt *string `json:"last_build_at"`
	NextBuildAt *string `json:"next_build_at"`
	CreatedAt   string  `json:"created_at"`
}

type BuildScheduleService struct {
	db *sql.DB
	bs *BuildService
}

func NewBuildScheduleService(db *sql.DB, bs *BuildService) *BuildScheduleService {
	return &BuildScheduleService{db: db, bs: bs}
}

func (s *BuildScheduleService) Create(ctx context.Context, projectID, userID, cronExpr, target, arch string) (*BuildSchedule, error) {
	nextBuild, err := NextCronTime(cronExpr, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	id := uuid.New().String()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO build_schedules (id, project_id, user_id, cron_expr, target, arch, next_build_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, userID, cronExpr, target, arch, nextBuild.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return &BuildSchedule{
		ID:         id,
		ProjectID:  projectID,
		UserID:     userID,
		CronExpr:   cronExpr,
		Target:     target,
		Arch:       arch,
		IsActive:   true,
		NextBuildAt: strPtr(nextBuild.Format(time.RFC3339)),
		CreatedAt:  time.Now().Format(time.RFC3339),
	}, nil
}

func (s *BuildScheduleService) List(ctx context.Context, projectID string) ([]BuildSchedule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, user_id, cron_expr, target, arch, is_active, last_build_at, next_build_at, created_at
		 FROM build_schedules WHERE project_id=? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []BuildSchedule
	for rows.Next() {
		var bs BuildSchedule
		var isActive int
		if err := rows.Scan(&bs.ID, &bs.ProjectID, &bs.UserID, &bs.CronExpr, &bs.Target, &bs.Arch,
			&isActive, &bs.LastBuildAt, &bs.NextBuildAt, &bs.CreatedAt); err != nil {
			continue
		}
		bs.IsActive = isActive == 1
		list = append(list, bs)
	}
	if list == nil {
		list = []BuildSchedule{}
	}
	return list, nil
}

func (s *BuildScheduleService) Toggle(ctx context.Context, id string, active bool) error {
	v := 0
	if active {
		v = 1
	}
	_, err := s.db.ExecContext(ctx, `UPDATE build_schedules SET is_active=? WHERE id=?`, v, id)
	return err
}

func (s *BuildScheduleService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM build_schedules WHERE id=?`, id)
	return err
}

// RunDue builds all schedules that are due now.
func (s *BuildScheduleService) RunDue(ctx context.Context) int {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, cron_expr, target, arch FROM build_schedules 
		 WHERE is_active=1 AND next_build_at <= datetime('now')`)
	if err != nil {
		return 0
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, projectID, cronExpr, target, arch string
		if err := rows.Scan(&id, &projectID, &cronExpr, &target, &arch); err != nil {
			continue
		}
		task, err := s.bs.CreateWithTrigger(ctx, projectID, target, "schedule", "", arch)
		if err != nil {
			slog.Error("scheduled build failed", "schedule_id", id, "error", err)
			continue
		}
		nextBuild, _ := NextCronTime(cronExpr, time.Now())
		s.db.ExecContext(ctx,
			`UPDATE build_schedules SET last_build_at=datetime('now'), next_build_at=? WHERE id=?`,
			nextBuild.Format(time.RFC3339), id)
		slog.Info("scheduled build triggered", "schedule_id", id, "project", projectID, "task_id", task.ID)
		count++
	}
	return count
}

func strPtr(s string) *string { return &s }

// --- Minimal cron parser for "minute hour day month weekday" ---
// Supports: * */N N,N N-N
func NextCronTime(expr string, from time.Time) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("cron must have 5 fields, got %d", len(fields))
	}

	minutes := parseCronField(fields[0], 0, 59)
	hours := parseCronField(fields[1], 0, 23)
	days := parseCronField(fields[2], 1, 31)
	months := parseCronField(fields[3], 1, 12)
	weekdays := parseCronField(fields[4], 0, 6)

	// Search up to 366 days ahead
	t := from.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 525600; i++ { // 366*24*60
		if !contains(minutes, t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}
		if !contains(hours, t.Hour()) {
			t = t.Add(time.Minute)
			continue
		}
		if !contains(days, t.Day()) {
			t = t.Add(time.Minute)
			continue
		}
		if !contains(months, int(t.Month())) {
			t = t.Add(time.Minute)
			continue
		}
		if !contains(weekdays, int(t.Weekday())) {
			t = t.Add(time.Minute)
			continue
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("no matching time found within 366 days")
}

func parseCronField(field string, min, max int) []int {
	set := make(map[int]bool)

	if field == "*" {
		for i := min; i <= max; i++ {
			set[i] = true
		}
		return mapToSlice(set)
	}

	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "/") {
			parts := strings.SplitN(part, "/", 2)
			start := min
			if parts[0] != "*" {
				start = parseOrDef(parts[0], min)
			}
			step := parseOrDef(parts[1], 1)
			if step <= 0 {
				step = 1
			}
			for i := start; i <= max; i += step {
				set[i] = true
			}
		} else if strings.Contains(part, "-") {
			parts := strings.SplitN(part, "-", 2)
			lo := parseOrDef(parts[0], min)
			hi := parseOrDef(parts[1], max)
			for i := lo; i <= hi; i++ {
				set[i] = true
			}
		} else {
			v := parseOrDef(part, min)
			set[v] = true
		}
	}

	return mapToSlice(set)
}

func parseOrDef(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func contains(vals []int, v int) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}

func mapToSlice(m map[int]bool) []int {
	var s []int
	for k := range m {
		s = append(s, k)
	}
	if len(s) == 0 {
		return nil
	}
	return s
}

// StartScheduler runs a goroutine that checks for due schedules every 60 seconds.
func (s *BuildScheduleService) StartScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count := s.RunDue(ctx)
				if count > 0 {
					slog.Info("scheduled builds triggered", "count", count)
				}
			}
		}
	}()
}
