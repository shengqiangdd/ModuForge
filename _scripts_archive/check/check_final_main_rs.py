#!/usr/bin/env python3
"""检查最终 main.rs"""
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
    
    project_path = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"
    
    # 1. 检查完整 main.rs
    print("=== 1. 完整 main.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/main.rs")
    print(out)
    
    # 2. 检查模块声明
    print("\n=== 2. 模块声明 ===")
    out, _ = run(f"grep -n 'mod ' {project_path}/src/rust/src/main.rs")
    print(out)
    
    # 3. 检查 use 语句
    print("\n=== 3. use 语句 ===")
    out, _ = run(f"grep -n 'use ' {project_path}/src/rust/src/main.rs")
    print(out)
    
    # 4. 检查 main 函数签名
    print("\n=== 4. main 函数签名 ===")
    out, _ = run(f"grep -n 'fn main' {project_path}/src/rust/src/main.rs")
    print(out)
    
    # 5. 检查模块调用
    print("\n=== 5. 模块调用 ===")
    out, _ = run(f"grep -n 'energy_profiler\\|fingerprint_detector\\|display_controller' {project_path}/src/rust/src/main.rs")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
