#!/usr/bin/env bash
#
# Pins the three-source necessity rule to the artifacts that enforce it.
#
# The rule has NO code. It lives in the prose the counting agents read, so the
# prose IS the implementation and a silent edit to it is a behaviour regression —
# the same reasoning scripts/struck-row-rule-test.sh gives for its own rule, and
# the reason this is a presence test rather than a fixture: nothing else can catch
# the rule being dropped.
#
# The discriminating facts are the three source names and the four-term
# reconciliation. An artifact naming fewer than three sources cannot be testing
# all three, and one that does not state `serving + none + unproven + skipped =
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

# The two agents carry the rule in full: all three source citations and the
# reconciliation that makes the counters checkable.
AGENTS=(
	agents/goal-manager-agent.md
	agents/task-manager-agent.md
)

# The two thin commands name the three sources in prose. They carry no `SC<n>` /
# `DoD<n>` citation form and no counters — deliberately, per agent-cmd/command-thin
# — so they are asserted only on the source names a reader would search for.
COMMANDS=(
	commands/verify-goal.md
	commands/verify-task.md
)

SOURCES=('goal sentence' 'SC<n>' 'DoD<n>')
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

# missing_sources <root> — agent files that do NOT name all three sources.
missing_sources() {
	local root=$1 f s
	for f in "${AGENTS[@]}"; do
		for s in "${SOURCES[@]}"; do
			grep -qF -- "$s" "$root/$f" 2>/dev/null || { printf '%s\n' "$f"; break; }
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

# --- every agent names all three sources, and states the reconciliation
check "every agent names all three serving sources" "" "$(missing_sources "$ROOT")"
check "every agent states the four-term reconciliation" "" "$(missing_reconcile "$ROOT")"

# --- every thin command names the two non-SC sources a reader would search for
for f in "${COMMANDS[@]}"; do
	for s in 'goal sentence' 'Definition of Done'; do
		check "$f names '$s'" "yes" \
			"$(grep -qF -- "$s" "$ROOT/$f" 2>/dev/null && echo yes)"
	done
done

# --- self-check: strip the reconciliation from ONE agent and require exactly that
# one to be reported.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
STRIPPED=agents/task-manager-agent.md
for f in "${AGENTS[@]}"; do
	mkdir -p "$TMP/$(dirname "$f")"
	cp "$ROOT/$f" "$TMP/$f"
done
sed "s/$RECONCILE/STRIPPED/g" "$ROOT/$STRIPPED" >"$TMP/$STRIPPED"
check "self-check: only the stripped agent is reported" "$STRIPPED" "$(missing_reconcile "$TMP")"

echo "necessity-rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
