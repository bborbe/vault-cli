#!/usr/bin/env bash
#
# Pins the session-connect claim branch to the artifact that performs it.
#
# A task whose recorded `claude_session_id` names a session that has ENDED must
# be claimable by the session actually working it. The field is read as an
# *ownership* stamp by every consumer — the manager sweep's orphan predicate, the
# vault UI's session column, fleet-drive reap, fleet-verify check 3 — so a
# session-connect that declines unconditionally leaves a live worker's row
# rendering as stale-owned on every tick, and the session doing the work can
# never take the row over.
#
# The bound matters as much as the claim. An unconditional overwrite is the
# documented defect in `docs/work-on-session-lifecycle.md`: it launders a false
# ownership fact into every downstream artifact written while it was wrong, and
# it outlives the restore. Claiming an `indeterminate` owner — a detached
# one-turn spawn's fresh transcript, with no process — hard-locks the task
# against the session actually working it (observed 2026-09-11).
#
# Step 3 is a Claude Code agent definition: it has no test harness and runs only
# inside a real LLM behind `mcp__supervisor__spawn_agent`. An integration test
# can exercise the CLI verbs, never the branch firing. The prose IS the
# implementation, so an edit that drops the claim — or that widens it into an
# unconditional overwrite — is a behaviour regression no unit or integration test
# can see. Which is why this test, rather than a fixture, is the regression check.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

AGENT=agents/work-on-task-assistant.md
SECTION='### Session connect (MANDATORY when Obsidian task file exists)'
STEP3='If `claude_session_id` is **already set**'
CLAIM='✅ Session: claimed'
LIVENESS='docs/session-liveness.md'

# section <file> <heading> — the lines under an exact heading, until the next
# heading. Exact whole-line match: a substring match would also open on a mention
# of the heading inside prose.
section() {
	awk -v h="$2" '$0 == h { f = 1; next } f && /^#/ { exit } f' "$1"
}

# --check-only <root>: the claim-branch check alone, against <root>. Exits 0 when
# <root>'s artifact carries the claim branch and 1 when it does not. The
# self-check below re-invokes this script in this mode; nothing else uses it.
if [ "${1:-}" = "--check-only" ]; then
	[ -n "$(section "${2:?--check-only needs a root}/$AGENT" "$SECTION" | grep -F -- "$CLAIM")" ] || exit 1
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

SEC=$(section "$ROOT/$AGENT" "$SECTION")

# --- 1. the claim branch exists, and inside § Session connect — section-scoped,
# so a prose mention elsewhere in the file cannot satisfy it.
check "§ Session connect carries the claim branch" "yes" \
	"$(printf '%s\n' "$SEC" | grep -qF -- "$CLAIM" && echo yes)"

# --- 2. step 3 itself names the liveness rule's home rather than restating it
# inline. Scoped to step 3's own line: the detection block above it already cites
# the same doc, so a section-wide match would pass on an unedited step 3.
STEP3_LINE=$(printf '%s\n' "$SEC" | grep -F -- "$STEP3" | head -1)
check "step 3 reads the rule from docs/session-liveness.md" "yes" \
	"$(printf '%s\n' "$STEP3_LINE" | grep -qF -- "$LIVENESS" && echo yes)"

# --- 3. all three states are named, so the branch cannot silently collapse into a
# two-state live/dead test that would claim an `indeterminate` owner.
for state in quiet live indeterminate; do
	check "the branch names the \`$state\` state" "yes" \
		"$(printf '%s\n' "$SEC" | grep -qF -- "\`$state\`" && echo yes)"
done

# --- 4. the claim is bound to `quiet` ALONE: the line naming `quiet` is the line
# that claims. This is the discriminating check — an unconditional overwrite
# satisfies 1–3 and fails here.
QUIET_LINE=$(printf '%s\n' "$SEC" | grep -F -- '`quiet`' | head -1)
check "the claim is bound to the quiet state" "yes" \
	"$(printf '%s\n' "$QUIET_LINE" | grep -qF -- "$CLAIM" && echo yes)"

# --- 5. the decline is stated explicitly for the other two states, so the bound
# is written down and not merely implied by the claim's own condition.
check "the decline is stated for live/indeterminate" "yes" \
	"$(printf '%s\n' "$SEC" | grep -F -- 'do NOT claim' | head -1 | grep -qF -- 'indeterminate' && echo yes)"

# --- 6. the old outcome survives for the non-claiming case, so the branch did not
# replace the already-connected report outright.
check "the non-claiming branch still reports already connected" "yes" \
	"$(printf '%s\n' "$SEC" | grep -qF -- 'already connected' && echo yes)"

# --- 7. the prior owner is preserved as authorship history, not discarded.
check "the claim records the prior owner in # Progress" "yes" \
	"$(printf '%s\n' "$SEC" | grep -qF -- '# Progress' && echo yes)"

# --- self-check: strip the claim from a throwaway copy and require the check to
# fail there. A check that passed on a tree without the branch would measure
# nothing.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/$(dirname "$AGENT")"
sed "s/$CLAIM/STRIPPED/" "$ROOT/$AGENT" >"$TMP/$AGENT"

# The non-zero direction: the same script, against a tree whose claim is gone.
if bash "$ROOT/scripts/session-claim-branch-test.sh" --check-only "$TMP" >/dev/null 2>&1; then
	fail=$((fail + 1))
	echo "❌ self-check: --check-only exited 0 on a tree without the claim branch" >&2
else
	pass=$((pass + 1))
fi

# The zero direction: the same script, against the real tree.
if bash "$ROOT/scripts/session-claim-branch-test.sh" --check-only "$ROOT" >/dev/null 2>&1; then
	pass=$((pass + 1))
else
	fail=$((fail + 1))
	echo "❌ self-check: --check-only exited non-zero on the real tree" >&2
fi

echo "session-claim-branch: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
