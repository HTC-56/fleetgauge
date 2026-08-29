#!/usr/bin/env bash
# scrub-check.sh -- public-repo hygiene gate.
#
# This repo is meant to be published. Greps the tracked tree for the four
# things that must never reach a public commit, plus the one structural rule
# the spec pre-registers (exec.Command lives only in the systemd backend).
#
# Exits non-zero on the first category with a hit. Run from anywhere.

set -uo pipefail

cd "$(dirname "$0")/.." || exit 2

fail=0

# Files to search: everything git tracks, plus anything staged. Untracked
# scratch files are not the gate's business.
files() {
	{
		git ls-files 2>/dev/null
		git diff --cached --name-only --diff-filter=d 2>/dev/null
	} | sort -u | while read -r f; do
		[ -f "$f" ] && printf '%s\n' "$f"
	done
}

report() {
	# $1 = category, rest = the offending grep output
	printf 'scrub-check: FAIL (%s)\n' "$1" >&2
	printf '%s\n' "$2" >&2
	fail=1
}

FILES=$(files)
if [ -z "$FILES" ]; then
	echo "scrub-check: no tracked files yet -- nothing to scrub"
	exit 0
fi

# 1. Absolute home paths. /home/<user> or /Users/<user> pins the author's box.
hits=$(printf '%s\n' "$FILES" | xargs -r grep -nE '(/home/[a-zA-Z0-9._-]+|/Users/[a-zA-Z0-9._-]+)' -- 2>/dev/null \
	| grep -v '^scripts/scrub-check.sh:')
[ -n "$hits" ] && report "absolute home path" "$hits"

# 2. Non-documentation IPv4 literals. RFC 5737 reserves 192.0.2.x / 198.51.100.x
#    / 203.0.113.x for docs; loopback and 0.0.0.0 are fine. Everything else --
#    especially RFC1918 LAN addresses -- is a leak.
hits=$(printf '%s\n' "$FILES" | xargs -r grep -nE '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' -- 2>/dev/null \
	| grep -vE '\b(127\.0\.0\.1|0\.0\.0\.0|255\.255\.255\.[0-9]{1,3}|192\.0\.2\.[0-9]{1,3}|198\.51\.100\.[0-9]{1,3}|203\.0\.113\.[0-9]{1,3})\b' \
	| grep -vE '^[^:]*:[0-9]+:.*\b[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\b.*(version|Version|v[0-9])' \
	| grep -v '^scripts/scrub-check.sh:')
[ -n "$hits" ] && report "non-documentation IP address" "$hits"

# 3. Key material.
hits=$(printf '%s\n' "$FILES" | xargs -r grep -nE 'BEGIN (RSA |EC |OPENSSH |PGP |DSA )?PRIVATE KEY|ssh-rsa AAAA|AKIA[0-9A-Z]{16}|xox[baprs]-[0-9A-Za-z-]{10,}|gh[pousr]_[0-9A-Za-z]{20,}|sk-[0-9A-Za-z]{32,}' -- 2>/dev/null \
	| grep -v '^scripts/scrub-check.sh:')
[ -n "$hits" ] && report "possible key material" "$hits"

# 4. References to other private local projects. The public HTC-56 repos are
#    explicitly allowed to be named (DECISIONS.md).
hits=$(printf '%s\n' "$FILES" | xargs -r grep -niE '\b(harpertechco|harper studio|harper-studio)\b' -- 2>/dev/null \
	| grep -v '^scripts/scrub-check.sh:')
[ -n "$hits" ] && report "reference to a private project or account" "$hits"

# 5. Structural: exec.Command outside the systemd backend package is a spec bug.
hits=$(printf '%s\n' "$FILES" | grep -E '\.go$' \
	| grep -v '^internal/backend/systemd/' \
	| xargs -r grep -n 'exec\.Command' -- 2>/dev/null)
[ -n "$hits" ] && report "exec.Command outside internal/backend/systemd" "$hits"

if [ "$fail" -ne 0 ]; then
	exit 1
fi

echo "scrub-check: clean"
