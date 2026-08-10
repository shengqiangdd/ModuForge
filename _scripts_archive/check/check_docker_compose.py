#!/usr/bin/env python3
"""检查 Docker Compose 配置"""
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
    
    # 1. 查找 docker-compose.yml
    print("=== 1. 查找 docker-compose.yml ===")
    out, _ = run("find /home/admin -name 'docker-compose.yml' -o -name 'docker-compose.yaml' 2>/dev/null")
    print(out)
    
    # 2. 检查 ModuForge 目录
    print("\n=== 2. ModuForge 目录 ===")
    out, _ = run("ls -la /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/ 2>/dev/null | head -20")
    print(out)
    
    # 3. 检查 docker-compose.yml 内容
    print("\n=== 3. docker-compose.yml 内容 ===")
    out, _ = run("cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml 2>/dev/null | head -50")
    print(out[:500])
    
    # 4. 检查容器启动命令
    print("\n=== 4. 容器启动命令 ===")
    out, _ = run("docker inspect moduforge --format '{{json .Config.Cmd}}' 2>/dev/null")
    print(out)
    
    # 5. 检查容器入口点
    print("\n=== 5. 容器入口点 ===")
    out, _ = run("docker inspect moduforge --format '{{json .Config.Entrypoint}}' 2>/dev/null")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
