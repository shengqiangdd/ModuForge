#!/usr/bin/env python3
"""Minimal test - just check if SQLite can create a database."""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Check if we can create files in /data ===")
    cmd = """docker run --rm --user root --entrypoint sh -v moduforge_data:/data moduforge:latest -c "touch /data/test_write.txt && ls -la /data/test_write.txt && echo 'Write works!' || echo 'Write failed'" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    print(stderr.read().decode())
    
    print("\n=== Test: Check SQLite version ===")
    cmd = """docker run --rm --entrypoint sh moduforge:latest -c "which sqlite3 && sqlite3 --version || echo 'sqlite3 not found'" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    print("\n=== Test: Try simple Go program ===")
    cmd = """docker run --rm --user root --entrypoint sh -v moduforge_data:/data moduforge:latest -c "echo 'package main; import (\"database/sql\"); import _ \"github.com/mattn/go-sqlite3\"; func main() { db, _ := sql.Open(\"sqlite3\", \"/data/test.db\"); db.Exec(\"CREATE TABLE t(id INT)\"); db.Close() }' > /tmp/test.go && cd /tmp && go run test.go && ls -la /data/test.db" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode()[:1000])
    
    ssh.close()

if __name__ == '__main__':
    test()
