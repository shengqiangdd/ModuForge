package domain

import "time"

type ModuleType string

const (
	ModuleMagisk    ModuleType = "magisk"
	ModuleKSU       ModuleType = "ksu"
	ModuleAPatch    ModuleType = "apatch"
	ModuleHybrid    ModuleType = "hybrid"
	ModuleUniversal ModuleType = "universal"
)

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"created_at"`
}

type Project struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	ModuleType  ModuleType `json:"module_type"`
	Description string     `json:"description"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
	DeletedAt   *string    `json:"deleted_at,omitempty"`
}

type ProjectFile struct {
	ID        int64  `json:"id"`
	ProjectID string `json:"project_id"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type BuildStatus string

const (
	BuildPending   BuildStatus = "pending"
	BuildRunning   BuildStatus = "running"
	BuildSuccess   BuildStatus = "success"
	BuildFailed    BuildStatus = "failed"
	BuildCancelled BuildStatus = "cancelled"
)

type BuildTask struct {
	ID           string      `json:"id"`
	ProjectID    string      `json:"project_id"`
	Status       BuildStatus `json:"status"`
	Target       string      `json:"target"`
	Log          string      `json:"log"`
	ArtifactPath *string     `json:"artifact_path,omitempty"`
	CreatedAt    string      `json:"created_at"`
	UpdatedAt    string      `json:"updated_at"`
}

type CreateProjectInput struct {
	Name        string     `json:"name"`
	ModuleType  ModuleType `json:"module_type"`
	Description string     `json:"description"`
}

type UpdateProjectInput struct {
	Name        *string     `json:"name,omitempty"`
	ModuleType  *ModuleType `json:"module_type,omitempty"`
	Description *string     `json:"description,omitempty"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type BuildRequest struct {
	Target string `json:"target"`
}

// ===== ModuForge 市场 =====

type MarketModule struct {
	ID          string    `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Slug        string    `json:"slug" db:"slug"`
	Description string    `json:"description" db:"description"`
	Category    string    `json:"category" db:"category"` // system, ui, audio, display, utility
	Tags        string    `json:"tags" db:"tags"`         // 逗号分隔
	Version     string    `json:"version" db:"version"`
	VersionCode int       `json:"version_code" db:"version_code"`
	Author      string    `json:"author" db:"author"`
	AuthorUID   string    `json:"author_uid" db:"author_uid"`
	License     string    `json:"license" db:"license"`
	Stars       int       `json:"stars" db:"stars"`
	Installs    int       `json:"installs" db:"installs"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type MarketReview struct {
	ID        string    `json:"id" db:"id"`
	ModuleID  string    `json:"module_id" db:"module_id"`
	UID       string    `json:"uid" db:"uid"`
	Username  string    `json:"username" db:"username"`
	Rating    int       `json:"rating" db:"rating"` // 1-5
	Comment   string    `json:"comment" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type ModuleFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type AIPrompt struct {
	ID        int64  `json:"id"`
	Mode      string `json:"mode"` // generate, chat, repair
	UserID    string `json:"user_id,omitempty"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updated_at"`
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
