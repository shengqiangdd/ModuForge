#!/usr/bin/env python3
"""Check authentication middleware"""

import sys
import json
import requests

# Fix Windows GBK encoding
if sys.platform == "win32":
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

base_url = "http://192.168.2.9:8086"

# Test 1: Check if login works
print("1. Testing login...")
try:
    login_data = {
        "username": "csq",
        "password": "csq0216"
    }
    response = requests.post(f"{base_url}/api/v1/auth/login", json=login_data, timeout=5)
    print(f"Login status: {response.status_code}")
    print(f"Login response: {response.text[:200]}")
    if response.status_code == 200:
        token = response.json().get("token")
        print(f"Token: {token[:50]}..." if token else "No token")
    else:
        token = None
except Exception as e:
    print(f"Login failed: {e}")
    token = None

# Set up headers with token
headers = {}
if token:
    headers["Authorization"] = f"Bearer {token}"

# Test 2: Check if we can access protected routes
print("\n2. Testing protected routes...")
try:
    response = requests.get(f"{base_url}/api/v1/projects", headers=headers, timeout=5)
    print(f"Projects status: {response.status_code}")
    print(f"Projects response: {response.text[:200]}")
except Exception as e:
    print(f"Projects failed: {e}")

# Test 3: Check if we can access the health endpoint
print("\n3. Testing health endpoint...")
try:
    response = requests.get(f"{base_url}/health", headers=headers, timeout=5)
    print(f"Health status: {response.status_code}")
    print(f"Health response: {response.text[:200]}")
except Exception as e:
    print(f"Health failed: {e}")

# Test 4: Check if we can access the MD prompts endpoint without auth
print("\n4. Testing MD prompts without auth...")
try:
    response = requests.get(f"{base_url}/api/v1/md-prompts", timeout=5)
    print(f"MD prompts status: {response.status_code}")
    print(f"MD prompts response: {response.text[:200]}")
except Exception as e:
    print(f"MD prompts failed: {e}")

# Test 5: Check if we can access the MD prompts endpoint with auth
print("\n5. Testing MD prompts with auth...")
try:
    response = requests.get(f"{base_url}/api/v1/md-prompts", headers=headers, timeout=5)
    print(f"MD prompts status: {response.status_code}")
    print(f"MD prompts response: {response.text[:200]}")
except Exception as e:
    print(f"MD prompts failed: {e}")

print("\n✓ Debug completed!")