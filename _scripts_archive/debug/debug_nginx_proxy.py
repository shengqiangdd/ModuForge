#!/usr/bin/env python3
"""Debug nginx proxy"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=15):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Check if nginx is listening on 8086
    print("=== Check nginx on 8086 ===")
    out, _ = run("netstat -tlnp | grep :8086")
    print(out)
    
    # Check nginx error log
    print("\n=== Nginx error log ===")
    out, _ = run("tail -20 /usr/trim/nginx/logs/error.log 2>/dev/null || echo 'no log'")
    print(out)
    
    # Check nginx access log
    print("\n=== Nginx access log ===")
    out, _ = run("tail -10 /usr/trim/nginx/logs/access.log 2>/dev/null || echo 'no log'")
    print(out)
    
    # Check ModuForge health
    print("\n=== ModuForge health ===")
    out, _ = run("curl -s http://localhost:8087/health")
    print(out)
    
    # Check moduforge.conf content
    print("\n=== moduforge.conf ===")
    out, _ = run("cat /usr/trim/nginx/conf/conf.d/moduforge.conf")
    print(out[:500])
    
    # Test nginx config again
    print("\n=== Test nginx config ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -t 2>&1")
    print(f"Output: {out}")
    print(f"Error: {err}")
    
    client.close()

if __name__ == "__main__":
    main()
