#!/bin/bash
set -euo pipefail

WORKSPACE="/workspace"
OUTPUT="/output/module.zip"

# 验证 module.prop 存在
if [ ! -f "$WORKSPACE/module.prop" ]; then
  echo "ERROR: module.prop not found in /workspace" >&2
  exit 1
fi

# 清理旧产物
rm -f "$OUTPUT"

# 打包所有文件
cd "$WORKSPACE"
zip -r "$OUTPUT" . -x ".*"

echo "Build complete: $OUTPUT"
