#!/usr/bin/env python3
"""Test rate limiting"""
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
    
    def run(cmd, timeout=10):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Test rate limiting on auth endpoint
    print("=== Rate limiting test (auth endpoint, limit 5r/m) ===")
    for i in range(8):
        out, _ = run(f"curl -s -o /dev/null -w '%{{http_code}}' http://localhost:8086/api/v1/auth/login")
        print(f"  Request {i+1}: {out}")
    
    print("\n=== Rate limiting test (API endpoint, limit 10r/s) ===")
    for i in range(25):
        out, _ = run(f"curl -s -o /dev/null -w '%{{http_code}}' http://localhost:8086/api/v1/agent/skills")
        print(f"  Request {i+1}: {out}")
    
    # Final status summary
    print("\n=== Final Status ===")
    out, _ = run("docker ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}'")
    print(out)
    
    out, _ = run("curl -sI http://localhost:8086/health")
    # Extract security headers
    for line in out.split('\n'):
        if any(h in line.lower() for h in ['x-content-type', 'x-frame', 'x-xss', 'referrer', 'strict-transport', 'permissions']):
            print(f"  ✓ {line.strip()}")
    
    client.close()

if __name__ == "__main__":
    main()
