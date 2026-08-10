#!/usr/bin/env python3
"""Simple SQLite test."""
import paramiko
import time
import os

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Create test Go program ===")
    
    # Simple Go program
    test_code = '''package main

import (
    "database/sql"
    "fmt"
    "os"
    
    _ "github.com/mattn/go-sqlite3"
)

func main() {
    dbPath := os.Getenv("DB_PATH")
    if dbPath == "" {
        dbPath = "/data/moduforge.db"
    }
    fmt.Println("Opening:", dbPath)
    
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        fmt.Println("ERROR open:", err)
        os.Exit(1)
    }
    defer db.Close()
    
    err = db.Ping()
    if err != nil {
        fmt.Println("ERROR ping:", err)
        os.Exit(1)
    }
    
    _, err = db.Exec("CREATE TABLE IF NOT EXISTS test (id INTEGER PRIMARY KEY)")
    if err != nil {
        fmt.Println("ERROR create:", err)
        os.Exit(1)
    }
    
    fmt.Println("SUCCESS!")
}
'''
    
    # Upload via sftp
    sftp = ssh.open_sftp()
    with sftp.open("/tmp/test_sqlite.go", "w") as f:
        f.write(test_code)
    sftp.close()
    
    # Copy to container and run
    print("2. Running test...")
    cmd = """docker run --rm --user root --entrypoint sh -v moduforge_data:/data -v /tmp/test_sqlite.go:/tmp/test_sqlite.go moduforge:latest -c "cd /tmp && go mod init test_sqlite 2>/dev/null; go mod tidy 2>/dev/null; go run test_sqlite.go" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:2000]}")
    if error:
        print(f"Error: {error[:1000]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
