#!/usr/bin/env python3
"""注册新用户"""
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
    
    # 1. 尝试注册
    print("=== 1. 注册用户 ===")
    out, _ = run('curl -s -X POST http://localhost:8086/api/v1/auth/register -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
    print(out)
    
    # 2. 尝试登录
    print("\n=== 2. 登录 ===")
    out, _ = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
    print(out)
    
    # 3. 检查数据库
    print("\n=== 3. 检查数据库 ===")
    run("docker cp moduforge:/data/moduforge.db /tmp/moduforge2.db")
    out, _ = run("python3 -c \"import sqlite3; conn=sqlite3.connect('/tmp/moduforge2.db'); cursor=conn.cursor(); cursor.execute('SELECT * FROM users'); print(cursor.fetchall())\"")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
