package mcp

import "strings"

// writePrefixes are tool-name prefixes that indicate a write/mutation
// operation. GitHub MCP and most well-behaved MCP servers follow the
// "verb_noun" convention, so prefix matching is reliable:
//   - get_*/list_*/search_*/read_*/download_* → read-only
//   - create_*/update_*/delete_*/push_*/add_*/remove_*/merge_* → write
//
// This mirrors the permission heuristics used by Claude Code / OpenCode to
// classify tools for their approval flows. The "_" suffix requirement avoids
// false positives on names like "created_at" or "update_info" (read-ish).
var writePrefixes = []string{
	"create_", "update_", "delete_", "push_", "write_", "add_", "remove_",
	"edit_", "rename_", "merge_", "close_", "open_", "set_", "unlock_",
	"lock_", "approve_", "reject_", "comment_", "mark_", "convert_", "move_",
	"copy_", "send_", "post_", "put_", "patch_", "cancel_", "commit_",
	"fork_", "invite_", "subscribe_", "unsubscribe_", "release_", "publish_",
	"import_", "export_", "assign_", "unassign_", "enable_", "disable_",
	"start_", "stop_", "restart_", "transfer_", "replace_", "revert_",
	"upload_", "download_", "generate_",
}

// readOnlyExact are tool names that are read-only even though their prefix
// appears in writePrefixes (exceptions).
var readOnlyExact = map[string]bool{
	"download_archive": true, // downloading a tarball has no side effects
	"download_artifact": true,
	"get":               true,
	"list":              true,
	"search":            true,
}

// IsWriteTool classifies an MCP tool as a write (side-effect) operation.
func IsWriteTool(tool Tool) bool {
	name := strings.ToLower(tool.Name)
	if readOnlyExact[name] {
		return false
	}
	for _, p := range writePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
