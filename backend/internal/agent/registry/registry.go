// Package registry provides the skill registry, types, and auto-registration.
//
// Architecture:
//
//	agent → registry (imports SkillRegistry, Skill, SkillMeta)
//	skills → registry (imports RegisterFactory, SkillMeta, MetadataProvider)
//	handler → registry (imports NewSkillRegistry, Deps)
//
// Skills register themselves via init() with factory functions.
// NewSkillRegistry(deps) instantiates all registered skills automatically.
package registry

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// SkillMeta declares metadata for a skill.
type SkillMeta struct {
	ReadOnly  bool // safe in Plan mode (no side effects)
	Essential bool // essential for free models (must be included)
	Core      bool // core tool for ALL models (reduces tool bloat)
	NeedsDB   bool // requires database connection
	NeedsLLM  bool // requires LLM API key/endpoint
	MinTier   int  // minimum model tier (0=free, 1=mid, 2=strong); 0 = all tiers
}

// MetadataProvider is an optional interface that skills can implement
// to declare their capabilities.
type MetadataProvider interface {
	Metadata() SkillMeta
}

// ParameterProvider is an optional interface for skills that want to expose
// a JSON-Schema parameter definition (used by MCP-backed tools instead of the
// generic {"input": string} fallback).
type ParameterProvider interface {
	Parameters() map[string]interface{}
}

// Skill is the interface all skills must implement.
type Skill interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input map[string]interface{}) (string, error)
}

// Deps holds shared dependencies injected into skill factories.
type Deps struct {
	DB            *sql.DB
	StoragePath   string
	LLMApiKey     string
	LLMEndpoint   string
	LLMModel      string
	HTTPClient    *http.Client // shared LLM HTTP client with connection pooling
	MemoryStore   interface{}  // *service.MemoryStore — avoid import cycle
	FileHashCache FileHashCacheI // file hash cache for UNCHANGED detection
	Storage       interface{}  // storage.StorageAdapter — avoid import cycle; cast at usage
}

// FileHashCacheI is the interface for file hash caching (avoids circular dependency).
type FileHashCacheI interface {
	Get(path string) string
	Set(path, hash string)
	Invalidate(path string)
}

// Factory creates a Skill instance from shared dependencies.
type Factory func(deps *Deps) Skill

// globalFactories holds factories registered via init() across skill packages.
var globalFactories = struct {
	factories map[string]Factory
	mu        sync.Mutex
}{
	factories: make(map[string]Factory),
}

// RegisterFactory adds a skill factory to the global registry. Call this in init().
func RegisterFactory(name string, factory Factory) {
	globalFactories.mu.Lock()
	defer globalFactories.mu.Unlock()
	globalFactories.factories[name] = factory
}

// ResetFactories clears all registered factories. For testing only.
func ResetFactories() {
	globalFactories.mu.Lock()
	defer globalFactories.mu.Unlock()
	globalFactories.factories = make(map[string]Factory)
}

// SkillRegistry manages skill registration and lookup.
type SkillRegistry struct {
	skills map[string]Skill
	mu     sync.RWMutex
}

// NewSkillRegistry creates a registry by instantiating all globally registered factories.
func NewSkillRegistry(deps *Deps) *SkillRegistry {
	globalFactories.mu.Lock()
	snapshot := make(map[string]Factory, len(globalFactories.factories))
	for k, v := range globalFactories.factories {
		snapshot[k] = v
	}
	globalFactories.mu.Unlock()

	r := &SkillRegistry{
		skills: make(map[string]Skill, len(snapshot)),
	}
	for name, factory := range snapshot {
		r.skills[name] = factory(deps)
	}
	return r
}

// Register adds a skill to the registry (manual registration for dynamic skills).
func (r *SkillRegistry) Register(skill Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name()] = skill
}

// Get returns a skill by name.
func (r *SkillRegistry) Get(name string) (Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", name)
	}
	return skill, nil
}

// List returns all registered skills sorted by name.
func (r *SkillRegistry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// ReadOnlySkills returns the set of skill names safe in Plan mode.
func (r *SkillRegistry) ReadOnlySkills() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := make(map[string]bool)
	for _, s := range r.skills {
		if mp, ok := s.(MetadataProvider); ok {
			if mp.Metadata().ReadOnly {
				m[s.Name()] = true
			}
		}
	}
	return m
}

// EssentialToolsForFree returns the set of skill names essential for free models.
func (r *SkillRegistry) EssentialToolsForFree() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := make(map[string]bool)
	for _, s := range r.skills {
		if mp, ok := s.(MetadataProvider); ok {
			if mp.Metadata().Essential {
				m[s.Name()] = true
			}
		}
	}
	return m
}

// CoreTools returns the set of skill names that are core for ALL models.
// Core tools are always exposed regardless of model tier, reducing tool bloat.
func (r *SkillRegistry) CoreTools() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := make(map[string]bool)
	for _, s := range r.skills {
		if mp, ok := s.(MetadataProvider); ok {
			if mp.Metadata().Core {
				m[s.Name()] = true
			}
		}
	}
	return m
}

// Execute runs a skill by name.
func (r *SkillRegistry) Execute(ctx context.Context, name string, input map[string]interface{}) (string, error) {
	skill, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return skill.Execute(ctx, input)
}
