#!/usr/bin/env python3
"""2. 速率限制调优 + 3. IP白名单 + 4. 文档"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

# 优化后的 nginx 配置 - 包含 IP 白名单和调优后的速率限制
NGINX_CONF_OPTIMIZED = """
# ModuForge 安全代理 - 优化版
# 调优：认证端点 3r/m (更严格)，API 15r/s (更宽松)

# IP 白名单 (内网 + 常用IP)
geo $whitelist {
    default 0;
    127.0.0.1/32 1;
    192.168.0.0/16 1;
    10.0.0.0/8 1;
    172.16.0.0/12 1;
    # 添加你的常用外部 IP (取消注释并修改)
    # 203.0.113.50/32 1;
}

# 速率限制区域
limit_req_zone $binary_remote_addr zone=moduforge_api:10m rate=15r/s;
limit_req_zone $binary_remote_addr zone=moduforge_login:10m rate=3r/m;

# 自定义 429 错误页面
limit_req_status 429;

server {
    listen 8086;
    server_name _;

    # 安全头
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
    add_header X-Rate-Limit-Policy "moduforge" always;

    # 请求限制
    client_max_body_size 10M;
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 120s;

    # 429 错误页面
    error_page 429 = @rate_limited;
    location @rate_limited {
        default_type application/json;
        return 429 '{"error": "rate_limited", "message": "Too many requests. Please retry later.", "retry_after": 60}';
    }

    # 认证端点 - 最严格限制
    location /api/v1/auth/ {
        limit_req zone=moduforge_login burst=3 nodelay;
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # API 端点 - 白名单用户更宽松
    location /api/ {
        # 白名单用户跳过速率限制
        if ($whitelist) {
            set $limit_req "";
        }
        limit_req zone=moduforge_api burst=30 nodelay;
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 健康检查 - 无限制
    location /health {
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
    }

    # WebSocket - 无限制
    location /ws/ {
        proxy_pass http://127.0.0.1:8087;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 前端 - 无限制
    location / {
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
"""

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # 1. 写入优化后的配置
    print("=== 1. 写入优化配置 ===")
    sftp = client.open_sftp()
    with sftp.open("/tmp/moduforge_nginx_optimized.conf", "w") as f:
        f.write(NGINX_CONF_OPTIMIZED)
    sftp.close()
    print("配置已写入")
    
    # 2. 备份旧配置
    print("\n=== 2. 备份旧配置 ===")
    run("cp /tmp/moduforge_nginx.conf /tmp/moduforge_nginx.conf.bak")
    print("旧配置已备份到 /tmp/moduforge_nginx.conf.bak")
    
    # 3. 替换配置
    print("\n=== 3. 替换配置 ===")
    run("cp /tmp/moduforge_nginx_optimized.conf /tmp/moduforge_nginx.conf")
    print("配置已替换")
    
    # 4. 重载 nginx
    print("\n=== 4. 重载 nginx ===")
    run("docker exec moduforge-nginx nginx -s reload")
    time.sleep(3)
    print("Nginx 已重载")
    
    # 5. 验证配置
    print("\n=== 5. 验证配置 ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(f"健康检查: {out[:200]}")
    
    # 6. 测试速率限制调优
    print("\n=== 6. 测试速率限制调优 ===")
    print("认证端点 (限制 3r/m):")
    for i in range(5):
        out, _ = run(f"curl -s -o /dev/null -w '%{{http_code}}' http://localhost:8086/api/v1/auth/login")
        print(f"  请求 {i+1}: {out}")
    
    print("\nAPI 端点 (限制 15r/s):")
    for i in range(20):
        out, _ = run(f"curl -s -o /dev/null -w '%{{http_code}}' http://localhost:8086/api/v1/agent/skills")
        print(f"  请求 {i+1}: {out}")
    
    # 7. 测试 IP 白名单
    print("\n=== 7. IP 白名单测试 ===")
    print("从本地访问 (应该被白名单):")
    out, _ = run("curl -s -o /dev/null -w '%{http_code}' http://localhost:8086/api/v1/agent/skills")
    print(f"  本地请求: {out}")
    
    client.close()

if __name__ == "__main__":
    main()
