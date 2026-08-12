"""Simple DB check using the running container"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# The container is running - use it to check
print('=== Check current DB ===')
cmds = [
    'docker exec moduforge ls -la /data/moduforge.db',
    # Install sqlite3 in running container
    'docker exec moduforge sh -c "apk add --no-cache sqlite 2>&1 | tail -3"',
    # Check integrity
    'docker exec moduforge sqlite3 /data/moduforge.db "PRAGMA integrity_check;"',
    # Check data
    'docker exec moduforge sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM users;"',
    'docker exec moduforge sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM projects;"',
    'docker exec moduforge sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM ai_conversations;"',
]

for cmd in cmds:
    print(f'\n>>> {cmd[:80]}')
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=15)
    out = stdout.read().decode(errors='replace')
    err = stderr.read().decode(errors='replace')
    if out: print(out)
    if err and 'Warning' not in err: print(f'ERR: {err[:200]}')

# If corrupted, try to recover from old volume
print('\n=== Try recovery from old volume ===')
cmd = """
# Stop container
cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down

# Try each backup
for bak in /vol1/docker/volumes/moduforge_data/_data/moduforge.db.bak \
           /vol1/docker/volumes/moduforge_data/_data/moduforge_recovered.db \
           /vol1/docker/volumes/moduforge_data/_data.bak.202608100015/moduforge.db; do
    echo "=== Trying $bak ==="
    if [ -f "$bak" ]; then
        cp "$bak" /vol1/docker/moduforge-data/moduforge.db
        # Start container briefly to test
        cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d
        sleep 5
        health=$(curl -s http://localhost:8086/health)
        if echo "$health" | grep -q '"ok"'; then
            echo "SUCCESS with $bak"
            # Test clear failed
            TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
            result=$(curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN")
            echo "Clear result: $result"
            if echo "$result" | grep -q "error"; then
                echo "Still broken, trying next..."
                cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down
            else
                echo "FIXED!"
                break
            fi
        else
            echo "Failed health check, trying next..."
            cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down
        fi
    fi
done

# Final status
cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=120)
print(stdout.read().decode(errors='replace'))

ssh.close()
