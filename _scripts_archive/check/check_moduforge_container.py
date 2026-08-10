#!/usr/bin/env python3
"""检查 ModuForge 容器状态和数据"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # 1. 检查容器状态
    print("=== 1. 容器状态 ===")
    out, _ = run("docker ps --filter name=moduforge --format '{{.Names}}\t{{.Status}}'")
    print(out)
    
    # 2. 进入容器检查
    print("\n=== 2. 容器内文件系统 ===")
    out, _ = run("docker exec moduforge ls -la /app/ 2>/dev/null")
    print(out[:500])
    
    # 3. 检查数据目录
    print("\n=== 3. 数据目录 ===")
    out, _ = run("docker exec moduforge ls -la /app/data/ 2>/dev/null")
    print(out)
    
    # 4. 检查项目目录
    print("\n=== 4. 项目目录 ===")
    out, _ = run("docker exec moduforge find /app/data -maxdepth 3 -type d 2>/dev/null | head -20")
    print(out)
    
    # 5. 检查数据库
    print("\n=== 5. 数据库文件 ===")
    out, _ = run("docker exec moduforge find /app -name '*.db' -o -name '*.sqlite' 2>/dev/null")
    print(out)
    
    # 6. 检查 ModuForge API
    print("\n=== 6. ModuForge API ===")
    out, _ = run("curl -s http://localhost:8086/health")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
