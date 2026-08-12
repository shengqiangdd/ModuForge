import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Find the seed/init functions in source code
print("=== Search for Seed functions ===")
out, err = run("grep -rn 'func.*Seed\\|func.*Init\\|func.*Migrate\\|AutoMigrate' /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/database/ 2>/dev/null | head -30")
print(out)

print("\n=== Search for seed calls in main/init ===")
out, err = run("grep -rn 'Seed\\|Init\\|Migrate' /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/cmd/moduforge/main.go 2>/dev/null")
print(out)

print("\n=== Search for AutoMigrate in database package ===")
out, err = run("grep -rn 'AutoMigrate' /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/database/*.go 2>/dev/null | head -20")
print(out)

print("\n=== database.go or db.go init function ===")
out, err = run("head -100 /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/database/database.go 2>/dev/null")
print(out)

client.close()
