#!/usr/bin/env python3
"""检查容器卷挂载"""
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
    
    # 1. 检查容器卷挂载
    print("=== 1. 容器卷挂载 ===")
    out, _ = run("docker inspect moduforge --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{println}}{{end}}'")
    print(out)
    
    # 2. 检查 Docker 卷内容
    print("\n=== 2. Docker 卷内容 ===")
    out, _ = run("docker volume inspect moduforge_moduforge_data --format '{{.Mountpoint}}'")
    mountpoint = out.strip()
    print(f"挂载点: {mountpoint}")
    
    out, _ = run(f"ls -la {mountpoint} 2>/dev/null")
    print(out)
    
    # 3. 检查数据库文件
    print("\n=== 3. 数据库文件 ===")
    out, _ = run(f"find {mountpoint} -name '*.db' -o -name '*.sqlite' 2>/dev/null")
    print(out)
    
    # 4. 检查项目目录
    print("\n=== 4. 项目目录 ===")
    out, _ = run(f"ls -la {mountpoint}/projects/ 2>/dev/null || echo '无项目目录'")
    print(out)
    
    # 5. 检查容器内数据
    print("\n=== 5. 容器内数据 ===")
    out, _ = run("docker exec moduforge ls -la /data/ 2>/dev/null")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
