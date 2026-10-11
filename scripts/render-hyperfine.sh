#!/bin/sh
# Typed benchmark demo: testdata/hyperfine.yml -> dist/hyperfine.gif. POSIX sh only.
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
mkdir -p logs
echo "==> render-hyperfine"
log="logs/render-hyperfine.txt"
status=0
go run ./cmd/termrec render testdata/hyperfine -o dist/hyperfine.gif -s 1 >"$log" 2>&1 || status=$?
cat "$log"
if [ "$status" -eq 0 ]; then
  echo "render-hyperfine: OK"
  ls -l dist/hyperfine.gif || true
else
  echo "render-hyperfine: FAILED"
  exit 1
fi
