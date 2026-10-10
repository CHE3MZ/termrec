#!/bin/sh
# Smoke render: testdata/demo.yml -> dist/demo.gif. POSIX sh only.
set -eu
cd "$(dirname "$0")/../.."
mkdir -p dist
mkdir -p logs
echo "==> render-sample"
log="logs/render-sample.txt"
status=0
go run ./cmd/termrec render testdata/demo -o dist/demo.gif -s 1 >"$log" 2>&1 || status=$?
cat "$log"
if [ "$status" -eq 0 ]; then
  echo "render-sample: OK"
  ls -l dist/demo.gif || true
else
  echo "render-sample: FAILED"
  exit 1
fi
