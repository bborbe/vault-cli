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
# Structural, not a literal blacklist: a reintroduced gate worded `👤 You: choose 1/2/3`
# or `⏰ Next: waiting for you` would pass any fixed string list. Pin the SHAPE of the
# panel instead — its answer slot is exactly `nothing`, and its next line names the exit.
PANEL=$(fenced "$TARGET" "$MODE2_PANEL")
check "mode-2 panel's 👤 You line is exactly nothing" "👤 You: nothing" \
	"$(printf '%s\n' "$PANEL" | grep -E '^👤 You:')"
check "mode-2 panel's ⏰ Next line names the self-exit" "found" \
	"$(printf '%s\n' "$PANEL" | grep -E '^⏰ Next:' | grep -F 'session-self-exit' >/dev/null && echo found)"

# --- and the self-exit gate must still admit modes 1 and 2 ONLY. Widening it to
# mode 3 would end an unclean session, which is the opposite failure from the one
# this PR fixes; the gate changed in this PR, so the invariant needs its own pin.
check "self-exit gate admits modes 1 and 2" "found" \
	"$(line "$TARGET" '**Modes 1 and 2.**' | grep -F 'Modes 1 and 2' >/dev/null && echo found)"
check "self-exit gate does not admit mode 3" "" \
	"$(between "$TARGET" '### Phase 9.5' '### Closer panel' | grep -E 'Modes 1, 2 and 3|Modes 1 and 2 and 3|all modes' >/dev/null && echo found)"

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

# --- self-check (menu): put the retired menu back into the mode-2 panel's ANSWER
# SLOT and require the pin's OWN expression to reject it. The earlier version
# injected a second `👤 You:` line and grepped for the injected text, which proved
# only that the sed fired — a pin weakened to `grep -E '^👤 You: nothing$'` would
# still have passed. This control runs the pin itself and asserts a mismatch.
menu_into_panel() { # menu_into_panel <src> <dst>
	awk '
		prev ~ /^👤 You: nothing$/ && $0 ~ /session-self-exit/ {
			print "👤 You: pick any — 1. /vault-cli:reflect"; print; prev = ""; next
		}
		prev != "" { print prev }
		{ prev = $0 }
		END { if (prev != "") print prev }
	' "$1" >"$2"
}

MENU_RESTORED="$TMP/menu/commands/session-close.md"
mkdir -p "$(dirname "$MENU_RESTORED")"
menu_into_panel "$ROOT/$TARGET" "$MENU_RESTORED"
check "self-check: the pin rejects a restored menu in the answer slot" "not-nothing" \
	"$(printf '%s\n' "$(fenced "$MENU_RESTORED" "$MODE2_PANEL")" | grep -E '^👤 You:' \
		| grep -qxF '👤 You: nothing' && echo nothing || echo not-nothing)"

# The next-line pin needs its own control for the same reason: restore a reply-
# waiting line and require the pin's expression to find no `session-self-exit`.
REPLY_RESTORED="$TMP/reply/commands/session-close.md"
mkdir -p "$(dirname "$REPLY_RESTORED")"
sed 's/⏰ Next: this pane closes now (session-self-exit)/⏰ Next: your reply/' \
	"$ROOT/$TARGET" >"$REPLY_RESTORED"
check "self-check: the pin rejects a restored reply-waiting line" "" \
	"$(printf '%s\n' "$(fenced "$REPLY_RESTORED" "$MODE2_PANEL")" | grep -E '^⏰ Next:' \
		| grep -F 'session-self-exit' >/dev/null && echo found)"

# --- Phase 9.4's DECISION RULE: reflect runs in mode 2 ONLY, and ONLY when BOTH
# (a) the Phase 8 reflect signal fired (score >= 3) and (b) the session recorded
# a finding that names no existing KB page. Every check above pins a CONSEQUENCE
# of that rule — the panel's shape, the flag, the NO-ASK contract — and none
# pinned the rule itself, so a later edit could drop a clause, or widen the
# scope to mode 1 or mode 3, with all of them still green. The rule IS the
# change this file's sibling PR is built on, so it gets its own pin, and each of
# its three conjuncts gets a mutation control below.
PHASE94=$(between "$TARGET" '### Phase 9.4' '### Phase 9.5')
CLAUSE_A='the Phase 8 reflect signal fired'
CLAUSE_B='a finding that names no existing KB page'

check "phase 9.4 is scoped to mode 2 only" "found" \
	"$(printf '%s' "$PHASE94" | grep -F 'mode 2 only' >/dev/null && echo found)"
check "phase 9.4 requires the Phase 8 reflect signal (clause a)" "found" \
	"$(printf '%s' "$PHASE94" | grep -F "$CLAUSE_A" >/dev/null && echo found)"
check "phase 9.4 requires a finding naming no existing KB page (clause b)" "found" \
	"$(printf '%s' "$PHASE94" | grep -F "$CLAUSE_B" >/dev/null && echo found)"
check "phase 9.4 states the rule as a conjunction, not a disjunction" "found" \
	"$(printf '%s' "$PHASE94" | grep -F 'if and only if BOTH hold' >/dev/null && echo found)"

# --- self-check (phase 9.4 rule): remove each conjunct in turn and require ITS
# OWN pin to fail. A pin that survives the removal of its clause is not pinning
# that clause — it is pinning something adjacent that happened to remain, which
# is the false green this whole file exists to prevent.
rule_mutant() { # rule_mutant <sed-expr> <name>
	local dest="$TMP/rule-$2/commands/session-close.md"
	mkdir -p "$(dirname "$dest")"
	sed "$1" "$ROOT/$TARGET" >"$dest"
	printf '%s' "$dest"
}

