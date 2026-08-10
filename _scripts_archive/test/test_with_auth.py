#!/usr/bin/env python3
"""Test security API with authentication"""
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
    
    # First, get a token by logging in
    print("1. Logging in...")
    stdin, stdout, stderr = client.exec_command(
        'curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\''
    )
    login_resp = stdout.read().decode()
    try:
        login_data = json.loads(login_resp)
        token = login_data.get("token", "")
        print(f"   ✓ Got token: {token[:20]}...")
    except:
        print(f"   ✗ Login failed: {login_resp[:200]}")
        return
    
    # Test security rules with auth
    print("\n2. Get Security Rules:")
    stdin, stdout, stderr = client.exec_command(
        f'curl -s http://localhost:8086/api/v1/agent/security/rules -H "Authorization: Bearer {token}"'
    )
    out = stdout.read().decode()
    try:
        data = json.loads(out)
        rules = data.get("rules", [])
        print(f"   ✓ {len(rules)} rules loaded")
        for r in rules[:3]:
            print(f"     - {r.get('Name', 'N/A')}: Risk {r.get('RiskScore', 0)}")
    except:
        print(f"   ✗ Failed: {out[:200]}")
    
    # Test dangerous command check
    print("\n3. Check Dangerous Command (rm -rf /):")
    stdin, stdout, stderr = client.exec_command(
        f'curl -s -X POST http://localhost:8086/api/v1/agent/security/check -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d \'{{"command": "rm -rf /"}}\''
    )
    out = stdout.read().decode()
    try:
        data = json.loads(out)
        print(f"   Level: {data.get('level')} (2=Deny)")
        print(f"   Risk Score: {data.get('risk_score')}")
        rules = data.get('rules', [])
        if rules:
            print(f"   Rules: {[r.get('Name', 'N/A') for r in rules]}")
    except:
        print(f"   ✗ Failed: {out[:200]}")
    
    # Test safe command
    print("\n4. Check Safe Command (git status):")
    stdin, stdout, stderr = client.exec_command(
        f'curl -s -X POST http://localhost:8086/api/v1/agent/security/check -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d \'{{"command": "git status"}}\''
    )
    out = stdout.read().decode()
    try:
        data = json.loads(out)
        print(f"   Level: {data.get('level')} (0=Auto)")
        print(f"   Risk Score: {data.get('risk_score')}")
    except:
        print(f"   ✗ Failed: {out[:200]}")
    
    print("\n✅ Security API tests completed!")
    client.close()

if __name__ == "__main__":
    main()
