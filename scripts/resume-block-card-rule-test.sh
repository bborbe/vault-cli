#!/usr/bin/env bash
#
# Pins the resume-block card rule to its ONE home, and keeps it there.
#
# The rule has no code. It lives in the prose a session reads, so the prose IS the
# implementation and a silent edit to it is a behaviour regression — the same
# reasoning scripts/necessity-rule-test.sh and scripts/struck-row-rule-test.sh give
# for their own rules, and the reason this is a presence/absence test rather than a
# fixture: nothing else can catch the rule being dropped or re-copied.
#
# ⚠️ **The ABSENCE half is the one that pays, and it is why this exists.** The rule
# was declared single-homed in `commands/prepare-compact.md` and then restated in
# `commands/post-compact.md`, and the two homes disagreed on the poll vocabulary —
# so a session following the narrower one would conclude a gate had closed while it
# was still owed. Measured 2026-10-08. A presence test alone was green throughout
# that drift, because the home carried the rule the whole time.
#
# The self-checks at the end falsify each half against a throwaway copy: a check
# that reported everything, or nothing, would satisfy the assertions without
# measuring anything.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

HOME_FILE=commands/prepare-compact.md
CONSUMER_FILE=commands/post-compact.md

# The vocabulary that makes a resume-block entry answerable. Each is pinned by the
# literal a reader searches for; delete any one and the rule loses a half — the
# floor without the flag is an unbuildable post, and the flag without the poll
# vocabulary leaves a gate that reads as closed while it is still owed.
MARKERS=(
	'at least two `--option` labels'
	'--dedup-key'
	'NOT_OPERATOR_ANSWERED:'
)

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

# missing_markers <root> <file> — the markers the file does not carry.
missing_markers() {
	local root=$1 file=$2 m
	for m in "${MARKERS[@]}"; do
		grep -qF -- "$m" "$root/$file" 2>/dev/null || printf '%s\n' "$m"
	done
	return 0
}

# restated_markers <root> <file> — the markers the file carries that it must not.
restated_markers() {
	local root=$1 file=$2 m
	for m in "${MARKERS[@]}"; do
		grep -qF -- "$m" "$root/$file" 2>/dev/null && printf '%s\n' "$m"
	done
	return 0
}

# 1. The home carries every marker.
missing=$(missing_markers . "$HOME_FILE")
check "home carries every marker" "" "$missing"

# 2. The consumer carries none of them — the single-homing half.
restated=$(restated_markers . "$CONSUMER_FILE")
check "consumer restates none of them" "" "$restated"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/commands"

# 3. Self-check: strip one marker from a throwaway copy of the home and require
#    exactly that marker to be reported. The absence half (2) would otherwise be
#    satisfied by a consumer that had lost nothing, and the presence half (1) by a
#    home that had.
sed 's/at least two `--option` labels/REMOVED/' "$HOME_FILE" > "$TMP/$HOME_FILE"
check "self-check reports the stripped marker" \
	'at least two `--option` labels' "$(missing_markers "$TMP" "$HOME_FILE")"

# 4. Self-check: a consumer that DOES restate a marker must be reported. This is
#    the half the drift actually hit, so it carries its own falsification.
printf '%s\n' 'at least two `--option` labels' > "$TMP/$CONSUMER_FILE"
check "self-check reports a restating consumer" \
	'at least two `--option` labels' "$(restated_markers "$TMP" "$CONSUMER_FILE")"

echo "resume-block card rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
