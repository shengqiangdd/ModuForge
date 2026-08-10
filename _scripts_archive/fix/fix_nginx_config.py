#!/usr/bin/env python3
"""Fix nginx config - write to correct location"""
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
    
    # Write config to temp file
    print("=== Write config to temp ===")
    sftp = client.open_sftp()
    with sftp.open("/tmp/moduforge.conf", "w") as f:
        f.write(MODUFORGE_CONF)
    sftp.close()
    
    # Copy to nginx conf.d
    print("\n=== Copy to nginx conf.d ===")
    run("sudo cp /tmp/moduforge.conf /usr/trim/nginx/conf/conf.d/moduforge.conf")
    run("sudo chmod 644 /usr/trim/nginx/conf/conf.d/moduforge.conf")
    
    # Verify
    print("\n=== Verify config ===")
    out, _ = run("cat /usr/trim/nginx/conf/conf.d/moduforge.conf | head -10")
    print(out)
    
    # Find nginx master PID
    print("\n=== Find nginx master PID ===")
    out, _ = run("ps aux | grep 'nginx: master' | grep -v grep | awk '{print $2}'")
    master_pid = out.strip()
    print(f"Master PID: {master_pid}")
    
    # Write PID file
    if master_pid:
        run(f"echo {master_pid} | sudo tee /run/nginx.pid")
    
    # Reload nginx
    print("\n=== Reload nginx ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -s reload")
    print(f"Reload: {out}")
    if err:
        print(f"Error: {err}")
    
    time.sleep(3)
    
    # Check if 8086 is now listening
    print("\n=== Check port 8086 ===")
    out, _ = run("netstat -tlnp | grep :8086")
    print(out)
    
    # Test
    print("\n=== Test ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(out[:300])
    
    client.close()

if __name__ == "__main__":
    main()
