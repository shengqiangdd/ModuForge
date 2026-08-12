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
TMP = "/tmp/moduforge_patch"

# Create temp dir on host
run(f"mkdir -p {TMP}")

# Upload via SFTP to /tmp on host
sftp = ssh.open_sftp()
files_to_upload = [
    ("module.prop", f"{TMP}/module.prop"),
    ("service.sh", f"{TMP}/service.sh"),
    ("customize.sh", f"{TMP}/customize.sh"),
    ("build.sh", f"{TMP}/build_rust.sh"),
]
for local_name, remote_path in files_to_upload:
    local_path = os.path.join(PATCH, local_name)
    print(f"Uploading {local_name}")
    sftp.put(local_path, remote_path)
sftp.close()

# Copy into container
print("\n=== Copying into container ===")
run(f"sudo docker cp {TMP}/module.prop moduforge:{PROJ}/module.prop")
run(f"sudo docker cp {TMP}/service.sh moduforge:{PROJ}/service.sh")
run(f"sudo docker cp {TMP}/customize.sh moduforge:{PROJ}/customize.sh")
run(f"sudo docker cp {TMP}/build_rust.sh moduforge:{PROJ}/src/rust/build.sh")

# Copy Go binary from wrong path to correct path
print("\n=== Moving Go binary ===")
GO_WRONG = f"{PROJ}/src/go/data/storage/projects/1785249992652501794-1864/system/bin/androwui"
GO_RIGHT = f"{PROJ}/system/bin/androwui"
run(f"sudo docker exec moduforge cp '{GO_WRONG}' '{GO_RIGHT}'")
run(f"sudo docker exec moduforge chmod 755 '{GO_RIGHT}'")

# Set permissions
print("\n=== Setting permissions ===")
run(f"sudo docker exec moduforge chmod 755 '{PROJ}/service.sh'")
run(f"sudo docker exec moduforge chmod 755 '{PROJ}/customize.sh'")
run(f"sudo docker exec moduforge chmod 755 '{PROJ}/src/rust/build.sh'")

# Cleanup temp
run(f"rm -rf {TMP}")

# === Verify ===
print("\n=== Verification ===")
print("--- module.prop ---")
run(f"sudo docker exec moduforge cat '{PROJ}/module.prop'")

print("\n--- system/bin/ ---")
run(f"sudo docker exec moduforge ls -la '{PROJ}/system/bin/' | grep -E '(andromon|linucb|androwui)'")

print("\n--- service.sh binary refs ---")
run(f"sudo docker exec moduforge grep -n 'system/bin' '{PROJ}/service.sh'")

print("\n--- customize.sh binary refs ---")
run(f"sudo docker exec moduforge grep -n 'system/bin' '{PROJ}/customize.sh'")

print("\n--- Rust build.sh cp ---")
run(f"sudo docker exec moduforge grep 'cp.*target' '{PROJ}/src/rust/build.sh'")

ssh.close()
print("\nAll fixes applied!")
