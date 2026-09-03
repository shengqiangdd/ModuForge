# Task: Fix XML Tool Call Parser for MiMo Model

## Problem
MiMo model generates tool calls in XML-like format that the current parser cannot parse.

Actual MiMo output format:
```
<tool_call>
<function_name>write_file</function_name>
<parameters>
<path>module.prop</path>
<content>id=BatteryGuard\nname=BatteryGuard\n...</content>
</parameters>
</tool_call>
```

Multiple calls:
```
<tool_call>
<function_name>glob_search</function_name>
<parameters>
<pattern>module.prop</pattern>
</parameters>
</tool_call><tool_call>
<function_name>bash</function_name>
<parameters>
<command>ls -la</command>
</parameters>
</tool_call>
```

## File to modify
`/app/working/workspaces/default/ModuForge/backend/internal/agent/llm_stream.go`

## What to do
1. Update `extractXMLToolCalls` to also handle `<function_name>...</function_name>` and `<parameters><key>value</key></parameters>` format (HTML-like tags, not square bracket tags)
2. The function currently only handles `[tool_call start]...[tool_call end]` with `[function_name]...[/function_name]` format
3. Add support for `<tool_call>...</tool_call>` blocks with `<function_name>` and `<key>value</key>` inside `<parameters>`

## Verification
```
gofmt -e -w internal/agent/llm_stream.go
cd /app/working/workspaces/default/ModuForge/backend && /root/go/bin/go1.25.0 build ./...
```

Report git diff stat and stop.
