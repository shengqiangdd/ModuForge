package skills

import (
	"github.com/moduforge/backend/internal/agent/registry"
)

func init() {
	// No-arg skills
	registry.RegisterFactory("think", func(d *registry.Deps) registry.Skill { return NewThinkSkill() })
	registry.RegisterFactory("web_search", func(d *registry.Deps) registry.Skill { return NewWebSearchSkill() })
	registry.RegisterFactory("validate", func(d *registry.Deps) registry.Skill { return NewValidateSkill() })
	registry.RegisterFactory("lint_code", func(d *registry.Deps) registry.Skill { return NewLintCodeSkill() })
	registry.RegisterFactory("review_code", func(d *registry.Deps) registry.Skill { return NewReviewCodeSkill() })
	registry.RegisterFactory("gen_docs", func(d *registry.Deps) registry.Skill { return NewGenDocsSkill() })
	registry.RegisterFactory("check_compat", func(d *registry.Deps) registry.Skill { return NewCheckCompatSkill() })
	registry.RegisterFactory("profile_code", func(d *registry.Deps) registry.Skill { return NewProfileCodeSkill() })
	registry.RegisterFactory("match_template", func(d *registry.Deps) registry.Skill { return NewMatchTemplateSkill() })
	registry.RegisterFactory("gather_requirements", func(d *registry.Deps) registry.Skill { return NewGatherRequirementsSkill() })
	registry.RegisterFactory("test_module", func(d *registry.Deps) registry.Skill { return NewTestModuleSkill() })
	registry.RegisterFactory("regression_test", func(d *registry.Deps) registry.Skill { return NewRegressionTestSkill() })

	// OpenCode-inspired tools
	registry.RegisterFactory("grep_search", func(d *registry.Deps) registry.Skill {
		return NewGrepSearchSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("glob_search", func(d *registry.Deps) registry.Skill {
		return NewGlobSearchSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("edit_file", func(d *registry.Deps) registry.Skill {
		return NewEditFileSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("bash", func(d *registry.Deps) registry.Skill {
		return NewBashSkillWithDB(d.StoragePath+"/projects", d.DB)
	})

	// DB-only skills
	registry.RegisterFactory("read_file", func(d *registry.Deps) registry.Skill { return NewReadFileSkill(d.DB) })
	registry.RegisterFactory("list_dir", func(d *registry.Deps) registry.Skill { return NewListDirSkill(d.DB) })
	registry.RegisterFactory("memory_manager", func(d *registry.Deps) registry.Skill { return NewMemoryManagerSkill() })
	registry.RegisterFactory("self_evolve", func(d *registry.Deps) registry.Skill { return NewSelfEvolvingSkill(d.DB) })
	registry.RegisterFactory("pattern_learning", func(d *registry.Deps) registry.Skill { return NewPatternLearningSkill(d.DB) })
	registry.RegisterFactory("agent_preset", func(d *registry.Deps) registry.Skill { return NewAgentPresetSkill(d.DB) })

	// LLM skills
	registry.RegisterFactory("generate_code", func(d *registry.Deps) registry.Skill {
		return NewGenerateCodeSkillWithClient(d.LLMApiKey, d.LLMEndpoint, d.LLMModel, d.HTTPClient)
	})
	registry.RegisterFactory("code_pipeline", func(d *registry.Deps) registry.Skill {
		return NewCodePipelineSkillWithClient(d.LLMApiKey, d.LLMEndpoint, d.LLMModel, d.HTTPClient)
	})

	// Filesystem skills (need project path)
	registry.RegisterFactory("write_file", func(d *registry.Deps) registry.Skill {
		return NewWriteFileSkillWithDB(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("write_file_batch", func(d *registry.Deps) registry.Skill {
		return NewWriteFileBatchSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("create_dir", func(d *registry.Deps) registry.Skill {
		return NewCreateDirSkill(d.StoragePath + "/projects")
	})
	registry.RegisterFactory("delete_file", func(d *registry.Deps) registry.Skill {
		return NewDeleteFileSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("delete_dir", func(d *registry.Deps) registry.Skill {
		return NewDeleteDirSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("move_file", func(d *registry.Deps) registry.Skill {
		return NewMoveFileSkill(d.StoragePath+"/projects", d.DB)
	})
	registry.RegisterFactory("build_module", func(d *registry.Deps) registry.Skill {
		return NewBuildModuleSkillWithDB(d.StoragePath+"/projects", d.DB)
	})

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
