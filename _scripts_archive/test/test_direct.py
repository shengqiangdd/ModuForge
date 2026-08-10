#!/usr/bin/env python3
"""Test security API directly on server"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import json

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    # Test with curl directly on server (no auth needed for local)
    print("Testing Security API on server...\n")
    
    # Test 1: Security rules
    print("1. Security Rules:")
    stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/api/v1/agent/security/rules')
    out = stdout.read().decode()
    print(f"   Response: {out[:300]}")
    
    # Test 2: Check command
    print("\n2. Check Command:")
    stdin, stdout, stderr = client.exec_command(
        '''curl -s -X POST http://localhost:8086/api/v1/agent/security/check -H "Content-Type: application/json" -d '{"command": "rm -rf /"}' '''
    )
    out = stdout.read().decode()
    print(f"   Response: {out[:300]}")
    
    # Test 3: Check container logs
    print("\n3. Recent Logs:")
    stdin, stdout, stderr = client.exec_command('docker logs moduforge --tail 5 2>&1')
    out = stdout.read().decode()
    print(f"   {out}")
    
    client.close()

if __name__ == "__main__":
    main()
