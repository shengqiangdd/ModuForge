#!/usr/bin/env python3
"""Check server status and deployment state."""
import paramiko

def check_server():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)
    
    print("=== Host /app/dist/ ===")
    stdin, stdout, stderr = ssh.exec_command('ls -la /app/dist/ 2>/dev/null || echo "dist not found"')
    print(stdout.read().decode())
    
    print("=== Container /app/dist/ ===")
    stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls -la /app/dist/ 2>/dev/null || echo "container dist not found"')
    print(stdout.read().decode())
    
    print("=== Container health ===")
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
    print(stdout.read().decode())
    
    print("=== Dockerfile COPY ===")
    stdin, stdout, stderr = ssh.exec_command('docker exec moduforge cat /app/Dockerfile 2>/dev/null || echo "no Dockerfile in container"')
    print(stdout.read().decode()[:2000])
    
    ssh.close()

if __name__ == '__main__':
    check_server()
