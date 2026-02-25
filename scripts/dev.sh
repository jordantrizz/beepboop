#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

case "$OS" in
  linux*) GOOS="linux" ;;
  darwin*) GOOS="darwin" ;;
  msys*|mingw*|cygwin*) GOOS="windows" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

mkdir -p dist/local

echo "Running tests..."
go test ./...

echo "Building beepboop for host ($GOOS/$GOARCH)..."
GOOS="$GOOS" GOARCH="$GOARCH" go build -o "dist/local/beepboop" ./cmd/beepboop

echo "Done. Binary: dist/local/beepboop"
echo "Try: ./dist/local/beepboop --target 1.1.1.1 --mode icmp --once"
