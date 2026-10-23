#!/usr/bin/env bash
#
# Pins the mode-membership invariant in `commands/session-close.md`.
#
# Phase 9 classifies a clean session into mode 1 (nothing fired) or mode 2
# (an advisory signal fired), and an unclean one into mode 3 (outstanding
# items). Three advisory signals can fire — reflect, self-improve, and a
# Phase 8.5 runbook gap — and each is named on FOUR surfaces: the mode-1
# header (as an exclusion), the mode-2 header, the mode-2 body list, and the
# mode-3 advisory tail.
#
# A fifth surface, the mode-2 closer panel, used to carry them as a `pick any —`
# operator menu. It no longer does: the session takes the reflect default
# itself (Phase 9.4) and exits, so the panel carries no options at all. Its
# membership obligation is therefore INVERTED into a pin that the menu stays
# gone — dropping it from the surface list alone would leave the removal
# unguarded, and a future edit could reintroduce the gate with every remaining
# check still green.
#
# The rule has NO code. It lives in that prose, so the prose IS the
# implementation and a silent edit to one surface is a behaviour regression.
# Measured 2026-10-11: adding the runbook gap as a mode-2 trigger left a
# surface stale across FIVE consecutive bot reviews, each time one surface
# further out — the mode-2 closer panel, then mode 1's header, then mode 3's
# advisory tail (which silently DROPPED the gap on any session reaching mode 3
# for an unrelated blocker), then a prose paragraph, then that same paragraph
# again asserting the opposite ROUTING. Every one was a real behavioural
# defect, and none was catchable by reading the diff.
#
# The class has TWO shapes, and they need two different assertions:
#
#   OMISSION — a surface that omits a signal cannot be routing it, so each
#     surface must NAME all three. Necessary, cheap, and NOT sufficient.
#
#   POLARITY — a surface can name a signal and still misstate what it does.
#     The fifth review's finding ("not what puts a session into mode 2") names
#     the runbook signal, so it passes every membership assertion while
#     contradicting the mode-1 header, the mode-2 header, Phase 8.5 and the
#     mode-2 closer rule. Pinned separately, below.
#
# The self-check at the end strips one signal from ONE surface in a throwaway
# copy and requires exactly that surface to be reported: a check that reported
# everything, or nothing, would satisfy the presence assertions without
# measuring anything. A second self-check restores each superseded wording and
# requires its pin to fire, so neither literal pin can be silently dead.
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

# The four membership surfaces, by the line prefix that opens each one.
#   mode-1 header      — the exclusion (what does NOT fire)
#   mode-2 header      — the admission
MODE1_HEADER='**1. Clean +'
MODE2_HEADER='**2. Clean +'
#   mode-2 panel       — the closer block. Its obligation is INVERTED: it must
#                        carry no operator menu at all. Pinned separately below.
MODE2_PANEL='**Clean verdict, mode 2'
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

