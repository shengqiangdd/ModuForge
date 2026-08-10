#!/usr/bin/env python3
"""检查 ModuForge 登录"""
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
    
    # 1. 检查数据库中的用户
    print("=== 1. 检查数据库用户 ===")
    out, _ = run("docker exec moduforge sqlite3 /data/moduforge.db \"SELECT username FROM users;\" 2>/dev/null")
    print(out)
    
    # 2. 尝试不同用户名
    print("\n=== 2. 尝试不同用户名 ===")
    for username in ["csq", "admin", "root"]:
        out, _ = run(f'curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{{"username":"{username}","password":"csq0216"}}\'')
        print(f"{username}: {out[:100]}")
    
    # 3. 检查 API 路由
    print("\n=== 3. 检查 API 路由 ===")
    out, _ = run("curl -s http://localhost:8086/api/v1/auth/register -X POST -H 'Content-Type: application/json' -d '{\"username\":\"csq\",\"password\":\"csq0216\"}'")
    print(out[:200])
    
    client.close()

if __name__ == "__main__":
    main()
