export interface Snippet {
	id: string;
	name: string;
	language: string;
	trigger: string;
	description: string;
	code: string;
}

export const snippets: Snippet[] = [
	{
		id: 'shell/module-prop',
		name: 'module.prop',
		language: 'shell',
		trigger: 'module-prop',
		description: 'Magisk module.prop metadata file',
		code: `id=<MODULE_ID>
name=<MODULE_NAME>
version=<VERSION>
versionCode=<VERSION_CODE>
author=<AUTHOR>
description=<DESCRIPTION>`
	},
	{
		id: 'shell/service-sh',
		name: 'service.sh',
		language: 'shell',
		trigger: 'service-sh',
		description: 'Boot service daemon skeleton',
		code: `#!/system/bin/sh
# Magisk Module Service Script
# Runs on every boot

MODDIR=\${0%/*}

# Wait for boot to complete
while [ "$(getprop sys.boot_completed)" != "1" ]; do
  sleep 1
done

# Log function
log_msg() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$MODDIR/service.log"
}

log_msg "Service started"

# Main daemon loop
while true; do
  # Add your monitoring logic here
  sleep 60
done`
	},
	{
		id: 'shell/customize-sh',
		name: 'customize.sh',
		language: 'shell',
		trigger: 'customize-sh',
		description: 'Magisk module installer script',
		code: `#!/system/bin/sh
# Magisk Module Installer Script

SKIPUNZIP=0

# Print module info
ui_print "========================================="
ui_print "  <MODULE_NAME> v<VERSION>"
ui_print "========================================="

# Extract module files
ui_print "- Extracting module files"
unzip -o "$ZIPFILE" -d "$MODPATH" >&2

# Set permissions
ui_print "- Setting permissions"
set_perm_recursive "$MODPATH" 0 0 0755 0644
set_perm "$MODPATH/system/bin/<binary>" 0 0 0755

ui_print "- Installation complete!"
ui_print "========================================="`
	},
	{
		id: 'shell/uninstall-sh',
		name: 'uninstall.sh',
		language: 'shell',
		trigger: 'uninstall-sh',
		description: 'Module uninstall cleanup script',
		code: `#!/system/bin/sh
# Magisk Module Uninstall Script

MODDIR=\${0%/*}

# Remove module data
rm -rf /data/adb/modules/$(basename "$MODDIR")

# Remove any created symlinks or files
# rm -f /system/bin/<binary>

echo "Module uninstalled successfully"`
	},
	{
		id: 'go/daemon',
		name: 'Go Daemon',
		language: 'go',
		trigger: 'go-daemon',
		description: 'Go daemon with graceful shutdown',
		code: `package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	log.Println("Daemon started")

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Daemon shutting down")
			return
		case <-ticker.C:
			if err := doWork(); err != nil {
				log.Printf("Error: %v", err)
			}
		}
	}
}

func doWork() error {
	// Add your logic here
	fmt.Println("Working...")
	return nil
}`
	},
	{
		id: 'go/sysfs-reader',
		name: 'Go Sysfs Reader',
		language: 'go',
		trigger: 'go-sysfs',
		description: 'Read sysfs values safely',
		code: `package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func readSysfs(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func readSysfsInt(path string) (int, error) {
	s, err := readSysfs(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(s)
}

func main() {
	// Example: read battery capacity
	cap, err := readSysfsInt("/sys/class/power_supply/battery/capacity")
	if err != nil {
		fmt.Printf("Error: %v\\n", err)
		return
	}
	fmt.Printf("Battery: %d%%\\n", cap)
}`
	}
];

/**
 * Search snippets by query and language
 */
export function searchSnippets(query: string, language?: string): Snippet[] {
	let results = snippets;

	if (language) {
		results = results.filter((s) => s.language === language);
	}

	if (!query || query.trim() === '') return results;

	const lowerQuery = query.toLowerCase();
	return results.filter(
		(s) =>
			s.name.toLowerCase().includes(lowerQuery) ||
			s.trigger.toLowerCase().includes(lowerQuery) ||
			s.description.toLowerCase().includes(lowerQuery)
	);
}

/**
 * Get snippet by ID
 */
export function getSnippetById(id: string): Snippet | undefined {
	return snippets.find((s) => s.id === id);
}

/**
 * Get snippets by language
 */
export function getSnippetsByLanguage(language: string): Snippet[] {
	return snippets.filter((s) => s.language === language);
}
