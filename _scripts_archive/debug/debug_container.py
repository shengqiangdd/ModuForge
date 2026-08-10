# -*- coding: utf-8 -*-
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check container state
print("Status:", run('docker inspect moduforge --format "{{.State.Status}}"'))
print("ExitCode:", run('docker inspect moduforge --format "{{.State.ExitCode}}"'))
print("Error:", run('docker inspect moduforge --format "{{.State.Error}}"'))
print("OOMKilled:", run('docker inspect moduforge --format "{{.State.OOMKilled}}"'))

# Check logs (might be in different format)
print("\nLogs (all):")
print(run("docker logs moduforge 2>&1"))

# Check if entrypoint is correct
print("\nEntrypoint:", run('docker inspect moduforge --format "{{.Config.Entrypoint}}"'))
print("Cmd:", run('docker inspect moduforge --format "{{.Config.Cmd}}"'))
print("User:", run('docker inspect moduforge --format "{{.Config.User}}"'))

# Try to start manually
print("\n=== Manual start ===")
print(run("docker start moduforge 2>&1"))
time.sleep(3)
print("Status after start:", run('docker inspect moduforge --format "{{.State.Status}}"'))
print("Logs after:", run("docker logs moduforge --tail 5 2>&1"))

# Check if the binary inside is executable
print("\n=== Binary check ===")
# Create a temp container to check
run("docker rm -f temp-check 2>/dev/null")
print(run("docker create --name temp-check --entrypoint /bin/sh moduforge:patched -c 'ls -la /server /docker-entrypoint.sh'"))
print(run("docker start temp-check 2>&1"))
time.sleep(1)
print(run("docker logs temp-check 2>&1"))
run("docker rm -f temp-check 2>/dev/null")

ssh.close()
