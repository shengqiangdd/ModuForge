package service

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/domain"
)

func scanModule(rows interface{ Scan(...interface{}) error }) (*domain.MarketModule, error) {
	var m domain.MarketModule
	err := rows.Scan(&m.ID, &m.Title, &m.Slug, &m.Description, &m.Category, &m.Tags,
		&m.Version, &m.VersionCode, &m.Changelog, &m.ParentID,
		&m.Author, &m.AuthorUID, &m.License,
		&m.Dependencies, &m.CoverImage,
		&m.Stars, &m.Installs, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

const moduleColumns = "id, title, slug, description, category, tags, version, version_code, COALESCE(changelog,''), COALESCE(parent_id,''), author, COALESCE(author_uid,''), COALESCE(license,''), COALESCE(dependencies,''), COALESCE(cover_image,''), stars, installs, created_at, updated_at"

type SQLiteMarketService struct {
	db *database.DB
}

func NewSQLiteMarketService(db *database.DB) *SQLiteMarketService {
	return &SQLiteMarketService{db: db}
}

func compareVersions(a, b string) int {
	aParts := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bParts := strings.Split(strings.TrimPrefix(b, "v"), ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &aNum)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bNum)
		}
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}
	return 0
}
