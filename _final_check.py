import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    if out: print(out)
    return out

PROJ = "/data/storage/projects/1785249992652501794-1864"

# Check if system/bin has daemon (Go binary)
print("=== Go binary check ===")
run(f"sudo docker exec moduforge file {PROJ}/system/bin/andromon 2>/dev/null")
run(f"sudo docker exec moduforge file {PROJ}/system/bin/linucb-engine 2>/dev/null")

# Check the actual zip output (full listing)
print("\n=== Full output.zip ===")
run(f"sudo docker exec moduforge unzip -l /data/storage/projects/output.zip 2>/dev/null")

# Check if androwui is in system/bin
print("\n=== androwui in system/bin? ===")
run(f"sudo docker exec moduforge ls -la {PROJ}/system/bin/androwui 2>/dev/null || echo 'androwui NOT in system/bin'")

# Check Go binary build output location
print("\n=== Go build output in source tree ===")
run(f"sudo docker exec moduforge find {PROJ}/src/go -type f -perm /111 | head -10")

ssh.close()
