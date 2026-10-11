#!/usr/bin/env bash
#
# Pins the checkpoint-state rule in `commands/post-compact.md`, and pins the two
# things that made it necessary.
#
# The rule has no code. It lives in the prose a session reads, so the prose IS the
# implementation and a silent edit to it is a behaviour regression — the same
# reasoning scripts/resume-block-card-rule-test.sh gives for its own rule, and the
# reason this is a presence/absence test rather than a fixture.
#
# ⚠️ **Three literals, not two, and the third is the one an edit is likeliest to
# lose.** The no-op was a single line that read as clean for a consumed checkpoint,
# an absent one, and — after the early write landed — a partial one. Measured
# 2026-10-11: `prepare-compact` was interrupted, wrote nothing, and `post-compact`
# reported the same clean line a consumed checkpoint produces. Splitting it is the
# fix; re-folding it is the regression, and only a literal check sees that.
#
# ⚠️ **The DISCRIMINATOR half is the one that pays.** A first cut of the rule keyed
# the partial state on a missing `## Carry-over items` section — which is absent by
# design on any COMPLETE checkpoint with zero open items, so every routine clean run
# would have been reported as interrupted. The reliable discriminator is the resume
# block's `Live background:` line: the complete write always emits all four frozen
# labels, the minimal write omits it. A test pinning only the three literals would
# have been green throughout that false positive, which is why it is pinned here too.
#
# ⚠️ **The absence half is scoped to the PRODUCER, not the consumer.** The consumer
# deliberately quotes the superseded line while explaining the bug, so asserting its
# absence there would fail on correct text. What must not happen is the producer
# restating a state line verbatim — that dual-homing is what this guard catches, and
# it is the shape scripts/resume-block-card-rule-test.sh records going wrong once
# already between these same two files.
#
# The self-checks at the end falsify each half against a throwaway copy.
#
# ⚠️ **What this does NOT measure: a REWORDED state line.** The guard pins literals,
# so a state restated in other words passes. Pinning prose by literal is the
# trade-off that makes this cheap and greppable; this note is here so the header
# does not claim more than the check delivers.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

CONSUMER_FILE=commands/post-compact.md
PRODUCER_FILE=commands/prepare-compact.md

for f in "$CONSUMER_FILE" "$PRODUCER_FILE"; do
	[ -f "$f" ] || { echo "❌ $f is missing" >&2; exit 2; }
done

# The three state lines, each pinned by a distinctive prefix of the literal a reader
# searches for. Delete any one and a state silently re-folds into another.
# ⚠️ **MARKERS is this file's extension point, and self-check 5 uses each entry as a
# REGEX** (`awk gsub`), not as a literal — keep new entries regex-safe (no `&`, `\`
# or bracket syntax), or escape them there.
MARKERS=(
	'Nothing pending — checkpoint already consumed.'
	'No checkpoint exists — prepare-compact did not complete'
	'Partial checkpoint — prepare-compact was interrupted'
)

# The sentence naming the discriminator. Without it the partial branch is
# unfalsifiable — and keying on the carry-over section instead is the false positive
# described in the header.
DISCRIMINATOR="A partial checkpoint is discriminated by the resume block's \`Live background:\` line"

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
# Both helpers below separate 0 / 1 / 2+ rather than 0 / non-0 for that reason, and
# their polarities are named in their own first line so a call site cannot invert
# them by accident — which is exactly how this file's first draft failed.
GREP_NO_MATCH=1

# absent_lines <root> <file> — prints each pattern NOT found (the "must be present"
# half), or one UNREADABLE line when the file cannot be read.
absent_lines() {
	local root=$1 file=$2 pat rc
	for pat in "${MARKERS[@]}"; do
		grep -qF -- "$pat" "$root/$file" 2>/dev/null
		rc=$?
		case $rc in
			0) : ;;
			$GREP_NO_MATCH) printf '%s\n' "$pat" ;;
			*) printf 'UNREADABLE: %s\n' "$file"; return 0 ;;
		esac
	done
	return 0
}

