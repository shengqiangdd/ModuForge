#!/usr/bin/env python3
"""Download and analyze the zip file."""

import paramiko
import zipfile
import os
from pathlib import Path

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
LOCAL_DIR = Path("C:/Users/22875/.qwenpaw/workspaces/default/ModuForge/build_artifacts")

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Download the zip file
print("Downloading zip file...")
sftp = ssh.open_sftp()
local_zip = LOCAL_DIR / "test_module_v2.zip"
sftp.get("/tmp/test_module_v2.zip", str(local_zip))
sftp.close()
print(f"Downloaded to: {local_zip}")

# Analyze the zip
print("\n=== Analyzing ZIP contents ===")
with zipfile.ZipFile(local_zip, 'r') as zip_ref:
    print("\nAll files in zip:")
    for info in zip_ref.infolist():
        print(f"  {info.filename} ({info.file_size} bytes)")
    
    print("\n=== Checking for webroot wrapper ===")
    has_webroot = any('webroot/' in info.filename for info in zip_ref.infolist())
    print(f"  Has webroot directory: {has_webroot}")
    
    if has_webroot:
        print("\n  Files under webroot/:")
        for info in zip_ref.infolist():
            if 'webroot/' in info.filename:
                print(f"    {info.filename}")
    
    print("\n=== Checking for excluded files (should NOT be present) ===")
    excluded_patterns = [
        ('.build_cache/', 'Build cache'),
        ('src/', 'Source code'),
        ('tmp/', 'Temp files'),
        ('DESIGN_DOC.md', 'Design doc'),
        ('app/backend/', 'Backend source'),
        ('.d', 'Debug symbols'),
        ('.cargo-', 'Cargo lock'),
    ]
    
    for pattern, desc in excluded_patterns:
        found = any(pattern in info.filename for info in zip_ref.infolist())
        status = "FOUND (BAD!)" if found else "excluded (GOOD)"
        print(f"  {desc} ({pattern}): {status}")
    
    print("\n=== Expected files (should be present) ===")
    expected = [
        'META-INF/com/google/android/update-binary',
        'module.prop',
        'service.sh',
        'customize.sh',
    ]
    
    all_files = [info.filename for info in zip_ref.infolist()]
    for exp in expected:
        found = exp in all_files
        status = "present (GOOD)" if found else "MISSING (BAD!)"
        print(f"  {exp}: {status}")

ssh.close()
print("\nDone!")
