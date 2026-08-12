#!/usr/bin/env python3
"""
Test script for ModuForge Git Push Optimizations
"""

import requests
import json
import sys

BASE_URL = "http://localhost:8086/api/v1"
TOKEN = "your_token_here"  # Replace with actual token

headers = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json"
}

def test_preview_files(project_id):
    """Test preview files endpoint"""
    print("Testing preview files...")
    
    payload = {
        "project_id": project_id,
        "exclude_patterns": ["*.log", "test/**"]
    }
    
    try:
        response = requests.post(f"{BASE_URL}/git/preview-files", 
                               headers=headers, 
                               json=payload)
        if response.status_code == 200:
            data = response.json()
            print(f"✅ Preview files successful: {data['count']} files")
            for f in data['files'][:5]:  # Show first 5 files
                print(f"  - {f}")
            return True
        else:
            print(f"❌ Preview files failed: {response.status_code} - {response.text}")
            return False
    except Exception as e:
        print(f"❌ Error: {e}")
        return False

def test_push_optimized(project_id, dry_run=True):
    """Test optimized push endpoint"""
    print(f"\nTesting optimized push (dry_run={dry_run})...")
    
    payload = {
        "project_id": project_id,
        "remote": "origin",
        "branch": "main",
        "exclude_patterns": ["*.log", "test/**"],
        "commit_message": "Test optimized push",
        "dry_run": dry_run
    }
    
    try:
        response = requests.post(f"{BASE_URL}/git/push-optimized", 
                               headers=headers, 
                               json=payload)
        if response.status_code == 200:
            data = response.json()
            print(f"✅ Optimized push successful: {data['status']}")
            print(f"   Output: {data['output']}")
            return True
        else:
            print(f"❌ Optimized push failed: {response.status_code} - {response.text}")
            return False
    except Exception as e:
        print(f"❌ Error: {e}")
        return False

def test_file_tree(project_id):
    """Test file tree endpoint"""
    print(f"\nTesting file tree...")
    
    try:
        response = requests.get(f"{BASE_URL}/projects/{project_id}/tree", 
                              headers=headers)
        if response.status_code == 200:
            data = response.json()
            print(f"✅ File tree successful")
            print(f"   Root has {len(data.get('children', []))} items")
            return True
        else:
            print(f"❌ File tree failed: {response.status_code} - {response.text}")
            return False
    except Exception as e:
        print(f"❌ Error: {e}")
        return False

def test_publish_release(project_id, build_id):
    """Test publish to release endpoint"""
    print(f"\nTesting publish to release...")
    
    payload = {
        "token": "your_github_token"  # Replace with actual token
    }
    
    try:
        response = requests.post(f"{BASE_URL}/projects/{project_id}/builds/{build_id}/release", 
                               headers=headers, 
                               json=payload)
        if response.status_code == 200:
            data = response.json()
            print(f"✅ Publish to release successful")
            print(f"   Release URL: {data['release']['html_url']}")
            return True
        else:
            print(f"❌ Publish to release failed: {response.status_code} - {response.text}")
            return False
    except Exception as e:
        print(f"❌ Error: {e}")
        return False

def main():
    """Main test function"""
    print("=" * 60)
    print("ModuForge Git Push Optimizations - Test Script")
    print("=" * 60)
    
    # You need to replace these with actual IDs
    project_id = "your_project_id"
    build_id = "your_build_id"
    
    if project_id == "your_project_id":
        print("\n⚠️  Please update project_id and build_id in the script")
        print("   Replace 'your_project_id' with an actual project ID")
        print("   Replace 'your_build_id' with an actual build ID")
        return
    
    # Run tests
    tests = [
        ("Preview Files", lambda: test_preview_files(project_id)),
        ("File Tree", lambda: test_file_tree(project_id)),
        ("Push Optimized (Dry Run)", lambda: test_push_optimized(project_id, dry_run=True)),
        ("Publish Release", lambda: test_publish_release(project_id, build_id)),
    ]
    
    results = []
    for test_name, test_func in tests:
        print(f"\n{'='*60}")
        print(f"Running: {test_name}")
        print('='*60)
        try:
            result = test_func()
            results.append((test_name, result))
        except Exception as e:
            print(f"❌ Test failed with exception: {e}")
            results.append((test_name, False))
    
    # Summary
    print("\n" + "=" * 60)
    print("TEST SUMMARY")
    print("=" * 60)
    
    passed = sum(1 for _, result in results if result)
    total = len(results)
    
    for test_name, result in results:
        status = "✅ PASS" if result else "❌ FAIL"
        print(f"{status} - {test_name}")
    
    print(f"\nTotal: {passed}/{total} tests passed")
    
    if passed == total:
        print("\n🎉 All tests passed!")
        sys.exit(0)
    else:
        print("\n❌ Some tests failed")
        sys.exit(1)

if __name__ == "__main__":
    main()