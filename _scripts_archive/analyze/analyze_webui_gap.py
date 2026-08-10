#!/usr/bin/env python3
"""分析 Go 后端剩余代码和 API"""
import sys, io
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

    proj = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"

    # 1. main.go 完整内容
    print("=== 1. Go main.go (完整) ===")
    out, _ = run(f"cat {proj}/src/go/main.go")
    print(out)

    # 2. 检查 Rust 端 API 响应结构
    print("\n=== 2. Rust main.rs API ===")
    out, _ = run(f"grep -n 'pub fn\\|api\\|web\\|route\\|handler' {proj}/src/rust/src/main.rs 2>/dev/null")
    print(out)

    # 3. 检查 DESIGN_DOC 中的 WebUI 需求
    print("\n=== 3. DESIGN_DOC WebUI 部分 ===")
    out, _ = run(f"grep -A 5 -i 'webui\\|web ui\\|仪表板\\|dashboard\\|调参\\|策略中心' {proj}/DESIGN_DOC.md 2>/dev/null")
    print(out[:1000])

    client.close()

if __name__ == "__main__":
    main()
