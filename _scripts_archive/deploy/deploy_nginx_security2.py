#!/usr/bin/env python3
"""Deploy nginx reverse proxy with security headers - fixed paths"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time
import os

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

NGINX_CONF = """
server {
    listen 80;
    server_name _;

    # Security headers
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:;" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;

    # Rate limiting zones
    limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=login:10m rate=5r/m;

    # Request size limit
    client_max_body_size 10M;

    # Timeouts
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;

    # API endpoints with rate limiting
    location /api/v1/auth/ {
        limit_req zone=login burst=3 nodelay;
        proxy_pass http://127.0.0.1:8086;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        limit_req zone=api burst=20 nodelay;
        proxy_pass http://127.0.0.1:8086;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Health endpoint (no rate limit)
    location /health {
        proxy_pass http://127.0.0.1:8086;
        proxy_set_header Host $host;
    }

    # WebSocket support
    location /ws/ {
        proxy_pass http://127.0.0.1:8086;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Frontend (Svelte app)
    location / {
        proxy_pass http://127.0.0.1:8086;
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
    
    # Step 1: Stop existing containers
    print("=== Step 1: Stop existing containers ===")
    run(f"docker stop {CONTAINER} 2>/dev/null")
    run("docker stop moduforge-nginx 2>/dev/null")
    run("docker rm moduforge-nginx 2>/dev/null")
    time.sleep(3)
    
    # Step 2: Start ModuForge on internal port
    print("\n=== Step 2: Start ModuForge on internal port ===")
    cmd = f"""docker run -d \
        --name {CONTAINER} \
        -p 127.0.0.1:8086:8080 \
        -e PORT=:8080 \
        moduforge:latest"""
    out, err = run(cmd)
    print(f"ModuForge: {out[:12]}")
    time.sleep(8)
    
    # Check health
    out, _ = run("curl -s http://localhost:8086/health")
    print(f"Health: {out}")
    
    # Step 3: Write nginx config to server
    print("\n=== Step 3: Write nginx config ===")
    # Use cat to write config
    config_cmd = f"""cat > /tmp/nginx_security.conf << 'NGINX_EOF'
{NGINX_CONF}
NGINX_EOF"""
    run(f"docker exec moduforge sh -c '{config_cmd}'")
    
    # Step 4: Start nginx container
    print("\n=== Step 4: Start nginx container ===")
    cmd = """docker run -d \
        --name moduforge-nginx \
        --network host \
        -v /tmp/nginx_security.conf:/etc/nginx/conf.d/default.conf:ro \
        nginx:alpine"""
    out, err = run(cmd)
    print(f"Nginx: {out[:12]}")
    if err:
        print(f"Error: {err}")
    
    time.sleep(3)
    
    # Step 5: Test
    print("\n=== Step 5: Test security headers ===")
    out, _ = run("curl -sI http://localhost:80/health")
    print(out)
    
    print("\n=== Step 6: Test rate limiting ===")
    for i in range(12):
        out, _ = run(f"curl -s -o /dev/null -w '%{{http_code}}' http://localhost:80/api/v1/agent/skills")
        print(f"  Request {i+1}: {out}")
    
    client.close()

if __name__ == "__main__":
    main()
