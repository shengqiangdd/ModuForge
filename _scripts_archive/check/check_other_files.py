#!/usr/bin/env python3
"""检查其他文件和编译状态"""
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
    
    # 1. 检查 Cargo.toml
    print("=== 1. Cargo.toml ===")
    out, _ = run(f"cat {project_path}/src/rust/Cargo.toml 2>/dev/null || echo '文件不存在'")
    print(out[:500])
    
    # 2. 检查 lib.rs
    print("\n=== 2. lib.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/lib.rs")
    print(out)
    
    # 3. 检查 energy.rs 是否有 calculate_eei 方法
    print("\n=== 3. energy.rs 方法 ===")
    out, _ = run(f"grep -n 'pub fn\\|pub struct' {project_path}/src/rust/src/energy.rs")
    print(out)
    
    # 4. 检查 display.rs 是否有 optimize_refresh_rate 方法
    print("\n=== 4. display.rs 方法 ===")
    out, _ = run(f"grep -n 'pub fn\\|pub struct' {project_path}/src/rust/src/display.rs")
    print(out)
    
    # 5. 检查 fingerprint.rs 是否有 detect 方法
    print("\n=== 5. fingerprint.rs 方法 ===")
    out, _ = run(f"grep -n 'pub fn\\|pub struct' {project_path}/src/rust/src/fingerprint.rs")
    print(out)
    
    # 6. 检查 error.rs
    print("\n=== 6. error.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/error.rs")
    print(out[:500])
    
    # 7. 检查 constants.rs
    print("\n=== 7. constants.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/constants.rs")
    print(out[:500])
    
    client.close()

if __name__ == "__main__":
    main()
