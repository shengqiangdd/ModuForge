#!/usr/bin/env python3
"""Test if the moduforge user can write to /data."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Write as moduforge user (uid 1000) ===")
    cmd = """docker run --rm --user 1000:1001 --entrypoint sh -v moduforge_data:/data moduforge:latest -c "whoami && id && ls -la /data && touch /data/test_from_user.txt && echo 'SUCCESS: Can write!' && ls -la /data/test_from_user.txt" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    print(stderr.read().decode())
    
    print("\n=== Test: Try creating database as moduforge user ===")
    cmd = """docker run --rm --user 1000:1001 --entrypoint sh -v moduforge_data:/data moduforge:latest -c "DB_PATH=/data/moduforge.db /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode()[:2000])
    print(stderr.read().decode()[:500])
    
    ssh.close()

if __name__ == '__main__':
    test()
