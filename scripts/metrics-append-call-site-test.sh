#!/usr/bin/env bash
#
# Pins the metrics-append call site to the artifact that performs it.
#
# A fleet-spawned worker records its session in the task's metrics only because
# § Session connect in `agents/work-on-task-assistant.md` invokes
# `vault-cli task append-metrics-session` in the branch that writes
# `claude_session_id`. The id write is what makes the task point at the session;
# the append is what makes that session visible to session-cost analytics. Drop
# the append and the id still lands — the run simply vanishes from the corpus,
# silently, which is exactly the defect the verb was added to repair.
#
# That step is a Claude Code agent definition: it has no test harness and runs
# only inside a real LLM in a real spawned worker behind
# `mcp__supervisor__spawn_agent`. An integration test can exercise the verb,
# never the call site firing. The prose IS the implementation, so an edit that
# drops the invocation is a behaviour regression no unit or integration test can
# see — which is why this test, rather than a fixture, is the regression check.
#
# The pattern is the discriminating fact: an artifact that does not name the
# invocation cannot be performing it. Presence alone is not enough, though — the
# invocation must be a step line in the branch that writes the id, so the checks
# below scope to § Session connect and bound the line between the two branch
# bullets. A passing mention elsewhere in the file cannot satisfy them.
#
# The self-check strips the invocation in a throwaway copy and re-invokes this
# script in `--check-only` mode against it, requiring a non-zero exit, then
# against the real tree requiring zero — so "the check can fail" is executed
# evidence rather than a claim.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

AGENT=agents/work-on-task-assistant.md
PATTERN='vault-cli task append-metrics-session'
SECTION='### Session connect (MANDATORY when Obsidian task file exists)'
ONE_BRANCH='If EXACTLY ONE UUID is returned'
ANY_BRANCH='If zero OR multiple UUIDs are returned'

# The artifacts that must carry the invocation. One entry today: § Session connect
# in the work-on-task agent definition is the only place a session-connect may
# invoke the verb. It stays a list so a second call site is one line away.
ARTIFACTS=("$AGENT")

# missing <root> — artifacts under <root> that do NOT name the invocation.
missing() {
	local root=$1 f
	for f in "${ARTIFACTS[@]}"; do
		grep -qF -- "$PATTERN" "$root/$f" 2>/dev/null || printf '%s\n' "$f"
	done
}

# section <file> <heading> — the lines under an exact heading, until the next
# heading. Exact whole-line match: a substring match would also open on a mention
# of the heading inside prose.
section() {
	awk -v h="$2" '$0 == h { f = 1; next } f && /^#/ { exit } f' "$1"
}

# lineOf <file> <fixed-string> — the first matching line number, empty when absent.
lineOf() {
	grep -nF -- "$2" "$1" 2>/dev/null | head -1 | cut -d: -f1
}

# --check-only <root>: the presence check alone, against <root>. Exits 0 when
# <root>'s artifact carries the invocation and 1 when it does not. The self-check
# below re-invokes this script in this mode; nothing else uses it.
if [ "${1:-}" = "--check-only" ]; then
	[ -z "$(missing "${2:?--check-only needs a root}")" ] || exit 1
	exit 0
fi

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

# --- 1. the invocation is present in every artifact
check "every artifact names the invocation" "" "$(missing "$ROOT")"

# --- 2. and it is inside § Session connect, section-scoped — not merely
# somewhere in the file, where a prose mention would satisfy it.
check "the invocation is inside § Session connect" "yes" \
	"$(section "$ROOT/$AGENT" "$SECTION" | grep -qF -- "$PATTERN" && echo yes)"

# --- 3. and it is an actionable step line, not a comment and not a fenced code
# block. Captured via lineOf + sed rather than by piping a `grep -nF` through
# `head -1` into a `grep -q` inside an `&&` condition, where `pipefail` can turn
# a present match into an absent one (the bug-4 shape in
# `scripts/daily-note-has-entry-test.sh`).
INV_NUM=$(lineOf "$ROOT/$AGENT" "$PATTERN")
INV_TEXT=""
[ -n "$INV_NUM" ] && INV_TEXT=$(sed -n "${INV_NUM}p" "$ROOT/$AGENT")
check "the invocation line is a bulleted step" "yes" \
	"$(printf '%s\n' "$INV_TEXT" | grep -qE '^[[:space:]]*- ' && echo yes)"

# --- 4. and it sits in the branch that writes the session id: strictly between
# the two branch bullets, so it is ordered after the id write and is not in the
# branch where the id is deliberately left unwritten.
ONE=$(lineOf "$ROOT/$AGENT" "$ONE_BRANCH")
ANY=$(lineOf "$ROOT/$AGENT" "$ANY_BRANCH")
check "the invocation is after the EXACTLY ONE bullet" "yes" \
	"$([ -n "$ONE" ] && [ -n "$INV_NUM" ] && [ "$INV_NUM" -gt "$ONE" ] && echo yes)"
check "the invocation is before the zero OR multiple bullet" "yes" \
	"$([ -n "$ANY" ] && [ -n "$INV_NUM" ] && [ "$INV_NUM" -lt "$ANY" ] && echo yes)"

# --- 5. and the section still states the non-fatal warning: a non-zero exit is
# reported as a warning naming metrics and the reason, with the id write left in
# place. Without it the step reads as mandatory and a vault error becomes a
# failure the worker must roll back.
check "the section states the warning" "yes" \
	"$(section "$ROOT/$AGENT" "$SECTION" | grep -qE 'warning.*metrics|metrics.*warning' && echo yes)"

# --- self-check: strip the invocation from ONE artifact in a throwaway copy and
# require exactly that artifact to be reported. A check that reported everything,
# or nothing, would satisfy the presence assertions without measuring anything.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/$(dirname "$AGENT")"
sed "s/$PATTERN/STRIPPED/" "$ROOT/$AGENT" >"$TMP/$AGENT"
check "self-check: only the stripped artifact is reported" "$AGENT" "$(missing "$TMP")"

# The non-zero direction: the same script, against a tree whose invocation is gone.
if bash "$ROOT/scripts/metrics-append-call-site-test.sh" --check-only "$TMP" >/dev/null 2>&1; then
	fail=$((fail + 1))
	echo "❌ self-check: --check-only exited 0 on a tree without the invocation" >&2
else
	pass=$((pass + 1))
fi

# The zero direction: the same script, against the real tree.
if bash "$ROOT/scripts/metrics-append-call-site-test.sh" --check-only "$ROOT" >/dev/null 2>&1; then
	pass=$((pass + 1))
else
	fail=$((fail + 1))
	echo "❌ self-check: --check-only exited non-zero on the real tree" >&2
fi

echo "metrics-append-call-site: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
