#!/usr/bin/env python3
"""查找 AndroBoost-SmartTune 项目实际位置"""
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
    
    # 1. 查找项目目录
    print("=== 1. 查找项目目录 ===")
    out, _ = run("find /app/data/storage/projects -maxdepth 2 -type d 2>/dev/null | head -20")
    print(out)
    
    # 2. 查找 Rust 源码
    print("\n=== 2. 查找 Rust 源码 ===")
    out, _ = run("find /app -name '*.rs' -type f 2>/dev/null | head -20")
    print(out)
    
    # 3. 查找 main.rs
    print("\n=== 3. 查找 main.rs ===")
    out, _ = run("find /app -name 'main.rs' -type f 2>/dev/null")
    print(out)
    
    # 4. 检查 ModuForge 数据目录
    print("\n=== 4. ModuForge 数据目录 ===")
    out, _ = run("ls -la /app/data/ 2>/dev/null")
    print(out)
    
    # 5. 检查项目列表
    print("\n=== 5. 项目列表 ===")
    out, _ = run("ls -la /app/data/storage/projects/ 2>/dev/null")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
