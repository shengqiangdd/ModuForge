package domain

import "time"

type ModuleType string

const (
	ModuleMagisk    ModuleType = "magisk"
	ModuleKSU       ModuleType = "ksu"
	ModuleAPatch    ModuleType = "apatch"
	ModuleHybrid    ModuleType = "hybrid"
	ModuleUniversal ModuleType = "universal"
	ModulePerformance ModuleType = "performance"
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
	GitURL      string     `json:"git_url"`
	GitBranch   string     `json:"git_branch"`
	BuildCron   string     `json:"build_cron"`
	AutoBuild   bool       `json:"auto_build"`
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
	SHA256    string `json:"sha256,omitempty"`
	FileSize  int64  `json:"file_size,omitempty"`
	MTime     string `json:"mtime,omitempty"`
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
	Trigger      string      `json:"trigger"`      // manual, git, webhook, schedule
	CommitHash   string      `json:"commit_hash,omitempty"`
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
	GitURL      *string     `json:"git_url,omitempty"`
	GitBranch   *string     `json:"git_branch,omitempty"`
	BuildCron   *string     `json:"build_cron,omitempty"`
	AutoBuild   *bool       `json:"auto_build,omitempty"`
}

type AuthResponse struct {
	Token      string `json:"token,omitempty"`
	User       *User  `json:"user,omitempty"`
	Requires2FA bool  `json:"requires_2fa,omitempty"`
	TempToken   string `json:"temp_token,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type BuildRequest struct {
	Target  string `json:"target"`
	Trigger string `json:"trigger,omitempty"` // manual, git, webhook
	Arch    string `json:"arch,omitempty"`    // arm64, arm, x86_64 (default: arm64)
}

type TeamMember struct {
	ID        int64  `json:"id"`
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"` // owner, admin, member, viewer
	InvitedBy string `json:"invited_by,omitempty"`
	CreatedAt string `json:"created_at"`
}

type AuditLog struct {
	ID        int64  `json:"id"`
	ProjectID string `json:"project_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Action    string `json:"action"`
	Details   string `json:"details"`
	CreatedAt string `json:"created_at"`
}

// ===== ModuForge 市场 =====

type ModuleComparison struct {
	TitleA       string  `json:"title_a"`
	TitleB       string  `json:"title_b"`
	DescriptionA string  `json:"description_a"`
	DescriptionB string  `json:"description_b"`
	VersionA     string  `json:"version_a"`
	VersionB     string  `json:"version_b"`
	StarsA       int     `json:"stars_a"`
	StarsB       int     `json:"stars_b"`
	InstallsA    int     `json:"installs_a"`
	InstallsB    int     `json:"installs_b"`
	CategoryA    string  `json:"category_a"`
	CategoryB    string  `json:"category_b"`
	AuthorA      string  `json:"author_a"`
	AuthorB      string  `json:"author_b"`
	LicenseA     string  `json:"license_a"`
	LicenseB     string  `json:"license_b"`
	DepCountA    int     `json:"dep_count_a"`
	DepCountB    int     `json:"dep_count_b"`
	RatingA      float64 `json:"rating_a"`
	RatingB      float64 `json:"rating_b"`
}

type ModuleDependency struct {
	ID         string `json:"id"`
	MinVersion string `json:"min_version,omitempty"`
	Optional   bool   `json:"optional,omitempty"`
}

type DependencyNode struct {
	Module   *MarketModule    `json:"module"`
	Level    int              `json:"level"`
	Children []*DependencyNode `json:"children,omitempty"`
}

type Conflict struct {
	ModuleA string `json:"module_a"`
	ModuleB string `json:"module_b"`
	Type    string `json:"type"` // circular, version_mismatch
	Detail  string `json:"detail"`
}

type MarketModule struct {
	ID           string    `json:"id" db:"id"`
	Title        string    `json:"title" db:"title"`
	Slug         string    `json:"slug" db:"slug"`
	Description  string    `json:"description" db:"description"`
	Category     string    `json:"category" db:"category"` // system, ui, audio, display, utility
	Tags         string    `json:"tags" db:"tags"`         // 逗号分隔
	Version      string    `json:"version" db:"version"`
	VersionCode  int       `json:"version_code" db:"version_code"`
	Changelog    string    `json:"changelog" db:"changelog"`
	ParentID     string    `json:"parent_id" db:"parent_id"`
	Author       string    `json:"author" db:"author"`
	AuthorUID    string    `json:"author_uid" db:"author_uid"`
	License      string    `json:"license" db:"license"`
	Dependencies string    `json:"dependencies" db:"dependencies"` // JSON array of ModuleDependency
	CoverImage   string    `json:"cover_image" db:"cover_image"`
	Stars        int       `json:"stars" db:"stars"`
	Installs     int       `json:"installs" db:"installs"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	Screenshots  []*ModuleScreenshot `json:"screenshots,omitempty"`
}

type ModuleScreenshot struct {
	ID        int64     `json:"id"`
	ModuleID  string    `json:"module_id"`
	URL       string    `json:"url"`
	SortOrder int       `json:"sort_order"`
	Caption   string    `json:"caption"`
	CreatedAt time.Time `json:"created_at"`
}

type ModuleVersion struct {
	ID        string    `json:"id" db:"id"`
	ModuleID  string    `json:"module_id" db:"module_id"`
	Version   string    `json:"version" db:"version"`
	Changelog string    `json:"changelog" db:"changelog"`
	FileHash  string    `json:"file_hash" db:"file_hash"`
	FilePath  string    `json:"file_path" db:"file_path"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
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

type BuildCacheResponse struct {
	Cached bool   `json:"cached"`
	TaskID string `json:"task_id"`
}

type ModuleDemo struct {
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	SimulatedOutput string     `json:"simulated_output"`
	Props           []DemoProp `json:"props"`
	Files           []string   `json:"files"`
}

type DemoProp struct {
	Path   string `json:"path"`
	Prop   string `json:"prop"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type ModuleTag struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	UsageCount int    `json:"usage_count"`
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
