#!/system/bin/sh

# ============================================
# 系统优化工具箱 - service 启动脚本
# 在 late_start 服务模式下执行
# 兼容 Magisk / KernelSU / APatch
# ============================================

MODDIR=${0%/*}

# 等待系统完全启动
while [ "$(getprop sys.boot_completed)" != "1" ]; do
  sleep 2
done

sleep 5

# 应用附加优化（需要系统就绪）
# 不建议 post-fs-data 中做的操作在此执行

# 设置 TCP 缓冲区（通过 sysctl）
sysctl -w net.ipv4.tcp_rmem="4096 87380 16777216" 2>/dev/null
sysctl -w net.ipv4.tcp_wmem="4096 65536 16777216" 2>/dev/null
sysctl -w net.core.rmem_max=16777216 2>/dev/null
sysctl -w net.core.wmem_max=16777216 2>/dev/null

# 优化 ZRAM (如果存在)
if [ -f /sys/block/zram0/disksize ]; then
  current_zram=$(cat /sys/block/zram0/disksize 2>/dev/null)
  total_ram=$(awk '/MemTotal/ {print $2}' /proc/meminfo 2>/dev/null)
  zram_size=$(( total_ram * 1024 / 2 ))
  if [ -n "$zram_size" ] && [ "$zram_size" -gt 0 ]; then
    echo "$zram_size" > /sys/block/zram0/disksize 2>/dev/null
    mkswap /dev/block/zram0 2>/dev/null
    swapon /dev/block/zram0 2>/dev/null
  fi
fi

# 应用 entropy 优化
echo "512" > /proc/sys/kernel/random/read_wakeup_threshold 2>/dev/null
echo "256" > /proc/sys/kernel/random/write_wakeup_threshold 2>/dev/null

# 文件系统优化
for fs in /sys/fs/ext4/*/errors; do
  [ -w "$fs" ] && echo "continue" > "$fs" 2>/dev/null
done

# ADB 调试增强（仅调试用途）
setprop persist.adb.notify 0 2>/dev/null
setprop persist.adb.monitoring 1 2>/dev/null

# 日志标记
log -t "SystemToolbox" "service.sh: 启动后优化已应用"

# 如果存在自定义配置，加载
if [ -f "$MODDIR/system/etc/optimize.conf" ]; then
  . "$MODDIR/system/etc/optimize.conf" 2>/dev/null
fi
