#!/usr/bin/env bash
#
# Pins the explicit `— closes SC<n>` rule for goal `# Tasks` entries to the
# artifacts that carry it.
#
# Like scripts/necessity-rule-test.sh, the rule has no code: it lives in the
# prose goal-auditor reads, so a silent edit to that prose is a behaviour
# regression only a presence test can catch. The discriminating facts are the
# reference form, the cutoff constant, and the two failure cases (no reference;
# undefined SC). The old `(→ SC<n>)` context form must be gone from docs/,
# agents/ and commands/ — the guidance a goal author copies — or they write an
# entry the auditor rejects. CHANGELOG.md is deliberately outside that scope: it
# quotes the old form as history.
#
# Among the carriers, the cutoff DATE lives in agents/goal-auditor.md alone; docs/goal-writing.md
# names the constant only, so the two carriers cannot disagree on the boundary.
#
# Two self-checks, run on throwaway copies: strip the cutoff from ONE carrier
# (docs/goal-writing.md) and require exactly it reported; then strip `closes SC`
# from the other and require has() to say no — so neither the carrier scan nor
# the presence predicate can pass vacuously.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 2

CARRIERS=(
	docs/goal-writing.md
	agents/goal-auditor.md
)

# Writers of goal `# Tasks` lines must emit the required form, or every goal
# they create is flagged by the auditor that enforces it.
WRITERS=(
	agents/goal-creator.md
	commands/launch-goal.md
)

for d in docs agents commands; do
	[ -d "$d" ] || { echo "❌ expected directory missing: $d" >&2; exit 1; }
done

pass=0
fail=0

check() { # check <label> <want> <got>
	if [ "$2" = "$3" ]; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ $1: want [$2], got [$3]" >&2
	fi
}

has() { grep -qF -- "$2" "$1" && echo yes || echo no; }

missing_cutoff() { # missing_cutoff <dir> -> prints carriers lacking the constant
	local f
	for f in "${CARRIERS[@]}"; do
		grep -qF 'CLOSES_SC_REQUIRED_AS_OF' "$1/$f" || echo "$f"
	done
}

for f in "${CARRIERS[@]}"; do
	check "$f names closes SC<n>" yes "$(has "$f" 'closes SC')"
	check "$f names CLOSES_SC_REQUIRED_AS_OF" yes "$(has "$f" 'CLOSES_SC_REQUIRED_AS_OF')"
done

check "goal-auditor flags missing reference" yes "$(has agents/goal-auditor.md 'names no Success Criterion')"
check "goal-auditor flags undefined SC" yes "$(has agents/goal-auditor.md 'which this goal does not define')"
check "goal-auditor rejects topical inference" yes "$(has agents/goal-auditor.md 'Topical fit is not a reference')"
check "goal-auditor requires the em-dash form" yes "$(has agents/goal-auditor.md '`— closes SC<n>`')"
check "goal-auditor grandfathers to WARN" yes "$(has agents/goal-auditor.md 'or `created` absent → WARN, not MAJOR')"
check "goal-auditor exempts completed/aborted" yes "$(has agents/goal-auditor.md 'Goals with `status: completed` or `aborted` are never flagged')"
check "goal-auditor exempts struck rows" yes "$(has agents/goal-auditor.md 'Struck rows (`- [ ] ~~[[Name]]~~`) are exempt')"
check "goal-auditor forbids double report" yes "$(has agents/goal-auditor.md 'is not re-reported as an orphan')"

for w in "${WRITERS[@]}"; do
	check "$w writes closes SC<n>" yes "$(has "$w" 'closes SC')"
done

# Single source for the cutoff date: exactly one carrier holds it.
date_carriers=$(grep -lE 'CLOSES_SC_REQUIRED_AS_OF.{0,60}2026-10-08|2026-10-08.{0,60}CLOSES_SC_REQUIRED_AS_OF' "${CARRIERS[@]}" | tr '\n' ' ')
check "cutoff date held by goal-auditor alone" "agents/goal-auditor.md " "$date_carriers"

old_form=$(grep -rlF -- '(→ SC' docs agents commands | tr '\n' ' ')
check "old (→ SC<n>) form absent" "" "$old_form"

# Self-check: strip the cutoff from exactly one carrier in a copy.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for f in "${CARRIERS[@]}"; do
	mkdir -p "$tmp/$(dirname "$f")"
	cp "$f" "$tmp/$f"
done
sed -i.bak 's/CLOSES_SC_REQUIRED_AS_OF/REMOVED/g' "$tmp/docs/goal-writing.md"
check "self-check reports exactly the stripped carrier" "docs/goal-writing.md" "$(missing_cutoff "$tmp")"
sed -i.bak 's/closes SC/REMOVED/g' "$tmp/agents/goal-auditor.md"
check "self-check: has() reports a stripped phrase absent" no "$(has "$tmp/agents/goal-auditor.md" 'closes SC')"

echo "closes-sc-rule-test: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
