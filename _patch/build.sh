#!/bin/bash
set -e

# Rust cross compilation for Android arm64
export RUSTFLAGS="-C target-cpu=generic"
export CARGO_TARGET_AARCH64_LINUX_ANDROID_LINKER=/opt/android-ndk/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android21-clang

cargo build --release --target aarch64-linux-android

PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"
mkdir -p "$PROJECT_ROOT/system/bin"
cp target/aarch64-linux-android/release/linucb-engine "$PROJECT_ROOT/system/bin/linucb-engine"
echo "Built linucb-engine"
