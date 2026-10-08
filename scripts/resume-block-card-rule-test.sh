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
# ⚠️ **The markers are the behaviours, not just the vocabulary.** Every literal below
# is a half that was added or moved by the change that introduced this guard; a
# MARKERS list that pinned only the pre-existing literals would guard everything
# except the edit that made it necessary.
#
# The self-checks at the end falsify each half against a throwaway copy, over two
# different literals: a check that reported everything, or nothing, would satisfy
# the assertions without measuring anything.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

HOME_FILE=commands/prepare-compact.md
CONSUMER_FILE=commands/post-compact.md

# ⚠️ **A missing file must FAIL, never pass vacuously.** `restated_markers` sends
# grep's non-zero status to /dev/null and returns 0, so an absent or renamed
# consumer reports nothing — and the absence half, the load-bearing one, would
# compare "" to "" and pass without measuring anything. The home has an accidental
# guard (a missing home makes `missing_markers` report every literal, failing
# assertion 1); the consumer has none, so both get an explicit one.
for f in "$HOME_FILE" "$CONSUMER_FILE"; do
	[ -f "$f" ] || { echo "❌ $f is missing" >&2; exit 2; }
done

# The halves that make a resume-block entry answerable, each pinned by the literal a
# reader searches for. Delete any one and the rule loses a half: the floor without
# the flag is an unbuildable post; the flag without the poll vocabulary leaves a gate
# that reads as closed while it is still owed; the escape hatch without its
# carry-over scoping cannot fire, because the consumer reads that list back; and the
# marker's reading, if it lives only in the consumer, is the dual-homing this guard
# exists to catch.
MARKERS=(
	'at least two `--option` labels'
	'--dedup-key'
	'NOT_OPERATOR_ANSWERED:'
	'fold it into `Next action:`'
	'is not a `gate` carry-over item either'
	'reads as no item id'
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

# ⚠️ **grep's status is read, never collapsed.** `grep -q ... && printf` and
# `grep -q ... || printf` both treat every non-zero as "no match" — so an
# UNREADABLE file reports nothing and the half passes without measuring anything.
# That is the second time this class surfaced in this file (the first was a missing
# file), which is why both halves now separate 0 / 1 / 2+ rather than 0 / non-0.
GREP_NO_MATCH=1

# missing_markers <root> <file> — the markers the file does not carry, or a single
# UNREADABLE line when the file cannot be read at all.
missing_markers() {
	local root=$1 file=$2 m rc
	for m in "${MARKERS[@]}"; do
		grep -qF -- "$m" "$root/$file" 2>/dev/null
		rc=$?
		case $rc in
			0) : ;;
			$GREP_NO_MATCH) printf '%s\n' "$m" ;;
			*) printf 'UNREADABLE: %s\n' "$file"; return 0 ;;
		esac
	done
	return 0
}

# restated_markers <root> <file> — the markers the file carries that it must not,
# or a single UNREADABLE line when the file cannot be read at all.
restated_markers() {
	local root=$1 file=$2 m rc
	for m in "${MARKERS[@]}"; do
		grep -qF -- "$m" "$root/$file" 2>/dev/null
		rc=$?
		case $rc in
			0) printf '%s\n' "$m" ;;
			$GREP_NO_MATCH) : ;;
			*) printf 'UNREADABLE: %s\n' "$file"; return 0 ;;
		esac
	done
	return 0
}

# 1. The home carries every marker.
check "home carries every marker" "" "$(missing_markers . "$HOME_FILE")"

# 2. The consumer carries none of them — the single-homing half.
check "consumer restates none of them" "" "$(restated_markers . "$CONSUMER_FILE")"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/commands"

# 3. Self-check, over two literals: strip a marker from a throwaway copy of the home
#    and require exactly that marker to be reported. Two, not one, because a
#    `missing_markers` that happened to detect a single hard-coded literal would
#    otherwise satisfy assertion 1 without iterating MARKERS at all.
for marker in 'at least two `--option` labels' 'reads as no item id'; do
	awk -v m="$marker" '{ gsub(m, "REMOVED"); print }' "$HOME_FILE" > "$TMP/$HOME_FILE"
	check "self-check reports the stripped marker" "$marker" \
		"$(missing_markers "$TMP" "$HOME_FILE")"
done

# 4. Self-check: a consumer that DOES restate a marker must be reported. This is the
#    half the drift actually hit, so it carries its own falsification.
printf '%s\n' 'at least two `--option` labels' > "$TMP/$CONSUMER_FILE"
check "self-check reports a restating consumer" \
	'at least two `--option` labels' "$(restated_markers "$TMP" "$CONSUMER_FILE")"

# 5. Self-check: an UNREADABLE consumer must be reported, not read as clean. This is
#    the class's second surface — a missing file was the first — and grep's status is
#    the only thing that can tell "no match" from "could not read". A directory is
#    used rather than a chmod-000 file because a chmod is bypassed when the suite runs
#    as root, and the check would then pass while exercising nothing.
check "self-check reports an unreadable consumer" \
	"UNREADABLE: commands" "$(restated_markers "$TMP" commands)"

echo "resume-block card rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
