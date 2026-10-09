#!/usr/bin/env bash
#
# Pins the struck-row exclusion to the artifacts that enforce it.
#
# A struck `# Tasks` row — `- [ ] ~~[[Task]]~~` — is the goal page's convention
# for a task deliberately removed from the tracked set: struck rather than
# deleted, so the two lists stop disagreeing without erasing the provenance. It
# is still a checkbox carrying a wikilink, so the literal token rule counts it as
# an UNMET subtask and the leading-`[[…]]` walk names it as the goal's next open
# task. Both are wrong, and both err in the direction that reports work where
# there is none. Measured 2026-09-22 on a goal whose `# Tasks` holds 16 checkbox
# rows (12 live + 4 struck): the literal rule scores 12/16 where the truth is
# 12/12, and the walk names the struck task.
#
# The rule has NO code. It lives in the prose that the counting agents and
# commands read, so the prose IS the implementation and a silent edit to it is a
# behaviour regression — which is why this test, rather than a fixture, is the
# regression check: nothing else can catch the rule being dropped.
#
# The pattern is the discriminating fact. An artifact that does not name the
# struck form cannot be excluding it, and every clause of the rule names it, so
# presence per artifact is both necessary and cheap to assert. The self-check at
# the end strips the pattern from ONE artifact in a throwaway copy and requires
# exactly that artifact to be reported: a check that reported everything, or
# nothing, would satisfy the presence assertions without measuring anything.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

# The artifacts that carry the rule, and what each enforces:
#   docs/output-formatting.md      § Counts + § Conditional segments — the spec
#   agents/goal-manager-agent.md   get_subtask_statuses — the subtask count
#   commands/goal-status.md        next-task resolution — the walk
#   commands/execute-goal.md       step 7 — the walk
#   commands/session-close.md      goal-anchored open-task resolution — the walk
ARTIFACTS=(
	docs/output-formatting.md
	agents/goal-manager-agent.md
	commands/goal-status.md
	commands/execute-goal.md
	commands/session-close.md
)

PATTERN='~~[['
SPEC=docs/output-formatting.md

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

# missing <root> — artifacts under <root> that do NOT name the struck pattern.
missing() {
	local root=$1 f
	for f in "${ARTIFACTS[@]}"; do
		grep -qF -- "$PATTERN" "$root/$f" 2>/dev/null || printf '%s\n' "$f"
	done
}

# section <file> <heading> — the lines under an exact heading, until the next
# heading. Exact whole-line match: a substring match would also open on a
# mention of the heading inside prose.
section() {
	awk -v h="$2" '$0 == h { f = 1; next } f && /^#/ { exit } f' "$1"
}

# --- the rule is present in every enforcing artifact
check "every artifact names the struck pattern" "" "$(missing "$ROOT")"

# The piped checks below read their input from a producer, so they use
# `grep -F … >/dev/null`, NOT `grep -qF`. Under `set -o pipefail`, `-q` exits at
# the first match and the producer's remaining write gets SIGPIPE (141), which
# fails the pipeline and reports a present pattern as missing — a race that hits
# only once a block exceeds the pipe buffer (PIPE_BUF, 4096 on Linux). Dropping
# `-q` keeps grep reading to EOF, so the check is deterministic. Do not
# "simplify" this back to `-q`.

# --- and, in the spec, in BOTH sections that state it. Presence anywhere in the
# file would pass on a clause that landed in only one of the two, which is
# exactly the half-fix this rule is prone to: the count and the walk are stated
# in different sections and drift apart.
for h in '### Counts' '### Conditional segments'; do
	check "spec § ${h#'### '} names the struck pattern" "yes" \
		"$(section "$SPEC" "$h" | grep -F -- "$PATTERN" >/dev/null && echo yes)"
done

# step <file> <heading> <number> — the lines under a numbered step, until the next
# numbered step or heading. Numbered steps are list items, not headings, so
# `section`'s exact-heading match does not reach them — and the numbering restarts
# inside every action, so the section is scoped first: an unscoped `^5. ` matches
# a Shared Operation's algorithm step long before the verify action's step 5.
step() {
	section "$1" "$2" |
		awk -v n="$3" '$0 ~ ("^" n "[.] ") { f = 1; next } f && (/^[0-9]+[.] / || /^#/) { exit } f'
}

# --- and in the goal agent, in EVERY site that enforces it. The file-level check
# above cannot tell "step 5 and step 8 both exclude struck rows" from "only
# get_subtask_statuses does" — the 2026-10-07 widening added the rule to two
# steps, and a later edit dropping it from one of them would otherwise pass green.
GOAL_AGENT=agents/goal-manager-agent.md
for n in 5 8; do
	check "goal-agent step $n names the struck pattern" "yes" \
		"$(step "$GOAL_AGENT" '### verify' "$n" | grep -F -- "$PATTERN" >/dev/null && echo yes)"
done
check "goal-agent get_subtask_statuses names the struck pattern" "yes" \
	"$(section "$GOAL_AGENT" '### get_subtask_statuses(goal_path)' | grep -F -- "$PATTERN" >/dev/null && echo yes)"

# --- self-check: strip the pattern from ONE artifact and require exactly that
# one to be reported.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
STRIPPED=commands/execute-goal.md
for f in "${ARTIFACTS[@]}"; do
	mkdir -p "$TMP/$(dirname "$f")"
	cp "$ROOT/$f" "$TMP/$f"
done
sed "s/~~\[\[/STRIPPED/g" "$ROOT/$STRIPPED" >"$TMP/$STRIPPED"
check "self-check: only the stripped artifact is reported" "$STRIPPED" "$(missing "$TMP")"

echo "struck-row-rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
