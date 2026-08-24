#!/bin/sh
# Build script for Go daemon module
# Cross-compile for Android ARM64

set -eu

echo "Building Go daemon..."

# Create output directory
mkdir -p ./bin

# Cross-compile for Android
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o ./bin/{{MODULE_ID}} \
    ./src/

echo "Build complete: ./bin/{{MODULE_ID}}"
