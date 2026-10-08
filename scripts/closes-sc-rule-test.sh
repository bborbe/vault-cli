#!/usr/bin/env bash
#
# Pins the explicit `— closes SC<n>` rule for goal `# Tasks` entries to the
# artifacts that carry it.
#
# Like scripts/necessity-rule-test.sh, the rule has no code: it lives in the
# prose goal-auditor reads, so a silent edit to that prose is a behaviour
# regression only a presence test can catch. The discriminating facts are the
# reference form, the cutoff constant, and the two failure cases (no reference;
# undefined SC). The old `(→ SC<n>)` context form must be gone everywhere, or
# a goal author copying it writes an entry the auditor rejects.
#
# The self-check strips the cutoff from ONE artifact in a throwaway copy and
# requires exactly that artifact to be reported.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 2

CARRIERS=(
	docs/goal-writing.md
	agents/goal-auditor.md
)

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

old_form=$(grep -rlF -- '(→ SC' docs agents commands 2>/dev/null | tr '\n' ' ')
check "old (→ SC<n>) form absent" "" "$old_form"

# Self-check: strip the cutoff from exactly one carrier in a copy.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for f in "${CARRIERS[@]}"; do
	mkdir -p "$tmp/$(dirname "$f")"
	cp "$f" "$tmp/$f"
done
sed -i.bak 's/CLOSES_SC_REQUIRED_AS_OF/REMOVED/g' "$tmp/agents/goal-auditor.md"
check "self-check reports exactly the stripped carrier" "agents/goal-auditor.md" "$(missing_cutoff "$tmp")"

echo "closes-sc-rule-test: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
