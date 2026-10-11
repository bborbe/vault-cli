#!/usr/bin/env bash
#
# Pins the mode-membership invariant in `commands/session-close.md`.
#
# Phase 9 classifies a clean session into mode 1 (nothing fired) or mode 2
# (an advisory signal fired), and an unclean one into mode 3 (outstanding
# items). Three advisory signals can fire — reflect, self-improve, and a
# Phase 8.5 runbook gap — and each is named on FIVE surfaces: the mode-1
# header (as an exclusion), the mode-2 header, the mode-2 body list, the
# mode-3 advisory tail, and the mode-2 closer panel.
#
# The rule has NO code. It lives in that prose, so the prose IS the
# implementation and a silent edit to one surface is a behaviour regression.
# Measured 2026-10-11: adding the runbook gap as a mode-2 trigger left three
# of the five surfaces stale across three consecutive bot reviews, each time
# one surface further out — the mode-2 closer panel, then mode 1's header,
# then mode 3's advisory tail (which silently DROPPED the gap on any session
# reaching mode 3 for an unrelated blocker). Every one was a real behavioural
# defect, and none was catchable by reading the diff.
#
# The discriminating fact is symmetric membership: a surface that omits a
# signal cannot be routing it. So the assertion is "each surface names all
# three", which is both necessary and cheap. The self-check at the end strips
# one signal from ONE surface in a throwaway copy and requires exactly that
# surface to be reported: a check that reported everything, or nothing, would
# satisfy the presence assertions without measuring anything.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

TARGET=commands/session-close.md

# The three advisory signals, as ERE patterns.
#
# The pattern must match the CONCEPT, not one spelling: each surface names a
# signal in its own vocabulary — the headers say `reflect score < 3` and
# `no Phase 8.5 runbook gap`, the body lists say `/vault-cli:reflect` and
# `Runbook improvements`, and the closer says `review the runbook gap`. Pinning
# a single literal per signal (the first draft did) reports every surface as
# omitting everything, which is a check that can only fail — the mirror of the
# false-green it exists to prevent. Observed 2026-10-11 on this script's first
# run: 3 passed, 5 failed, all five on vocabulary, none on a real gap.
SIGNALS=(
	'reflect'
	'self-improve'
	'runbook-improvement|runbook gap|Runbook improvements'
)

# The five surfaces, by the line prefix that opens each one.
#   mode-1 header      — the exclusion (what does NOT fire)
#   mode-2 header      — the admission
#   mode-2 closer      — the `pick` line, which must offer every fired signal
MODE1_HEADER='**1. Clean +'
MODE2_HEADER='**2. Clean +'
MODE2_CLOSER='👤 You: pick —'
#   mode-2 body / mode-3 tail — fenced blocks opened by their headers
MODE2_BLOCK='**2. Clean +'
MODE3_BLOCK='**3. Outstanding items**'

pass=0
fail=0

check() { # check <label> <want> <got>
	local label=$1 want=$2 got=$3
	if [ "$want" = "$got" ]; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ $label: want [$want], got [$got]" >&2
	fi
}

# line <file> <prefix> — the first line starting with <prefix>.
# `index(...) == 1` is a prefix test: a `~` match would also fire on a
# mention of the header inside prose.
line() {
	awk -v p="$2" 'index($0, p) == 1 { print; exit }' "$1"
}

# fenced <file> <line-prefix> — the body of the first ``` fenced block that
# follows the first line starting with <line-prefix>.
fenced() {
	awk -v p="$2" '
		!want && index($0, p) == 1 { want = 1; next }
		want && /^```/ { if (!inblk) { inblk = 1; next } else { exit } }
		inblk { print }
	' "$1"
}

# omitted <text> — the signals NOT named in <text>, one per line.
omitted() {
	local s
	for s in "${SIGNALS[@]}"; do
		printf '%s' "$1" | grep -E -- "$s" >/dev/null 2>&1 || printf '%s\n' "$s"
	done
}

# --- every surface names every signal
# The piped checks read from a producer, so they use `grep -F … >/dev/null`,
# NOT `grep -qF`: under `set -o pipefail` a `-q` exits at the first match and
# the producer's remaining write gets SIGPIPE (141), failing the pipeline and
# reporting a present signal as missing. Do not "simplify" this back to `-q`.
for spec in \
	"mode-1 header|$(line "$TARGET" "$MODE1_HEADER")" \
	"mode-2 header|$(line "$TARGET" "$MODE2_HEADER")" \
	"mode-2 closer|$(line "$TARGET" "$MODE2_CLOSER")" \
	"mode-2 body list|$(fenced "$TARGET" "$MODE2_BLOCK")" \
	"mode-3 advisory tail|$(fenced "$TARGET" "$MODE3_BLOCK")"; do
	label=${spec%%|*}
	text=${spec#*|}
	check "$label names all three signals" "" "$(omitted "$text")"
done

# --- mode 3 must SURFACE the runbook gap, not merely fail to be forced by it.
# "Does not force mode 3" and "is dropped in mode 3" are different claims; the
# 2026-10-11 regression was the second being read into the first, which lost
# the gap entirely on any session reaching mode 3 for an unrelated blocker.
check "mode-3 header does not read as an outright exclusion" "" \
	"$(line "$TARGET" "$MODE3_BLOCK" | grep -F 'not** Phase 8.5' >/dev/null && echo found)"

# --- self-check: remove ONE signal from ONE surface and require exactly that
# surface to report it. Deleting the mode-3 advisory tail's runbook line is the
# precise shape of the 2026-10-11 regression, so the self-check reproduces the
# defect it exists to catch rather than an arbitrary edit.
RUNBOOK='runbook-improvement|runbook gap|Runbook improvements'
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/$(dirname "$TARGET")"
sed '/^8\. Runbook improvements:/d' "$ROOT/$TARGET" >"$TMP/$TARGET"
check "self-check: the removed signal is reported on mode 3" "$RUNBOOK" \
	"$(omitted "$(fenced "$TMP/$TARGET" "$MODE3_BLOCK")" | grep -E "$RUNBOOK")"
check "self-check: the same signal is NOT reported on mode 2" "" \
	"$(omitted "$(fenced "$TMP/$TARGET" "$MODE2_BLOCK")" | grep -E "$RUNBOOK")"

echo "mode-membership-rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
