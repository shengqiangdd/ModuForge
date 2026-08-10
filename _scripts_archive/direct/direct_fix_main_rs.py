#!/usr/bin/env python3
"""直接修复 main.rs 模块集成"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    project_path = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"
    
    # 1. 读取当前 main.rs
    print("=== 1. 读取当前 main.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/main.rs")
    current_content = out
    print(f"当前长度: {len(current_content)} 字符")
    
    # 2. 检查是否已有 energy 模块
    if "mod energy;" in current_content:
        print("已有 energy 模块，跳过")
        return
    
    # 3. 构建新的 main.rs
    print("\n=== 2. 构建新的 main.rs ===")
    
    # 新增模块声明
    new_mods = """mod linucb;
mod ipc;
mod config;
mod scenario;
mod thermal_control;
mod io_control;
mod doze;
mod energy;
mod fingerprint;
mod display;
mod error;
mod constants;"""
    
    # 新增 use 语句
    new_uses = """use ipc::SharedMemoryIPC;
use linucb::LinUCB;
use config::AppConfig;
use scenario::ScenarioDetector;
use thermal_control::ThermalController;
use io_control::IOController;
use doze::DozeController;
use energy::EnergyProfiler;
use fingerprint::FingerprintDetector;
use display::DisplayController;
use error::SmartTuneError;
use constants::thermal;"""
    
    # 替换模块声明
    new_content = current_content.replace(
        "mod linucb;\nmod ipc;\nmod config;\nmod scenario;\nmod thermal_control;\nmod io_control;\nmod doze;",
        new_mods
    )
    
    # 替换 use 语句
    new_content = new_content.replace(
        "use ipc::SharedMemoryIPC;\nuse linucb::LinUCB;\nuse config::AppConfig;\nuse scenario::ScenarioDetector;\nuse thermal_control::ThermalController;\nuse io_control::IOController;\nuse doze::DozeController;",
        new_uses
    )
    
    # 修改 main 函数签名
    new_content = new_content.replace(
        "fn main() {",
        "fn main() -> Result<(), SmartTuneError> {"
    )
    
    # 替换 expect 调用为 ? 操作符
    new_content = new_content.replace(
        '.expect("Failed to read config file")',
        '?'
    )
    new_content = new_content.replace(
        '.expect("Invalid JSON config")',
        '?'
    )
    new_content = new_content.replace(
        '.expect("Failed to initialize shared memory IPC")',
        '?'
    )
    
    # 添加新模块初始化
    init_code = """
    // Initialize new modules
    let energy_profiler = EnergyProfiler::new();
    let fingerprint_detector = FingerprintDetector::new();
    let display_controller = DisplayController::new();
    eprintln!("[AndroSt] New subsystems initialized: energy, fingerprint, display");"""
    
    new_content = new_content.replace(
        '    eprintln!("[AndroSt] Subsystems initialized: scenario, thermal, io, doze");',
        f'    eprintln!("[AndroSt] Subsystems initialized: scenario, thermal, io, doze");{init_code}'
    )
    
    # 添加能耗记录到主循环
    energy_log = """
                // Record energy data
                energy_profiler.record_sample(&feature);"""
    
    new_content = new_content.replace(
        '                // Get arm (policy level) decision',
        f'{energy_log}\n\n                // Get arm (policy level) decision'
    )
    
    # 添加应用类型识别
    app_type_log = """
                // Detect application type
                let app_type = fingerprint_detector.detect(&feature);
                eprintln!("[AndroSt] App type: {:?}", app_type);"""
    
    new_content = new_content.replace(
        '                // Get arm (policy level) decision',
        f'{app_type_log}\n{energy_log}\n\n                // Get arm (policy level) decision'
    )
    
    # 4. 写入新文件
    print("\n=== 3. 写入新文件 ===")
    sftp = client.open_sftp()
    with sftp.open(f"{project_path}/src/rust/src/main.rs", "w") as f:
        f.write(new_content)
    sftp.close()
    print("文件已更新")
    
    # 5. 验证
    print("\n=== 4. 验证 ===")
    out, _ = run(f"grep -n 'mod energy\\|mod fingerprint\\|mod display\\|mod error\\|mod constants' {project_path}/src/rust/src/main.rs")
    print(f"模块检查: {out}")
    
    out, _ = run(f"head -30 {project_path}/src/rust/src/main.rs")
    print(f"文件内容:\n{out}")
    
    client.close()

if __name__ == "__main__":
    main()
