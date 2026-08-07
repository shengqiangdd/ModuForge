package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestModuleTypeValues(t *testing.T) {
	if ModuleMagisk != "magisk" {
		t.Errorf("expected magisk, got %s", ModuleMagisk)
	}
	if ModuleKSU != "ksu" {
		t.Errorf("expected ksu, got %s", ModuleKSU)
	}
	if ModuleAPatch != "apatch" {
		t.Errorf("expected apatch, got %s", ModuleAPatch)
	}
	if ModuleHybrid != "hybrid" {
		t.Errorf("expected hybrid, got %s", ModuleHybrid)
	}
	if ModuleUniversal != "universal" {
		t.Errorf("expected universal, got %s", ModuleUniversal)
	}
}

func TestBuildStatusValues(t *testing.T) {
	if BuildPending != "pending" {
		t.Errorf("expected pending, got %s", BuildPending)
	}
	if BuildRunning != "running" {
		t.Errorf("expected running, got %s", BuildRunning)
	}
	if BuildSuccess != "success" {
		t.Errorf("expected success, got %s", BuildSuccess)
	}
	if BuildFailed != "failed" {
		t.Errorf("expected failed, got %s", BuildFailed)
	}
	if BuildCancelled != "cancelled" {
		t.Errorf("expected cancelled, got %s", BuildCancelled)
	}
}

func TestUserJSON(t *testing.T) {
	u := User{
		ID:           "user-1",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "secret-hash",
		CreatedAt:    "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != u.ID {
		t.Errorf("expected %s, got %s", u.ID, decoded.ID)
	}
	if decoded.PasswordHash != "" {
		t.Error("expected PasswordHash to be omitted from JSON")
	}
}

func TestProjectJSON(t *testing.T) {
	p := Project{
		ID:          "proj-1",
		UserID:      "user-1",
		Name:        "Test Project",
		ModuleType:  ModuleUniversal,
		Description: "A test project",
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Project
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Name != p.Name {
		t.Errorf("expected %s, got %s", p.Name, decoded.Name)
	}
}

func TestProjectWithDeletedAt(t *testing.T) {
	deleted := "2024-06-01T00:00:00Z"
	p := Project{
		ID:        "proj-1",
		DeletedAt: &deleted,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Project
	json.Unmarshal(data, &decoded)
	if decoded.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be present")
	}
	if *decoded.DeletedAt != deleted {
		t.Errorf("expected %s, got %s", deleted, *decoded.DeletedAt)
	}
}

func TestProjectWithoutDeletedAt(t *testing.T) {
	p := Project{
		ID:   "proj-1",
		Name: "Test",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Project
	json.Unmarshal(data, &decoded)
	if decoded.DeletedAt != nil {
		t.Error("expected DeletedAt to be nil/omitted")
	}
}

func TestAuthResponse(t *testing.T) {
	resp := AuthResponse{
		Token: "jwt-token",
		User: &User{
			ID:       "user-1",
			Username: "testuser",
			Email:    "test@example.com",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded AuthResponse
	json.Unmarshal(data, &decoded)
	if decoded.Token != "jwt-token" {
		t.Errorf("expected jwt-token, got %s", decoded.Token)
	}
	if decoded.User.Username != "testuser" {
		t.Errorf("expected testuser, got %s", decoded.User.Username)
	}
}

func TestBuildRequest(t *testing.T) {
	br := BuildRequest{
		Target:  "magisk",
		Trigger: "manual",
		Arch:    "arm64",
	}

	if br.Target != "magisk" {
		t.Errorf("expected magisk, got %s", br.Target)
	}
	if br.Arch != "arm64" {
		t.Errorf("expected arm64, got %s", br.Arch)
	}
}

func TestNow(t *testing.T) {
	now := Now()
	parsed, err := time.Parse(time.RFC3339, now)
	if err != nil {
		t.Fatalf("invalid time format: %v", err)
	}
	if time.Since(parsed) > 5*time.Second {
		t.Error("expected Now() to return current time")
	}
}

func TestMarketModule(t *testing.T) {
	m := MarketModule{
		ID:    "mod-1",
		Title: "Test Module",
		Slug:  "test-module",
		Stars: 42,
	}

	if m.ID != "mod-1" {
		t.Errorf("expected mod-1, got %s", m.ID)
	}
	if m.Stars != 42 {
		t.Errorf("expected 42, got %d", m.Stars)
	}
}

func TestModuleComparison(t *testing.T) {
	mc := ModuleComparison{
		TitleA:    "Module A",
		TitleB:    "Module B",
		RatingA:   4.5,
		RatingB:   3.8,
		StarsA:    100,
		StarsB:    50,
		InstallsA: 1000,
		InstallsB: 500,
	}

	if mc.TitleA != "Module A" {
		t.Errorf("expected Module A, got %s", mc.TitleA)
	}
	if mc.RatingA != 4.5 {
		t.Errorf("expected 4.5, got %f", mc.RatingA)
	}
}

func TestDependencyNode(t *testing.T) {
	child := &DependencyNode{
		Module: &MarketModule{ID: "child", Title: "Child"},
		Level:  1,
	}
	root := &DependencyNode{
		Module:   &MarketModule{ID: "root", Title: "Root"},
		Level:    0,
		Children: []*DependencyNode{child},
	}

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.Children))
	}
	if root.Children[0].Module.Title != "Child" {
		t.Errorf("expected Child, got %s", root.Children[0].Module.Title)
	}
}

func TestConflict(t *testing.T) {
	c := Conflict{
		ModuleA: "mod-a",
		ModuleB: "mod-b",
		Type:    "circular",
		Detail:  "circular dependency detected",
	}

	if c.Type != "circular" {
		t.Errorf("expected circular, got %s", c.Type)
	}
}

func TestModuleFile(t *testing.T) {
	f := ModuleFile{
		Path:    "src/main.c",
		Content: "int main() {}",
	}

	if f.Path != "src/main.c" {
		t.Errorf("expected src/main.c, got %s", f.Path)
	}
	if f.Content != "int main() {}" {
		t.Errorf("expected int main() {}, got %s", f.Content)
	}
}

func TestTeamMember(t *testing.T) {
	tm := TeamMember{
		ID:        1,
		ProjectID: "proj-1",
		UserID:    "user-1",
		Role:      "owner",
	}

	if tm.Role != "owner" {
		t.Errorf("expected owner, got %s", tm.Role)
	}
}

func TestModuleScreenshot(t *testing.T) {
	now := time.Now()
	ms := ModuleScreenshot{
		ID:        1,
		ModuleID:  "mod-1",
		URL:       "https://example.com/screenshot.png",
		SortOrder: 1,
		Caption:   "Main screen",
		CreatedAt: now,
	}

	if ms.URL != "https://example.com/screenshot.png" {
		t.Errorf("expected URL, got %s", ms.URL)
	}
	if ms.Caption != "Main screen" {
		t.Errorf("expected Main screen, got %s", ms.Caption)
	}
}

func TestModuleVersion(t *testing.T) {
	mv := ModuleVersion{
		ID:        "ver-1",
		ModuleID:  "mod-1",
		Version:   "1.0.0",
		Changelog: "Initial release",
	}

	if mv.Version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", mv.Version)
	}
}

