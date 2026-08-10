#!/usr/bin/env python3
"""检查 AndroBoost-SmartTune 项目文件"""
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
    
    # 1. 检查项目目录
    print("=== 1. 项目目录 ===")
    project_path = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"
    out, _ = run(f"ls -la {project_path}")
    print(out)
    
    # 2. 检查 Rust 源码
    print("\n=== 2. Rust 源码 ===")
    out, _ = run(f"find {project_path}/src/rust -name '*.rs' 2>/dev/null | head -20")
    print(out)
    
    # 3. 检查 main.rs
    print("\n=== 3. main.rs 内容 ===")
    out, _ = run(f"head -80 {project_path}/src/rust/src/main.rs 2>/dev/null")
    print(out[:800])
    
    # 4. 检查 LinUCB 算法
    print("\n=== 4. LinUCB 算法 ===")
    out, _ = run(f"head -50 {project_path}/src/rust/src/linucb.rs 2>/dev/null")
    print(out[:600])
    
    # 5. 检查 WebUI
    print("\n=== 5. WebUI 文件 ===")
    out, _ = run(f"ls -la {project_path}/src/go/web/ 2>/dev/null")
    print(out)
    
    # 6. 检查代码行数
    print("\n=== 6. 代码行数统计 ===")
    out, _ = run(f"find {project_path}/src/rust -name '*.rs' 2>/dev/null | xargs wc -l 2>/dev/null | tail -1")
    print(f"Rust: {out.strip()}")
    
    out, _ = run(f"find {project_path}/src/go -name '*.go' 2>/dev/null | xargs wc -l 2>/dev/null | tail -1")
    print(f"Go: {out.strip()}")
    
    out, _ = run(f"find {project_path}/src/cpp -name '*.cpp' -o -name '*.h' 2>/dev/null | xargs wc -l 2>/dev/null | tail -1")
    print(f"C++: {out.strip()}")
    
    client.close()

if __name__ == "__main__":
    main()
