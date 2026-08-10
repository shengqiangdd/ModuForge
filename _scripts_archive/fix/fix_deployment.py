#!/usr/bin/env python3
"""Fix ModuForge deployment"""
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
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Step 1: Start ModuForge
    print("=== Step 1: Start ModuForge ===")
    cmd = """docker run -d \
        --name moduforge \
        -p 127.0.0.1:8086:8080 \
        moduforge:latest"""
    out, err = run(cmd)
    print(f"Container: {out[:12]}")
    if err and "already exists" in err:
        print("Container exists, starting...")
        run("docker start moduforge")
    
    time.sleep(10)
    
    # Check health
    print("\n=== Step 2: Check health ===")
    out, _ = run("curl -s http://localhost:8086/health")
    print(f"Health: {out}")
    
    # Step 3: Check nginx config
    print("\n=== Step 3: Check nginx ===")
    out, _ = run("docker exec moduforge-nginx cat /etc/nginx/conf.d/default.conf")
    print(f"Config:\n{out[:500]}")
    
    # Step 4: Update nginx config if needed
    print("\n=== Step 4: Update nginx config ===")
    nginx_conf = """server {
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
    
    # Write to server
    sftp = client.open_sftp()
    with sftp.open("/tmp/nginx_security.conf", "w") as f:
        f.write(nginx_conf)
    sftp.close()
    
    # Copy to nginx container
    run("docker cp /tmp/nginx_security.conf moduforge-nginx:/etc/nginx/conf.d/default.conf")
    run("docker exec moduforge-nginx nginx -s reload")
    
    time.sleep(2)
    
    # Step 5: Test
    print("\n=== Step 5: Test security headers ===")
    out, _ = run("curl -sI http://localhost:80/health")
    print(out)
    
    print("\n=== Step 6: Test ModuForge directly ===")
    out, _ = run("curl -s http://localhost:8086/api/v1/agent/skills -H 'Authorization: Bearer test'")
    print(out[:200])
    
    client.close()

if __name__ == "__main__":
    main()