SCOPE_MUTANT=$(rule_mutant 's/(mode 2 only)/(modes 1 and 2)/' scope)
check "self-check: widening phase 9.4 past mode 2 breaks the scope pin" "" \
	"$(between "$SCOPE_MUTANT" '### Phase 9.4' '### Phase 9.5' | grep -F 'mode 2 only' >/dev/null && echo found)"

CLAUSE_A_MUTANT=$(rule_mutant 's/the Phase 8 reflect signal fired/the Phase 8 signal is advisory/' a)
check "self-check: dropping clause (a) breaks the clause-a pin" "" \
	"$(between "$CLAUSE_A_MUTANT" '### Phase 9.4' '### Phase 9.5' | grep -F "$CLAUSE_A" >/dev/null && echo found)"

CLAUSE_B_MUTANT=$(rule_mutant 's/a finding that names no existing KB page/a finding/' b)
check "self-check: dropping clause (b) breaks the clause-b pin" "" \
	"$(between "$CLAUSE_B_MUTANT" '### Phase 9.4' '### Phase 9.5' | grep -F "$CLAUSE_B" >/dev/null && echo found)"

# --- the mixed written/skipped terminal line. The Sensitivity bullet promised a
# skip would be "said in the terminal line", while the Terminal-line bullet
# defined exactly one line per run — so a mixed run could only print `wrote N`
# and the skip was silently lost. Pinned in BOTH files, each against its own
# section range: a whole-file grep passes on the literal pasted into the wrong
# section, which is the drift being guarded against.
MIXED='reflect: wrote <N> page(s): [[<a>]]; skipped <M> — sensitivity'
check "reflect.md defines the mixed written/skipped line" "found" \
	"$(between "$REFLECT" '## Non-interactive contract' '## Runtime context' | grep -F "$MIXED" >/dev/null && echo found)"
check "session-close phase 9.4 records the mixed form" "found" \
	"$(between "$TARGET" '### Phase 9.4' '### Phase 9.5' | grep -F 'skipped <M> (sensitivity)' >/dev/null && echo found)"

# --- and the changelog bullet must enumerate EVERY terminal-line form. The
# released v0.181.0 bullet named only `wrote` and `skipped`, which is how
# `failed` — mandatory in reflect.md — went missing from the changelog in the
# first place. Scoped to `## Unreleased` so a later release rewrite cannot make
# this pass by accident off an older section.
check "changelog unreleased names the failed terminal line" "found" \
	"$(between CHANGELOG.md '## Unreleased' '## v' | grep -F 'reflect: failed — <reason>' >/dev/null && echo found)"

# --- the script and its prose fallback must not drift. reflect.md keeps a class
# list for the case the plugin cache is unreachable; a class the script
# implements but the prose never names is one the model will not apply when the
# script cannot be found — and nothing else would notice. The list is DERIVED
# from the script's own `--classes` output, never restated here, so adding a
# class to the script immediately fails until the prose names it too.
# The list is captured FIRST, and its length asserted, because the `while read`
# form fails OPEN: if `--classes` errors or the script is gone, the loop body
# never executes, zero checks run, and the script reports a clean run having
# pinned nothing. A missing script must fail the pin, not retire it.
CLASSES=$(bash "$ROOT/scripts/reflect-secret-scan.sh" --classes 2>/dev/null)
check "the secret-scan script publishes its class list" "11" \
	"$(printf '%s\n' "$CLASSES" | grep -c .)"
# Scoped to the fallback paragraph, not the whole file: a class named anywhere
# in reflect.md satisfies a whole-file grep, but the list the model actually
# applies when the script is unreachable is only the one in that paragraph. A
# class moved out of it must fail here rather than pass silently.
FALLBACK=$(awk '/Fallback when the script is unreachable/{f=1} f && /Those eleven class names/{exit} f' "$REFLECT")
while IFS= read -r class; do
	[ -n "$class" ] || continue
	check "reflect.md names the '$class' class in the fallback list" "found" \
		"$(printf '%s' "$FALLBACK" | grep -F "\`$class\`" >/dev/null && echo found)"
done <<<"$CLASSES"

# --- the two session-close prose collisions. Both were live contradictions, not
# style nits: the goal-anchored clean verdict mandates one `⏰ Next:` line and
# the refused-exit carve-out mandates a different one with no stated precedence,
# and the mode-3 advisory tail called all three signals "never auto-run" while
# Phase 9.4 auto-runs reflect in mode 2.
check "refused-exit carve-out states it wins over the goal-anchored line" "found" \
	"$(grep -F 'This carve-out wins over the goal-anchored' "$TARGET" >/dev/null && echo found)"
check "the goal-anchored refused-exit combined line is given" "found" \
	"$(between "$TARGET" '**Goal-anchored clean verdict**' '**Outstanding items (mode 3):**' | grep -F 'the exit was refused (<reason>); the pane stays open' >/dev/null && echo found)"
check "mode-3 advisory tail is scoped to a mode-3 session" "found" \
	"$(grep -F 'in a mode-3 session none of the three is auto-run' "$TARGET" >/dev/null && echo found)"

ADVISORY_MUTANT=$(rule_mutant 's/in a mode-3 session none of the three is auto-run/all three are advisory/' advisory)
check "self-check: the unscoped advisory claim is detected when restored" "" \
	"$(grep -F 'in a mode-3 session none of the three is auto-run' "$ADVISORY_MUTANT" >/dev/null && echo found)"

echo "mode-membership-rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
