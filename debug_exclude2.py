#!/usr/bin/env python3
"""Debug exclusion logic."""

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Create test Go file on server
test_go = '''package main

import (
\t"fmt"
\t"strings"
)

var excludedPatterns = []string{
\t"src/",
\t"*.go",
\t"*.rs",
\t"*.c",
\t"*.h",
\t"*.cpp",
\t"*.py",
\t"*.java",
\t"*.kt",
\t"build.sh",
\t"compile.sh",
\t"go.mod",
\t"go.sum",
\t"Cargo.toml",
\t"Cargo.lock",
\t"target/",
\t"Makefile",
\t".build_cache/",
\t".build_cache.json",
\t".build_status/",
\t".build_status.json",
\t"build-cache/",
\t".cargo-artifact-lock",
\t".cargo-build-lock",
\t".cargo-lock",
\t"*.d",
\t"*.o",
\t"*.a",
\t"node_modules/",
\t".git/",
\t".DS_Store",
\t"__pycache__/",
\t"*.tmp",
\t"*.xml",
\t"*.gradle",
\t"*.md",
\t"docs/",
\t"doc/",
\t".idea/",
\t".vscode/",
\t".vs/",
\t"tmp/",
\t"upload",
\t"DESIGN_DOC.md",
\t"rust_files_list.txt",
\t"app/backend/",
}

func isExcluded(path string) bool {
\tfor _, pat := range excludedPatterns {
\t\tif strings.HasSuffix(pat, "/") {
\t\t\tif strings.Contains(path, pat) {
\t\t\t\treturn true
\t\t\t}
\t\t} else if strings.HasPrefix(pat, "*") {
\t\t\tsuffix := pat[1:]
\t\t\tif strings.HasSuffix(path, suffix) {
\t\t\t\treturn true
\t\t\t}
\t\t} else {
\t\t\tif path == pat || strings.HasSuffix(path, "/"+pat) {
\t\t\t\treturn true
\t\t\t}
\t\t}
\t}
\treturn false
}

func main() {
\ttestPaths := []string{
\t\t"tmp/test.sh",
\t\t"DESIGN_DOC.md",
\t\t"src/main.go",
\t\t"index.html",
\t\t"css/styles.css",
\t\t"module.prop",
\t\t"app/backend/main.go",
\t\t"build.sh",
\t\t"go.mod",
\t\t"Cargo.toml",
\t\t".build_cache/file",
\t\t"node_modules/package",
\t\t".git/config",
\t\t"*.md",
\t}
\tfmt.Println("Testing isExcluded function:")
\tfor _, p := range testPaths {
\t\tfmt.Printf("  %s: excluded=%v\\n", p, isExcluded(p))
\t}
}
'''

# Write the test file
sftp = ssh.open_sftp()
with sftp.open("/tmp/test_exclude.go", "w") as f:
    f.write(test_go)
sftp.close()

# Copy to container and run
ssh.exec_command(f"docker cp /tmp/test_exclude.go {CONTAINER}:/tmp/test_exclude.go")
stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} sh -c 'cd /tmp && go run test_exclude.go'")
print("=== Test Results ===")
print(stdout.read().decode())
print(stderr.read().decode())

ssh.close()
print("\nDone!")
