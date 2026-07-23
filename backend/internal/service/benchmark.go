package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type BenchmarkService struct {
	db *sql.DB
}

func NewBenchmarkService(db *sql.DB) *BenchmarkService {
	return &BenchmarkService{db: db}
}

type BenchmarkResult struct {
	ID         string                 `json:"id"`
	ModuleID   string                 `json:"module_id"`
	DeviceSN   string                 `json:"device_serial"`
	Before     map[string]interface{} `json:"before"`
	After      map[string]interface{} `json:"after"`
	Diff       map[string]interface{} `json:"diff"`
	CreatedAt  time.Time              `json:"created_at"`
}

// SaveBenchmark saves a benchmark result to the database
func (s *BenchmarkService) SaveBenchmark(ctx context.Context, result *BenchmarkResult) error {
	id := uuid.New().String()
	beforeJSON, _ := json.Marshal(result.Before)
	afterJSON, _ := json.Marshal(result.After)
	diffJSON, _ := json.Marshal(result.Diff)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO benchmark_results (id, module_id, device_serial, before_data, after_data, diff_data, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, result.ModuleID, result.DeviceSN, beforeJSON, afterJSON, diffJSON, time.Now())
	return err
}

// GetBenchmarkHistory retrieves benchmark history for a module
func (s *BenchmarkService) GetBenchmarkHistory(ctx context.Context, moduleID string, limit int) ([]BenchmarkResult, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, module_id, device_serial, before_data, after_data, diff_data, created_at
		 FROM benchmark_results WHERE module_id = ? ORDER BY created_at DESC LIMIT ?`,
		moduleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BenchmarkResult
	for rows.Next() {
		var r BenchmarkResult
		var beforeJSON, afterJSON, diffJSON []byte
		if err := rows.Scan(&r.ID, &r.ModuleID, &r.DeviceSN, &beforeJSON, &afterJSON, &diffJSON, &r.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal(beforeJSON, &r.Before)
		json.Unmarshal(afterJSON, &r.After)
		json.Unmarshal(diffJSON, &r.Diff)
		results = append(results, r)
	}
	return results, nil
}
