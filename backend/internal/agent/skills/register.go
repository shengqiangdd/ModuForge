package skills

import (
	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/storage"
)

// getStorage attempts to extract a StorageAdapter from Deps.
func getStorage(d *registry.Deps) storage.StorageAdapter {
	if d.Storage != nil {
		if s, ok := d.Storage.(storage.StorageAdapter); ok {
			return s
		}
	}
	return nil
}

func init() {
	// Core file operations (OpenCode-inspired)
	registry.RegisterFactory("grep_search", func(d *registry.Deps) registry.Skill {
		skill := NewGrepSearchSkillWithDB(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("glob_search", func(d *registry.Deps) registry.Skill {
		skill := NewGlobSearchSkillWithDB(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("edit_file", func(d *registry.Deps) registry.Skill {
		skill := NewEditFileSkillWithDB(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("read_file", func(d *registry.Deps) registry.Skill {
		skill := NewReadFileSkillWithDB(d.StoragePath+"/projects", d.DB)
		if d.FileHashCache != nil {
			skill.SetFileHashCache(d.FileHashCache)
		}
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("write_file", func(d *registry.Deps) registry.Skill {
		skill := NewWriteFileSkillWithDB(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("write_file_batch", func(d *registry.Deps) registry.Skill {
		skill := NewWriteFileBatchSkill(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	// P1-3: Batch edit for atomic multi-file editing
	registry.RegisterFactory("batch_edit_file", func(d *registry.Deps) registry.Skill {
		skill := NewBatchEditFileSkill(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("list_dir", func(d *registry.Deps) registry.Skill { return NewListDirSkill(d.DB) })
	registry.RegisterFactory("delete_file", func(d *registry.Deps) registry.Skill {
		skill := NewDeleteFileSkill(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("delete_dir", func(d *registry.Deps) registry.Skill {
		skill := NewDeleteDirSkill(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("move_file", func(d *registry.Deps) registry.Skill {
		skill := NewMoveFileSkill(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})

	// Shell execution
	registry.RegisterFactory("bash", func(d *registry.Deps) registry.Skill {
		skill := NewBashSkillWithDB(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})

	// Build & test
	registry.RegisterFactory("build_module", func(d *registry.Deps) registry.Skill {
		skill := NewBuildModuleSkillWithDB(d.StoragePath+"/projects", d.DB)
		if st := getStorage(d); st != nil {
			skill.WithStorage(st)
		}
		return skill
	})
	registry.RegisterFactory("syntax_checker", func(d *registry.Deps) registry.Skill {
		return NewSyntaxCheckerSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("test_module", func(d *registry.Deps) registry.Skill { return NewTestModuleSkill() })

	// Device testing (ADB integration)
	registry.RegisterFactory("device_test", func(d *registry.Deps) registry.Skill {
		return NewDeviceTestSkill(d.DB, d.StoragePath)
	})

	// Project management
	registry.RegisterFactory("agent_preset", func(d *registry.Deps) registry.Skill { return NewAgentPresetSkill(d.DB) })
	registry.RegisterFactory("self_evolve", func(d *registry.Deps) registry.Skill { return NewSelfEvolvingSkill(d.DB) })
	registry.RegisterFactory("pattern_learn", func(d *registry.Deps) registry.Skill { return NewPatternLearningSkill(d.DB) })

	// Enhanced Memory V2 (semantic search, tiered storage)
	registry.RegisterFactory("memory_v2", func(d *registry.Deps) registry.Skill {
		return &MemoryV2Skill{db: d.DB}
	})

	// Skill Manager (version control, dependencies, rollback)
	registry.RegisterFactory("skill_manager", func(d *registry.Deps) registry.Skill {
		return &SkillManagerSkill{db: d.DB}
	})

	// Self-Reflection (failure diagnosis, pattern detection)
	registry.RegisterFactory("self_reflection", func(d *registry.Deps) registry.Skill {
		return &SelfReflectionSkill{db: d.DB}
	})

	// Session Summary (compression, knowledge reuse)
	registry.RegisterFactory("session_summary", func(d *registry.Deps) registry.Skill {
		return &SessionSummarySkill{db: d.DB}
	})

	// ============================================
	// Code Quality & Development Workflow Skills
	// ============================================

	// Code Review (automated quality/security scan)
	registry.RegisterFactory("code_review", func(d *registry.Deps) registry.Skill {
		return NewCodeReviewSkill()
	})

	// Test Generator (unit test generation)
	registry.RegisterFactory("test_generator", func(d *registry.Deps) registry.Skill {
		return NewTestGeneratorSkill()
	})

	// Doc Generator (API documentation)
	registry.RegisterFactory("doc_generator", func(d *registry.Deps) registry.Skill {
		return NewDocGeneratorSkill()
	})

	// Git Operations (version control)
	registry.RegisterFactory("git_ops", func(d *registry.Deps) registry.Skill {
		return NewGitOpsSkill(d.StoragePath + "/projects")
	})

	// Dependency Graph (structured dependency analysis)
	registry.RegisterFactory("dependency_graph", func(d *registry.Deps) registry.Skill {
		return NewDependencyGraphSkill()
	})

	// Refactor (cross-file refactoring)
	registry.RegisterFactory("refactor", func(d *registry.Deps) registry.Skill {
		return NewRefactorSkill(d.StoragePath + "/projects")
	})

	// ============================================
	// Security & Quality Analysis Skills
	// ============================================

	// Security Scan (static security analysis)
	registry.RegisterFactory("security_scan", func(d *registry.Deps) registry.Skill {
		return NewSecurityScanSkill()
	})

	// Code Quality (metrics and analysis)
	registry.RegisterFactory("code_quality", func(d *registry.Deps) registry.Skill {
		return NewCodeQualitySkill()
	})

	// Profiling (performance guidance)
	registry.RegisterFactory("profiling", func(d *registry.Deps) registry.Skill {
		return NewProfilingSkill()
	})
}
