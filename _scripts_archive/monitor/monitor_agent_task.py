#!/usr/bin/env python3
"""监控 Agent 任务进度"""
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
    
    # 1. 获取 Token
    print("=== 1. 获取 Token ===")
    out, _ = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
    data = json.loads(out)
    token = data.get("token", "")
    print(f"Token: {token[:50]}...")
    
    # 2. 检查任务列表
    print("\n=== 2. 检查任务列表 ===")
    out, _ = run(f'curl -s http://localhost:8086/api/v1/agent/tasks -H "Authorization: Bearer {token}"')
    print(out[:500])
    
    # 3. 检查最近的日志
    print("\n=== 3. 最近日志 ===")
    out, _ = run("docker logs moduforge --tail 30 2>&1 | grep -i 'agent\\|task\\|run'")
    print(out[:500])
    
    # 4. 检查项目文件是否更新
    print("\n=== 4. 检查项目文件更新 ===")
    project_path = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"
    out, _ = run(f"ls -la {project_path}/src/rust/src/main.rs")
    print(out)
    
    # 5. 检查 main.rs 是否有 energy 模块
    print("\n=== 5. 检查 main.rs 是否有 energy 模块 ===")
    out, _ = run(f"grep -n 'energy\\|fingerprint\\|display\\|error\\|constants' {project_path}/src/rust/src/main.rs")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
