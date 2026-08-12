import paramiko, json

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

# Check DB tables and data
print("=== DB Tables ===")
stdin, stdout, stderr = client.exec_command('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \".tables\""')
print(stdout.read().decode())

print("\n=== Users ===")
stdin, stdout, stderr = client.exec_command('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT id, username, role FROM users;\\""')
print(stdout.read().decode())

print("\n=== Projects ===")
stdin, stdout, stderr = client.exec_command('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT id, name, user_id FROM projects;\\""')
print(stdout.read().decode())

print("\n=== Messages count ===")
stdin, stdout, stderr = client.exec_command('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT COUNT(*) FROM messages;\\""')
print(stdout.read().decode())

print("\n=== AI Conversations ===")
stdin, stdout, stderr = client.exec_command('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT COUNT(*) FROM ai_conversations;\\""')
print(stdout.read().decode())

# Check container logs for errors
print("\n=== Container logs (last 20 lines) ===")
stdin, stdout, stderr = client.exec_command('docker logs moduforge --tail 20 2>&1')
print(stdout.read().decode())

# Check the docker-compose.yml on server
print("\n=== docker-compose.yml on server ===")
stdin, stdout, stderr = client.exec_command('cat /vol1/docker/docker-compose.yml 2>&1')
print(stdout.read().decode())

client.close()
