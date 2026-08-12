import paramiko
import zipfile
import io

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Test EnsureMetaInf by creating a test zip without META-INF
print('=== Test: EnsureMetaInf with Go code ===')

# Create a test module structure
test_code = '''
package main

import (
    "fmt"
    "os"
)

func main() {
    fmt.Println("Test module")
    // Security: no hardcoded keys
    fmt.Println("Version:", os.Getenv("VERSION"))
}
'''

# Upload test file and build
_, o, _ = c.exec_command('''cat > /tmp/test_module.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Test")
}
EOF
echo "Test file created"
''')
print(o.read().decode())

# Check if we can use the Go build system in the container
_, o2, _ = c.exec_command('''docker exec moduforge ls /app/backend/internal/builder/ 2>&1''')
files = o2.read().decode().strip().split('\n')
print(f'Builder files: {files}')

# Check if metainf.go exists
if 'metainf.go' in files:
    print('✅ metainf.go exists in container')
else:
    print('❌ metainf.go NOT found - code not deployed?')

# Check the binary
_, o3, _ = c.exec_command('''docker exec moduforge ls -la /server 2>&1''')
print(f'Server binary: {o3.read().decode().strip()}')

c.close()
