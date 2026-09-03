# Task: Fix MiMo XML Tool Call Parsing

## Problem
MiMo model (xiaomi/mimo-v2.5) generates tool calls in XML-like format that the backend cannot parse.

## File to modify
`/app/working/workspaces/default/ModuForge/backend/internal/agent/llm_stream.go`

## What to do
1. Add a new function `extractXMLToolCalls` that parses XML-like tool call format
2. Call it from `extractTextToolCalls` when no other format matches

## MiMo XML format examples

Pattern 1:
[tool_call start]
[function_name]write_file[/function_name]
[parameters]
[path]module.prop[/path]
[content]id=batteryguard[/content]
[/parameters]
[tool_call end]

Pattern 2 (multiple calls):
[tool_call start]
[function_name]bash[/function_name]
[parameters]
[command]ls -la[/command]
[/parameters]
[tool_call end]
[tool_call start]
[function_name]read_file[/function_name]
[parameters]
[path]module.prop[/path]
[/parameters]
[tool_call end]

## Implementation

See the existing `extractInlineToolCalls` and `extractTextToolCalls` functions for reference.
The new function should:
1. Find all blocks between [tool_call start] and [tool_call end]
2. Extract function_name using [function_name]...[/function_name] tags
3. Extract parameters using [parameters]...[/parameters] blocks
4. Parse [key]value[/key] pairs inside parameters
5. Return []LLMToolCall slice

Then in extractTextToolCalls, add call to extractXMLToolCalls before extractInlineToolCalls.

## Verify
gofmt -e -w internal/agent/llm_stream.go
cd /app/working/workspaces/default/ModuForge/backend && /root/go/bin/go1.25.0 build ./...
