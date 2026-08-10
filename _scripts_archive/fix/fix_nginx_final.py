#!/usr/bin/env python3
"""Fix nginx - write config and restart properly"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

MODUFORGE_CONF = """# ModuForge reverse proxy with security headers
server {
    listen 8086;
    server_name _;

    # Security headers
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=moduforge_api:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=moduforge_login:10m rate=5r/m;

    # Request size limit
    client_max_body_size 10M;

    # Auth rate limiting
    location /api/v1/auth/ {
        limit_req zone=moduforge_login burst=3 nodelay;
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # API rate limiting
    location /api/ {
        limit_req zone=moduforge_api burst=20 nodelay;
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Health endpoint
    location /health {
        proxy_pass http://127.0.0.1:8087;
    }

    # WebSocket
    location /ws/ {
        proxy_pass http://127.0.0.1:8087;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Frontend
    location / {
        proxy_pass http://127.0.0.1:8087;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}"""

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=15):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Step 1: Stop nginx
    print("=== Step 1: Stop nginx ===")
    run("sudo killall nginx 2>/dev/null")
    time.sleep(2)
    
    # Step 2: Write config
    print("\n=== Step 2: Write config ===")
    sftp = client.open_sftp()
    with sftp.open("/tmp/moduforge.conf", "w") as f:
        f.write(MODUFORGE_CONF)
    sftp.close()
    
    # Copy with sudo
    run("sudo cp /tmp/moduforge.conf /usr/trim/nginx/conf/conf.d/moduforge.conf")
    run("sudo chmod 644 /usr/trim/nginx/conf/conf.d/moduforge.conf")
    
    # Verify
    out, _ = run("cat /usr/trim/nginx/conf/conf.d/moduforge.conf | head -5")
    print(f"Config written: {out}")
    
    # Step 3: Start nginx
    print("\n=== Step 3: Start nginx ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx")
    print(f"Start: {out}")
    if err:
        print(f"Error: {err}")
    
    time.sleep(3)
    
    # Step 4: Check ports
    print("\n=== Step 4: Check ports ===")
    out, _ = run("netstat -tlnp | grep nginx")
    print(out)
    
    # Step 5: Test
    print("\n=== Step 5: Test ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(out[:300])
    
    client.close()

if __name__ == "__main__":
    main()
