#!/bin/sh
# Security check. POSIX sh only. gosec optional (warn-only; old gosec breaks on Go 1.27).
set -eu
cd "$(dirname "$0")/../.."
mkdir -p logs

if ! command -v gosec >/dev/null 2>&1; then
  echo "==> Skipping gosec (not installed)"
  exit 0
fi

echo "==> Running gosec (optional)"
log="logs/go-seccheck-gosec.txt"
status=0
gosec ./... >"$log" 2>&1 || status=$?
cat "$log"
if [ "$status" -eq 0 ]; then
  echo "    gosec: OK"
else
  echo "    gosec: WARN (exit $status, ignored)"
fi
exit 0