func TestMarketReview(t *testing.T) {
	mr := MarketReview{
		ID:       "rev-1",
		ModuleID: "mod-1",
		UID:      "user-1",
		Username: "testuser",
		Rating:   5,
		Comment:  "Excellent!",
	}

	if mr.Rating != 5 {
		t.Errorf("expected 5, got %d", mr.Rating)
	}
	if mr.Comment != "Excellent!" {
		t.Errorf("expected Excellent!, got %s", mr.Comment)
	}
}

func TestAIPrompt(t *testing.T) {
	ap := AIPrompt{
		ID:      1,
		Mode:    "generate",
		UserID:  "user-1",
		Content: "Generate a module",
	}

	if ap.Mode != "generate" {
		t.Errorf("expected generate, got %s", ap.Mode)
	}
}

func TestBuildCacheResponse(t *testing.T) {
	br := BuildCacheResponse{
		Cached: true,
		TaskID: "task-1",
	}

	if !br.Cached {
		t.Error("expected Cached to be true")
	}
	if br.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", br.TaskID)
	}
}

func TestModuleDemo(t *testing.T) {
	md := ModuleDemo{
		Slug:            "demo-module",
		Title:           "Demo Module",
		SimulatedOutput: "Output placeholder",
		Files:           []string{"main.c", "config.h"},
	}

	if md.Title != "Demo Module" {
		t.Errorf("expected Demo Module, got %s", md.Title)
	}
	if len(md.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(md.Files))
	}
}

func TestDemoProp(t *testing.T) {
	dp := DemoProp{
		Path:   "config.prop",
		Prop:   "debug",
		Before: "false",
		After:  "true",
	}

	if dp.Prop != "debug" {
		t.Errorf("expected debug, got %s", dp.Prop)
	}
}

func TestModuleTag(t *testing.T) {
	mt := ModuleTag{
		ID:         1,
		Name:       "system",
		Color:      "#ff0000",
		UsageCount: 42,
	}

	if mt.Name != "system" {
		t.Errorf("expected system, got %s", mt.Name)
	}
}

func TestLoginRequest(t *testing.T) {
	lr := LoginRequest{
		Username: "admin",
		Password: "secret",
		TOTPCode: "123456",
	}

	if lr.Username != "admin" {
		t.Errorf("expected admin, got %s", lr.Username)
	}
	if lr.TOTPCode != "123456" {
		t.Errorf("expected 123456, got %s", lr.TOTPCode)
	}
}

func TestRegisterRequest(t *testing.T) {
	rr := RegisterRequest{
		Username: "newuser",
		Email:    "new@example.com",
		Password: "secure-password",
	}

	if rr.Email != "new@example.com" {
		t.Errorf("expected new@example.com, got %s", rr.Email)
	}
}

func TestAuditLog(t *testing.T) {
	al := AuditLog{
		ID:        1,
		ProjectID: "proj-1",
		UserID:    "user-1",
		Action:    "project.created",
		Details:   "Created project 'Test'",
	}

	if al.Action != "project.created" {
		t.Errorf("expected project.created, got %s", al.Action)
	}
}