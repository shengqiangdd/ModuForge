#!/system/bin/sh
# {{MODULE_NAME}} - Boot Service
# This script runs on boot

MODDIR=${0%/*}

# Wait for boot to complete
while [ "$(getprop sys.boot_completed)" != "1" ]; do
    sleep 5
done

# Start the daemon
if [ -f "$MODDIR/system/bin/{{MODULE_ID}}" ]; then
    $MODDIR/system/bin/{{MODULE_ID}} "$MODDIR/config.json" &
fi
