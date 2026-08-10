#!/usr/bin/env python3
"""Deploy security improvements to ModuForge server"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

import paramiko
import os

# Server config
HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
LOCAL_BINARY = "moduforge-server-linux"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    
    try:
        print(f"Connecting to {HOST}...")
        client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
        print("Connected!")
        
        # Upload binary to /tmp first (world-writable)
        print(f"Uploading {LOCAL_BINARY}...")
        sftp = client.open_sftp()
        local_path = os.path.join(os.path.dirname(__file__), LOCAL_BINARY)
        sftp.put(local_path, f"/tmp/moduforge-server")
        sftp.close()
        print("Upload complete!")
        
        # Copy to container - replace the actual running binary
        print("Copying to container...")
        stdin, stdout, stderr = client.exec_command(
            f"docker cp /tmp/moduforge-server {CONTAINER}:/server"
        )
        exit_code = stdout.channel.recv_exit_status()
        if exit_code != 0:
            print(f"Copy failed: {stderr.read().decode()}")
            return False
        
        # Make executable
        print("Setting permissions...")
        client.exec_command(f"docker exec {CONTAINER} chmod +x /server")
        
        # Restart container
        print("Restarting container...")
        stdin, stdout, stderr = client.exec_command(f"docker restart {CONTAINER}")
        stdout.read()  # Wait for completion
        
        # Wait for health check
        import time
        print("Waiting for health check...")
        time.sleep(5)
        
        # Verify
        stdin, stdout, stderr = client.exec_command("curl -s http://localhost:8086/health")
        health = stdout.read().decode()
        print(f"Health check: {health}")
        
        print("\n✅ Deployment successful!")
        print(f"   URL: http://{HOST}:8086")
        return True
        
    except Exception as e:
        print(f"Error: {e}")
        return False
    finally:
        client.close()

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
