"""Find the /api/v1/ai/stream handler and understand the routing"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Find the route registration
print('=== Route registration ===')
stdin, stdout, stderr = ssh.exec_command('grep -rn "ai/stream\\|ai/chat\\|/api/v1/ai\\|agent/run\\|agent/stream" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/ 2>/dev/null | head -20')
print(stdout.read().decode('utf-8', errors='replace'))

# 2. Find the main.go or router setup
print('\n=== Router setup ===')
stdin, stdout, stderr = ssh.exec_command('find /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app -name "main.go" -o -name "router.go" -o -name "routes.go" 2>/dev/null | head -5')
out = stdout.read().decode('utf-8', errors='replace').strip()
print(out)
for f in out.split('\n'):
    if f.strip():
        stdin, stdout, stderr = ssh.exec_command(f'grep -n "ai\\|agent\\|stream" {f} 2>/dev/null | head -20')
        print(stdout.read().decode('utf-8', errors='replace'))

# 3. Check the Chat function in ai.go
print('\n=== Chat function (line 225+) ===')
stdin, stdout, stderr = ssh.exec_command('sed -n "225,280p" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/ai.go 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

# 4. Check if there's a streaming chat handler
print('\n=== Streaming handler ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "stream\\|Stream\\|SSE\\|sse\\|event-stream" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/ai.go 2>/dev/null | head -20')
print(stdout.read().decode('utf-8', errors='replace'))

# 5. Check the AIService
print('\n=== AIService ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "func.*Chat\\|func.*Stream\\|func.*Generate\\|func.*Send" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/service/ai*.go 2>/dev/null | head -20')
print(stdout.read().decode('utf-8', errors='replace'))

# 6. Check the actual /api/v1/ai/stream implementation
print('\n=== /api/v1/ai/stream implementation ===')
stdin, stdout, stderr = ssh.exec_command('grep -rn "ai/stream" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/ 2>/dev/null | head -10')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
