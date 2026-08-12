"""Check frontend JS to understand what API it calls"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Find frontend JS files
print('=== Frontend JS files ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls /app/dist/assets/*.js 2>/dev/null | head -10')
print(stdout.read().decode('utf-8', errors='replace'))

# 2. Search for API calls in frontend
print('\n=== Frontend API calls (ai/stream, agent/run) ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge grep -l "ai/stream\\|agent/run\\|ai/chat" /app/dist/assets/*.js 2>/dev/null')
files = stdout.read().decode('utf-8', errors='replace').strip().split('\n')
for f in files:
    if f.strip():
        stdin, stdout, stderr = ssh.exec_command(f'docker exec moduforge grep -o "[^"]*ai/stream[^"]*\\|[^"]*agent/run[^"]*\\|[^"]*ai/chat[^"]*" {f} 2>/dev/null | head -10')
        print(f'{f}:')
        print(stdout.read().decode('utf-8', errors='replace'))

# 3. Check the AIStreamHandler.StreamChat function
print('\n=== AIStreamHandler.StreamChat ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/aistream.go 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
