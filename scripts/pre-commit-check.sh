#!/bin/bash
# pre-commit-check.sh — 防止提交大文件、编译产物和不相关文件
# 用法: 在 .git/hooks/pre-commit 中调用，或手动运行
# 规范参考: docs/REPOSITORY_GUIDELINES.md

set -euo pipefail

MAX_SIZE_KB=1024  # 1MB 上限
ERRORS=0

# 不相关文件黑名单（新增文件检查）：
#  - 根目录散落脚本（项目源码只允许在 backend/ frontend/ scripts/ docs/）
#  - 二进制/编译产物、日志、备份、压缩包、一次性部署脚本
BAD_FILE_PATTERNS=(
  '^[^/]*\.py$'          # 根目录 .py 调试脚本
  '\.exe$' '\.dll$' '\.so$' '\.o$' '\.a$' '\.bin$'
  '\.log$' '\.tmp$' '\.swp$' '\.orig$' '\.rej$'
  '\.zip$' '\.tar$' '\.tar\.gz$' '\.tgz$'
  '\.bak$' '\.bak-' '\.hostmode'
  '^deploy.*\.sh$' '^deploy.*\.ps1$'
  '^moduforge-server' '^server\.exe$' '^test_ziputil\.exe$'
  '^opencode_analysis\.txt$' '^opencode\.json$'
  '^_scripts_archive/' '^build_artifacts/'
)

# 检查暂存区中的新增文件
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

  # 检查是否是不相关文件
  for pat in "${BAD_FILE_PATTERNS[@]}"; do
    if echo "$file" | grep -qE "$pat"; then
      echo "❌ 拒绝提交: $file (不相关文件: $pat)"
      echo "   根目录不放调试脚本/二进制/日志/备份; 源码进 backend|frontend, 配置进 scripts|docs"
      echo "   如确需提交，先加入 .gitignore 白名单或使用: git commit --no-verify"
      ERRORS=$((ERRORS + 1))
      break
    fi
  done

  # 检查是否是编译产物模式
  case "$file" in
    */target/debug/*|*/target/release/*|*/node_modules/*|*.dylib|*/binary/*)
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