# present_lines <root> <file> — prints each pattern FOUND (the "must be absent"
# half), or one UNREADABLE line when the file cannot be read.
present_lines() {
	local root=$1 file=$2 pat rc
	for pat in "${MARKERS[@]}"; do
		grep -qF -- "$pat" "$root/$file" 2>/dev/null
		rc=$?
		case $rc in
			0) printf '%s\n' "$pat" ;;
			$GREP_NO_MATCH) : ;;
			*) printf 'UNREADABLE: %s\n' "$file"; return 0 ;;
		esac
	done
	return 0
}

# 1. The consumer carries all three state lines.
check "consumer carries every state line" "" "$(absent_lines . "$CONSUMER_FILE")"

# 2. The three state lines occupy three DISTINCT lines of the consumer — the
#    anti-re-fold half. ⚠️ The first draft of this assertion iterated MARKERS and
#    compared its entries to each other, which is tautological: a constant defined in
#    this file can never disagree with itself, so it could not fail whatever the
#    consumer said and the guarantee rested entirely on assertion 1. This reads the
#    file, where a re-fold would actually show: two states on one line, or one gone.
state_lines=$(grep -cF -e "${MARKERS[0]}" -e "${MARKERS[1]}" -e "${MARKERS[2]}" \
	"$CONSUMER_FILE" 2>/dev/null)
check "state lines occupy three distinct lines" "3" "$state_lines"

# 3. The discriminator sentence is present.
grep -qF -- "$DISCRIMINATOR" "$CONSUMER_FILE" 2>/dev/null
check "consumer names the Live background discriminator" "0" "$?"

# 4. The producer restates none of the three state lines verbatim — single-homing.
check "producer restates no state line" "" "$(present_lines . "$PRODUCER_FILE")"

# 4b. The producer states the omission the consumer's discriminator depends on.
#     Without it a model can read the early write as "emit the resume block", emit all
#     four labels, and the partial file then reads as complete — the false negative
#     returning one layer down. Assertion 3 pins the consumer half; this pins the half
#     it depends on, which was unenforced when the discriminator first shipped.
PRODUCER_OMISSION="omit the resume block's \`Live background:\` label deliberately"
grep -qF -- "$PRODUCER_OMISSION" "$PRODUCER_FILE" 2>/dev/null
check "producer states the Live background omission" "0" "$?"

# `|| exit` for the same reason as the `cd` above: without `-e` a failed mktemp
# leaves TMP empty and the trap's `rm -rf ""` is a no-op.
TMP=$(mktemp -d) || exit 2
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/commands"

# 5. Self-check, over two literals: strip a state line from a throwaway copy and
#    require exactly that line to be reported. Two, not one, so a helper that
#    happened to detect a single hard-coded literal cannot satisfy assertion 1.
for marker in 'Nothing pending — checkpoint already consumed.' 'Partial checkpoint — prepare-compact was interrupted'; do
	awk -v m="$marker" '{ gsub(m, "REMOVED"); print }' "$CONSUMER_FILE" > "$TMP/$CONSUMER_FILE"
	check "self-check reports the stripped line" "$marker" \
		"$(absent_lines "$TMP" "$CONSUMER_FILE")"
done

# 6. Self-check: a producer that DOES restate a state line must be reported. This is
#    the dual-homing half, so it carries its own falsification.
printf '%s\n' 'Partial checkpoint — prepare-compact was interrupted' > "$TMP/$PRODUCER_FILE"
check "self-check reports a restating producer" \
	'Partial checkpoint — prepare-compact was interrupted' \
	"$(present_lines "$TMP" "$PRODUCER_FILE")"

# 7. Self-check: an UNREADABLE file must be reported, not read as clean. A directory
#    is used rather than a chmod-000 file because a chmod is bypassed when the suite
#    runs as root, and the check would then pass while exercising nothing.
check "self-check reports an unreadable file" \
	"UNREADABLE: commands" "$(absent_lines "$TMP" commands)"

echo "checkpoint state rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
