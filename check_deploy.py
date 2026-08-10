#!/usr/bin/env python3
"""Check deployment status."""
import paramiko

def check():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)
    
    print("=== /app/dist/ structure ===")
    stdin, stdout, stderr = ssh.exec_command('docker exec moduforge find /app/dist -type f')
    print(stdout.read().decode())
    
    print("=== File sizes ===")
    stdin, stdout, stderr = ssh.exec_command('docker exec moduforge du -sh /app/dist/*')
    print(stdout.read().decode())
    
    print("=== Total size ===")
    stdin, stdout, stderr = ssh.exec_command('docker exec moduforge du -sh /app/dist')
    print(stdout.read().decode())
    
    print("=== Health check ===")
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    check()
