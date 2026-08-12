import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=30):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    err = e.read().decode(errors='replace').strip()
    if out: print(out[:2000])
    if err: print(f"STDERR: {err[:500]}")
    return out

PROJ = "/data/storage/projects/1785249992652501794-1864"

# === 1. Fix module.prop version ===
print("=== 1. Fix module.prop ===")
run(f"""sudo docker exec moduforge sh -c 'cat > {PROJ}/module.prop << "EOF"
id=AndroBoost-SmartTune
name=AndroBoost SmartTune
version=v2.0
versionCode=20260812
author=AndroBoost Dev
description=次世代全栈自适应调优引擎 | LinUCB强化学习 | 165Hz+超高刷支持 | 零侵入探测 | MD3 WebUI | 三级容灾
EOF'""")

# === 2. Fix service.sh ===
print("\n=== 2. Fix service.sh ===")
run(f"""sudo docker exec moduforge sh -c 'cat > {PROJ}/service.sh << "SERVEOF"
#!/system/bin/sh
# AndroBoost SmartTune service launcher (late_start)
set -e
MODDIR=${{0%/*}}
LOGFILE="$MODDIR/logs/service.log"

# Wait for boot complete
while [ "$(getprop sys.boot_completed)" != "1" ]; do sleep 1; done

# Redirect output to logfile
exec >"$LOGFILE" 2>&1

cd "$MODDIR" || exit 1

/bin/echo "AndroBoost Service started at $(date)"

# Binaries (installed in MODPATH/system/bin/)
ANDROMON="$MODDIR/system/bin/andromon"
ANDROENGINE="$MODDIR/system/bin/linucb-engine"
ANDROWEBUI="$MODDIR/system/bin/androwui"

# Config and shm paths
CONFIG="$MODDIR/data/config.txt"
SHM="$MODDIR/data/shm_memory"

# Three-phase startup
# Phase 0: monitor only (0-3s)
$ANDROMON --config "$CONFIG" --shm "$SHM" --stage 0 &
ANDROMON_PID=$!
echo "Phase 0: andromon PID $ANDROMON_PID started (monitoring only)"

sleep 3

# Phase 1: enable mapping (3-10s)
echo "Phase 1: advancing to mapping..."
kill -USR1 "$ANDROMON_PID" 2>/dev/null || killall -USR1 andromon 2>/dev/null || true

sleep 7

# Phase 2: full activation (10s+)
echo "Phase 2: full activation..."
kill -USR2 "$ANDROMON_PID" 2>/dev/null || killall -USR2 andromon 2>/dev/null || true

# Start strategy engine (Rust)
echo "Starting LinUCB strategy engine..."
"$ANDROENGINE" --shm "$SHM" &
ANDROENGINE_PID=$!

# Start WebUI backend (Go)
echo "Starting WebUI web server..."
"$ANDROWEBUI" --addr :8080 --shm "$SHM" &
ANDROWEBUI_PID=$!

echo "All services started. Waiting for termination."

# Trap signals to propagate
cleanup() {
  echo "Received signal, shutting down..."
  kill "$ANDROMON_PID" 2>/dev/null || true
  kill "$ANDROENGINE_PID" 2>/dev/null || true
  kill "$ANDROWEBUI_PID" 2>/dev/null || true
  echo "Services stopped."
  exit 0
}

trap cleanup SIGINT SIGTERM

# Keep script alive waiting for children
wait
SERVEOF'""")

