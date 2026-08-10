#!/usr/bin/env python3
"""Debug database path issue."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def debug():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Volume mount point ===")
    stdin, stdout, stderr = ssh.exec_command("docker volume inspect moduforge_data --format='{{.Mountpoint}}'")
    mountpoint = stdout.read().decode().strip()
    print(f"Mountpoint: {mountpoint}")
    
    print("\n=== Files in volume ===")
    stdin, stdout, stderr = ssh.exec_command(f"ls -la {mountpoint}")
    print(stdout.read().decode())
    
    print("\n=== Container env ===")
    stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format='{{{{range .Config.Env}}}}{{println .}}{{end}}'")
    print(stdout.read().decode())
    
    print("\n=== Container mounts ===")
    stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format='{{{{range .Mounts}}}}Type:{{.Type}} Source:{{.Source}} Destination:{{.Destination}}{{println}}{{end}}'")
    print(stdout.read().decode())
    
    print("\n=== Check if /data exists in container ===")
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} ls -la /data/ 2>&1 || echo 'no /data'")
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    debug()