# between <file> <start-prefix> <end-prefix> — the lines from the first line
# starting with <start-prefix> up to, but not including, the first following
# line starting with <end-prefix>. The start line itself is included.
between() {
	awk -v s="$2" -v e="$3" '
		!on && index($0, s) == 1 { on = 1 }
		on && index($0, e) == 1 { exit }
		on { print }
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
	"mode-2 body list|$(fenced "$TARGET" "$MODE2_BLOCK")" \
	"mode-3 advisory tail|$(fenced "$TARGET" "$MODE3_BLOCK")"; do
	label=${spec%%|*}
	text=${spec#*|}
	check "$label names all three signals" "" "$(omitted "$text")"
done

# --- the mode-2 SECTION, not only its fenced block. The round-4 finding lived
# in the prose paragraph under the list, which a fenced-block extractor cannot
# see: that paragraph still read "never the mode-3 one" after the mode-3
# advisory tail had been added, so the file contradicted itself while all five
# block/header surfaces agreed. Asserting the whole section closes the blind
# spot the per-surface checks left open.
check "mode-2 section names all three signals" "" \
	"$(omitted "$(between "$TARGET" "$MODE2_HEADER" "$MODE3_BLOCK")")"

# --- and no surface may re-state the superseded ban. "never the mode-3" was the
# exact wording of the reversed claim, so pinning the literal is the same shape
# as this script's sibling (struck-row-rule-test pins `~~[[`): a prose rule with
# no code has no other regression check. A future edit that wants to say this
# must say it as "never a mode-3 *blocker*" — which is true — and will trip this.
check "no superseded 'never the mode-3' exclusion remains" "" \
	"$(grep -F 'never the mode-3' "$TARGET" >/dev/null && echo found)"

# --- and no surface may DENY the runbook gap's mode-2 membership. The round-5
# finding asserted the item "is therefore not what puts a session into mode 2" —
# contradicted by the mode-1 header, the mode-2 header, Phase 8.5 and the
# mode-2 closer rule, every one of which the same PR had already updated. No
# membership assertion can see it: the paragraph NAMES the runbook signal, so it
# passes all five surface checks and the section check. Same shape as the pin
# above — a prose rule with no code has no other regression check — and the
# reason this script carries a polarity assertion at all rather than only a
# membership one.
check "no routing denial of the runbook gap's mode-2 membership remains" "" \
	"$(grep -E 'not what puts a session into mode 2|not a mode-2 trigger|does not put a session into mode 2' "$TARGET" >/dev/null && echo found)"

# --- mode 3 must SURFACE the runbook gap, not merely fail to be forced by it.
# "Does not force mode 3" and "is dropped in mode 3" are different claims; the
# 2026-10-11 regression was the second being read into the first, which lost
# the gap entirely on any session reaching mode 3 for an unrelated blocker.
check "mode-3 header does not read as an outright exclusion" "" \
	"$(line "$TARGET" "$MODE3_BLOCK" | grep -F 'not** Phase 8.5' >/dev/null && echo found)"

# --- the mode-2 closer must carry NO operator menu. The panel used to end on
# `pick any — 1. /vault-cli:reflect · 2. /coding:self-improve · 3. review the
# runbook gap · 4. nothing — close`. That menu minted an attention card, and
# `session-self-exit.py` refuses while a card is open for the session, so a
# finished worker held its pane open until a human answered — the gate this
# whole change removes. Membership is the WRONG assertion for this surface: the
# old menu named all three signals and was still the defect, so the pin is its
# inverse.
check "mode-2 closer carries no operator menu and no reply-waiting line" "" \
	"$(fenced "$TARGET" "$MODE2_PANEL" | grep -E '👤 You: pick|⏰ Next: your reply' >/dev/null && echo found)"

# --- and the self-exit gate must still admit modes 1 and 2 ONLY. Widening it to
# mode 3 would end an unclean session, which is the opposite failure from the one
# this PR fixes; the gate changed in this PR, so the invariant needs its own pin.
check "self-exit gate admits modes 1 and 2" "found" \
	"$(line "$TARGET" '**Modes 1 and 2.**' | grep -F 'Modes 1 and 2' >/dev/null && echo found)"
check "self-exit gate does not admit mode 3" "" \
	"$(between "$TARGET" '### Phase 9.5' '### Closer panel' | grep -E 'Modes 1, 2 and 3|Modes 1 and 2 and 3' >/dev/null && echo found)"

# --- Phase 9.4 must invoke reflect NON-INTERACTIVELY. `commands/reflect.md` asks
# two questions when run bare (which learnings, then `Proceed?`), so an
# interactive invocation replaces the removed gate with a worse one: it blocks
# before the exit this phase exists to reach, and mints the card that makes
# `session-self-exit.py` refuse. The flag is the whole fix, so it is pinned.
check "mode-2 default invokes reflect with --non-interactive" "found" \
	"$(between "$TARGET" '### Phase 9.4' '### Phase 9.5' | grep -F 'vault-cli:reflect --non-interactive' >/dev/null && echo found)"

# --- reflect's OWN NO-ASK contract must keep both halves: the prohibition on
# asking, and the parseable terminal line Phase 9.4 records. Pinning only that
# session-close passes the flag leaves the contract itself unguarded — a later
# edit could reintroduce an `AskUserQuestion` in reflect.md with every other
# check still green, which is the exact failure this PR removes.
REFLECT=commands/reflect.md
NOASK=$(between "$REFLECT" '## Non-interactive contract' '## Runtime context')
check "reflect NO-ASK contract forbids asking" "found" \
	"$(printf '%s' "$NOASK" | grep -F 'never call `AskUserQuestion`' >/dev/null && echo found)"
check "reflect NO-ASK contract defines the wrote terminal line" "found" \
	"$(printf '%s' "$NOASK" | grep -F 'reflect: wrote' >/dev/null && echo found)"
check "reflect NO-ASK contract defines the skipped terminal line" "found" \
	"$(printf '%s' "$NOASK" | grep -F 'reflect: skipped' >/dev/null && echo found)"

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

# --- self-check (polarity): RESTORE each superseded wording and require its pin
# to fire. The membership control above proves the extractors resolve; this one
# proves the two literal pins discriminate — a pin that can never fire is
# indistinguishable from a file that never regressed, which is the false green
# this script exists to prevent. Each restoration is a `sed` on the CURRENT
# wording, so a later edit that changes that wording turns the substitution into
# a no-op, the pin then finds nothing, and the control FAILS loudly instead of
# passing spuriously.
RESTORED="$TMP/restored/commands/session-close.md"
mkdir -p "$(dirname "$RESTORED")"
sed -e 's/never a mode-3 \*blocker\*/never the mode-3 one/' \
	-e 's/not what puts a session into mode 3/not what puts a session into mode 2 or mode 3/' \
	"$ROOT/$TARGET" >"$RESTORED"
check "self-check: the superseded mode-3 exclusion is detected when restored" "found" \
	"$(grep -F 'never the mode-3' "$RESTORED" >/dev/null && echo found)"
check "self-check: the superseded mode-2 routing denial is detected when restored" "found" \
	"$(grep -E 'not what puts a session into mode 2' "$RESTORED" >/dev/null && echo found)"

# --- self-check (menu): restore the retired menu into the mode-2 panel and
# require the inverse pin to fire. Same reason as the polarity controls above —
# a pin that can never fire is indistinguishable from a file that never
# regressed. The substitution targets the CURRENT `👤 You: nothing`, so a later
# edit to that line turns it into a no-op and this control fails loudly.
MENU_RESTORED="$TMP/menu/commands/session-close.md"
mkdir -p "$(dirname "$MENU_RESTORED")"
# Scoped by the mode-2 panel's OWN unique line, not by `👤 You: nothing` — that
# string also appears in the mode-1 and goal-anchored panels, so an unscoped
# substitution rewrites all three and proves nothing about this one.
sed 's/⏰ Next: this pane closes now (session-self-exit)/👤 You: pick any — 1. \/vault-cli:reflect/' \
	"$ROOT/$TARGET" >"$MENU_RESTORED"
check "self-check: the restored mode-2 menu is detected" "found" \
	"$(fenced "$MENU_RESTORED" "$MODE2_PANEL" | grep -E '👤 You: pick' >/dev/null && echo found)"

# The pin's second alternative needs its own control: a two-branch regex with one
# dead branch still passes the first control above, so restoring only the menu
# would leave `⏰ Next: your reply` unpinned in fact.
REPLY_RESTORED="$TMP/reply/commands/session-close.md"
mkdir -p "$(dirname "$REPLY_RESTORED")"
sed 's/⏰ Next: this pane closes now (session-self-exit)/⏰ Next: your reply/' \
	"$ROOT/$TARGET" >"$REPLY_RESTORED"
check "self-check: the restored reply-waiting line is detected" "found" \
	"$(fenced "$REPLY_RESTORED" "$MODE2_PANEL" | grep -E '⏰ Next: your reply' >/dev/null && echo found)"

echo "mode-membership-rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
