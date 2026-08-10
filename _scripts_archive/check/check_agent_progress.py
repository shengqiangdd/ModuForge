#!/usr/bin/env python3
"""检查 Agent 进度"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

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
    
    # 1. 检查容器日志
    print("=== 1. 容器日志（最近50行）===")
    out, _ = run("docker logs moduforge --tail 50 2>&1")
    print(out[:1500])
    
    # 2. 检查项目文件更新时间
    print("\n=== 2. 项目文件更新时间 ===")
    project_path = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"
    out, _ = run(f"ls -la {project_path}/src/rust/src/main.rs")
    print(out)
    
    # 3. 检查 main.rs 是否有 energy 模块
    print("\n=== 3. 检查 main.rs 模块 ===")
    out, _ = run(f"grep -n 'mod energy\\|mod fingerprint\\|mod display\\|mod error\\|mod constants' {project_path}/src/rust/src/main.rs")
    print(out)
    
    # 4. 检查 main.rs 内容
    print("\n=== 4. main.rs 内容（前50行）===")
    out, _ = run(f"head -50 {project_path}/src/rust/src/main.rs")
    print(out)
    
    # 5. 等待并再次检查
    print("\n=== 5. 等待10秒后再次检查 ===")
    time.sleep(10)
    
    out, _ = run(f"ls -la {project_path}/src/rust/src/main.rs")
    print(f"文件时间: {out}")
    
    out, _ = run(f"grep -n 'mod energy\\|mod fingerprint\\|mod display\\|mod error\\|mod constants' {project_path}/src/rust/src/main.rs")
    print(f"模块检查: {out}")
    
    client.close()

if __name__ == "__main__":
    main()
