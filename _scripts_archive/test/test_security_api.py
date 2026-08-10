#!/usr/bin/env python3
"""Test security API endpoints"""
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
    
    print("Testing Security API Endpoints...\n")
    
    # Test 1: Get security rules
    print("1. Get Security Rules:")
    stdin, stdout, stderr = client.exec_command("curl -s http://localhost:8086/api/v1/agent/security/rules")
    out = stdout.read().decode()
    try:
        data = json.loads(out)
        rules = data.get("rules", [])
        print(f"   ✓ {len(rules)} rules loaded")
        for r in rules[:3]:
            print(f"     - {r['Name']}: Risk {r['RiskScore']}")
    except:
        print(f"   ✗ Failed: {out[:200]}")
    
    # Test 2: Check dangerous command
    print("\n2. Check Dangerous Command (rm -rf /):")
    stdin, stdout, stderr = client.exec_command(
        'curl -s -X POST http://localhost:8086/api/v1/agent/security/check -H "Content-Type: application/json" -d \'{"command": "rm -rf /"}\''
    )
    out = stdout.read().decode()
    try:
        data = json.loads(out)
        print(f"   Level: {data.get('level')}")
        print(f"   Risk Score: {data.get('risk_score')}")
        print(f"   Rules: {[r['Name'] for r in data.get('rules', [])]}")
    except:
        print(f"   ✗ Failed: {out[:200]}")
    
    # Test 3: Check safe command
    print("\n3. Check Safe Command (git status):")
    stdin, stdout, stderr = client.exec_command(
        'curl -s -X POST http://localhost:8086/api/v1/agent/security/check -H "Content-Type: application/json" -d \'{"command": "git status"}\''
    )
    out = stdout.read().decode()
    try:
        data = json.loads(out)
        print(f"   Level: {data.get('level')} (0=Auto)")
        print(f"   Risk Score: {data.get('risk_score')}")
    except:
        print(f"   ✗ Failed: {out[:200]}")
    
    # Test 4: Check confirm command
    print("\n4. Check Confirm Command (git push --force):")
    stdin, stdout, stderr = client.exec_command(
        'curl -s -X POST http://localhost:8086/api/v1/agent/security/check -H "Content-Type: application/json" -d \'{"command": "git push --force"}\''
    )
    out = stdout.read().decode()
    try:
        data = json.loads(out)
        print(f"   Level: {data.get('level')} (1=Confirm)")
        print(f"   Risk Score: {data.get('risk_score')}")
    except:
        print(f"   ✗ Failed: {out[:200]}")
    
    # Test 5: Get audit log
    print("\n5. Get Security Audit Log:")
    stdin, stdout, stderr = client.exec_command("curl -s http://localhost:8086/api/v1/agent/security/audit")
    out = stdout.read().decode()
    try:
        data = json.loads(out)
        entries = data.get("entries", [])
        print(f"   ✓ {len(entries)} audit entries")
    except:
        print(f"   ✗ Failed: {out[:200]}")
    
    print("\n✅ All tests completed!")
    client.close()

if __name__ == "__main__":
    main()
