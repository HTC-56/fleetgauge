#!/usr/bin/env bash
# verify.sh -- phase gates composed into one command.
#
# Runs gofmt, vet, test, scrub-check, and README-quickstart lint.
# Exits non-zero if any step fails. Run from anywhere.

set -uo pipefail

cd "$(dirname "$0")" || exit 2

fail=0
steps=0
passed=0

report() {
  printf '  %s: %s\n' "$1" "$2"
  if [[ "$2" = skip* ]]; then
    return # skip doesn't count
  fi
  steps=$((steps + 1))
  if [ "$2" != "pass" ]; then
    fail=1
  else
    passed=$((passed + 1))
  fi
}

echo "=== verify.sh ==="

# 1. gofmt
if gofmt -l . | grep -q .; then
  report "gofmt" "fail"
else
  report "gofmt" "pass"
fi

# 2. go vet
if go vet ./... 2>&1; then
  report "go vet" "pass"
else
  report "go vet" "fail"
fi

# 3. go test
if go test ./... 2>&1 | tail -1; then
  report "go test" "pass"
else
  report "go test" "fail"
fi

# 4. scrub-check
if bash scripts/scrub-check.sh; then
  report "scrub-check" "pass"
else
  report "scrub-check" "fail"
fi

# 5. README-quickstart lint: every file path in backticks inside README.md
#    must actually exist. Skip if README.md does not exist yet.
if [ -f "README.md" ]; then
  # Extract file paths from backticks (e.g. `scripts/foo.sh`)
  paths=$(grep -oE '`[^`]+\.sh|`[^`]+\.go|`[^`]+\.yaml|`[^`]+\.md|`[^`]+/[^`]*`' README.md \
    | sed 's/^`//;s/`$//' | sort -u)
  readme_ok=1
  for p in $paths; do
    if [ ! -e "$p" ]; then
      printf '  README lint: missing %s\n' "$p" >&2
      readme_ok=0
    fi
  done
  if [ "$readme_ok" -eq 1 ]; then
    report "README lint" "pass"
  else
    report "README lint" "fail"
  fi
else
  report "README lint" "skip (README.md not present)"
fi

echo "---"
printf '  %d/%d steps passed\n' "$passed" "$steps"

if [ "$fail" -ne 0 ]; then
  echo "verify: FAILED"
  exit 1
fi

echo "verify: all clear"
exit 0
