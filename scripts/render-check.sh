#!/bin/sh
# Render every fixture and verify outputs. POSIX sh only.
# Used by test.yml and locally via `task render-check`.
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
mkdir -p logs
log="logs/render-check.txt"
: >"$log"
failed=0
count=0
echo "==> render-check" | tee -a "$log"
for f in testdata/*.yml; do
  base=$(basename "$f" .yml)
  out="dist/check-${base}.gif"
  count=$((count + 1))
  if go run ./cmd/termrec render "$f" -o "$out" -s 1 >>"$log" 2>&1; then
    if [ -s "$out" ]; then
      size=$(wc -c <"$out" | tr -d ' ')
      echo "    $base: OK ($size bytes)" | tee -a "$log"
    else
      echo "    $base: FAILED (empty output)" | tee -a "$log"
      failed=1
    fi
  else
    echo "    $base: FAILED (render error)" | tee -a "$log"
    failed=1
  fi
done
# step + quality variants on demo
if go run ./cmd/termrec render testdata/demo -o dist/check-demo-step2.gif -s 2 >>"$log" 2>&1 \
  && go run ./cmd/termrec render testdata/demo -o dist/check-demo-q30.gif -q 30 >>"$log" 2>&1; then
  echo "    variants(step2,q30): OK" | tee -a "$log"
else
  echo "    variants(step2,q30): FAILED" | tee -a "$log"
  failed=1
fi
# white-background variants of every fixture (-b white)
for f in testdata/*.yml; do
  base=$(basename "$f" .yml)
  out="dist/whitebg-${base}.gif"
  if go run ./cmd/termrec render "$f" -o "$out" -s 1 -b white >>"$log" 2>&1; then
    if [ -s "$out" ]; then
      size=$(wc -c <"$out" | tr -d ' ')
      echo "    whitebg-$base: OK ($size bytes)" | tee -a "$log"
    else
      echo "    whitebg-$base: FAILED (empty output)" | tee -a "$log"
      failed=1
    fi
  else
    echo "    whitebg-$base: FAILED (render error)" | tee -a "$log"
    failed=1
  fi
done
echo "checked $count fixtures" | tee -a "$log"
exit "$failed"
