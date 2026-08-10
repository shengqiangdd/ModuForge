#!/usr/bin/env python3
"""Download and analyze build artifacts."""

import paramiko
import os
import zipfile
from pathlib import Path
import tempfile

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
LOCAL_DIR = Path("C:/Users/22875/.qwenpaw/workspaces/default/ModuForge/build_artifacts")

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Create local directory
LOCAL_DIR.mkdir(exist_ok=True)

# Get a JWT token first
print("Getting JWT token...")
stdin, stdout, stderr = ssh.exec_command(
    """curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'"""
)
token_response = stdout.read().decode()
import json
token = json.loads(token_response).get('token', '')
print(f"Token: {token[:20]}...")

# Download artifacts
artifacts = [
    "/data/storage/artifacts/fa6d604a-9ae1-42a3-b4f3-a3a38a93c6b5/module.zip",
    "/data/storage/artifacts/31f46b36-a6ad-4a41-9b9e-face6dbfc461/module.zip",
    "/data/storage/artifacts/e60f6052-a706-45dd-9e44-510410adc9f2/module.zip",
    "/data/storage/artifacts/2521c9f1-dae9-4be2-915f-699c591f8a70/module.zip",
    "/data/storage/artifacts/f6fde127-d99b-48e5-b617-592d73073a65/module.zip",
    "/data/storage/artifacts/f26e14d4-528b-480a-96af-4ca5526ed815/module.zip",
]

sftp = ssh.open_sftp()

for i, artifact_path in enumerate(artifacts):
    print(f"\n=== Downloading artifact {i+1}: {artifact_path} ===")
    local_file = LOCAL_DIR / f"artifact_{i+1}.zip"
    try:
        sftp.get(artifact_path, str(local_file))
        print(f"Downloaded to: {local_file}")
        
        # Extract and analyze
        extract_dir = LOCAL_DIR / f"artifact_{i+1}"
        extract_dir.mkdir(exist_ok=True)
        
        with zipfile.ZipFile(local_file, 'r') as zip_ref:
            zip_ref.extractall(extract_dir)
        
        # List contents
        print(f"\nContents of artifact {i+1}:")
        for root, dirs, files in os.walk(extract_dir):
            level = root.replace(str(extract_dir), '').count(os.sep)
            indent = ' ' * 2 * level
            print(f"{indent}{os.path.basename(root)}/")
            subindent = ' ' * 2 * (level + 1)
            for file in files:
                print(f"{subindent}{file}")
                
    except Exception as e:
        print(f"Error downloading {artifact_path}: {e}")

sftp.close()
ssh.close()
print("\nDone!")
