import paramiko, json

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Check backup DB
print("=== Backup DB tables ===")
out, err = run('docker exec moduforge sh -c "wc -c /data/moduforge.db /data/moduforge.db.bak"')
print(out)

# Try to install sqlite3 in the container
print("\n=== Install sqlite3 ===")
out, err = run('docker exec moduforge apk add --no-cache sqlite 2>&1 | tail -5')
print(out)

# Now check backup DB
print("\n=== Backup DB - Users ===")
out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db.bak \\"SELECT id, username, email, created_at FROM users;\\""')
print(out)

print("\n=== Backup DB - Projects ===")
out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db.bak \\"SELECT id, name, user_id FROM projects;\\""')
print(out)

print("\n=== Backup DB - Messages count ===")
out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db.bak \\"SELECT COUNT(*) FROM messages;\\""')
print(out)

print("\n=== Current DB - Users ===")
out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT id, username, email, created_at FROM users;\\""')
print(out)

print("\n=== Current DB - Projects ===")
out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT id, name, user_id FROM projects;\\""')
print(out)

client.close()
