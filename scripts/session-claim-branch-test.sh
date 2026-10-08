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
# The claim also has to say WHERE ITS UUID COMES FROM. Step 2's title-match
# detection lives inside its empty-field branch and does not run on the
# already-set path, so a claim branch that just says "this session's uuid" names
# a value it never resolved — and a guessed or invented id lands in the very
# field every consumer reads as ownership. Checks 8 and 9 pin that.
#
# Step 3 is a Claude Code agent definition: it has no test harness and runs only
# inside a real LLM behind `mcp__supervisor__spawn_agent`. An integration test
# can exercise the CLI verbs, never the branch firing. The prose IS the
# implementation, so an edit that drops the claim — or that widens it into an
# unconditional overwrite — is a behaviour regression no unit or integration test
# can see. Which is why this test, rather than a fixture, is the regression check.
#
# `--check-only <root>` runs the WHOLE check set against <root> and exits
# non-zero if any check fails, so the self-check below proves that a tree with
# the claim stripped fails the discriminating bound check — not merely the
# presence check. (An earlier version ran only the presence check there, which
# proved the mutation against the weakest assertion.)
#
# Text is matched with bash string tests and here-strings, never by piping into
# `grep -q` or `head -1`: under `pipefail` an upstream SIGPIPE (a downstream
# stage closing the pipe early) turns a present match into a failure — the
# "bug-4 shape" the sibling `scripts/metrics-append-call-site-test.sh` documents.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

CHECK_ONLY=false
ROOT_DIR="$ROOT"
if [ "${1:-}" = "--check-only" ]; then
	CHECK_ONLY=true
	ROOT_DIR="${2:?--check-only needs a root}"
fi

AGENT=agents/work-on-task-assistant.md
SECTION='### Session connect (MANDATORY when Obsidian task file exists)'
STEP3='If `claude_session_id` is **already set**'
CLAIM='✅ Session: claimed'
CLAIM_BULLET='- **`quiet`**'
LIVENESS='docs/session-liveness.md'

# section <file> <heading> — the lines under an exact heading, until the next
# heading. Exact whole-line match: a substring match would also open on a mention
# of the heading inside prose.
section() {
	awk -v h="$2" '$0 == h { f = 1; next } f && /^#/ { exit } f' "$1"
}

# contains <haystack> <needle> — "yes" when <needle> occurs in <haystack>.
contains() {
	case "$1" in
	*"$2"*) printf 'yes' ;;
	*) printf '' ;;
	esac
}

# firstLine <text> <needle> — the first line of <text> containing <needle>,
# empty when none. Here-string, so no pipeline stage can be SIGPIPE'd.
firstLine() {
	awk -v n="$2" 'index($0, n) { print; exit }' <<< "$1"
}

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

SEC=$(section "$ROOT_DIR/$AGENT" "$SECTION")

# --- 1. the claim branch exists, and inside § Session connect — section-scoped,
# so a prose mention elsewhere in the file cannot satisfy it.
check "§ Session connect carries the claim branch" "yes" "$(contains "$SEC" "$CLAIM")"

# --- 2. step 3 itself names the liveness rule's home rather than restating it
# inline. Scoped to step 3's own line: the detection block above it already cites
# the same doc, so a section-wide match would pass on an unedited step 3.
STEP3_LINE=$(firstLine "$SEC" "$STEP3")
check "step 3 reads the rule from docs/session-liveness.md" "yes" \
	"$(contains "$STEP3_LINE" "$LIVENESS")"

# --- 3. all three states are named, so the branch cannot silently collapse into a
# two-state live/dead test that would claim an `indeterminate` owner.
for state in quiet live indeterminate; do
	check "the branch names the \`$state\` state" "yes" "$(contains "$SEC" "\`$state\`")"
done

# --- 4. the claim is bound to `quiet` ALONE: the claim BULLET is the line that
# claims. This is the discriminating check — an unconditional overwrite satisfies
# 1–3 and fails here. Anchored on the bullet's own leading marker, not on a bare
# `` `quiet` `` match: an earlier version took the first `quiet` mention, which
# silently rebinds if a mention is ever inserted above the bullet.
QUIET_LINE=$(firstLine "$SEC" "$CLAIM_BULLET")
check "the claim is bound to the quiet bullet" "yes" "$(contains "$QUIET_LINE" "$CLAIM")"

# --- 5. the decline is stated explicitly for the other two states, so the bound
# is written down and not merely implied by the claim's own condition. Anchored
# on the `live` bullet's marker, not on the `do NOT claim` phrase: the claim
# bullet also refuses ("do NOT claim" on a non-unique title match), so a phrase
# anchor picks the wrong line.
DECLINE_LINE=$(firstLine "$SEC" '- **`live`**')
check "the decline is stated for the live bullet" "yes" "$(contains "$DECLINE_LINE" 'do NOT claim')"
check "the decline names indeterminate" "yes" "$(contains "$DECLINE_LINE" 'indeterminate')"

# --- 6. the old outcome survives for the non-claiming case, so the branch did not
# replace the already-connected report outright.
check "the non-claiming branch still reports already connected" "yes" \
	"$(contains "$SEC" 'already connected')"

# --- 7. the prior owner is preserved as authorship history — via the
# accumulating metrics append, NOT a task-body write. The agent's declared write
# surface is `<constraints>`'s "READ-ONLY except: status frontmatter +
# `claude_session_id` frontmatter + daily-note tracking", which does not include
# the body, so an instruction to append a `# Progress` line would be out of
# contract. Both halves are pinned: the preserving mechanism is named, and the
# body write is absent.
check "the claim preserves the prior owner via the metrics append" "yes" \
	"$(contains "$QUIET_LINE" 'metrics')"
check "the claim requires no task-body write" "" "$(contains "$QUIET_LINE" '# Progress')"

# --- 8. the claim says where its uuid comes from. Step 2's title-match lives in
# its empty-field branch and does not run on the already-set path, so a claim
# branch naming "this session's uuid" without re-running that resolution writes a
# value it never obtained into the ownership stamp.
check "the claim re-runs the title-match for its own uuid" "yes" \
	"$(contains "$QUIET_LINE" 'title-match')"

# --- 9. and it refuses rather than guessing when that match is not unique — the
# same refusal step 2 makes, because a guessed id is worse than a stale one.
check "the claim refuses on a non-unique match" "yes" \
	"$(contains "$QUIET_LINE" 'refusing to guess')"

# --- self-check (skipped in --check-only mode, which would recurse): strip the
# claim from a throwaway copy and require the FULL check set to fail there. A
# check that passed on a tree without the branch would measure nothing.
if [ "$CHECK_ONLY" = false ]; then
	TMP=$(mktemp -d)
	trap 'rm -rf "$TMP"' EXIT
	mkdir -p "$TMP/$(dirname "$AGENT")"
	sed "s/$CLAIM/STRIPPED/" "$ROOT/$AGENT" >"$TMP/$AGENT"

	# The non-zero direction: the same script, whole check set, against a tree
	# whose claim is gone. Must fail — including the bound check 4.
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
fi

echo "session-claim-branch: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
