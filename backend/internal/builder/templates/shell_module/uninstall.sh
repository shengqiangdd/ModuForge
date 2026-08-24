#!/system/bin/sh
# {{MODULE_NAME}} - Uninstall Script
# Clean up on module removal

MODDIR=${0%/*}

# Stop the daemon if running
pkill -f "{{MODULE_ID}}" 2>/dev/null

# Remove module data
rm -rf /data/adb/modules/$(basename $MODDIR)

# Clean up any log files
rm -f /data/local/tmp/{{MODULE_ID}}.log
