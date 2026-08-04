package registry

import (
	"context"
	"testing"
)

// mockSkill implements Skill for testing.
type mockSkill struct {
	name string
}

func (m *mockSkill) Name() string        { return m.name }
func (m *mockSkill) Description() string { return "mock " + m.name }
func (m *mockSkill) Execute(_ context.Context, _ map[string]interface{}) (string, error) {
	return "ok:" + m.name, nil
}

// mockMetadata implements MetadataProvider.
type mockMetadata struct {
	mockSkill
	meta SkillMeta
}

func (m *mockMetadata) Metadata() SkillMeta { return m.meta }

func TestNewSkillRegistry_AutoRegistration(t *testing.T) {
	ResetFactories()
	defer ResetFactories()

	RegisterFactory("test_a", func(d *Deps) Skill { return &mockSkill{name: "test_a"} })
	RegisterFactory("test_b", func(d *Deps) Skill { return &mockSkill{name: "test_b"} })

	registry := NewSkillRegistry(&Deps{StoragePath: "/tmp"})

	skills := registry.List()
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name()] = true
	}
	if !names["test_a"] || !names["test_b"] {
		t.Fatalf("expected test_a and test_b, got %v", names)
	}
}

func TestNewSkillRegistry_FactoryReceivesDeps(t *testing.T) {
	ResetFactories()
	defer ResetFactories()

	var receivedDeps *Deps
	RegisterFactory("dep_test", func(d *Deps) Skill {
		receivedDeps = d
		return &mockSkill{name: "dep_test"}
	})

	deps := &Deps{StoragePath: "/test/path", LLMApiKey: "sk-123"}
	registry := NewSkillRegistry(deps)

	_, _ = registry.Get("dep_test")

	if receivedDeps == nil {
		t.Fatal("factory was not called")
	}
	if receivedDeps.StoragePath != "/test/path" {
		t.Errorf("expected StoragePath=/test/path, got %s", receivedDeps.StoragePath)
	}
	if receivedDeps.LLMApiKey != "sk-123" {
		t.Errorf("expected LLMApiKey=sk-123, got %s", receivedDeps.LLMApiKey)
	}
}

func TestReadOnlySkills_DerivedFromMetadata(t *testing.T) {
	ResetFactories()
	defer ResetFactories()

	RegisterFactory("ro_skill", func(d *Deps) Skill {
		return &mockMetadata{
			mockSkill: mockSkill{name: "ro_skill"},
			meta:      SkillMeta{ReadOnly: true},
		}
	})
	RegisterFactory("rw_skill", func(d *Deps) Skill {
		return &mockMetadata{
			mockSkill: mockSkill{name: "rw_skill"},
			meta:      SkillMeta{ReadOnly: false},
		}
	})

	registry := NewSkillRegistry(&Deps{})
	ro := registry.ReadOnlySkills()

	if !ro["ro_skill"] {
		t.Error("expected ro_skill to be read-only")
	}
	if ro["rw_skill"] {
		t.Error("expected rw_skill to NOT be read-only")
	}
}

func TestEssentialToolsForFree_DerivedFromMetadata(t *testing.T) {
	ResetFactories()
	defer ResetFactories()

	RegisterFactory("essential", func(d *Deps) Skill {
		return &mockMetadata{
			mockSkill: mockSkill{name: "essential"},
			meta:      SkillMeta{Essential: true},
		}
	})
	RegisterFactory("optional", func(d *Deps) Skill {
		return &mockMetadata{
			mockSkill: mockSkill{name: "optional"},
			meta:      SkillMeta{Essential: false},
		}
	})

	registry := NewSkillRegistry(&Deps{})
	essential := registry.EssentialToolsForFree()

	if !essential["essential"] {
		t.Error("expected essential to be in free tools")
	}
	if essential["optional"] {
		t.Error("expected optional to NOT be in free tools")
	}
}

func TestExecute(t *testing.T) {
	ResetFactories()
	defer ResetFactories()

	RegisterFactory("exec_test", func(d *Deps) Skill {
		return &mockSkill{name: "exec_test"}
	})

	registry := NewSkillRegistry(&Deps{})
	result, err := registry.Execute(context.Background(), "exec_test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok:exec_test" {
		t.Errorf("expected 'ok:exec_test', got '%s'", result)
	}

	// Non-existent skill
	_, err = registry.Execute(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestManualRegister(t *testing.T) {
	ResetFactories()
	defer ResetFactories()

	registry := NewSkillRegistry(&Deps{})
	registry.Register(&mockSkill{name: "manual"})

	result, err := registry.Execute(context.Background(), "manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok:manual" {
		t.Errorf("expected 'ok:manual', got '%s'", result)
	}
}

func TestListSorted(t *testing.T) {
	ResetFactories()
	defer ResetFactories()

	RegisterFactory("zebra", func(d *Deps) Skill { return &mockSkill{name: "zebra"} })
	RegisterFactory("alpha", func(d *Deps) Skill { return &mockSkill{name: "alpha"} })
	RegisterFactory("mid", func(d *Deps) Skill { return &mockSkill{name: "mid"} })

	registry := NewSkillRegistry(&Deps{})
	list := registry.List()

	if len(list) != 3 {
		t.Fatalf("expected 3, got %d", len(list))
	}
	if list[0].Name() != "alpha" || list[1].Name() != "mid" || list[2].Name() != "zebra" {
		t.Errorf("expected sorted [alpha, mid, zebra], got [%s, %s, %s]",
			list[0].Name(), list[1].Name(), list[2].Name())
	}
}
