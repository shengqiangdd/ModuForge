package service

import (
	"encoding/json"
	"fmt"

	"github.com/moduforge/backend/internal/domain"
)

func (s *SQLiteMarketService) GetModuleDependencies(slug string) ([]domain.ModuleDependency, error) {
	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}
	if mod.Dependencies == "" || mod.Dependencies == "[]" {
		return []domain.ModuleDependency{}, nil
	}
	var deps []domain.ModuleDependency
	if err := json.Unmarshal([]byte(mod.Dependencies), &deps); err != nil {
		return nil, fmt.Errorf("parse dependencies: %w", err)
	}
	return deps, nil
}

func (s *SQLiteMarketService) resolveDeps(slug string, visited map[string]bool, level int) (*domain.DependencyNode, error) {
	if visited[slug] {
		return nil, fmt.Errorf("circular dependency detected: %s", slug)
	}
	visited[slug] = true

	mod, err := s.GetModule(slug)
	if err != nil {
		return nil, err
	}

	node := &domain.DependencyNode{Module: mod, Level: level}

	deps, err := s.GetModuleDependencies(slug)
	if err != nil {
		return node, nil
	}

	for _, dep := range deps {
		if dep.Optional {
			continue
		}
		child, err := s.resolveDeps(dep.ID, visited, level+1)
		if err != nil {
			return node, fmt.Errorf("resolve dep %s: %w", dep.ID, err)
		}
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	return node, nil
}

func (s *SQLiteMarketService) ResolveDependencies(slug string) (*domain.DependencyNode, error) {
	visited := make(map[string]bool)
	return s.resolveDeps(slug, visited, 0)
}

func (s *SQLiteMarketService) CheckDependencyConflicts(slug string) ([]domain.Conflict, error) {
	var conflicts []domain.Conflict

	deps, err := s.GetModuleDependencies(slug)
	if err != nil {
		return nil, err
	}

	visited := make(map[string]bool)
	visited[slug] = true

	for _, dep := range deps {
		if err := s.checkDepCycle(dep.ID, visited, &conflicts, slug); err != nil {
			conflicts = append(conflicts, domain.Conflict{
				ModuleA: slug,
				ModuleB: dep.ID,
				Type:    "circular",
				Detail:  err.Error(),
			})
		}
	}

	for _, dep := range deps {
		depMod, err := s.GetModule(dep.ID)
		if err != nil {
			continue
		}
		if dep.MinVersion != "" && depMod.Version != "" {
			if cmp := compareVersions(depMod.Version, dep.MinVersion); cmp < 0 {
				conflicts = append(conflicts, domain.Conflict{
					ModuleA: slug,
					ModuleB: dep.ID,
					Type:    "version_mismatch",
					Detail: fmt.Sprintf("需要 %s >= %s，当前版本 %s", dep.ID, dep.MinVersion, depMod.Version),
				})
			}
		}
	}

	return conflicts, nil
}

func (s *SQLiteMarketService) checkDepCycle(slug string, visited map[string]bool, conflicts *[]domain.Conflict, rootSlug string) error {
	if visited[slug] {
		return fmt.Errorf("circular dependency: %s", slug)
	}
	visited[slug] = true

	deps, err := s.GetModuleDependencies(slug)
	if err != nil {
		visited[slug] = false
		return nil
	}

	for _, dep := range deps {
		if err := s.checkDepCycle(dep.ID, visited, conflicts, rootSlug); err != nil {
			visited[slug] = false
			return err
		}
	}

	visited[slug] = false
	return nil
}
