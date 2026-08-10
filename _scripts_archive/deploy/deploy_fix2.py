# -*- coding: utf-8 -*-
"""
Deploy fixed binary + fix entrypoint permissions.
Uses a temp container to fix permissions since the main container can't start.
"""
import paramiko
import subprocess
import sys
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
IMAGE = "moduforge-app"  # The original image name

def run_cmd(ssh, cmd, timeout=30):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    exit_code = stdout.channel.recv_exit_status()
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return exit_code, out, err

def main():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    print("Connected to %s" % HOST)

    # Step 1: Check current state
    print("\n=== Current state ===")
    _, out, _ = run_cmd(ssh, "docker inspect %s --format '{{.Config.Image}}' 2>/dev/null || echo 'no-image'" % CONTAINER)
    print("Image: %s" % out)
    
    _, out, _ = run_cmd(ssh, "docker inspect %s --format '{{.State.Status}}' 2>/dev/null || echo 'no-container'" % CONTAINER)
    print("Status: %s" % out)
    
    # Step 2: Get the image from the container (or find it)
    _, out, _ = run_cmd(ssh, "docker inspect %s --format '{{.Config.Image}}'" % CONTAINER)
    image_name = out.strip().strip("'\"")
    if not image_name or image_name == "<no value>":
        # Try to find the image
        _, out, _ = run_cmd(ssh, "docker images --format '{{.Repository}}:{{.Tag}}' | grep -i moduforge | head -1")
        image_name = out.strip()
    
    if not image_name:
        print("ERROR: Cannot find image name")
        ssh.close()
        return 1
    
    print("Using image: %s" % image_name)
    
    # Step 3: Remove the broken container
    print("\n=== Removing broken container ===")
    run_cmd(ssh, "docker rm -f %s 2>/dev/null" % CONTAINER)
    
    # Step 4: Find the data volume
    print("\n=== Finding data volume ===")
    _, out, _ = run_cmd(ssh, "docker volume ls | grep -i moduforge | head -3")
    print("Volumes: %s" % out)
    
    # Step 5: Create a temp container from the image, fix the binary and entrypoint
    print("\n=== Creating temp container to fix permissions ===")
    
    # Create container with /bin/sh entrypoint (bypass broken entrypoint)
    code, out, err = run_cmd(ssh, 
        'docker create --name moduforge-fix --entrypoint /bin/sh %s -c "echo ready"' % image_name)
    print("Create temp: %s" % (out or err))
    
    if code != 0:
        print("ERROR: Could not create temp container")
        ssh.close()
        return 1
    
    # Fix entrypoint permissions inside the temp container
    # First, copy the entrypoint out, chmod it, copy it back
    code, out, err = run_cmd(ssh,
        'docker cp moduforge-fix:/docker-entrypoint.sh /tmp/docker-entrypoint.sh')
    print("Copy entrypoint: %s" % (out or err))
    
    code, out, err = run_cmd(ssh, 'chmod +x /tmp/docker-entrypoint.sh')
    print("Chmod entrypoint: %s" % (out or err))
    
    code, out, err = run_cmd(ssh,
        'docker cp /tmp/docker-entrypoint.sh moduforge-fix:/docker-entrypoint.sh')
    print("Copy back entrypoint: %s" % (out or err))
    
    # Now copy our fixed binary
    code, out, err = run_cmd(ssh, 'docker cp /tmp/moduforge-server-new moduforge-fix:/server')
    print("Copy binary: %s" % (out or err))
    
    code, out, err = run_cmd(ssh, 'docker exec moduforge-fix chmod +x /server')
    print("Chmod binary: %s" % (out or err))
    
    # Commit the fixed container as a new image
    code, out, err = run_cmd(ssh,
        'docker commit moduforge-fix %s-fixed' % image_name.replace(':', '-'))
    print("Commit: %s" % (out or err))
    
    new_image = "%s-fixed" % image_name.replace(':', '-')
    
    # Step 6: Find and mount the data volume
    print("\n=== Starting new container ===")
    
    # Get volume info from the original container config
    # Try common volume mount patterns
    code, out, _ = run_cmd(ssh, 
        "docker inspect moduforge --format '{{range .Mounts}}{{.Source}}:{{.Destination}} {{end}}' 2>/dev/null")
    mounts = out.strip()
    print("Original mounts: %s" % mounts)
    
    if mounts:
        # Use the same mounts
        mount_args = ""
        for mount in mounts.split():
            src, dst = mount.split(':', 1)
            mount_args += " -v %s:%s" % (src, dst)
    else:
        # Fallback: use /home/admin/moduforge as data dir
        mount_args = " -v /home/admin/moduforge/data:/data"
    
    # Start the new container
    code, out, err = run_cmd(ssh,
        'docker run -d --name %s --restart unless-stopped -p 8086:8080 %s %s' % (
            CONTAINER, mount_args, new_image))
    print("Run: %s" % (out or err))
    
    # Step 7: Wait and verify
    print("\n=== Waiting for startup ===")
    time.sleep(5)
    
    code, out, _ = run_cmd(ssh, "docker logs %s --tail 10 2>&1" % CONTAINER)
    print("Logs:\n%s" % out)
    
    # Verify the binary has our fix
    code, out, _ = run_cmd(ssh, 
        "docker exec %s strings /server | grep -c 'WHERE name='" % CONTAINER)
    print("\nBinary 'WHERE name=' count: %s" % out)
    
    # Health check
    code, out, _ = run_cmd(ssh, 
        "docker exec %s curl -s http://localhost:8080/health" % CONTAINER)
    print("Health: %s" % out)
    
    # Cleanup temp container
    run_cmd(ssh, "docker rm -f moduforge-fix 2>/dev/null")
    
    ssh.close()
    print("\nDone")
    return 0

if __name__ == "__main__":
    sys.exit(main())
