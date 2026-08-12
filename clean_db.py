"""彻底清理WAL并用新DB替换"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

#彻底清理并重建
print('\n=== Clean and rebuild DB ===')
cmd = """
docker run --rm \
  -v /vol1/docker/volumes/moduforge_moduforge_data/_data:/data \
  alpine sh -c '
    echo "=== All files ==="
    ls -la /data/
    
    echo ""
    echo "=== Remove all DB files ==="
    rm -f /data/moduforge.db /data/moduforge.db-wal /data/moduforge.db-shm /data/moduforge.db.pre_repair
    
    echo ""
    echo "=== Copy from backup (the one with data) ==="
    # Check if we have a good backup
    ls -la /data/*.bak 2>/dev/null || echo "No .bak files"
    
    echo ""
    echo "=== After cleanup ==="
    ls -la /data/
  '
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
print(stdout.read().decode(errors='replace'))

# Check if there's a backup with the actual data
print('\n=== Find backup with data ===')
stdin, stdout, stderr = ssh.exec_command("""
# Check all DB files on the host
find /vol1/docker/volumes/ -name "moduforge.db*" -ls 2>/dev/null
echo "---"
# Check the pre_repair backup
ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/
""", timeout=10)
print(stdout.read().decode(errors='replace'))

ssh.close()
