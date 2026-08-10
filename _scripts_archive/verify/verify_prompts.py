#!/usr/bin/env python3
"""Verify prompt system is working correctly"""

import sys
import json
import requests

# Fix Windows GBK encoding
if sys.platform == "win32":
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

base_url = "http://192.168.2.9:8086"

# Test 1: Check health endpoint
print("1. Testing health endpoint...")
try:
    response = requests.get(f"{base_url}/health", timeout=5)
    print(f"✓ Health: {response.json()}")
except Exception as e:
    print(f"✗ Health failed: {e}")

# Test 2: Check MD prompts endpoint
print("\n2. Testing MD prompts endpoint...")
try:
    response = requests.get(f"{base_url}/api/v1/md-prompts", timeout=5)
    if response.status_code == 200:
        prompts = response.json()
        print(f"✓ Found {len(prompts)} MD prompts:")
        for p in prompts[:5]:  # Show first 5
            print(f"  - {p.get('name', 'unknown')}: {p.get('size', 0)} bytes")
    else:
        print(f"✗ MD prompts failed: {response.status_code} {response.text[:200]}")
except Exception as e:
    print(f"✗ MD prompts failed: {e}")

# Test 3: Check a specific prompt
print("\n3. Testing specific prompt (generate.md)...")
try:
    response = requests.get(f"{base_url}/api/v1/md-prompts/generate.md", timeout=5)
    if response.status_code == 200:
        content = response.text
        print(f"✓ generate.md loaded: {len(content)} chars")
        print(f"  Preview: {content[:100]}...")
    else:
        print(f"✗ generate.md failed: {response.status_code} {response.text[:200]}")
except Exception as e:
    print(f"✗ generate.md failed: {e}")

# Test 4: Check prompts API (database-backed)
print("\n4. Testing prompts API (database-backed)...")
try:
    response = requests.get(f"{base_url}/api/v1/prompts", timeout=5)
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