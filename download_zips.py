#!/usr/bin/env python3
"""Download and analyze the actual zip files."""

import paramiko
import os
import zipfile
from pathlib import Path

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
LOCAL_DIR = Path("C:/Users/22875/.qwenpaw/workspaces/default/ModuForge/build_artifacts")

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Copy the zip files to a temp location on server
print("Copying zip files on server...")
ssh.exec_command(f"docker cp {CONTAINER}:/tmp/build_cache_latest.zip /tmp/build_cache_latest.zip")
ssh.exec_command(f"docker cp {CONTAINER}:/tmp/output.zip /tmp/output.zip")

# Also copy the project directory's output
ssh.exec_command(f"docker exec {CONTAINER} cd /data/storage/projects/1785249992652501794-1864 && zip -r /tmp/project_output.zip . -x 'src/*' '.git/*' 'tmp/*' '*.log'")
ssh.exec_command(f"docker cp {CONTAINER}:/tmp/project_output.zip /tmp/project_output.zip")

sftp = ssh.open_sftp()

# Download the files
files_to_download = [
    ("/tmp/build_cache_latest.zip", "build_cache_latest.zip"),
    ("/tmp/output.zip", "output.zip"),
    ("/tmp/project_output.zip", "project_output.zip"),
]

for remote_path, local_name in files_to_download:
    print(f"\n=== Downloading {local_name} ===")
    local_file = LOCAL_DIR / local_name
    try:
        sftp.get(remote_path, str(local_file))
        print(f"Downloaded: {local_file} ({local_file.stat().st_size} bytes)")
        
        # Analyze zip contents
        print(f"\nContents of {local_name}:")
        with zipfile.ZipFile(local_file, 'r') as zip_ref:
            for info in zip_ref.infolist():
                print(f"  {info.filename} ({info.file_size} bytes)")
                
    except Exception as e:
        print(f"Error: {e}")

sftp.close()
ssh.close()
print("\nDone!")
