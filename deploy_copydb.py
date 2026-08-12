import paramiko, os

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

sftp = client.open_sftp()

# Copy backup DB to local
local_bak = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge.db.bak'
local_db = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge.db'

print("Copying backup DB...")
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db.bak', local_bak)
print(f"Backup DB: {os.path.getsize(local_bak)} bytes")

print("Copying current DB...")
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db', local_db)
print(f"Current DB: {os.path.getsize(local_db)} bytes")

sftp.close()
client.close()
print("Done!")
