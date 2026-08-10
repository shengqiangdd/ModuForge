#!/usr/bin/env python3
"""
ModuForge Unified Deployment Script
使用 paramiko 通过 SSH 部署到远程服务器
参考: deploy_optimized.py 的优化策略
"""

import os
import sys
import time
import json
import argparse
import hashlib
from pathlib import Path
from typing import Optional, Dict, Any
from dataclasses import dataclass

# Fix Windows GBK encoding
if sys.platform == "win32":
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

try:
    import paramiko
except ImportError:
    print("Error: paramiko not installed. Run: pip install paramiko")
    sys.exit(1)


@dataclass
class ServerConfig:
    """Server configuration"""
    host: str
    port: int = 22
    username: str = "admin"
    password: Optional[str] = None
    key_filename: Optional[str] = None
    
    # Docker config
    container_name: str = "moduforge"
    image_name: str = "moduforge:latest"
    compose_file: str = "docker-compose.yml"
    
    # Project paths
    remote_repo: str = "/home/admin/moduforge_build"
    remote_build: str = "/home/admin/moduforge_build"


class ModuForgeDeployer:
    """ModuForge deployment manager"""
    
    def __init__(self, config: ServerConfig):
        self.config = config
        self.client: Optional[paramiko.SSHClient] = None
        self.sftp: Optional[paramiko.SFTPClient] = None
        
    def connect(self) -> bool:
        """Establish SSH connection"""
        try:
            self.client = paramiko.SSHClient()
            self.client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
            
            connect_kwargs = {
                "hostname": self.config.host,
                "port": self.config.port,
                "username": self.config.username,
                "timeout": 30,
            }
            
            if self.config.key_filename:
                connect_kwargs["key_filename"] = self.config.key_filename
            elif self.config.password:
                connect_kwargs["password"] = self.config.password
            else:
                # Try default SSH key
                default_key = Path.home() / ".ssh" / "id_rsa"
                if default_key.exists():
                    connect_kwargs["key_filename"] = str(default_key)
                else:
                    print("Error: No authentication method provided")
                    return False
            
            self.client.connect(**connect_kwargs)
            self.sftp = self.client.open_sftp()
            print(f"✓ Connected to {self.config.host}")
            return True
            
        except Exception as e:
            print(f"✗ Connection failed: {e}")
            return False
    
    def disconnect(self):
        """Close SSH connection"""
        if self.sftp:
            self.sftp.close()
        if self.client:
            self.client.close()
        print("✓ Disconnected")
    
    def run_command(self, command: str, check: bool = True) -> tuple:
        """Run command on remote server"""
        print(f"  $ {command}")
        stdin, stdout, stderr = self.client.exec_command(command)
        
        exit_code = stdout.channel.recv_exit_status()
        out = stdout.read().decode()
        err = stderr.read().decode()
        
        if check and exit_code != 0:
            print(f"  ✗ Command failed (exit {exit_code})")
            if err:
                print(f"    stderr: {err[:500]}")
            return exit_code, out, err
        
        return exit_code, out, err
    
    def get_local_checksum(self, file_path: str) -> str:
        """Calculate MD5 checksum of local file"""
        md5 = hashlib.md5()
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(8192), b""):
                md5.update(chunk)
        return md5.hexdigest()
    
    def get_remote_checksum(self, remote_path: str) -> Optional[str]:
        """Get MD5 checksum of remote file"""
        try:
            exit_code, out, _ = self.run_command(f"md5sum {remote_path} | cut -d' ' -f1", check=False)
            if exit_code == 0 and out.strip():
                return out.strip()
        except:
            pass
        return None
    
    def upload_file(self, local_path: str, remote_path: str, force: bool = False) -> bool:
        """Upload file if changed"""
        try:
            local_md5 = self.get_local_checksum(local_path)
            remote_md5 = self.get_remote_checksum(remote_path)
            
            if not force and local_md5 == remote_md5:
                print(f"  ⊘ {Path(local_path).name} (unchanged)")
                return True
            
            self.sftp.put(local_path, remote_path)
            print(f"  ✓ {Path(local_path).name} uploaded")
            return True
            
        except Exception as e:
            print(f"  ✗ Upload failed: {e}")
            return False
    
    def check_docker(self) -> bool:
        """Check Docker is running"""
        exit_code, out, _ = self.run_command("docker info --format '{{.ServerVersion}}'", check=False)
        if exit_code == 0:
            print(f"✓ Docker {out.strip()} running")
            return True
        print("✗ Docker not running")
        return False
    
    def check_container(self) -> Dict[str, str]:
        """Check container status"""
        exit_code, out, _ = self.run_command(
            f"docker inspect {self.config.container_name} --format '{{{{.State.Status}}}}'",
            check=False
        )
        
        if exit_code == 0:
            status = out.strip()
            print(f"✓ Container {self.config.container_name}: {status}")
            return {"status": status, "exists": "true"}
        
        print(f"⊘ Container {self.config.container_name} not found")
        return {"status": "not_found", "exists": "false"}
    
    def git_pull(self) -> bool:
        """Pull latest code"""
        print("\n📥 Pulling latest code...")
        
        # Set git config to avoid permission issues
        self.run_command("GIT_CONFIG_GLOBAL=/tmp/gitconfig_safe git config --global safe.directory '*'", check=False)
        
        exit_code, out, err = self.run_command(
            f"cd {self.config.remote_repo} && GIT_CONFIG_GLOBAL=/tmp/gitconfig_safe git pull origin main",
            check=False
        )
        
        if exit_code == 0:
            if "Already up to date" in out:
                print("  ⊘ Already up to date")
            else:
                print(f"  ✓ Pulled changes")
            return True
        
        print(f"  ✗ Git pull failed: {err[:200]}")
        return False
    
    def build_docker(self, no_cache: bool = False) -> bool:
        """Build Docker image"""
        print("\n🔨 Building Docker image...")
        
        cache_flag = "--no-cache" if no_cache else ""
        cmd = f"cd {self.config.remote_repo} && docker build {cache_flag} -t moduforge-app . 2>&1"
        
        exit_code, out, err = self.run_command(cmd, check=False)
        
        if exit_code == 0:
            # Extract build time
            for line in out.split('\n'):
                if 'successfully built' in line.lower() or 'done' in line.lower():
                    print(f"  ✓ {line.strip()}")
                    break
            return True
        
        print(f"  ✗ Build failed")
        # Show last 10 lines of output
        lines = out.split('\n')[-10:]
        for line in lines:
            if line.strip():
                print(f"    {line}")
        return False
    
    def restart_container(self) -> bool:
        """Restart Docker container"""
        print("\n🔄 Restarting container...")
        
        # Stop existing container
        self.run_command(f"docker stop {self.config.container_name}", check=False)
        self.run_command(f"docker rm {self.config.container_name}", check=False)
        
        # Start new container
        exit_code, out, err = self.run_command(
            f"cd {self.config.remote_repo} && docker compose up -d",
            check=False
        )
        
        if exit_code == 0:
            print("  ✓ Container started")
            
            # Wait for health check
            print("  ⏳ Waiting for health check...")
            time.sleep(5)
            
            # Verify health
            exit_code, out, _ = self.run_command(
                f"docker inspect {self.config.container_name} --format '{{{{.State.Health.Status}}}}'",
                check=False
            )
            
            if exit_code == 0 and "healthy" in out:
                print("  ✓ Container healthy")
                return True
            else:
                print(f"  ⚠ Container status: {out.strip()}")
                return True  # Container is running, just not healthy yet
        
        print(f"  ✗ Start failed: {err[:200]}")
        return False
    
    def check_api(self) -> bool:
        """Check API is responding"""
        print("\n🌐 Checking API...")
        
        # Try multiple health endpoints
        endpoints = [
            "http://localhost:8086/health",
            "http://localhost:8086/api/v1/health",
        ]
        
        for endpoint in endpoints:
            exit_code, out, _ = self.run_command(f"curl -s {endpoint}", check=False)
            
            if exit_code == 0 and out.strip():
                # Check for successful response
                if "ok" in out.lower() or "status" in out.lower() or "version" in out.lower():
                    try:
                        data = json.loads(out)
                        version = data.get('version', data.get('status', 'unknown'))
                        print(f"  ✓ API healthy: {version} ({endpoint})")
                        return True
                    except:
                        print(f"  ✓ API responding: {out[:100]}")
                        return True
        
        print("  ✗ API not responding")
        return False
    
    def deploy_full(self, skip_git: bool = False, no_cache: bool = False) -> bool:
        """Full deployment pipeline"""
        print("=" * 60)
        print("🚀 ModuForge Deployment")
        print("=" * 60)
        
        # Connect
        if not self.connect():
            return False
        
        try:
            # Check prerequisites
            print("\n📋 Checking prerequisites...")
            if not self.check_docker():
                return False
            
            container = self.check_container()
            
            # Git pull
            if not skip_git:
                if not self.git_pull():
                    print("⚠ Git pull failed, continuing with existing code...")
            
            # Build
            if not self.build_docker(no_cache):
                return False
            
            # Restart
            if not self.restart_container():
                return False
            
            # Verify
            if not self.check_api():
                return False
            
            print("\n" + "=" * 60)
            print("✅ Deployment successful!")
            print(f"   URL: http://{self.config.host}:8086")
            print("=" * 60)
            return True
            
        finally:
            self.disconnect()
    
    def deploy_binary(self, local_binary: str) -> bool:
        """Fast deployment by replacing binary only"""
        print("=" * 60)
        print("🚀 Fast Binary Deployment")
        print("=" * 60)
        
        if not self.connect():
            return False
        
        try:
            # Check container exists
            container = self.check_container()
            if container["exists"] != "true":
                print("✗ Container not found, run full deployment first")
                return False
            
            # Upload binary
            remote_binary = f"{self.config.remote_repo}/moduforge-server"
            print(f"\n📤 Uploading {Path(local_binary).name}...")
            
            if not self.upload_file(local_binary, remote_binary, force=True):
                return False
            
            # Make executable
            self.run_command(f"chmod +x {remote_binary}")
            
            # Copy to container
            print("\n📦 Copying to container...")
            self.run_command(
                f"docker cp {remote_binary} {self.config.container_name}:/app/moduforge-server"
            )
            
            # Restart container
            print("\n🔄 Restarting container...")
            self.run_command(f"docker restart {self.config.container_name}")
            
            # Wait and verify
            time.sleep(3)
            if self.check_api():
                print("\n" + "=" * 60)
                print("✅ Binary deployment successful!")
                print(f"   Time: ~30s")
                print("=" * 60)
                return True
            
            return False
            
        finally:
            self.disconnect()
    
    def upload_files(self, local_files: Dict[str, str]) -> bool:
        """Upload multiple files"""
        print("=" * 60)
        print("📤 Uploading Files")
        print("=" * 60)
        
        if not self.connect():
            return False
        
        try:
            for local_path, remote_path in local_files.items():
                if not Path(local_path).exists():
                    print(f"  ✗ {local_path} not found")
                    continue
                
                full_remote = f"{self.config.remote_repo}/{remote_path}"
                self.upload_file(local_path, full_remote)
            
            print("\n✓ Upload complete")
            return True
            
        finally:
            self.disconnect()


