#!/usr/bin/env python3
"""Deploy nginx reverse proxy with security headers"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

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
        proxy_pass http://host.docker.internal:8086;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        limit_req zone=api burst=20 nodelay;
        proxy_pass http://host.docker.internal:8086;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Health endpoint (no rate limit)
    location /health {
        proxy_pass http://host.docker.internal:8086;
        proxy_set_header Host $host;
    }

    # WebSocket support
    location /ws/ {
        proxy_pass http://host.docker.internal:8086;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Frontend (Svelte app)
    location / {
        proxy_pass http://host.docker.internal:8086;
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
    
    # Step 1: Stop existing container
    print("=== Step 1: Stop existing container ===")
    run(f"docker stop {CONTAINER}")
    time.sleep(3)
    
    # Step 2: Start ModuForge on internal port only
    print("\n=== Step 2: Start ModuForge on internal port ===")
    cmd = f"""docker run -d \
        --name {CONTAINER} \
        --network host \
        -p 127.0.0.1:8086:8080 \
        -e PORT=:8080 \
        moduforge:latest"""
    run(cmd)
    time.sleep(5)
    
    # Step 3: Create nginx config
    print("\n=== Step 3: Create nginx config ===")
    # Write config to file
    with open("ModuForge/nginx_security.conf", "w", encoding="utf-8") as f:
        f.write(NGINX_CONF)
    
    # Upload to server
    sftp = client.open_sftp()
    sftp.put("ModuForge/nginx_security.conf", "/tmp/nginx_security.conf")
    sftp.close()
    
    # Step 4: Start nginx container
    print("\n=== Step 4: Start nginx container ===")
    cmd = f"""docker run -d \
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
