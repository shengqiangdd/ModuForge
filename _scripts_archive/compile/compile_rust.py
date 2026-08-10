#!/usr/bin/env python3
"""编译 Rust 代码"""
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
    
    def run(cmd, timeout=120):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    project_path = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"
    
    # 1. 检查 Rust 环境
    print("=== 1. 检查 Rust 环境 ===")
    out, _ = run("which rustc && rustc --version")
    print(out)
    
    # 2. 检查目标平台
    print("\n=== 2. 检查目标平台 ===")
    out, _ = run("rustup target list --installed")
    print(out)
    
    # 3. 尝试编译（使用 cargo check）
    print("\n=== 3. 尝试编译 ===")
    out, _ = run(f"cd {project_path}/src/rust && cargo check 2>&1", timeout=180)
    print(out[:2000])
    
    # 4. 如果有错误，分析
    if "error" in out.lower():
        print("\n=== 4. 错误分析 ===")
        # 提取错误信息
        lines = out.split('\n')
        for line in lines:
            if 'error' in line.lower():
                print(line)
    
    client.close()

if __name__ == "__main__":
    main()
