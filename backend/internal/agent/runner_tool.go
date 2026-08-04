package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// toolResult pairs an executed tool call with its returned content.
type toolResult struct {
	tc     LLMToolCall
	result string
}

// appendToolResultToList appends a tool result to the results slice.
func appendToolResultToList(results *[]toolResult, tc LLMToolCall, result string) {
	*results = append(*results, toolResult{tc: tc, result: result})
}

// truncateResultForModel truncates a tool result to fit the model tier's
// context budget. Free models get a smarter summarization that preserves
// key facts at a smaller size.
func truncateResultForModel(result, skillName string, tier ModelTier, maxLen int) string {
	if len(result) > maxLen {
		if tier == TierFree {
			return smartSummarizeResult(result, skillName, maxLen)
		}
		return summarizeResult(result, maxLen)
	}
	return result
}

// appendResultsToConversation adds executed tool results back into the
// conversation, merging consecutive write_file results to save tokens
// (Optimization 18).
func (r *AgentRunner) appendResultsToConversation(conversation []map[string]interface{}, sessionID string, results []toolResult, uniqueOps map[string]bool, totalToolCalls int) []map[string]interface{} {
	var pendingWriteFiles []struct {
		tc     LLMToolCall
		result string
		path   string
	}
	flushWriteFiles := func() {
		if len(pendingWriteFiles) == 0 {
			return
		}
		if len(pendingWriteFiles) == 1 {
			// Single write_file — add as-is
			wf := pendingWriteFiles[0]
			conversation = r.appendToolResult(conversation, sessionID, wf.tc.ID, wf.result)
		} else {
			// Multiple consecutive write_files — merge into summary
			var paths []string
			for _, wf := range pendingWriteFiles {
				paths = append(paths, wf.path)
			}
			merged := fmt.Sprintf("✅ Successfully wrote %d files: %s",
				len(pendingWriteFiles), strings.Join(paths, ", "))
			// Add one merged result for the first tool_call_id, skip the rest
			conversation = r.appendToolResult(conversation, sessionID, pendingWriteFiles[0].tc.ID, merged)
			// For the remaining write_file calls, add empty acknowledges
			for _, wf := range pendingWriteFiles[1:] {
				conversation = r.appendToolResult(conversation, sessionID, wf.tc.ID, "(merged into previous result)")
			}
			debugLog("merged %d write_file results into one message (saved ~%d tokens)",
				len(pendingWriteFiles), len(pendingWriteFiles)*50)
		}
		pendingWriteFiles = nil
	}

	for _, res := range results {
		isWrite := res.tc.Function.Name == "write_file"
		if isWrite {
			// Extract path from the result or tool call
			path := ""
			if idx := strings.Index(res.result, "[project_id:"); idx >= 0 {
				// Result contains project_id prefix, path is in the tool call
				path = res.tc.Function.Arguments
			}
			// Try to get path from arguments
			var args map[string]interface{}
			if json.Unmarshal([]byte(res.tc.Function.Arguments), &args) == nil {
				if p, ok := args["path"].(string); ok {
					path = p
				}
			}
			if path == "" {
				path = fmt.Sprintf("file_%d", len(pendingWriteFiles))
			}
			pendingWriteFiles = append(pendingWriteFiles, struct {
				tc     LLMToolCall
				result string
				path   string
			}{tc: res.tc, result: res.result, path: path})
			continue
		}
		// Non-write tool: flush pending writes first
		flushWriteFiles()
		conversation = r.appendToolResult(conversation, sessionID, res.tc.ID, res.result)

		debugLog("tool=%s resultLen=%d uniqueOps=%d totalCalls=%d",
			res.tc.Function.Name, len(res.result), len(uniqueOps), totalToolCalls)
	}
	// Flush any remaining write_file results
	flushWriteFiles()
	return conversation
}
