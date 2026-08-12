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

# Full file tree
print("=== File tree ===")
run(f"sudo docker exec moduforge find {PROJ} -type f | sort")

# module.prop
print("\n=== module.prop ===")
run(f"sudo docker exec moduforge cat {PROJ}/module.prop")

# service.sh
print("\n=== service.sh ===")
run(f"sudo docker exec moduforge cat {PROJ}/service.sh")

# customize.sh
print("\n=== customize.sh ===")
run(f"sudo docker exec moduforge cat {PROJ}/customize.sh")

# META-INF
print("\n=== META-INF structure ===")
run(f"sudo docker exec moduforge find {PROJ}/META-INF -type f 2>/dev/null || echo 'META-INF NOT FOUND!'")

# update-binary
print("\n=== META-INF/com/google/android/update-binary ===")
run(f"sudo docker exec moduforge cat {PROJ}/META-INF/com/google/android/update-binary 2>/dev/null || echo 'update-binary NOT FOUND!'")

# updater-script
print("\n=== META-INF/com/google/android/updater-script ===")
run(f"sudo docker exec moduforge cat {PROJ}/META-INF/com/google/android/updater-script 2>/dev/null || echo 'updater-script NOT FOUND!'")

ssh.close()
