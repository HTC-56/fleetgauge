#!/usr/bin/env bash
# live-check.sh -- the real-systemd proof. A HUMAN runs this; CI never does.
#
# Everything else in this repo is proved against the fake backend. That is
# deliberate — the test suite must run on any OS with no systemd — but it
# means the systemd backend itself (systemctl show parsing, journalctl tails,
# a real restart) is unproved by every green build. This script is the only
# thing that proves it, and DECISIONS.md makes its output the sole basis for
# any claim that fleetgauge has run against real units. Until a human runs it
# and reads the verdict below, the README, STATUS.md and docs/PROCESS.md say
# so plainly and claim nothing.
#
# Usage:
#   bash scripts/live-check.sh [--unit NAME] [--restart NAME]
#
#   --unit NAME     unit to observe read-only. Default systemd-journald.service,
#                   which exists on every systemd box and is never restarted here.
#   --restart NAME  ALSO prove the mutating path against NAME: opt it in, POST
#                   a token-bearing restart, and check the ledger and the
#                   restart counter. Omitted by default -- this script does not
#                   restart anything unless you name the unit yourself.
#
# It builds the binary into a temp dir, writes a temp config, serves on
# loopback, curls the read-only surface, and tears the process down. Nothing
# is installed and nothing in the repo is modified.

set -uo pipefail

cd "$(dirname "$0")/.." || exit 2

PORT="${FLEETGAUGE_PORT:-8137}"
BASE="http://127.0.0.1:${PORT}"
UNIT="systemd-journald.service"
RESTART_UNIT=""

while [ $# -gt 0 ]; do
	case "$1" in
	--unit)
		UNIT="${2:-}"
		shift 2
		;;
	--restart)
		RESTART_UNIT="${2:-}"
		shift 2
		;;
	-h | --help)
		sed -n '2,28p' "$0"
		exit 0
		;;
	*)
		printf 'live-check: unknown argument %q\n' "$1" >&2
		exit 2
		;;
	esac
done

if [ -z "$UNIT" ]; then
	echo "live-check: --unit needs a unit name" >&2
	exit 2
fi

pass=0
fail=0
tmp=""
pid=""

check() {
	# $1 = what was checked, $2 = "pass" or a failure detail
	if [ "$2" = "pass" ]; then
		pass=$((pass + 1))
		printf '  PASS  %s\n' "$1"
	else
		fail=$((fail + 1))
		printf '  FAIL  %s -- %s\n' "$1" "$2"
	fi
}

cleanup() {
	if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
		kill "$pid" 2>/dev/null
		wait "$pid" 2>/dev/null
	fi
	# Only ever the scratch directory this run created with mktemp, never a
	# path that came from an argument.
	case "$tmp" in
	*/fleetgauge-live.*)
		[ -d "$tmp" ] && rm -rf "$tmp"
		;;
	esac
}
trap cleanup EXIT

echo "=== live-check.sh -- real systemd, human-run ==="
echo

# --- preconditions ---------------------------------------------------------
# Each of these is a reason the run would prove nothing, so stop rather than
# emit a green that means "we did not actually test systemd".

if [ -n "${CI:-}" ]; then
	echo "live-check: refusing to run under CI. This script needs a real" >&2
	echo "systemd box and a human reading the verdict; a CI pass here would" >&2
	echo "be a claim nobody checked." >&2
	exit 2
fi

if [ ! -d /run/systemd/system ]; then
	echo "live-check: /run/systemd/system is absent -- this box is not running" >&2
	echo "systemd as PID 1, so there is nothing here to prove. Skipped, not passed." >&2
	exit 2
fi

for tool in systemctl curl go; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		printf 'live-check: %s not found in PATH\n' "$tool" >&2
		exit 2
	fi
done

if ! systemctl show -p ActiveState --value "$UNIT" >/dev/null 2>&1; then
	printf 'live-check: systemctl does not know unit %q\n' "$UNIT" >&2
	exit 2
fi

printf 'observing:  %s\n' "$UNIT"
if [ -n "$RESTART_UNIT" ]; then
	printf 'restarting: %s  (mutating path enabled)\n' "$RESTART_UNIT"
else
	printf 'restarting: nothing (read-only run; pass --restart NAME to prove it)\n'
fi
echo

# --- build and configure ---------------------------------------------------

tmp=$(mktemp -d "${TMPDIR:-/tmp}/fleetgauge-live.XXXXXX") || exit 2
bin="$tmp/fleetgauge"
cfg="$tmp/fleetgauge.yaml"
ledger="$tmp/ledger.jsonl"
logfile="$tmp/stderr.log"

echo "building CGO_ENABLED=0 binary..."
if ! CGO_ENABLED=0 go build -trimpath -o "$bin" ./cmd/fleetgauge; then
	echo "live-check: build failed" >&2
	exit 1
fi

# A throwaway token: this config lives for the length of the run and is never
# written into the repo.
token=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')

{
	printf 'listen: "127.0.0.1:%s"\n' "$PORT"
	printf 'bearer_token: "%s"\n' "$token"
	printf 'poll_interval: "1s"\n'
	printf 'journal_lines: 10\n'
	printf 'ledger_path: "%s"\n' "$ledger"
	printf 'units:\n'
	printf '  - name: "%s"\n' "$UNIT"
	if [ -n "$RESTART_UNIT" ] && [ "$RESTART_UNIT" != "$UNIT" ]; then
		printf '  - name: "%s"\n' "$RESTART_UNIT"
		printf '    allow_restart: true\n'
	elif [ -n "$RESTART_UNIT" ]; then
		printf '    allow_restart: true\n'
	fi
} >"$cfg"

"$bin" -config "$cfg" >"$logfile" 2>&1 &
pid=$!

