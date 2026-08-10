#!/usr/bin/env python3
"""检查 PRD 需求和差距"""
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
    
    # 1. 检查 DESIGN_DOC.md
    print("=== 1. DESIGN_DOC.md ===")
    out, _ = run(f"cat {project_path}/DESIGN_DOC.md 2>/dev/null | head -100")
    print(out[:1000])
    
    # 2. 检查 config.rs
    print("\n=== 2. config.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/config.rs")
    print(out[:800])
    
    # 3. 检查 error.rs
    print("\n=== 3. error.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/error.rs")
    print(out[:500])
    
    # 4. 检查 constants.rs
    print("\n=== 4. constants.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/constants.rs")
    print(out[:500])
    
    # 5. 检查 main.rs 完整内容
    print("\n=== 5. main.rs 完整内容 ===")
    out, _ = run(f"cat {project_path}/src/rust/src/main.rs")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
