#!/usr/bin/env python3
"""检查 AndroBoost WebUI 现状"""
import sys, io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko, json

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

    proj = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"

    # 1. 读取 WebUI
    print("=== 1. WebUI index.html ===")
    out, _ = run(f"cat {proj}/src/go/web/index.html")
    print(out)

    # 2. 检查 Go 后端 API
    print("\n=== 2. Go 后端 main.go ===")
    out, _ = run(f"cat {proj}/src/go/main.go")
    print(out[:5000])

    # 3. 检查 shm.go
    print("\n=== 3. Go shm.go ===")
    out, _ = run(f"cat {proj}/src/go/shm.go")
    print(out[:3000])

    client.close()

if __name__ == "__main__":
    main()