def main():
    parser = argparse.ArgumentParser(description="ModuForge Deployment Script")
    parser.add_argument("--host", default="192.168.2.9", help="Server host")
    parser.add_argument("--port", type=int, default=22, help="SSH port")
    parser.add_argument("--user", default="admin", help="SSH username")
    parser.add_argument("--password", help="SSH password")
    parser.add_argument("--key", help="SSH private key file")
    
    subparsers = parser.add_subparsers(dest="command", help="Deployment command")
    
    # Full deployment
    full_parser = subparsers.add_parser("full", help="Full deployment (git + build + restart)")
    full_parser.add_argument("--skip-git", action="store_true", help="Skip git pull")
    full_parser.add_argument("--no-cache", action="store_true", help="Build without cache")
    
    # Binary deployment
    binary_parser = subparsers.add_parser("binary", help="Fast binary-only deployment")
    binary_parser.add_argument("binary", help="Path to local binary")
    
    # Check status
    subparsers.add_parser("status", help="Check deployment status")
    
    # Restart
    subparsers.add_parser("restart", help="Restart container only")
    
    args = parser.parse_args()
    
    # Create config
    config = ServerConfig(
        host=args.host,
        port=args.port,
        username=args.user,
        password=args.password,
        key_filename=args.key,
    )
    
    # Create deployer
    deployer = ModuForgeDeployer(config)
    
    # Execute command
    if args.command == "full":
        success = deployer.deploy_full(skip_git=args.skip_git, no_cache=args.no_cache)
    elif args.command == "binary":
        success = deployer.deploy_binary(args.binary)
    elif args.command == "status":
        if deployer.connect():
            deployer.check_docker()
            deployer.check_container()
            deployer.check_api()
            deployer.disconnect()
        success = True
    elif args.command == "restart":
        if deployer.connect():
            deployer.restart_container()
            deployer.check_api()
            deployer.disconnect()
        success = True
    else:
        parser.print_help()
        success = False
    
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
