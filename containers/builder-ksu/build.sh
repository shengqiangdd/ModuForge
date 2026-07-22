#!/bin/bash
set -euo pipefail

WORKSPACE="/workspace"
OUTPUT="/output/module.zip"

# 验证 KSU 入口脚本存在
if [ ! -f "$WORKSPACE/ksu.sh" ] && [ ! -f "$WORKSPACE/webroot/ksu.sh" ]; then
  echo "ERROR: ksu.sh or webroot/ksu.sh not found in /workspace" >&2
  exit 1
fi

# 清理旧产物
rm -f "$OUTPUT"

# 打包所有文件
cd "$WORKSPACE"
zip -r "$OUTPUT" . -x ".*"

echo "Build complete: $OUTPUT"
