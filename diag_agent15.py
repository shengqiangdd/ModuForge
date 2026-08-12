"""Find the frontend JS and check what API it calls"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. List all frontend assets
print('=== Frontend assets ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge find /app/dist -name "*.js" -o -name "*.ts" 2>/dev/null | head -20')
print(stdout.read().decode('utf-8', errors='replace'))

# 2. Search for API calls in all frontend files
print('\n=== Search for ai/stream in frontend ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge grep -rn "ai/stream\\|agent/run\\|ai/chat" /app/dist/ 2>/dev/null | head -20')
print(stdout.read().decode('utf-8', errors='replace'))

# 3. Check if the frontend has source maps
print('\n=== Source maps ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge find /app/dist -name "*.map" 2>/dev/null | head -5')
print(stdout.read().decode('utf-8', errors='replace'))

# 4. Check the Svelte source files
print('\n=== Svelte source files ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge find /app/dist -name "*.svelte.js" -o -name "*.svelte" 2>/dev/null | head -10')
print(stdout.read().decode('utf-8', errors='replace'))

# 5. Check the frontend build output
print('\n=== Frontend build output ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls -la /app/dist/assets/ 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

# 6. Read a sample JS file to understand the API calls
print('\n=== Sample JS file content ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge head -100 /app/dist/assets/$(docker exec moduforge ls /app/dist/assets/ | grep "index.*js$" | head -1) 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
