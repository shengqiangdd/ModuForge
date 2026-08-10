#!/usr/bin/env python3
"""Add ModuForge proxy to system nginx"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

MODUFORGE_CONF = """
# ModuForge reverse proxy with security headers
server {
    listen 8086;
    server_name _;

    # Security headers
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;

    # Rate limiting zones
    limit_req_zone $binary_remote_addr zone=moduforge_api:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=moduforge_login:10m rate=5r/m;

    # Request size limit
    client_max_body_size 10M;

    # Timeouts
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 60s;

    # Auth rate limiting
    location /api/v1/auth/ {
        limit_req zone=moduforge_login burst=3 nodelay;
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # API rate limiting
    location /api/ {
        limit_req zone=moduforge_api burst=20 nodelay;
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Health endpoint (no rate limit)
    location /health {
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
    }

    # WebSocket support
    location /ws/ {
        proxy_pass http://127.0.0.1:8087;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Frontend (Svelte app)
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
    
    def run(cmd, timeout=15):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Step 1: Stop ModuForge and restart on 8087
    print("=== Step 1: Restart ModuForge on port 8087 ===")
    run("docker stop moduforge")
    time.sleep(2)
    run("docker rm moduforge")
    time.sleep(2)
    
    cmd = """docker run -d \
        --name moduforge \
        -p 127.0.0.1:8087:8080 \
        moduforge:latest"""
    out, err = run(cmd)
    print(f"Container: {out[:12]}")
    time.sleep(10)
    
    # Check health
    out, _ = run("curl -s http://localhost:8087/health")
    print(f"Health: {out}")
    
    # Step 2: Write nginx config
    print("\n=== Step 2: Write nginx config ===")
    sftp = client.open_sftp()
    with sftp.open("/usr/trim/nginx/conf/conf.d/moduforge.conf", "w") as f:
        f.write(MODUFORGE_CONF)
    sftp.close()
    print("Config written")
    
    # Step 3: Test nginx config
    print("\n=== Step 3: Test nginx config ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -t")
    print(f"Test: {out}")
    if err:
        print(f"Error: {err}")
    
    # Step 4: Reload nginx
    print("\n=== Step 4: Reload nginx ===")
    run("sudo /usr/trim/nginx/sbin/nginx -s reload")
    time.sleep(2)
    
    # Step 5: Test
    print("\n=== Step 5: Test security headers ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(out)
    
    # Step 6: Test rate limiting
    print("\n=== Step 6: Test rate limiting ===")
    for i in range(12):
        out, _ = run(f"curl -s -o /dev/null -w '%{{http_code}}' http://localhost:8086/api/v1/agent/skills")
        print(f"  Request {i+1}: {out}")
    
    client.close()

if __name__ == "__main__":
    main()
