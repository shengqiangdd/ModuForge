import subprocess
import sys

# Run in background with nohup, then poll
cmd = [
    'ssh', '-o', 'ConnectTimeout=10', 'root@192.168.2.9',
    'cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && docker compose down 2>&1 && docker compose up -d --build > /tmp/moduforge_deploy.log 2>&1 && echo DEPLOY_DONE'
]

# Use Popen with longer timeout
proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
try:
    stdout, stderr = proc.communicate(timeout=600)
    print('BUILD:', proc.returncode)
    if stdout:
        print(stdout[-500:])
    if stderr:
        print(stderr[-500:])
except subprocess.TimeoutExpired:
    proc.kill()
    print("Timed out after 600s, build likely still running on server")
    # Check if it finished
    r2 = subprocess.run(['ssh', '-o', 'ConnectTimeout=10', 'root@192.168.2.9', 'tail -5 /tmp/moduforge_deploy.log 2>&1'], capture_output=True, text=True, timeout=15)
    print('LOG:', r2.stdout)