# Wait for the first poll to land: /healthz is 503 until the store has data.
ready=""
for _ in $(seq 1 40); do
	if [ "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/healthz" 2>/dev/null)" = "200" ]; then
		ready=yes
		break
	fi
	if ! kill -0 "$pid" 2>/dev/null; then
		break
	fi
	sleep 0.5
done

if [ -z "$ready" ]; then
	echo "live-check: fleetgauge never became healthy. Its stderr:" >&2
	cat "$logfile" >&2
	exit 1
fi

echo "serving on $BASE"
echo
echo "--- read-only surface ---"

# 1. The page.
body=$(curl -s "$BASE/")
case "$body" in
'<!DOCTYPE html>'*) check "GET / serves the page" "pass" ;;
*) check "GET / serves the page" "did not start with <!DOCTYPE html>" ;;
esac

# 2. /metrics knows the real unit by name.
metrics=$(curl -s "$BASE/metrics")
if printf '%s' "$metrics" | grep -q "fleetgauge_unit_up{.*$UNIT"; then
	check "/metrics reports $UNIT" "pass"
else
	check "/metrics reports $UNIT" "no fleetgauge_unit_up line for it"
fi

# 3. The parsed state agrees with systemctl -- this is the parser proof.
want=$(systemctl show -p ActiveState --value "$UNIT")
got=$(curl -s "$BASE/metrics" | grep "fleetgauge_unit_state{.*$UNIT" | head -1)
if [ -n "$got" ] && printf '%s' "$got" | grep -q "state=\"$want\""; then
	check "parsed ActiveState matches systemctl ($want)" "pass"
else
	check "parsed ActiveState matches systemctl ($want)" "metrics line was: ${got:-<none>}"
fi

# 4. The journal drawer really reads journalctl.
jr=$(curl -s -o "$tmp/journal.json" -w '%{http_code}' "$BASE/units/$UNIT/journal")
if [ "$jr" = "200" ] && grep -q '"lines"' "$tmp/journal.json"; then
	check "journal drawer returns lines for $UNIT" "pass"
else
	check "journal drawer returns lines for $UNIT" "HTTP $jr; journalctl may need root or the systemd-journal group"
fi

# 5. Restart is refused without the token even on a live box.
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/units/$UNIT/restart")
if [ "$code" = "401" ]; then
	check "restart without a token is 401" "pass"
else
	check "restart without a token is 401" "got HTTP $code"
fi

# 6. And refused for a unit that did not opt in, even with the token.
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST \
	-H "Authorization: Bearer $token" "$BASE/units/$UNIT/restart")
if [ -n "$RESTART_UNIT" ] && [ "$RESTART_UNIT" = "$UNIT" ]; then
	check "opt-in gate (skipped: $UNIT is the restart target)" "pass"
elif [ "$code" = "403" ]; then
	check "restart of a unit that did not opt in is 403" "pass"
else
	check "restart of a unit that did not opt in is 403" "got HTTP $code"
fi

# --- the mutating path, only when asked -------------------------------------

if [ -n "$RESTART_UNIT" ]; then
	echo
	echo "--- restart proof ($RESTART_UNIT) ---"

	before=$(systemctl show -p NRestarts --value "$RESTART_UNIT" 2>/dev/null)
	resp=$(curl -s -X POST -H "Authorization: Bearer $token" \
		"$BASE/units/$RESTART_UNIT/restart")

	if printf '%s' "$resp" | grep -q '"result":"ok"'; then
		check "POST restart returned result ok" "pass"
	else
		check "POST restart returned result ok" "body was: $resp"
	fi

	# The ledger must hold the requested line AND the outcome line, in that
	# order -- written before the backend was touched, per SPEC feature 6.
	if [ -f "$ledger" ]; then
		lines=$(grep -c . "$ledger")
		first=$(grep . "$ledger" | head -1)
		if [ "$lines" -ge 2 ] && printf '%s' "$first" | grep -q '"result":"requested"'; then
			check "ledger recorded requested-then-outcome ($lines lines)" "pass"
		else
			check "ledger recorded requested-then-outcome" "$lines line(s), first: ${first:-<none>}"
		fi
	else
		check "ledger recorded requested-then-outcome" "no ledger file at $ledger"
	fi

	sleep 2
	after=$(systemctl show -p NRestarts --value "$RESTART_UNIT" 2>/dev/null)
	if [ -n "$before" ] && [ -n "$after" ] && [ "$after" != "$before" ]; then
		check "systemd restart counter moved ($before -> $after)" "pass"
	else
		check "systemd restart counter moved" "NRestarts stayed at ${before:-?} (some unit types do not count manual restarts)"
	fi

	# And the journal should now show systemd stopping/starting it.
	if journalctl -u "$RESTART_UNIT" -n 20 --no-pager 2>/dev/null | grep -qiE 'stopp|start'; then
		check "journal shows the restart" "pass"
	else
		check "journal shows the restart" "no stop/start lines (journal access may be restricted)"
	fi
fi

# --- verdict ---------------------------------------------------------------

echo
echo "--- structured logs (stderr sample) ---"
head -5 "$logfile"

echo
echo "=== verdict ==="
printf '  %d passed, %d failed\n' "$pass" "$fail"
printf '  host:  %s\n' "$(systemctl --version | head -1)"
printf '  unit:  %s\n' "$UNIT"
if [ "$fail" -ne 0 ]; then
	echo
	echo "  live-check: FAILED. The real systemd backend is NOT proved."
	echo "  Do not claim it works in the README, STATUS.md or PROCESS.md."
	exit 1
fi

echo
echo "  live-check: all clear on real systemd."
echo
echo "  This output is the only evidence that permits a real-systemd claim."
echo "  Paste it into STATUS.md with the date and the systemd version above"
echo "  if you want that claim written down."
exit 0
