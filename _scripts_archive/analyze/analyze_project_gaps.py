#!/usr/bin/env python3
"""分析项目差距和待改进项"""
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
    
    # 1. 检查 main.rs 模块集成
    print("=== 1. main.rs 模块集成 ===")
    out, _ = run(f"cat {project_path}/src/rust/src/main.rs")
    print(out[:1500])
    
    # 2. 检查 lib.rs 导出
    print("\n=== 2. lib.rs 导出 ===")
    out, _ = run(f"cat {project_path}/src/rust/src/lib.rs")
    print(out[:500])
    
    # 3. 检查 energy.rs
    print("\n=== 3. energy.rs ===")
    out, _ = run(f"head -50 {project_path}/src/rust/src/energy.rs")
    print(out[:600])
    
    # 4. 检查 display.rs
    print("\n=== 4. display.rs ===")
    out, _ = run(f"head -50 {project_path}/src/rust/src/display.rs")
    print(out[:600])
    
    # 5. 检查 fingerprint.rs
    print("\n=== 5. fingerprint.rs ===")
    out, _ = run(f"head -50 {project_path}/src/rust/src/fingerprint.rs")
    print(out[:600])
    
    # 6. 检查 WebUI index.html
    print("\n=== 6. WebUI index.html (前100行) ===")
    out, _ = run(f"head -100 {project_path}/src/go/web/index.html")
    print(out[:800])
    
    # 7. 检查 Rust 编译状态
    print("\n=== 7. Rust 编译状态 ===")
    out, _ = run(f"ls -la {project_path}/src/rust/target/aarch64-linux-android/release/ 2>/dev/null | head -10")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
