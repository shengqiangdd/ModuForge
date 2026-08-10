#!/usr/bin/env python3
"""Fix nginx proxy - use 127.0.0.1"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

NGINX_CONF = """# ModuForge security proxy
limit_req_zone $binary_remote_addr zone=moduforge_api:10m rate=10r/s;
limit_req_zone $binary_remote_addr zone=moduforge_login:10m rate=5r/m;

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

    client_max_body_size 10M;
    proxy_connect_timeout 60s;
    proxy_send_timeout 60s;
    proxy_read_timeout 120s;

    location /api/v1/auth/ {
        limit_req zone=moduforge_login burst=3 nodelay;
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        limit_req zone=moduforge_api burst=20 nodelay;
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /health {
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
    }

    location /ws/ {
        proxy_pass http://127.0.0.1:8087;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

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
    
    # Write config
    print("=== Write config ===")
    sftp = client.open_sftp()
    with sftp.open("/tmp/moduforge_nginx.conf", "w") as f:
        f.write(NGINX_CONF)
    sftp.close()
    
    # Reload nginx
    print("\n=== Reload nginx ===")
    run("docker exec moduforge-nginx nginx -s reload")
    time.sleep(3)
    
    # Test
    print("\n=== Test health ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(out)
    
    print("\n=== Test API ===")
    out, _ = run("curl -s http://localhost:8086/api/v1/agent/skills -H 'Authorization: Bearer test'")
    print(out[:300])
    
    client.close()

if __name__ == "__main__":
    main()
