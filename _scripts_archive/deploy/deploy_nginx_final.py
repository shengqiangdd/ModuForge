#!/usr/bin/env python3
"""Deploy nginx security layer"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

NGINX_CONF = """server {
    listen 80;
    server_name _;

    # Security headers
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=login:10m rate=5r/m;

    # Request size limit
    client_max_body_size 10M;

    # Auth rate limiting
    location /api/v1/auth/ {
        limit_req zone=login burst=3 nodelay;
        proxy_pass http://127.0.0.1:8086;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # API rate limiting
    location /api/ {
        limit_req zone=api burst=20 nodelay;
        proxy_pass http://127.0.0.1:8086;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Health
    location /health {
        proxy_pass http://127.0.0.1:8086;
    }

    # WebSocket
    location /ws/ {
        proxy_pass http://127.0.0.1:8086;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Frontend
    location / {
        proxy_pass http://127.0.0.1:8086;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}"""

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Step 1: Write nginx config
    print("=== Step 1: Write nginx config ===")
    sftp = client.open_sftp()
    with sftp.open("/tmp/nginx_security.conf", "w") as f:
        f.write(NGINX_CONF)
    sftp.close()
    print("Config written")
    
    # Step 2: Start nginx
    print("\n=== Step 2: Start nginx ===")
    cmd = """docker run -d \
        --name moduforge-nginx \
        --network host \
        -v /tmp/nginx_security.conf:/etc/nginx/conf.d/default.conf:ro \
        nginx:alpine"""
    out, err = run(cmd)
    print(f"Container: {out[:12]}")
    if err:
        print(f"Error: {err}")
    
    time.sleep(3)
    
    # Step 3: Test
    print("\n=== Step 3: Test security headers ===")
    out, _ = run("curl -sI http://localhost:80/health")
    print(out)
    
    # Step 4: Test rate limiting
    print("\n=== Step 4: Test rate limiting ===")
    for i in range(12):
        out, _ = run(f"curl -s -o /dev/null -w '%{{http_code}}' http://localhost:80/api/v1/agent/skills")
        print(f"  Request {i+1}: {out}")
    
    client.close()

if __name__ == "__main__":
    main()
