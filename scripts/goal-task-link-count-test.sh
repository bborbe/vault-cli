#!/usr/bin/env bash
#
# Tests for goal-task-link-count.sh.
#
# Each case pins a rule the counter commits to, so a regression fails here
# instead of silently shifting the headline. The rules that need pinning are the
# ones where the obvious implementation is wrong:
#
#   - terminal tasks excluded from the denominator
#   - a declaration whose goal FILE is absent counts as missing, subtotalled
#   - `|alias`, `#heading`, `dir/` wikilink forms all resolve to one title
#   - membership is the UNION of every `# Tasks` section (a goal in the real
#     corpus has two, and first-heading-only moves the count by 2)
#   - a wikilink outside `# Tasks` is a mention, not a declaration
#
# The last two are the ones a reader would not predict, and both were found by
# running against the real vault rather than by reading the code.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e` (it must survive
# non-zero exits from the script under test), so a failed cd would otherwise
# continue and run every case from the wrong directory.
cd "$ROOT" || exit 2

SCRIPT=scripts/goal-task-link-count.sh
FIX=scripts/testdata/goal-task-links

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

# num <output> <key> — the leading integer of a `<key>: <n> ...` line.
num() { printf '%s\n' "$1" | awk -F': *' -v k="$2" '$1 == k { print $2 + 0; exit }'; }

# absent <output> — the "goal file absent" subtotal.
absent() { printf '%s\n' "$1" | awk '/goal file absent:/ { print $NF; exit }'; }

# pergoal <output> <goal> — the missing count for one goal, or empty.
pergoal() { printf '%s\n' "$1" | grep -E "^ *[0-9]+ +$2\$" | head -1 | awk '{ print $1 }'; }

out=$(bash "$SCRIPT" "$FIX" --tasks-dir Tasks --goals-dir Goals 2>&1)
rc=$?
check "fixture exit 0" 0 "$rc"

# --- headline. The fixture holds 12 task files; 2 are terminal, leaving 10.
# `No Goals Task` declares nothing and `Two Goals Task` declares two, so 9
# contributing files yield 10 declarations.
# The two terminal files are LISTED in Alpha on purpose: if status filtering
# breaks they become linked, and the denominator moves to 12 — which is the
# signal this case exists to catch.
check "declarations" 10 "$(num "$out" declarations)"
check "linked" 5 "$(num "$out" linked)"
check "missing" 5 "$(num "$out" missing)"
check "goal-file-absent subtotal" 1 "$(absent "$out")"

# --- per-goal breakdown
check "per-goal Alpha" 3 "$(pergoal "$out" Alpha)"
check "per-goal Outside" 1 "$(pergoal "$out" Outside)"
check "per-goal No Such Goal" 1 "$(pergoal "$out" 'No Such Goal')"

# --- union of duplicated `# Tasks`: Dup Section Task is listed only under the
# SECOND heading. First-heading-only would move it to missing and print a Dup row.
check "dup # Tasks section -> no Dup miss" "" "$(pergoal "$out" Dup)"

# --- section scoping: Outside Section Task appears in Outside's `# Related`,
# never in its `# Tasks`, so it must be missing (already asserted via per-goal
# Outside == 1, which is exactly that one task).

# --- derived, not recited: the fixture's answer differs from the figure this
# script was written to reproduce, so a hardcoded 112/256 cannot pass.
check "fixture answer != 112" "yes" "$([ "$(num "$out" missing)" != 112 ] && echo yes || echo no)"
check "fixture total != 256" "yes" "$([ "$(num "$out" declarations)" != 256 ] && echo yes || echo no)"

# --- mutation: adding one unlisted declaration must move the count by exactly
# one. This is the strongest available proof that the numbers are measured —
# a hardcoded or fixture-blind implementation passes every case above and fails
# this one.
MUT=$(mktemp -d -t gtl-mut.XXXXXX)
trap 'rm -rf "$MUT"' EXIT
cp -R "$FIX/." "$MUT/"
cat >"$MUT/Tasks/Mutant Task.md" <<'EOF'
---
status: next
goals:
    - '[[Alpha]]'
---

body
EOF
mout=$(bash "$SCRIPT" "$MUT" --tasks-dir Tasks --goals-dir Goals 2>&1)
check "mutation: declarations +1" 11 "$(num "$mout" declarations)"
check "mutation: missing +1" 6 "$(num "$mout" missing)"
check "mutation: per-goal Alpha +1" 4 "$(pergoal "$mout" Alpha)"

# --- determinism: "run twice with the same result" is one of the properties the
# script exists to demonstrate, so it is asserted rather than assumed.
again=$(bash "$SCRIPT" "$FIX" --tasks-dir Tasks --goals-dir Goals 2>&1)
check "deterministic across runs" "$out" "$again"

# --- input errors are exit 2, distinct from "counted and non-zero"
bash "$SCRIPT" >/dev/null 2>&1
check "no args -> 2" 2 "$?"
bash "$SCRIPT" "$FIX" --tasks-dir Tasks --goals-dir Nope >/dev/null 2>&1
check "missing goals dir -> 2" 2 "$?"
bash "$SCRIPT" "$FIX" --bogus >/dev/null 2>&1
check "unknown option -> 2" 2 "$?"
bash "$SCRIPT" --help >/dev/null 2>&1
check "--help -> 0" 0 "$?"

echo "goal-task-link-count: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
