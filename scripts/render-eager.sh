#!/bin/sh
# System-font showcase: testdata/eager.yml (Eager Naturalist) -> dist/eager.gif. POSIX sh only.
# Falls back to embedded Go Mono on machines without the font installed.
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
mkdir -p logs
echo "==> render-eager"
log="logs/render-eager.txt"
status=0
go run ./cmd/termrec render testdata/eager -o dist/eager.gif -s 1 >"$log" 2>&1 || status=$?
cat "$log"
if [ "$status" -eq 0 ]; then
  echo "render-eager: OK"
  ls -l dist/eager.gif || true
else
  echo "render-eager: FAILED"
  exit 1
fi
