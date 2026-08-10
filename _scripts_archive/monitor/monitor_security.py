#!/usr/bin/env python3
"""1. Nginx 监控脚本 - 定期检查日志和速率限制效果"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=15):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # 1. 检查 nginx 容器状态
    print("=== 1. Nginx 容器状态 ===")
    out, _ = run("docker ps --filter name=moduforge-nginx --format '{{.Status}}'")
    print(f"状态: {out.strip()}")
    
    # 2. 检查最近的 503 (速率限制) 错误
    print("\n=== 2. 速率限制统计 (最近5分钟) ===")
    out, _ = run("docker logs moduforge-nginx --since 5m 2>&1 | grep -c '503'")
    print(f"503 响应数: {out.strip()}")
    
    # 3. 检查 429 响应 (如果 nginx 返回 429)
    out, _ = run("docker logs moduforge-nginx --since 5m 2>&1 | grep -c '429'")
    print(f"429 响应数: {out.strip()}")
    
    # 4. 检查认证失败 (401)
    out, _ = run("docker logs moduforge-nginx --since 5m 2>&1 | grep -c '401'")
    print(f"401 响应数: {out.strip()}")
    
    # 5. 检查成功请求 (200)
    out, _ = run("docker logs moduforge-nginx --since 5m 2>&1 | grep -c '200'")
    print(f"200 响应数: {out.strip()}")
    
    # 6. 检查访问 IP 分布
    print("\n=== 3. 访问 IP 分布 (最近5分钟) ===")
    out, _ = run("docker logs moduforge-nginx --since 5m 2>&1 | grep -oE '[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+' | sort | uniq -c | sort -rn | head -5")
    print(out)
    
    # 7. 检查请求路径分布
    print("\n=== 4. 请求路径分布 (最近5分钟) ===")
    out, _ = run("docker logs moduforge-nginx --since 5m 2>&1 | grep -oE 'GET|POST|PUT|DELETE' | sort | uniq -c | sort -rn")
    print(out)
    
    # 8. 检查错误日志
    print("\n=== 5. 最近错误日志 ===")
    out, _ = run("docker logs moduforge-nginx --since 5m 2>&1 | grep -i error | tail -5")
    print(out[:500] if out else "无错误")
    
    # 9. 检查 ModuForge 健康状态
    print("\n=== 6. ModuForge 健康状态 ===")
    out, _ = run("curl -s http://localhost:8086/health")
    print(f"代理健康: {out}")
    
    out, _ = run("curl -s http://localhost:8087/health")
    print(f"直接健康: {out}")
    
    client.close()

if __name__ == "__main__":
    main()
