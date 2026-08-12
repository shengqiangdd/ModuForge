"""Read the actual LLM call function and check why agent doesn't use tools"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Read llm.go to understand how LLM calls work
print('=== llm.go (first 100 lines) ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/llm.go 2>/dev/null | head -100')
print(stdout.read().decode('utf-8', errors='replace'))

# 2. Read runner_llm.go - the actual LLM call
print('\n=== runner_llm.go (first 150 lines) ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner_llm.go 2>/dev/null | head -150')
print(stdout.read().decode('utf-8', errors='replace'))

# 3. Check the system prompt construction
print('\n=== System prompt construction ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "systemPrompt\|system_prompt\|buildPrompt\|BuildPrompt\|getPrompt\|GetPrompt" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -20')
lines = stdout.read().decode('utf-8', errors='replace').strip()
print(lines)

# 4. Check how tools are sent to LLM
print('\n=== Tool definitions format ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "func.*getToolDef\|func.*buildTool\|func.*ToolDefs\|func.*toolDefs" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -10')
lines = stdout.read().decode('utf-8', errors='replace').strip()
print(lines)
if lines:
    first_line = int(lines.split('\n')[0].split(':')[0])
    stdin, stdout, stderr = ssh.exec_command(f'sed -n "{first_line},{first_line+50}p" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null')
    print(stdout.read().decode('utf-8', errors='replace'))

# 5. Check if there's a "simple mode" that bypasses the agent loop
print('\n=== Simple/AI chat handler ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "func.*Chat\|func.*Stream\|func.*AI\|/api/v1/ai" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/*.go 2>/dev/null | head -20')
print(stdout.read().decode('utf-8', errors='replace'))

# 6. Check the AI handler
print('\n=== AI handler ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/ai.go 2>/dev/null | head -100')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
