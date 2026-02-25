#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -x "dist/local/beepboop" ]]; then
  echo "Missing dist/local/beepboop. Run ./scripts/dev.sh first."
  exit 1
fi

echo "Smoke test: HTTP once check against https://example.com"
./dist/local/beepboop --target https://example.com --mode auto --once --timeout 5s

echo "Smoke test passed"
