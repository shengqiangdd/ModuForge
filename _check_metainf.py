import paramiko
import zipfile
import io

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Get the latest build artifact
_, o, _ = c.exec_command('''ls -t /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/data/storage/artifacts/*/module.zip 2>/dev/null | head -1''')
zip_path = o.read().decode().strip()
print(f'Checking: {zip_path}')

# Download and check META-INF
_, o2, _ = c.exec_command(f'cat {zip_path}')
zip_data = o2.read()

if zip_data:
    print(f'Zip size: {len(zip_data)} bytes')
    try:
        with zipfile.ZipFile(io.BytesIO(zip_data)) as z:
            files = z.namelist()
            print(f'Total files in zip: {len(files)}')
            
            # Check META-INF
            metainf_files = [f for f in files if f.startswith('META-INF/')]
            print(f'META-INF files: {metainf_files}')
            
            # Check webroot
            webroot_files = [f for f in files if f.startswith('webroot/')]
            print(f'webroot files: {webroot_files}')
            
            # List all files
            print('\nAll files:')
            for f in sorted(files):
                print(f'  {f}')
    except Exception as e:
        print(f'Error reading zip: {e}')
else:
    print('No zip data received')

c.close()