# === 3. Fix customize.sh binary references ===
print("\n=== 3. Fix customize.sh ===")
run(f"""sudo docker exec moduforge sh -c 'cat > {PROJ}/customize.sh << "CUSTEOF"
#!/system/bin/sh
set -euo pipefail

# AndroBoost SmartTune Installer
MODPATH=${{0%/*}}
[ -d "$MODPATH" ] || abort "Error: Module directory not found"

ui_print "+============================"
ui_print "| AndroBoost SmartTune v2.0"
ui_print "+============================"

# Helper functions
set_perm() {
  local perms=$1 path=$2
  chmod "$perms" "$path" 2>/dev/null || true
}

log_info() {
  ui_print "- $1"
}

log_warn() {
  ui_print "! $1"
}

detect_env() {
  if [ -n "$KSU" ]; then
    log_info "KernelSU environment"
    export KSU=true
  elif [ -n "$APATCH" ]; then
    log_info "APatch environment"
    export APATCH=true
  else
    log_info "Magisk environment"
  fi
}

detect_soc() {
  if [ -f /proc/device-tree/compatible ]; then
    SOC=$(cat /proc/device-tree/compatible | tr '"'"'\\0'"'"' '"'"'\\n'"'"' | head -1)
    log_info "SoC: $SOC"
  else
    log_info "SoC: unknown"
  fi
  # GPU detection
  if [ -d /sys/class/kgsl/kgsl-3d0 ]; then
    log_info "GPU: Adreno (KGSL)"
    echo "gpu_type=adreno" >> "$MODPATH/data/config.txt"
  elif [ -d /sys/class/mali ]; then
    log_info "GPU: Mali"
    echo "gpu_type=mali" >> "$MODPATH/data/config.txt"
  else
    log_info "GPU: unknown/other"
    echo "gpu_type=other" >> "$MODPATH/data/config.txt"
  fi
}

detect_android() {
  local sdk=$(getprop ro.build.version.sdk 2>/dev/null || echo 28)
  log_info "Android SDK: $sdk"
  local pagesize=$(getconf PAGE_SIZE 2>/dev/null || echo 4096)
  if [ "$pagesize" = "16384" ]; then
    log_info "16KB page size enabled"
    echo "page_size=16KB" >> "$MODPATH/data/config.txt"
  else
    log_info "4KB page size"
    echo "page_size=4KB" >> "$MODPATH/data/config.txt"
  fi
  # AutoFDO detection
  if grep -q "auto_fdo" /sys/kernel/debug/sched_features 2>/dev/null; then
    log_info "AutoFDO kernel optimization detected"
    echo "autofdo=1" >> "$MODPATH/data/config.txt"
  else
    echo "autofdo=0" >> "$MODPATH/data/config.txt"
  fi
  # LTPO detection
  if [ -f /sys/devices/platform/soc/panel/panel0/refresh_rate ] || [ -d /sys/class/drm/card0/device/ltpo ]; then
    log_info "LTPO display supported"
    echo "ltpo=1" >> "$MODPATH/data/config.txt"
  else
    log_info "Standard display"
    echo "ltpo=0" >> "$MODPATH/data/config.txt"
  fi
}

create_dirs() {
  mkdir -p "$MODPATH/system/bin"
  mkdir -p "$MODPATH/data"
  mkdir -p "$MODPATH/logs"
  log_info "Directories created"
}

set_permissions() {
  set_perm 0755 "$MODPATH/system/bin/andromon"
  set_perm 0755 "$MODPATH/system/bin/linucb-engine"
  set_perm 0755 "$MODPATH/system/bin/androwui"
  # config files
  set_perm 0644 "$MODPATH/data/config.json"
  set_perm 0644 "$MODPATH/data/config.txt"
  set_perm 0644 "$MODPATH/data/node_map.txt"
  set_perm 0644 "$MODPATH/data/shm_memory"
  log_info "Permissions set"
}

generate_configs() {
  # JSON config
  if [ ! -f "$MODPATH/data/config.json" ]; then
    cat > "$MODPATH/data/config.json" <<'"'"'EOF'"'"'
{
  "version": "2.0",
  "monitoring": {
    "interval_ms": 100,
    "enable_temp": true,
    "enable_fps": true,
    "enable_power": true
  },
  "scaling": {
    "resolution": 100,
    "refresh_min": 60,
    "refresh_max": 165,
    "ltpo": true
  },
  "thermal": [
    {"temp": 43, "action": "light"},
    {"temp": 46, "action": "moderate"},
    {"temp": 48, "action": "big_core_off"},
    {"temp": 52, "action": "force30hz"}
  ],
  "io": {
    "foreground": "mq-deadline",
    "idle": "bfq",
    "dirty_ratio": 50,
    "dirty_expire": 6000
  },
  "linucb": {
    "alpha": 0.1,
    "dim": 7,
    "arms": 5,
    "explore": 0.3
  }
}
EOF
    log_info "Default config.json created"
  fi
  # TXT config for C++ daemon
  if [ ! -f "$MODPATH/data/config.txt" ]; then
    local interval=$(grep -o '"'"'"interval_ms": [0-9]*'"'"' "$MODPATH/data/config.json" | cut -d'"'"' '"'"' -f2)
    echo "interval_ms=${{interval:-100}}" > "$MODPATH/data/config.txt"
    echo "enable_temp=1" >> "$MODPATH/data/config.txt"
    echo "enable_fps=1" >> "$MODPATH/data/config.txt"
    echo "enable_power=1" >> "$MODPATH/data/config.txt"
    echo "resolution=100" >> "$MODPATH/data/config.txt"
    echo "refresh_min=60" >> "$MODPATH/data/config.txt"
    echo "refresh_max=165" >> "$MODPATH/data/config.txt"
    echo "ltpo=1" >> "$MODPATH/data/config.txt"
    log_info "Default config.txt created"
  fi
  # Shared memory file
  if [ ! -f "$MODPATH/data/shm_memory" ]; then
    dd if=/dev/zero of="$MODPATH/data/shm_memory" bs=1024 count=10240 2>/dev/null
    log_info "Shared memory file (10MB) created"
  fi
}

probe_nodes() {
  if [ -f "$MODPATH/data/node_map.txt" ]; then
    log_info "Node map already exists"
    return
  fi
  log_info "Probing system nodes for hardware mapping..."
  NODE_FILE="$MODPATH/data/node_map.txt"
  > "$NODE_FILE"
  # Platform devices
  for dir in /sys/devices/platform/*/; do
    [ -d "$dir" ] || continue
    name=$(basename "$dir")
    [ -f "${{dir}}temp" ] && echo "temp ${{dir}}temp" >> "$NODE_FILE"
    [ -f "${{dir}}name" ] && echo "zone_name ${{dir}}name" >> "$NODE_FILE"
  done
  # GPU
  if [ -f /sys/class/kgsl/kgsl-3d0/gpu_busy_percentage ]; then
    echo "gpu_busy /sys/class/kgsl/kgsl-3d0/gpu_busy_percentage" >> "$NODE_FILE"
  fi
  if [ -f /sys/class/kgsl/kgsl-3d0/clock_mhz ]; then
    echo "gpu_freq /sys/class/kgsl/kgsl-3d0/clock_mhz" >> "$NODE_FILE"
  fi
  # CPU
  for cpu in /sys/devices/system/cpu/cpu*/cpufreq/scaling_cur_freq; do
    [ -f "$cpu" ] && echo "cpu_freq $cpu" >> "$NODE_FILE"
  done
  # Temperature (mtk, qcom common)
  [ -f /sys/class/thermal/thermal_message/temp_mB ] && echo "thermal_global /sys/class/thermal/thermal_message/temp_mB" >> "$NODE_FILE"
  [ -f /sys/class/thermal/thermal_zone0/temp ] && echo "thermal0 /sys/class/thermal/thermal_zone0/temp" >> "$NODE_FILE"
  # Battery
  [ -f /sys/class/power_supply/battery/capacity ] && echo "battery /sys/class/power_supply/battery/capacity" >> "$NODE_FILE"
  [ -f /sys/class/power_supply/battery/current_now ] && echo "battery_current /sys/class/power_supply/battery/current_now" >> "$NODE_FILE"
  # Display / VSYNC
  [ -f /sys/class/graphics/fb0/vsync_event ] && echo "vsync /sys/class/graphics/fb0/vsync_event" >> "$NODE_FILE"
  # Refresh rate
  [ -f /sys/class/graphics/fb0/msm_fb_vsync_mode ] && echo "refresh /sys/class/graphics/fb0/msm_fb_vsync_mode" >> "$NODE_FILE"
  log_info "Node map written with $(wc -l <"$NODE_FILE") entries"
}

selinux_policy() {
  if command -v magiskpolicy >/dev/null 2>&1; then
    log_info "Injecting SELinux rules..."
    magiskpolicy --live --allow system_app sysfs file read 2>/dev/null || true
    magiskpolicy --live --allow system_app proc file write 2>/dev/null || true
    magiskpolicy --live --allow system_app kernel system write 2>/dev/null || true
    log_info "SELinux rules applied"
  else
    log_info "No magiskpolicy found, SELinux may restrict some operations"
  fi
}

install_cleanup() {
  # Remove old logs
  rm -f "$MODPATH/logs/"*.old 2>/dev/null || true
  log_info "Installation complete!"
  log_info "Reboot or run manually:"
  log_info "  /system/bin/andromon --config $MODPATH/data/config.txt --shm $MODPATH/data/shm_memory"
  log_info "  /system/bin/linucb-engine --shm $MODPATH/data/shm_memory"
  log_info "  /system/bin/androwui --addr :8080 --shm $MODPATH/data/shm_memory"
}

# Main installation
ui_print "+ Starting installation..."

detect_env
detect_soc
detect_android
create_dirs
generate_configs
probe_nodes
set_permissions
selinux_policy
install_cleanup

exit 0
CUSTEOF'""")

