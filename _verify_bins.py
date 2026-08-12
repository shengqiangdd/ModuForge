import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    err = e.read().decode(errors='replace').strip()
    if out: print(out)
    if err and 'ERR' not in err: print(f"STDERR: {err[:500]}")
    return out

PROJ = "/data/storage/projects/1785249992652501794-1864"

# 1. Confirm actual Rust binary name from Cargo.toml
print("=== Cargo.toml bin name ===")
run(f"sudo docker exec moduforge grep -A2 '\\[\\[bin\\]\\]' {PROJ}/src/rust/Cargo.toml")

# 2. Confirm Rust build.sh copies what name
print("\n=== Rust build.sh copy line ===")
run(f"sudo docker exec moduforge grep 'cp.*target' {PROJ}/src/rust/build.sh")

# 3. Confirm what's in system/bin
print("\n=== system/bin actual files ===")
run(f"sudo docker exec moduforge ls -la {PROJ}/system/bin/ | grep -v total | grep -v '^\.' | grep -v cargo")

# 4. Confirm Go binary
print("\n=== Go binary path ===")
run(f"sudo docker exec moduforge find {PROJ}/src/go -name 'androwui' -o -name 'daemon' 2>/dev/null")
run(f"sudo docker exec moduforge ls -la {PROJ}/src/go/data/ 2>/dev/null || echo 'no data dir'")

# 5. Check Go build output path (the weird nested path)
print("\n=== Go build output ===")
run(f"sudo docker exec moduforge ls -la '{PROJ}/src/go/data/storage/projects/1785249992652501794-1864/system/bin/' 2>/dev/null || echo 'nested path missing'")

ssh.close()
