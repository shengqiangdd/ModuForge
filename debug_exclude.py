#!/usr/bin/env python3
"""Check database paths and debug exclusion."""

import paramiko
import json

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd, desc=""):
    print(f"=== {desc} ===")
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()
    return out

# Get the project files from the database
project_id = "73ab21c8-758e-430a-9124-82ce8636d8ca"
run(f"""docker exec {CONTAINER} sh -c "sqlite3 /data/moduforge.db 'SELECT path FROM project_files WHERE project_id=\\'{project_id}\\' ORDER BY path;'" """, "Database paths")

# Check if the isExcluded function is working
print("\n=== Debugging isExcluded ===")
run(f"""docker exec {CONTAINER} sh -c "
# Create a simple test program
cat > /tmp/test_exclude.go << 'EOF'
package main

import (
\t\"fmt\"
\t\"strings\"
)

var excludedPatterns = []string{
\t\"src/\",
\t\"*.go\",
\t\"*.rs\",
\t\"*.c\",
\t\"*.h\",
\t\"*.cpp\",
\t\"*.py\",
\t\"*.java\",
\t\"*.kt\",
\t\"build.sh\",
\t\"compile.sh\",
\t\"go.mod\",
\t\"go.sum\",
\t\"Cargo.toml\",
\t\"Cargo.lock\",
\t\"target/\",
\t\"Makefile\",
\t\".build_cache/\",
\t\".build_cache.json\",
\t\".build_status/\",
\t\".build_status.json\",
\t\"build-cache/\",
\t\".cargo-artifact-lock\",
\t\".cargo-build-lock\",
\t\".cargo-lock\",
\t\"*.d\",
\t\"*.o\",
\t\"*.a\",
\t\"node_modules/\",
\t\".git/\",
\t\".DS_Store\",
\t\"__pycache__/\",
\t\"*.tmp\",
\t\"*.xml\",
\t\"*.gradle\",
\t\"*.md\",
\t\"docs/\",
\t\"doc/\",
\t\".idea/\",
\t\".vscode/\",
\t\".vs/\",
\t\"tmp/\",
\t\"upload\",
\t\"DESIGN_DOC.md\",
\t\"rust_files_list.txt\",
\t\"app/backend/\",
}

func isExcluded(path string) bool {
\tfor _, pat := range excludedPatterns {
\t\tif strings.HasSuffix(pat, \"/\") {
\t\t\tif strings.Contains(path, pat) {
\t\t\t\treturn true
\t\t\t}
\t\t} else if strings.HasPrefix(pat, \"*\") {
\t\t\tsuffix := pat[1:]
\t\t\tif strings.HasSuffix(path, suffix) {
\t\t\t\treturn true
\t\t\t}
\t\t} else {
\t\t\tif path == pat || strings.HasSuffix(path, \"/\"+pat) {
\t\t\t\treturn true
\t\t\t}
\t\t}
\t}
\treturn false
}

func main() {
\ttestPaths := []string{
\t\t\"tmp/test.sh\",
\t\t\"DESIGN_DOC.md\",
\t\t\"src/main.go\",
\t\t\"index.html\",
\t\t\"css/styles.css\",
\t\t\"module.prop\",
\t}
\tfor _, p := range testPaths {
\t\tfmt.Printf(\"%s: excluded=%v\\n\", p, isExcluded(p))
\t}
}
EOF
cd /tmp && go run test_exclude.go
" """, "Test isExcluded")

ssh.close()
print("\nDone!")
