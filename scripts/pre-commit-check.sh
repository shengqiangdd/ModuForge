#!/bin/bash
# pre-commit-check.sh — 防止提交大文件和编译产物
# 用法: 在 .git/hooks/pre-commit 中调用，或手动运行

set -euo pipefail

MAX_SIZE_KB=1024  # 1MB 上限
ERRORS=0

# 检查暂存区中的文件
for file in $(git diff --cached --name-only --diff-filter=A 2>/dev/null); do
  # 跳过非文件
  [ -f "$file" ] || continue
  
  # 检查文件大小
  size_kb=$(du -k "$file" 2>/dev/null | awk '{print $1}')
  if [ "${size_kb:-0}" -gt "$MAX_SIZE_KB" ]; then
    echo "❌ 拒绝提交: $file (${size_kb}KB > ${MAX_SIZE_KB}KB)"
    echo "   如果这是编译产物，请添加到 .gitignore"
    echo "   如果确实需要提交大文件，使用: git commit --no-verify"
    ERRORS=$((ERRORS + 1))
  fi
  
  # 检查是否是编译产物模式
  case "$file" in
    */target/debug/*|*/target/release/*|*/node_modules/*|*.o|*.so|*.dylib|*/binary/*)
      echo "⚠️  警告: $file 看起来像编译产物"
      echo "   如果确实需要提交，使用: git commit --no-verify"
      ERRORS=$((ERRORS + 1))
      ;;
  esac
done

if [ "$ERRORS" -gt 0 ]; then
  echo ""
  echo "共 $ERRORS 个问题。使用 'git commit --no-verify' 跳过检查。"
  exit 1
fi

echo "✅ Pre-commit 检查通过"
exit 0
