#!/bin/sh
# Full bugcheck: vet + tests + optional linters if installed.
# POSIX sh only. Run constantly during dev and in CI.
# Missing optional tools are skipped (warn), go vet/test must pass.
set -eu
cd "$(dirname "$0")/../.."
mkdir -p logs
failed=0

run_required() {
  name="$1"
  shift
  echo "==> Running $name"
  log="logs/go-bugcheck-${name}.txt"
  status=0
  "$@" >"$log" 2>&1 || status=$?
  cat "$log"
  if [ "$status" -eq 0 ]; then
    echo "    $name: OK"
  else
    echo "    $name: FAILED (exit $status)"
    failed=1
  fi
  echo ""
}

run_optional() {
  name="$1"
  shift
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "==> Skipping $name ($1 not installed)"
    echo ""
    return 0
  fi
  # Optional tools warn only (never fail build; old versions break on Go 1.27).
  log="logs/go-bugcheck-${name}.txt"
  echo "==> Running $name (optional)"
  status=0
  "$@" >"$log" 2>&1 || status=$?
  cat "$log"
  if [ "$status" -eq 0 ]; then
    echo "    $name: OK"
  else
    echo "    $name: WARN (exit $status, ignored)"
  fi
  echo ""
}

run_required vet go vet ./...
run_required test go test ./...

run_optional golangci-lint golangci-lint run ./...
run_optional govulncheck govulncheck ./...
run_optional staticcheck staticcheck ./...
run_optional errcheck errcheck ./...
run_optional gopls gopls check ./...

exit "$failed"
