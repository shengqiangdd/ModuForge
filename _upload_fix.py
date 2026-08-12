import paramiko
import os

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=30):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    err = e.read().decode(errors='replace').strip()
    if out: print(out[:1500])
    if err: print(f"STDERR: {err[:500]}")
    return out

PROJ = "/data/storage/projects/1785249992652501794-1864"
PATCH = "ModuForge/_patch"

# Upload all patch files via SCP
sftp = ssh.open_sftp()

files_to_upload = [
    ("module.prop", f"{PROJ}/module.prop"),
    ("service.sh", f"{PROJ}/service.sh"),
    ("customize.sh", f"{PROJ}/customize.sh"),
    ("build.sh", f"{PROJ}/src/rust/build.sh"),
]

for local_name, remote_path in files_to_upload:
    local_path = os.path.join(PATCH, local_name)
    print(f"Uploading {local_name} -> {remote_path}")
    sftp.put(local_path, remote_path)

# Copy Go binary from wrong path to correct path
print("\nCopying Go binary...")
GO_WRONG = f"{PROJ}/src/go/data/storage/projects/1785249992652501794-1864/system/bin/androwui"
GO_RIGHT = f"{PROJ}/system/bin/androwui"
run(f"sudo docker exec moduforge cp '{GO_WRONG}' '{GO_RIGHT}'")
run(f"sudo docker exec moduforge chmod 755 '{GO_RIGHT}'")

# Set permissions on shell scripts
print("\nSetting permissions...")
run(f"sudo docker exec moduforge chmod 755 '{PROJ}/service.sh'")
run(f"sudo docker exec moduforge chmod 755 '{PROJ}/customize.sh'")
run(f"sudo docker exec moduforge chmod 755 '{PROJ}/src/rust/build.sh")

sftp.close()

# Verify
print("\n=== Verification ===")
print("--- module.prop ---")
run(f"sudo docker exec moduforge cat '{PROJ}/module.prop'")

print("\n--- system/bin/ binaries ---")
run(f"sudo docker exec moduforge ls -la '{PROJ}/system/bin/' | grep -E '(andromon|linucb|androwui)'")

print("\n--- service.sh: binary refs ---")
run(f"sudo docker exec moduforge grep -n 'system/bin' '{PROJ}/service.sh'")

print("\n--- customize.sh: binary refs ---")
run(f"sudo docker exec moduforge grep -n 'system/bin' '{PROJ}/customize.sh'")

print("\n--- Rust build.sh: cp line ---")
run(f"sudo docker exec moduforge grep 'cp.*target' '{PROJ}/src/rust/build.sh'")

ssh.close()
print("\nDone!")