# === 4. Fix Rust build.sh ===
print("\n=== 4. Fix Rust build.sh ===")
run(f"""sudo docker exec moduforge sh -c 'cat > {PROJ}/src/rust/build.sh << "RUSTEOF"
#!/bin/bash
set -e

# Rust cross compilation for Android arm64
export RUSTFLAGS="-C target-cpu=generic"
export CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER=/opt/android-ndk/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang

cargo build --release --target aarch64-linux-android

PROJECT_ROOT="${{PROJECT_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}}"
mkdir -p "$PROJECT_ROOT/system/bin"
cp target/aarch64-linux-android/release/linucb-engine "$PROJECT_ROOT/system/bin/linucb-engine"
echo "Built linucb-engine"
RUSTEOF'""")

# === 5. Move Go binary to correct location ===
print("\n=== 5. Move Go binary ===")
run(f"sudo docker exec moduforge cp '{PROJ}/src/go/data/storage/projects/1785249992652501794-1864/system/bin/androwui' {PROJ}/system/bin/androwui")
run(f"sudo docker exec moduforge chmod 755 {PROJ}/system/bin/androwui")

# === 6. Verify all fixes ===
print("\n=== 6. Verify ===")
print("--- module.prop ---")
run(f"sudo docker exec moduforge cat {PROJ}/module.prop")
print("\n--- system/bin/ ---")
run(f"sudo docker exec moduforge ls -la {PROJ}/system/bin/ | grep -E '(andromon|linucb|androwui|total)'")
print("\n--- service.sh binaries ---")
run(f"sudo docker exec moduforge grep -E 'ANDRO|system/bin' {PROJ}/service.sh | head -5")
print("\n--- customize.sh binaries ---")
run(f"sudo docker exec moduforge grep -E 'system/bin' {PROJ}/customize.sh | head -5")
print("\n--- Rust build.sh cp line ---")
run(f"sudo docker exec moduforge grep 'cp.*target' {PROJ}/src/rust/build.sh")

ssh.close()
