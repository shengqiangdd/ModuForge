#!/usr/bin/env python3
"""Upload and run deployment script on server."""
import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8')
sys.stderr.reconfigure(encoding='utf-8')

SERVER = '192.168.2.9'
USER = 'admin'
PASS = 'csq0216'

def run(ssh, cmd, timeout=60):
    print(f"  > {cmd[:100]}...")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode()
    err = stderr.read().decode()
    if out.strip():
        print(f"  OUT: {out.strip()[:300]}")
    if err.strip():
        print(f"  ERR: {err.strip()[:300]}")
    return out.strip(), err.strip()

def main():
    print("Connecting to server...")
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(SERVER, username=USER, password=PASS, timeout=15)
    print("[OK] Connected")

    # Upload the deployment script
    print("\nUploading deployment script...")
    sftp = ssh.open_sftp()
    sftp.put('deploy_remote.sh', '/tmp/deploy_remote.sh')
    sftp.close()
    print("[OK] Script uploaded")

    # Make it executable and run it
    print("\nRunning deployment (this may take several minutes)...")
    run(ssh, 'chmod +x /tmp/deploy_remote.sh')
    
    # Run with nohup and capture output
    run(ssh, 'nohup /tmp/deploy_remote.sh > /tmp/deploy_output.log 2>&1 &', timeout=10)
    
    # Wait for completion
    print("\nWaiting for deployment to complete...")
    import time
    for i in range(60):  # Wait up to 10 minutes
        time.sleep(10)
        out, _ = run(ssh, 'tail -5 /tmp/deploy_output.log 2>/dev/null')
        if 'Deployment Complete' in out or 'Error' in out:
            print("\nFinal output:")
            out, _ = run(ssh, 'cat /tmp/deploy_output.log')
            break
        print(f"  Still running... ({(i+1)*10}s)")

    # Check container status
    print("\nChecking container status...")
    out, _ = run(ssh, 'docker ps --filter name=moduforge --format "{{.ID}} {{.Names}} {{.Status}}"')
    
    ssh.close()
    print("\n[DONE] Deployment finished")

if __name__ == '__main__':
    main()
