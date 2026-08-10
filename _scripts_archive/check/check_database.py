#!/usr/bin/env python3
"""检查数据库中的用户"""
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
    
    # 1. 复制数据库到服务器
    print("=== 1. 复制数据库 ===")
    run("docker cp moduforge:/data/moduforge.db /tmp/moduforge.db")
    
    # 2. 检查数据库
    print("\n=== 2. 检查数据库 ===")
    out, _ = run("sqlite3 /tmp/moduforge.db '.tables' 2>/dev/null || echo 'sqlite3 not found'")
    print(out)
    
    # 3. 尝试用 python 读取
    print("\n=== 3. 用 python 读取 ===")
    out, _ = run("python3 -c \"import sqlite3; conn=sqlite3.connect('/tmp/moduforge.db'); cursor=conn.cursor(); cursor.execute('SELECT name FROM sqlite_master WHERE type=\\'table\\''); print(cursor.fetchall())\" 2>/dev/null")
    print(out)
    
    # 4. 检查用户表
    print("\n=== 4. 检查用户表 ===")
    out, _ = run("python3 -c \"import sqlite3; conn=sqlite3.connect('/tmp/moduforge.db'); cursor=conn.cursor(); cursor.execute('SELECT * FROM users'); print(cursor.fetchall())\" 2>/dev/null")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
