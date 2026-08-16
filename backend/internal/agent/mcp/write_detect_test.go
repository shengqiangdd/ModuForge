package mcp

import "testing"

func TestIsWriteTool(t *testing.T) {
	cases := []struct {
		name string
		tool Tool
		want bool
	}{
		{"read: get_issue", Tool{Name: "get_issue"}, false},
		{"read: list_repositories", Tool{Name: "list_repositories"}, false},
		{"read: search_repositories", Tool{Name: "search_repositories"}, false},
		{"read: get_me", Tool{Name: "get_me"}, false},
		{"read: download_archive (exception)", Tool{Name: "download_archive"}, false},
		{"write: create_issue", Tool{Name: "create_issue"}, true},
		{"write: update_issue", Tool{Name: "update_issue"}, true},
		{"write: delete_issue", Tool{Name: "delete_issue"}, true},
		{"write: push_files", Tool{Name: "push_files"}, true},
		{"write: merge_pull_request", Tool{Name: "merge_pull_request"}, true},
		{"write: add_issue_labels", Tool{Name: "add_issue_labels"}, true},
		{"write: remove_issue_labels", Tool{Name: "remove_issue_labels"}, true},
		{"write: close_issue", Tool{Name: "close_issue"}, true},
		{"read-ish: created_at (no underscore prefix)", Tool{Name: "created_at"}, false},
	}
	for _, tc := range cases {
		if got := IsWriteTool(tc.tool); got != tc.want {
			t.Errorf("%s: IsWriteTool(%q) = %v, want %v", tc.name, tc.tool.Name, got, tc.want)
		}
	}
}
