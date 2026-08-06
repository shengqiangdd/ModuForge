package skills

import (
	"github.com/moduforge/backend/internal/agent/registry"
)

func init() {
	// Core file operations (OpenCode-inspired)
	registry.RegisterFactory("grep_search", func(d *registry.Deps) registry.Skill {
		return NewGrepSearchSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("glob_search", func(d *registry.Deps) registry.Skill {
		return NewGlobSearchSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("edit_file", func(d *registry.Deps) registry.Skill {
		return NewEditFileSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("read_file", func(d *registry.Deps) registry.Skill {
		skill := NewReadFileSkillWithDB(d.StoragePath+"/projects", d.DB)
		if d.FileHashCache != nil {
			skill.SetFileHashCache(d.FileHashCache)
		}
		return skill
	})
	registry.RegisterFactory("write_file", func(d *registry.Deps) registry.Skill {
		return NewWriteFileSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("write_file_batch", func(d *registry.Deps) registry.Skill {
		return NewWriteFileBatchSkill(d.StoragePath+"/projects", d.DB)
	})
	// P1-3: Batch edit for atomic multi-file editing
	registry.RegisterFactory("batch_edit_file", func(d *registry.Deps) registry.Skill {
		return NewBatchEditFileSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("list_dir", func(d *registry.Deps) registry.Skill { return NewListDirSkill(d.DB) })
	registry.RegisterFactory("delete_file", func(d *registry.Deps) registry.Skill {
		return NewDeleteFileSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("delete_dir", func(d *registry.Deps) registry.Skill {
		return NewDeleteDirSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("move_file", func(d *registry.Deps) registry.Skill {
		return NewMoveFileSkill(d.StoragePath+"/projects", d.DB)
	})

	// Shell execution
	registry.RegisterFactory("bash", func(d *registry.Deps) registry.Skill {
		return NewBashSkillWithDB(d.StoragePath+"/projects", d.DB)
	})

	// Build & test
	registry.RegisterFactory("build_module", func(d *registry.Deps) registry.Skill {
		return NewBuildModuleSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("syntax_checker", func(d *registry.Deps) registry.Skill {
		return NewSyntaxCheckerSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("test_module", func(d *registry.Deps) registry.Skill { return NewTestModuleSkill() })

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
}
