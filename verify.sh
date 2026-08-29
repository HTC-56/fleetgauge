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
#    Deliberately unpiped: `go test ./... | tail -1` yields the exit status of
#    tail, which is always 0, so a red suite was reported as a pass. Capture the
#    output and print it only when the suite actually fails.
if test_out=$(go test ./... 2>&1); then
  report "go test" "pass"
else
  printf '%s\n' "$test_out" >&2
  report "go test" "fail"
fi

# 4. scrub-check
if bash scripts/scrub-check.sh; then
  report "scrub-check" "pass"
else
  report "scrub-check" "fail"
fi

# 5. README-quickstart lint: every repo path named in backticks inside
#    README.md must actually exist. Skip if README.md does not exist yet.
#
#    Most backticked spans in a quickstart are COMMANDS, not paths -- so split
#    each span into words and keep only the words that really are a path into
#    this repo. Everything else is skipped by shape: flags (-demo), URLs,
#    Go package patterns (./...), absolute deploy paths (/etc/...), route
#    templates ({name}), and module paths whose first segment is a domain
#    (gopkg.in/yaml.v3). Without that filter the lint reads "go test ./..."
#    as three missing files and fails every README that shows a command.
if [ -f "README.md" ]; then
  readme_ok=1
  spans=$(grep -oE '`[^`]+`' README.md | sed 's/^`//;s/`$//')
  for p in $spans; do
    case "$p" in
      -* | /* | *://* | *...* | *'{'* | *'*'* | *'$'* | *'~'*) continue ;;
    esac
    p=${p#./}
    case "$p" in
      */*)
        # A first segment with a dot in it is a domain, not a directory.
        case "${p%%/*}" in *.*) continue ;; esac
        ;;
      *.sh | *.go | *.yaml | *.yml | *.md | *.html) ;;
      *) continue ;;
    esac
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
