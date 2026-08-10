#!/usr/bin/env python3
"""检查构建设置"""
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
    
    # 1. 检查构建脚本
    print("=== 1. 检查构建脚本 ===")
    out, _ = run(f"ls -la {project_path}/ | grep -i build")
    print(out)
    
    # 2. 检查 Dockerfile
    print("\n=== 2. 检查 Dockerfile ===")
    out, _ = run(f"find {project_path} -name 'Dockerfile*' -type f")
    print(out)
    
    # 3. 检查 docker-compose.yml
    print("\n=== 3. 检查 docker-compose.yml ===")
    out, _ = run(f"cat {project_path}/docker-compose.yml 2>/dev/null || echo '文件不存在'")
    print(out[:500])
    
    # 4. 检查 build.sh 或 build.bat
    print("\n=== 4. 检查构建脚本 ===")
    out, _ = run(f"cat {project_path}/build.sh 2>/dev/null || cat {project_path}/build.bat 2>/dev/null || echo '无构建脚本'")
    print(out[:500])
    
    # 5. 检查 Rust target 目录
    print("\n=== 5. Rust target 目录 ===")
    out, _ = run(f"ls -la {project_path}/src/rust/target/ 2>/dev/null | head -10")
    print(out)
    
    # 6. 检查是否有预编译的二进制文件
    print("\n=== 6. 预编译的二进制文件 ===")
    out, _ = run(f"find {project_path} -name '*.bin' -o -name 'linucb-engine' -type f 2>/dev/null")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
