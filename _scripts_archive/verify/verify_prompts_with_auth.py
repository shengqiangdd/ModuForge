#!/usr/bin/env python3
"""Verify prompt system with authentication"""

import sys
import json
import requests

# Fix Windows GBK encoding
if sys.platform == "win32":
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

base_url = "http://192.168.2.9:8086"

# First, login to get a token
print("1. Logging in...")
try:
    login_data = {
        "username": "csq",
        "password": "csq0216"
    }
    response = requests.post(f"{base_url}/api/v1/auth/login", json=login_data, timeout=5)
    if response.status_code == 200:
        token = response.json().get("token")
        print(f"✓ Login successful, token: {token[:20]}...")
    else:
        print(f"✗ Login failed: {response.status_code} {response.text[:200]}")
        token = None
except Exception as e:
    print(f"✗ Login failed: {e}")
    token = None

# Set up headers with token
headers = {}
if token:
    headers["Authorization"] = f"Bearer {token}"

# Test 1: Check MD prompts endpoint with auth
print("\n2. Testing MD prompts endpoint with auth...")
try:
    response = requests.get(f"{base_url}/api/v1/md-prompts", headers=headers, timeout=5)
    if response.status_code == 200:
        prompts = response.json()
        print(f"✓ Found {len(prompts.get('prompts', []))} MD prompts:")
        for p in prompts.get('prompts', [])[:5]:  # Show first 5
            print(f"  - {p.get('name', 'unknown')}: {p.get('size', 0)} bytes")
    else:
        print(f"✗ MD prompts failed: {response.status_code} {response.text[:200]}")
except Exception as e:
    print(f"✗ MD prompts failed: {e}")

# Test 2: Check a specific prompt with auth
print("\n3. Testing specific prompt (generate.md) with auth...")
try:
    response = requests.get(f"{base_url}/api/v1/md-prompts/generate.md", headers=headers, timeout=5)
    if response.status_code == 200:
        content = response.json().get("content", "")
        print(f"✓ generate.md loaded: {len(content)} chars")
        print(f"  Preview: {content[:100]}...")
    else:
        print(f"✗ generate.md failed: {response.status_code} {response.text[:200]}")
except Exception as e:
    print(f"✗ generate.md failed: {e}")

# Test 3: Check prompts API (database-backed) with auth
print("\n4. Testing prompts API (database-backed) with auth...")
try:
    response = requests.get(f"{base_url}/api/v1/prompts", headers=headers, timeout=5)
    if response.status_code == 200:
        prompts = response.json()
        print(f"✓ Found {len(prompts)} prompts in database")
        for p in prompts[:3]:  # Show first 3
            mode = p.get('mode', 'unknown')
            content_len = len(p.get('content', ''))
            print(f"  - {mode}: {content_len} chars")
    else:
        print(f"✗ Prompts API failed: {response.status_code} {response.text[:200]}")
except Exception as e:
    print(f"✗ Prompts API failed: {e}")

print("\n✓ Verification completed!")