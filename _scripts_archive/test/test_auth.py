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

# Use Go to test bcrypt in the container
print('=== TEST BCRYPT IN CONTAINER ===')
# Create a Go test program
go_test = r'''
package main

import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)

func main() {
    hash := "$2b$12$.5hsQQOpTNgBz8YHSeE2L.Wu.3uFksCddm3GujA2xuvHucCt232SW"
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin123"))
    if err != nil {
        fmt.Printf("FAIL: %v\n", err)
    } else {
        fmt.Println("OK: password matches")
    }
}
'''

# Write and run in container
run(f'cat > /tmp/test.go << \'EOF\'\n{go_test}\nEOF')
# Actually, let's just use the container's Go runtime if available
# Or use python with bcrypt

# Simpler: use sqlite3 to check the hash directly
print('\n=== CHECK HASH IN CONTAINER ===')
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT password_hash FROM users WHERE username=\'admin\';"')

# Compare with what we set
print('\n=== HASH COMPARISON ===')
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT length(password_hash), password_hash FROM users WHERE username=\'admin\';"')

# Check if the DB is actually the volume mount
print('\n=== DB FILE INFO ===')
run(SUDO + ' docker exec moduforge ls -la /data/moduforge.db')
run(SUDO + ' docker exec moduforge md5sum /data/moduforge.db')

# Check host DB md5
print('\n=== HOST DB MD5 ===')
run(SUDO + ' md5sum /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')

# The real test: try to register a new user via API
print('\n=== TRY REGISTER ===')
import urllib.request, json
data = json.dumps({"username": "newtest", "password": "test1234", "email": "new@test.com"}).encode()
req = urllib.request.Request('http://192.168.2.9:8086/api/v1/auth/register', data=data, headers={'Content-Type': 'application/json'})
try:
    resp = urllib.request.urlopen(req)
    result = json.loads(resp.read().decode())
    print("Register result:", result)
except Exception as e:
    print("Register error:", e)

# Now try to login with the new user
print('\n=== LOGIN NEW USER ===')
data = json.dumps({"username": "newtest", "password": "test1234"}).encode()
req = urllib.request.Request('http://192.168.2.9:8086/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
try:
    resp = urllib.request.urlopen(req)
    result = json.loads(resp.read().decode())
    print("Login result:", result)
except Exception as e:
    print("Login error:", e)

ssh.close()
