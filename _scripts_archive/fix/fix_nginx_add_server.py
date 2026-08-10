#!/usr/bin/env python3
"""Fix nginx - add server block at http level"""
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
    
    # Step 1: Create moduforge directory
    print("=== Step 1: Create moduforge dir ===")
    run("sudo mkdir -p /usr/trim/nginx/conf/moduforge")
    
    # Step 2: Write config to moduforge dir
    print("\n=== Step 2: Write config ===")
    sftp = client.open_sftp()
    with sftp.open("/tmp/moduforge_server.conf", "w") as f:
        f.write(MODUFORGE_CONF)
    sftp.close()
    
    run("sudo cp /tmp/moduforge_server.conf /usr/trim/nginx/conf/moduforge/moduforge.conf")
    run("sudo chmod 644 /usr/trim/nginx/conf/moduforge/moduforge.conf")
    
    # Step 3: Add include to nginx.conf at http level
    print("\n=== Step 3: Add include to nginx.conf ===")
    # Check if include already exists
    out, _ = run("grep -n 'moduforge' /usr/trim/nginx/conf/nginx.conf")
    if "moduforge" in out:
        print("Include already exists")
    else:
        # Add include after "http {" block
        run("sudo sed -i '/^http {/a\\    include moduforge/*.conf;' /usr/trim/nginx/conf/nginx.conf")
        print("Include added")
    
    # Verify
    print("\n=== Verify include ===")
    out, _ = run("grep -n 'moduforge' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Step 4: Test nginx config
    print("\n=== Step 4: Test nginx config ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -t")
    print(f"Test: {out}")
    if err:
        print(f"Error: {err}")
    
    # Step 5: Find nginx master PID and reload
    print("\n=== Step 5: Reload nginx ===")
    out, _ = run("ps aux | grep 'nginx: master' | grep -v grep | awk '{print $2}'")
    master_pid = out.strip()
    if master_pid:
        run(f"echo {master_pid} | sudo tee /run/nginx.pid")
        run("sudo /usr/trim/nginx/sbin/nginx -s reload")
    
    time.sleep(3)
    
    # Step 6: Check if 8086 is now listening
    print("\n=== Step 6: Check port 8086 ===")
    out, _ = run("netstat -tlnp | grep :8086")
    print(out)
    
    # Step 7: Test
    print("\n=== Step 7: Test ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(out[:300])
    
    # Step 8: Test security headers
    print("\n=== Step 8: Full headers ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
