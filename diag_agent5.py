"""Read the actual runner.go source from overlay2 and understand agent engine"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Read runner.go from overlay2 (the most recent build)
print('=== RUNNER.GO (from overlay2) ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -200')
print(stdout.read().decode())

# 2. Read agent.md
print('\n=== AGENT.MD ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/prompts/agent.md 2>/dev/null')
print(stdout.read().decode())

# 3. Read act.md
print('\n=== ACT.MD ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/prompts/act.md 2>/dev/null')
print(stdout.read().decode())

# 4. Check the runner.go for iteration limits and tool execution
print('\n=== Runner.go: limits and tools ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | grep -n "maxIter\\|MaxIter\\|timeout\\|Timeout\\|tool_call\\|ToolCall\\|executeTool\\|ExecuteTool\\|MaxResult\\|maxResult\\|maxResultLen\\|forceAnswer\\|ForceAnswer\\|stall\\|Stall\\|iteration\\|Iteration" | head -30')
print(stdout.read().decode())

# 5. Read the full runner.go
print('\n=== RUNNER.GO (full) ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null')
content = stdout.read().decode()
print(content[:5000])
if len(content) > 5000:
    print(f'\n... ({len(content)} total chars)')

ssh.close()
