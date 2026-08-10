#!/usr/bin/env python3
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode()

# Check zipper.go in container
print("=== Current zipper.go ===")
print(run("docker exec moduforge cat /src/backend/internal/service/zipper.go"))

# Check ExportModuleZip function
print("\n=== ExportModuleZip function ===")
print(run("docker exec moduforge grep -A 30 'func.*ExportModuleZip' /src/backend/internal/service/zipper.go"))

ssh.close()
