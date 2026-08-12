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

# system/bin contents
print("=== system/bin/ contents ===")
run(f"sudo docker exec moduforge ls -la {PROJ}/system/bin/")

# Rust Cargo.toml bin name
print("\n=== Rust Cargo.toml [[bin]] ===")
run(f"sudo docker exec moduforge grep -A3 '\\[\\[bin\\]\\]' {PROJ}/src/rust/Cargo.toml")

# Rust build.sh
print("\n=== src/rust/build.sh ===")
run(f"sudo docker exec moduforge cat {PROJ}/src/rust/build.sh")

# Go main.go binary name
print("\n=== Go build.sh ===")
run(f"sudo docker exec moduforge cat {PROJ}/src/go/build.sh 2>/dev/null || echo 'no build.sh'")

# C++ build.sh
print("\n=== C++ build.sh ===")
run(f"sudo docker exec moduforge cat {PROJ}/src/cpp/build.sh 2>/dev/null || echo 'no build.sh'")

# Check the output.zip
print("\n=== output.zip contents ===")
run(f"sudo docker exec moduforge unzip -l /data/storage/projects/output.zip 2>/dev/null | head -30")

# Check if androwui binary exists anywhere
print("\n=== Find androwui ===")
run(f"sudo docker exec moduforge find {PROJ} -name 'androwui' -o -name 'daemon' 2>/dev/null")

# Check the zip builder code
print("\n=== Check builder zip code ===")
run(f"sudo docker exec moduforge grep -rn 'androst\\|andromon\\|androwui\\|androboost\\|daemon' {PROJ}/app/ 2>/dev/null | head -20")

ssh.close()
