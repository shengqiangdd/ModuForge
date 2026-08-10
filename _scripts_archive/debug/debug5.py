import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode(errors='replace')
    err = stderr.read().decode(errors='replace')
    if out: print(out)
    if err: print(err)
    return out

SUDO = 'echo "csq0216" | sudo -S'

# Check the server binary
print('=== SERVER BINARY ===')
run(SUDO + ' docker exec moduforge ls -la /server')
run(SUDO + ' docker exec moduforge file /server')

# Check if it's the latest build
print('\n=== BUILD TIME ===')
run(SUDO + ' docker exec moduforge stat /server | grep Modify')

# Test the actual auth endpoint with verbose logging
print('\n=== DETAILED LOGIN TEST ===')
# The binary reads DATABASE_PATH, which is set correctly
# Let's check if there's a rate limiter or lockout
# Check the auth.go for any lockout mechanism
run('grep -n "lock\|Lock\|attempt\|Attempt\|rate\|Rate\|cooldown\|Cooldown" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/service/auth.go 2>/dev/null | head -10')

# Check if the container binary matches the source
print('\n=== SOURCE VERSION ===')
run('grep -n "version\|Version\|VERSION" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/cmd/moduforge/main.go 2>/dev/null | head -5')

# Check the login handler more carefully
print('\n=== LOGIN HANDLER ===')
run('grep -n "Login\|login\|LoginHandler" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/handler/auth.go 2>/dev/null | head -10')

# Check if there's TOTP or 2FA requirement
print('\n=== TOTP/2FA CHECK ===')
run('grep -n "totp\|TOTP\|2fa\|2FA\|mfa\|MFA" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/service/auth.go 2>/dev/null | head -10')

# The most likely issue: the container binary is OLD and doesn't have the password_changed_at column handling
# Let's check if we need to rebuild
print('\n=== DOCKER IMAGE AGE ===')
run(SUDO + ' docker inspect moduforge --format "{{.Created}}"')

ssh.close()
