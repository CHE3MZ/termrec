#!/bin/sh
# Build termrec into dist/. POSIX sh, no bashisms.
set -eu
cd "$(dirname "$0")/../.."
mkdir -p dist
mkdir -p logs
echo "==> Building termrec"
log="logs/build.txt"
status=0
go build -o dist/termrec ./cmd/termrec >"$log" 2>&1 || status=$?
cat "$log"
if [ "$status" -eq 0 ]; then
  echo "build: OK"
  ls -l dist/termrec || true
else
  echo "build: FAILED"
  exit 1
fi
