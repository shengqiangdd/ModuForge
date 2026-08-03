# ModuForge Memory & Skill Enhancements

## Overview

Enhanced memory and skill management systems for ModuForge Agent, inspired by modern AI agent frameworks like OpenCode, Mem0, and OpenClaw.

## Memory V2 System

### Features

1. **Tiered Storage**
   - `short_term`: 7-day expiry, auto-promoted on access/importance
   - `long_term`: 90-day expiry, for frequently accessed memories
   - `archive`: No expiry, for historical reference

2. **Semantic Search**
   - Full-text search using SQLite FTS5
   - Keyword and tag-based filtering
   - Relevance ranking

3. **Memory Categories**
   - `episodic`: Events and experiences
   - `semantic`: Knowledge and facts
   - `procedural`: How-to guides and procedures

4. **Automatic Consolidation**
   - Short-term memories accessed 3+ times → long-term
   - Long-term memories unused for 30 days → archive
   - Expired short-term memories auto-deleted

### API Usage

```javascript
// Store a memory
memory_v2({
  action: "store",
  content: "User prefers REST API over GraphQL",
  category: "semantic",
  importance: 8,
  tags: ["api", "preference"]
})

// Recall memories by category
memory_v2({
  action: "recall",
  category: "semantic",
  tier: "long_term"
})

// Search by meaning
memory_v2({
  action: "search",
  query: "API design patterns"
})

// Get statistics
memory_v2({
  action: "stats"
})

// Consolidate memories
memory_v2({
  action: "consolidate"
})

// Forget a memory
memory_v2({
  action: "forget",
  entry_id: "mem_user_1234567890"
})

// Promote to long-term
memory_v2({
  action: "promote",
  entry_id: "mem_user_1234567890"
})
```

## Skill Manager System

### Features

1. **Version Control**
   - Create named versions with changelogs
   - Track version history
   - Rollback to any previous version

2. **Dependency Management**
   - Declare dependencies on other skills
   - Version constraints (min/max)
   - Optional vs required dependencies
   - Compatibility checking

3. **Skill Lifecycle**
   - Activate/deactivate skills
   - Clone existing skills
   - Export/import skill configurations

4. **Snapshot & Rollback**
   - Automatic snapshots on version creation
   - One-click rollback to any version

### API Usage

```javascript
// Create a new version
skill_manager({
  action: "version",
  skill_name: "code_review",
  version: "2.0.0",
  changelog: "Added security scanning",
  config: { strict_mode: true },
  dependencies: ["security_scanner"]
})

// List versions
skill_manager({
  action: "versions",
  skill_name: "code_review"
})

// Manage dependencies
skill_manager({
  action: "dependencies",
  skill_name: "code_review",
  dep_action: "add",
  depends_on: "security_scanner",
  min_version: "1.0.0"
})

// Check compatibility
skill_manager({
  action: "check_compatibility",
  skill_name: "code_review"
})

// Rollback to version
skill_manager({
  action: "rollback",
  skill_name: "code_review",
  target_version: "1.0.0"
})

// Clone a skill
skill_manager({
  action: "clone",
  skill_name: "code_review",
  new_name: "security_review"
})

// Export skill
skill_manager({
  action: "export",
  skill_name: "code_review"
})

// Import skill
skill_manager({
  action: "import",
  skill_name: "code_review",
  version: "1.0.0",
  config: "{...}"
})
```

## Integration with Agent System

### System Prompt Updates

The Agent system prompt now includes:
- Memory V2 usage examples
- Skill Manager usage examples
- Best practices for memory consolidation
- Version control workflows

### Example Workflow

```
1. Store important decision
   memory_v2({action: "store", content: "Use REST API", category: "semantic", importance: 8})

2. Later, recall related memories
   memory_v2({action: "search", query: "API architecture"})

3. Create skill version
   skill_manager({action: "version", skill_name: "api_builder", version: "1.0.0"})

4. Check dependencies
   skill_manager({action: "check_compatibility", skill_name: "api_builder"})

5. Rollback if needed
   skill_manager({action: "rollback", skill_name: "api_builder", target_version: "0.9.0"})
```

## Performance Considerations

- **Memory Search**: FTS5 index enables fast full-text search
- **Lazy Table Creation**: Tables created on first use
- **Automatic Cleanup**: Expired memories auto-deleted
- **Access Tracking**: Frequently accessed memories auto-promoted

## Migration

No migration required. New tables are created automatically on first use:
- `memory_v2`: Main memory storage
- `memory_v2_fts`: Full-text search index
- `skill_versions`: Version history
- `skill_dependencies`: Dependency graph
- `skill_snapshots`: Rollback snapshots