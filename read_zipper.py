#!/usr/bin/env python3
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode()

# Read the actual running zipper.go
print("=== zipper.go (from overlay2) ===")
print(run("cat /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/service/zipper.go"))

ssh.close()
