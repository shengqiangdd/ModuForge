#!/usr/bin/env python3
"""检查 ModuForge 数据持久化"""
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
    
    # 1. 检查容器挂载
    print("=== 1. 容器挂载 ===")
    out, _ = run("docker inspect moduforge --format '{{json .Mounts}}' 2>/dev/null | python -m json.tool")
    print(out)
    
    # 2. 检查容器环境变量
    print("\n=== 2. 容器环境变量 ===")
    out, _ = run("docker inspect moduforge --format '{{json .Config.Env}}' 2>/dev/null | python -m json.tool")
    print(out)
    
    # 3. 检查服务器上的数据目录
    print("\n=== 3. 服务器数据目录 ===")
    out, _ = run("ls -la /home/admin/moduforge_data/ 2>/dev/null || echo '目录不存在'")
    print(out)
    
    # 4. 检查 Docker 卷
    print("\n=== 4. Docker 卷 ===")
    out, _ = run("docker volume ls | grep moduforge")
    print(out)
    
    # 5. 检查容器日志
    print("\n=== 5. 容器日志（最近20行）===")
    out, _ = run("docker logs moduforge --tail 20 2>&1")
    print(out[:500])
    
    # 6. 检查数据库文件
    print("\n=== 6. 服务器上的数据库 ===")
    out, _ = run("find /home/admin -name '*.db' -o -name 'moduforge*' 2>/dev/null | head -10")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
