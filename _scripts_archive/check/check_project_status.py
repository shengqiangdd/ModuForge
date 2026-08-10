#!/usr/bin/env python3
"""检查 AndroBoost-SmartTune 项目当前状态"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time
import json

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
    
    # 1. 检查项目文件结构
    print("=== 1. 项目文件结构 ===")
    out, _ = run("find /app/data/storage/projects/1785249992652501794-1864 -type f -name '*.rs' -o -name '*.go' -o -name '*.cpp' -o -name '*.h' 2>/dev/null | head -30")
    print(out)
    
    # 2. 检查 Rust 代码行数
    print("\n=== 2. Rust 代码统计 ===")
    out, _ = run("find /app/data/storage/projects/1785249992652501794-1864/src/rust -name '*.rs' 2>/dev/null | xargs wc -l 2>/dev/null | tail -1")
    print(f"Rust 总行数: {out.strip()}")
    
    # 3. 检查 main.rs 集成状态
    print("\n=== 3. main.rs 模块集成 ===")
    out, _ = run("head -50 /app/data/storage/projects/1785249992652501794-1864/src/rust/src/main.rs 2>/dev/null")
    print(out[:500])
    
    # 4. 检查 LinUCB 算法
    print("\n=== 4. LinUCB 算法状态 ===")
    out, _ = run("head -30 /app/data/storage/projects/1785249992652501794-1864/src/rust/src/linucb.rs 2>/dev/null")
    print(out[:400])
    
    # 5. 检查 WebUI 状态
    print("\n=== 5. WebUI 文件 ===")
    out, _ = run("ls -la /app/data/storage/projects/1785249992652501794-1864/src/go/web/ 2>/dev/null")
    print(out)
    
    # 6. 检查最近的 Agent 任务
    print("\n=== 6. 最近 Agent 任务 ===")
    out, _ = run("curl -s http://localhost:8086/api/v1/agent/tasks -H 'Authorization: Bearer test' 2>/dev/null | head -200")
    print(out[:300])
    
    client.close()

if __name__ == "__main__":
    main()
