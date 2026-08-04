import paramiko
import sys
import os
import time

if sys.platform == 'win32':
    os.system('chcp 65001 >nul 2>&1')
    sys.stdout.reconfigure(encoding='utf-8')
    sys.stderr.reconfigure(encoding='utf-8')

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

def execute(cmd, timeout=120):
    print(f"  >> {cmd[:150]}...")
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout)
    exit_code = stdout.channel.recv_exit_status()
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out.strip(): print(f"  OUT: {out.strip()[-500:]}")
    if err.strip(): print(f"  ERR: {err.strip()[-500:]}")
    return exit_code, out, err

# First, check what groups admin belongs to
print("=== Check user groups ===")
execute("id admin")

# Fix permissions - clean up and recreate properly
print("\n=== Fix permissions ===")
execute("echo 'csq0216' | sudo -S rm -rf /home/admin/mf_build /home/admin/mf_out")
execute("echo 'csq0216' | sudo -S mkdir -p /home/admin/mf_build /home/admin/mf_out")
# Get the group name from id output
execute("echo 'csq0216' | sudo -S chown -R admin /home/admin/mf_build /home/admin/mf_out")
execute("echo 'csq0216' | sudo -S chmod -R 755 /home/admin/mf_build /home/admin/mf_out")

# Verify
execute("ls -la /home/admin/ | grep mf_")

BUILD_DIR = "/home/admin/mf_build"
OUT_DIR = "/home/admin/mf_out"

print("\n=== Upload source code ===")
sftp = client.open_sftp()
local_backend = r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend"

uploaded = 0
for root, dirs, files in os.walk(local_backend):
    for f in files:
        if f.endswith('.go') or f in ('go.mod', 'go.sum'):
            local_path = os.path.join(root, f)
            rel = os.path.relpath(local_path, local_backend).replace('\\', '/')
            remote_path = f"{BUILD_DIR}/{rel}"
            remote_dir = os.path.dirname(remote_path)
            execute(f"mkdir -p {remote_dir}")
            try:
                sftp.put(local_path, remote_path)
                uploaded += 1
            except Exception as e:
                print(f"  Failed: {rel}: {e}")

sftp.close()
print(f"  Uploaded {uploaded} files")

print("\n=== Build with CGO_ENABLED=1 ===")
code, out, err = execute(
    f'docker run --rm --entrypoint sh '
    f'-v {BUILD_DIR}:/src '
    f'-v {OUT_DIR}:/out '
    f'moduforge-app -c "ls /src/go.mod && cd /src && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/moduforge_cgo ./cmd/moduforge 2>&1"',
    timeout=600
)
print(f"  Build exit code: {code}")

print("\n=== Verify ===")
code, out, err = execute(f"ls -la {OUT_DIR}/")

if "moduforge_cgo" in out:
    print("\n=== Deploy ===")
    execute("docker stop moduforge-app-1 2>/dev/null || true")
    time.sleep(2)
    execute(f"docker cp {OUT_DIR}/moduforge_cgo moduforge-app-1:/app/server")
    execute("docker start moduforge-app-1")
    time.sleep(15)
    code, out, err = execute('docker ps --filter name=moduforge-app-1 --format "{{.Status}}"')
    print(f"  Status: {out.strip()}")
    time.sleep(3)
    code, out, err = execute("docker logs moduforge-app-1 --tail 15 2>&1")
    print(f"  Logs:\n{out.strip()[-500:]}")
else:
    print("  Binary not found!")

execute(f"echo 'csq0216' | sudo -S rm -rf {BUILD_DIR} {OUT_DIR}")
client.close()
