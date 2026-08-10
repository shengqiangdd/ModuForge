#!/usr/bin/env python3
"""Test Go + SQLite directly."""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Create a minimal Go program that tests SQLite ===")
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
    fmt.Printf("Opening database: %s\\n", dbPath)
    
    conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=10000&_foreign_keys=ON&_loc=auto")
    if err != nil {
        fmt.Printf("ERROR opening: %v\\n", err)
        os.Exit(1)
    }
    defer conn.Close()
    
    // Test connection
    err = conn.Ping()
    if err != nil {
        fmt.Printf("ERROR pinging: %v\\n", err)
        os.Exit(1)
    }
    
    // Create table
    _, err = conn.Exec("CREATE TABLE IF NOT EXISTS test (id INTEGER PRIMARY KEY, name TEXT)")
    if err != nil {
        fmt.Printf("ERROR creating table: %v\\n", err)
        os.Exit(1)
    }
    
    fmt.Println("SUCCESS: SQLite works!")
}
'''
    
    # Write test file to container
    cmd = f"""docker run --rm --user root --entrypoint sh -v moduforge_data:/data moduforge:latest -c "mkdir -p /tmp/test && cat > /tmp/test/main.go << 'EOF'
{test_code}
EOF
cd /tmp/test && go mod init test && go mod tidy && go run main.go" """
    
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:2000]}")
    print(f"Error: {error[:1000]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
