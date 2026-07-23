#!/system/bin/sh

# ============================================
# 系统优化工具箱 - post-fs-data 启动脚本
# 在 post-fs-data 模式下执行 (Magisk)
# ============================================

MODDIR=${0%/*}

# 等待系统完成挂载
while [ "$(getprop sys.boot_completed)" != "1" ]; do
  sleep 1
done

# 优化内核参数 - 网络
echo "bbr2" > /proc/sys/net/ipv4/tcp_congestion_control 2>/dev/null
echo "1" > /proc/sys/net/ipv4/tcp_mtu_probing 2>/dev/null
echo "3" > /proc/sys/net/ipv4/tcp_fastopen 2>/dev/null
echo "0" > /proc/sys/net/ipv4/tcp_slow_start_after_idle 2>/dev/null
echo "15" > /proc/sys/net/ipv4/tcp_fin_timeout 2>/dev/null
echo "1800" > /proc/sys/net/ipv4/tcp_keepalive_time 2>/dev/null
echo "30" > /proc/sys/net/ipv4/tcp_keepalive_intvl 2>/dev/null
echo "3" > /proc/sys/net/ipv4/tcp_keepalive_probes 2>/dev/null
echo "16777216" > /proc/sys/net/core/rmem_max 2>/dev/null
echo "16777216" > /proc/sys/net/core/wmem_max 2>/dev/null
echo "5000" > /proc/sys/net/core/netdev_max_backlog 2>/dev/null
echo "1024" > /proc/sys/net/core/somaxconn 2>/dev/null

# 优化内核参数 - 内存管理
echo "20" > /proc/sys/vm/dirty_ratio 2>/dev/null
echo "5" > /proc/sys/vm/dirty_background_ratio 2>/dev/null
echo "3000" > /proc/sys/vm/dirty_expire_centisecs 2>/dev/null
echo "1500" > /proc/sys/vm/dirty_writeback_centisecs 2>/dev/null
echo "100" > /proc/sys/vm/vfs_cache_pressure 2>/dev/null
echo "60" > /proc/sys/vm/swappiness 2>/dev/null
echo "4096" > /proc/sys/vm/min_free_kbytes 2>/dev/null
echo "0" > /proc/sys/vm/page-cluster 2>/dev/null

# 优化 I/O 调度器
for queue in /sys/block/*/queue/scheduler; do
  [ -w "$queue" ] && echo "bfq" > "$queue" 2>/dev/null
done

# 设置 I/O 调度器参数 (BFQ)
for queue in /sys/block/*/queue/iosched; do
  if [ -d "$queue" ]; then
    echo "0" > "$queue/low_latency" 2>/dev/null
    echo "1" > "$queue/group_idle" 2>/dev/null
    echo "100" > "$queue/timeout_sync" 2>/dev/null
  fi
done

# 优化读取前移
for ra in /sys/block/*/queue/read_ahead_kb; do
  [ -w "$ra" ] && echo "2048" > "$ra" 2>/dev/null
done

# 优化 CPU 调节器
for gov in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
  [ -w "$gov" ] && echo "schedutil" > "$gov" 2>/dev/null
done

# 启用 GPU 渲染优化
setprop debug.composition.type gpu 2>/dev/null
setprop debug.hwui.renderer opengl 2>/dev/null
setprop video.accelerate.hw 1 2>/dev/null

# 日志标记
log -t "SystemToolbox" "post-fs-data: 优化已应用"
