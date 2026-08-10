#!/usr/bin/env python3
"""Debug API calls to see what's happening"""

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
        print(f"✓ Login successful")
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

# Test 1: Check if the API is responding at all
print("\n2. Testing basic API endpoints...")
try:
    response = requests.get(f"{base_url}/api/v1/health", headers=headers, timeout=5)
    print(f"✓ /api/v1/health: {response.status_code}")
except Exception as e:
    print(f"✗ /api/v1/health failed: {e}")

# Test 2: Check if the prompts endpoint exists
print("\n3. Testing prompts endpoint...")
try:
    response = requests.get(f"{base_url}/api/v1/prompts", headers=headers, timeout=5)
    print(f"✓ /api/v1/prompts: {response.status_code}")
    if response.status_code != 200:
        print(f"  Response: {response.text[:200]}")
except Exception as e:
    print(f"✗ /api/v1/prompts failed: {e}")

# Test 3: Check if the MD prompts endpoint exists
print("\n4. Testing MD prompts endpoint...")
try:
    response = requests.get(f"{base_url}/api/v1/md-prompts", headers=headers, timeout=5)
    print(f"✓ /api/v1/md-prompts: {response.status_code}")
    if response.status_code != 200:
        print(f"  Response: {response.text[:200]}")
except Exception as e:
    print(f"✗ /api/v1/md-prompts failed: {e}")

# Test 4: Check if the skills endpoint exists
print("\n5. Testing skills endpoint...")
try:
    response = requests.get(f"{base_url}/api/v1/skills", headers=headers, timeout=5)
    print(f"✓ /api/v1/skills: {response.status_code}")
    if response.status_code != 200:
        print(f"  Response: {response.text[:200]}")
except Exception as e:
    print(f"✗ /api/v1/skills failed: {e}")

print("\n✓ Debug completed!")