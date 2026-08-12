#!/bin/sh
# AndroBoost SmartTune service launcher (late_start)
set -e
MODDIR=${0%/*}
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
