#!/bin/sh
# Complex demo: testdata/micro.yml (full-screen editor session) -> dist/micro.gif. POSIX sh only.
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
mkdir -p logs
echo "==> render-micro"
log="logs/render-micro.txt"
status=0
go run ./cmd/termrec render testdata/micro -o dist/micro.gif -s 1 >"$log" 2>&1 || status=$?
cat "$log"
if [ "$status" -eq 0 ]; then
  echo "render-micro: OK"
  ls -l dist/micro.gif || true
else
  echo "render-micro: FAILED"
  exit 1
fi
