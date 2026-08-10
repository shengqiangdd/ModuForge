#!/usr/bin/env python3
"""修复方法签名不匹配"""
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
    
    # 2. 修复 EnergyProfiler::new() 调用
    print("\n=== 2. 修复 EnergyProfiler::new() ===")
    new_content = current_content.replace(
        "let energy_profiler = EnergyProfiler::new();",
        "let energy_profiler = EnergyProfiler::new(1000, 60);"
    )
    
    # 3. 修复 FingerprintDetector -> FingerprintRecognizer
    print("\n=== 3. 修复 FingerprintDetector -> FingerprintRecognizer ===")
    new_content = new_content.replace(
        "use fingerprint::FingerprintDetector;",
        "use fingerprint::FingerprintRecognizer;"
    )
    new_content = new_content.replace(
        "let fingerprint_detector = FingerprintDetector::new();",
        "let fingerprint_detector = FingerprintRecognizer::new();"
    )
    new_content = new_content.replace(
        "let app_type = fingerprint_detector.detect(&feature);",
        "let app_type = fingerprint_detector.recognize(&feature);"
    )
    
    # 4. 修复 DisplayController::new() 调用
    print("\n=== 4. 修复 DisplayController::new() ===")
    new_content = new_content.replace(
        "let display_controller = DisplayController::new();",
        "let display_controller = DisplayController::new(display::RefreshRate::Hz60, true);"
    )
    
    # 5. 修复 calculate_eei -> get_eei
    print("\n=== 5. 修复 calculate_eei -> get_eei ===")
    new_content = new_content.replace(
        "let eei = energy_profiler.calculate_eei(&feature);",
        "let eei = energy_profiler.get_eei(&feature);"
    )
    
    # 6. 修复 optimize_refresh_rate 方法（需要添加到 display.rs）
    print("\n=== 6. 修复 optimize_refresh_rate ===")
    # 先注释掉这个调用，因为 display.rs 没有这个方法
    new_content = new_content.replace(
        "// Apply display control based on scenario\n                    let target_refresh_rate = display_controller.optimize_refresh_rate(\n                        current_scenario,\n                        &feature,\n                        thermal_state\n                    );\n                    \n                    // Get arm (policy level) decision",
        "// Apply display control based on scenario\n                    // TODO: Add optimize_refresh_rate method to DisplayController\n                    // let target_refresh_rate = display_controller.optimize_refresh_rate(\n                    //     current_scenario,\n                    //     &feature,\n                    //     thermal_state\n                    // );\n                    \n                    // Get arm (policy level) decision"
    )
    
    # 7. 写入新文件
    print("\n=== 7. 写入新文件 ===")
    sftp = client.open_sftp()
    with sftp.open(f"{project_path}/src/rust/src/main.rs", "w") as f:
        f.write(new_content)
    sftp.close()
    print("文件已更新")
    
    # 8. 验证
    print("\n=== 8. 验证 ===")
    out, _ = run(f"grep -n 'EnergyProfiler\\|FingerprintRecognizer\\|DisplayController' {project_path}/src/rust/src/main.rs")
    print(f"模块调用:\n{out}")
    
    out, _ = run(f"wc -l {project_path}/src/rust/src/main.rs")
    print(f"总行数: {out.strip()}")
    
    client.close()

if __name__ == "__main__":
    main()
