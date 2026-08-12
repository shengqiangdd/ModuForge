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

print("=== module.prop ===")
run(f"sudo docker exec moduforge cat '{PROJ}/module.prop'")

print("\n=== service.sh binary refs ===")
run(f"sudo docker exec moduforge grep -n 'system/bin' '{PROJ}/service.sh'")

print("\n=== customize.sh binary refs ===")
run(f"sudo docker exec moduforge grep -n 'system/bin' '{PROJ}/customize.sh'")

print("\n=== system/bin/ ===")
run(f"sudo docker exec moduforge ls -la '{PROJ}/system/bin/' | grep -E '(andromon|linucb|androwui)'")

ssh.close()
