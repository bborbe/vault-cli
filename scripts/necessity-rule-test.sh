#!/usr/bin/env bash
#
# Pins the two-source necessity rule to the artifacts that enforce it.
#
# The rule has NO code. It lives in the prose the counting agents read, so the
# prose IS the implementation and a silent edit to it is a behaviour regression —
# the same reasoning scripts/struck-row-rule-test.sh gives for its own rule, and
# the reason this is a presence test rather than a fixture: nothing else can catch
# the rule being dropped.
#
# The discriminating facts are the two source names and the four-term
# reconciliation. An artifact naming fewer than two sources cannot be testing
# both, and one that does not state `serving + none + unproven + skipped =
# linked` cannot be reconciling its counters — so presence per artifact is both
# necessary and cheap to assert.
#
# The self-check at the end strips the reconciliation from ONE artifact in a
# throwaway copy and requires exactly that artifact to be reported: a check that
# reported everything, or nothing, would satisfy the presence assertions without
# measuring anything.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

# The two agents carry the rule in full: both source citations and the
# reconciliation that makes the counters checkable.
AGENTS=(
	agents/goal-manager-agent.md
	agents/task-manager-agent.md
)

# The two thin commands name the sources in prose. They carry no counters —
# deliberately, per agent-cmd/command-thin — so they are asserted on the source
# names a reader would search for, plus the exclusion of the retired one.
COMMANDS=(
	commands/verify-goal.md
	commands/verify-task.md
)

SOURCES=('SC<n>' 'DoD<n>')
RECONCILE='serving + none + unproven + skipped = linked'

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

# section <file> <heading> — the lines under an exact heading, until the next
# heading. Exact whole-line match: a substring match would also open on a
# mention of the heading inside prose.
section() {
	awk -v h="$2" '$0 == h { f = 1; next } f && /^#/ { exit } f' "$1"
}

# step <file> <heading> <number> — the lines under a numbered step, until the next
# numbered step or heading. Numbered steps are list items, not headings, so
# `section`'s exact-heading match does not reach them — and the numbering restarts
# inside every action, so the section is scoped first.
step() {
	section "$1" "$2" |
		awk -v n="$3" '$0 ~ ("^" n "[.] ") { f = 1; next } f && (/^[0-9]+[.] / || /^#/) { exit } f'
}

# The verify step that carries the rule, per agent.
RULE_STEP_GOAL=8
RULE_STEP_TASK=5

# missing_sources_in_step <root> — the agent/step pairs whose RULE STEP does not
# name both sources. ⚠️ **A per-file grep is not enough here, and that is the
# whole point of this helper:** both agents' Shared Operations prose names both
# sources on its own (`parse_success_criteria` says "cite `SC<n>`",
# `parse_definition_of_done` says "cite `DoD<n>`"), so deleting the rule step
# entirely would leave a file-level assertion green.
missing_sources_in_step() {
	local root=$1 f n s
	for spec in "agents/goal-manager-agent.md:$RULE_STEP_GOAL" "agents/task-manager-agent.md:$RULE_STEP_TASK"; do
		f=${spec%:*}
		n=${spec##*:}
		for s in "${SOURCES[@]}"; do
			step "$root/$f" '### verify' "$n" 2>/dev/null | grep -qF -- "$s" ||
				{ printf '%s step %s\n' "$f" "$n"; break; }
		done
	done
}

# missing_reconcile <root> — agent files that do NOT state the reconciliation.
missing_reconcile() {
	local root=$1 f
	for f in "${AGENTS[@]}"; do
		grep -qF -- "$RECONCILE" "$root/$f" 2>/dev/null || printf '%s\n' "$f"
	done
}

# --- the rule step in each agent names all three sources, and each agent states
# the reconciliation
check "the rule step in each agent names both serving sources" "" \
	"$(missing_sources_in_step "$ROOT")"
check "every agent states the four-term reconciliation" "" "$(missing_reconcile "$ROOT")"

# --- every thin command names the surviving sources a reader would search for,
# and none still presents the retired goal sentence as one. The negative half is
# the half that matters: a command that merely gained the new wording while
# keeping the old sentence clause would otherwise pass on presence alone.
for f in "${COMMANDS[@]}"; do
	for s in 'SC<n>' 'Definition of Done'; do
		check "$f names '$s'" "yes" \
			"$(grep -qF -- "$s" "$ROOT/$f" 2>/dev/null && echo yes)"
	done
	check "$f no longer names 'goal sentence'" "" \
		"$(grep -qF -- 'goal sentence' "$ROOT/$f" 2>/dev/null && printf '%s\n' "$f")"
done

# --- self-check: strip the reconciliation from ONE agent and require exactly that
# one to be reported.
TMP=$(mktemp -d)
TMP2=$(mktemp -d)
trap 'rm -rf "$TMP" "$TMP2"' EXIT
STRIPPED=agents/task-manager-agent.md
for f in "${AGENTS[@]}"; do
	mkdir -p "$TMP/$(dirname "$f")" "$TMP2/$(dirname "$f")"
	cp "$ROOT/$f" "$TMP/$f"
	cp "$ROOT/$f" "$TMP2/$f"
done
sed "s/$RECONCILE/STRIPPED/g" "$ROOT/$STRIPPED" >"$TMP/$STRIPPED"
check "self-check: only the stripped agent is reported" "$STRIPPED" "$(missing_reconcile "$TMP")"

# --- self-check 2: drop the RULE STEP from one agent and require it to be
# reported. This is the fixture a file-level check cannot have: it proves the
# per-step assertion measures the step, not the file — deleting step 8 leaves every
# source string present in that agent's Shared Operations prose.
# ⚠️ The drop must be scoped to the `### verify` section. Unscoped, `^8[.] ` opens on
# the *status* action's step 8 — the numbering restarts inside every action — and the
# drop then also swallows the `### verify` heading, so `step()` returns empty for every
# number and the assertion passes for a degenerate reason rather than the one it names.
awk '
	/^### verify$/ { inverify = 1 }
	inverify && /^#/ && !/^### verify$/ { inverify = 0 }
	inverify && /^8[.] / { skip = 1 }
	skip && /^[0-9]+[.] / && !/^8[.] / { skip = 0 }
	!skip
' "$ROOT/agents/goal-manager-agent.md" >"$TMP2/agents/goal-manager-agent.md"
check "self-check: a dropped rule step is reported" \
	"agents/goal-manager-agent.md step $RULE_STEP_GOAL" "$(missing_sources_in_step "$TMP2")"
# ...and the drop must be surgical: if the section heading went with it, the report
# above would name step 8 for the wrong reason. Step 5 still carrying the struck
# pattern proves the section survived the edit.
check "self-check: the drop left the rest of the verify section intact" "yes" \
	"$(step "$TMP2/agents/goal-manager-agent.md" '### verify' 5 | grep -qF -- '~~[[' && echo yes)"

echo "necessity-rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
