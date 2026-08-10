#!/usr/bin/env python3
"""检查数据库表结构"""
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
    
    # 1. 复制数据库
    print("=== 1. 复制数据库 ===")
    run("docker cp moduforge:/data/moduforge.db /tmp/moduforge.db")
    
    # 2. 检查数据库表
    print("\n=== 2. 检查数据库表 ===")
    out, _ = run("python3 -c \"import sqlite3; conn=sqlite3.connect('/tmp/moduforge.db'); cursor=conn.cursor(); cursor.execute('SELECT name FROM sqlite_master WHERE type=\\'table\\''); print([t[0] for t in cursor.fetchall()])\"")
    print(out)
    
    # 3. 检查 users 表结构
    print("\n=== 3. 检查 users 表结构 ===")
    out, _ = run("python3 -c \"import sqlite3; conn=sqlite3.connect('/tmp/moduforge.db'); cursor=conn.cursor(); cursor.execute('PRAGMA table_info(users)'); print(cursor.fetchall())\"")
    print(out)
    
    # 4. 检查 users 表数据
    print("\n=== 4. 检查 users 表数据 ===")
    out, _ = run("python3 -c \"import sqlite3; conn=sqlite3.connect('/tmp/moduforge.db'); cursor=conn.cursor(); cursor.execute('SELECT * FROM users'); print(cursor.fetchall())\"")
    print(out)
    
    # 5. 检查其他表
    print("\n=== 5. 检查其他表 ===")
    out, _ = run("python3 -c \"import sqlite3; conn=sqlite3.connect('/tmp/moduforge.db'); cursor=conn.cursor(); cursor.execute('SELECT name FROM sqlite_master WHERE type=\\'table\\' AND name!=\\'users\\''); tables=[t[0] for t in cursor.fetchall()]; [print(f'{t}: {cursor.execute(f\\'SELECT COUNT(*) FROM {t}\\').fetchone()[0]} rows') for t in tables]\"")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
