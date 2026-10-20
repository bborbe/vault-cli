#!/usr/bin/env bash
#
# Tests for session-close-goals.sh.
#
# Each case pins a rule the script commits to, so a regression fails here instead
# of silently changing what a close reports. The rules that need pinning are the
# ones where the obvious implementation is wrong:
#
#   - a goal with an open task prints the report-only `goal ` form, and a DRAINED
#     goal prints `closable goal ` instead — the two are routed differently by
#     Phase 4.5, so one form for both would erase the distinction the phase exists
#     for
#   - `aborted` is an OPEN task here: the goal is not drained, so a decision is
#     still owed about the goal itself
#   - a non-`in_progress` goal is silent — its status is the managing session's own
#     write
#   - a manager id is a LEAD: `--check` non-zero prints `no live manager` even
#     though an id is present. This is the case an implementation that routes on
#     id presence alone gets wrong
#   - a topic page whose `## Goals` section lists the goal supplies the manager
#     when the goal page carries none
#
# The liveness probe is stubbed via SESSION_LIVENESS, so the verdict is the test's
# input rather than whatever the machine has installed.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 2

SCRIPT="$ROOT/scripts/session-close-goals.sh"
FAILS=0
CASES=0

ok()   { CASES=$((CASES+1)); printf '  ok   %s\n' "$1"; }
fail() { CASES=$((CASES+1)); FAILS=$((FAILS+1)); printf '  FAIL %s\n     expected: %s\n     actual:   %s\n' "$1" "$2" "$3"; }

expect_line() { # name expected actual
  if [ "$2" = "$3" ]; then ok "$1"; else fail "$1" "$2" "$3"; fi
}

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
V="$WORK/vault"
mkdir -p "$V/24 Goals" "$V/25 Tasks" "$V/23 Topics"

# --- fixtures ---------------------------------------------------------------
goal() { # file status manager tasks...
  local file="$1" status="$2" manager="$3"; shift 3
  {
    echo "---"
    echo "status: $status"
    [ -n "$manager" ] && echo "manager_session_id: $manager"
    echo "---"
    echo
    echo "# Success Criteria"
    echo
    echo "- [x] one"
    echo "- [ ] two"
    echo
    echo "# Tasks"
    echo
    for t in "$@"; do echo "- [[$t]]"; done
  } > "$V/24 Goals/$file.md"
}

task() { printf -- '---\nstatus: %s\n---\n' "$2" > "$V/25 Tasks/$1.md"; }

task "Open A"    completed
task "Open B"    in_progress
task "Closed A"  completed
task "Closed B"  completed
task "Abort A"   aborted

goal "Open Goal"     in_progress ""                     "Open A" "Open B"
goal "Abort Goal"    in_progress ""                     "Closed A" "Abort A"
goal "Closed Goal"   in_progress ""                     "Closed A" "Closed B"
goal "Done Goal"     completed    ""                     "Open A"
goal "Managed Goal"  in_progress "11111111-1111-1111-1111-111111111111" "Open A" "Open B"

# A topic page that lists Abort Goal and carries its own manager id.
{
  echo "---"
  echo "manager_session_id: 22222222-2222-2222-2222-222222222222"
  echo "---"
  echo
  echo "## Goals"
  echo
  echo "- [[Abort Goal]]"
} > "$V/23 Topics/Topic One.md"

# Stub probe: exit 0 for the topic id, 3 (AMBIGUOUS) for anything else.
STUB="$WORK/liveness.py"
cat > "$STUB" <<'STUBEOF'
import json,sys
if "--list" in sys.argv:
    print(json.dumps([{"sessionId":"22222222-2222-2222-2222-222222222222","name":"Topic One Manager"}]))
    raise SystemExit(0)
sys.exit(0 if "22222222-2222-2222-2222-222222222222" in sys.argv else 3)
STUBEOF

run() { SESSION_LIVENESS="$STUB" bash "$SCRIPT" "$V" "$@" 2>&1; }

# --- cases ------------------------------------------------------------------
echo "session-close-goals"

out=$(run --goal "Open Goal")
expect_line "open task -> report-only form" \
  "goal Open Goal: in_progress, 1/2 SC — no live manager" "$out"

out=$(run --goal "Closed Goal")
expect_line "drained goal -> closable form" \
  "closable goal Closed Goal: in_progress, 1/2 SC — no live manager" "$out"

out=$(run --goal "Abort Goal")
expect_line "aborted task counts as open, topic supplies the manager" \
  "goal Abort Goal: in_progress, 1/2 SC — managed by Topic One Manager" "$out"

out=$(run --goal "Done Goal")
expect_line "non-in_progress goal is silent" "" "$out"

out=$(run --goal "Managed Goal")
expect_line "id present but probe non-zero -> no live manager" \
  "goal Managed Goal: in_progress, 1/2 SC — no live manager" "$out"

out=$(run --goal "Open Goal" --goal "Done Goal" --goal "Closed Goal")
expect_line "input order preserved, silent goal skipped" \
  "goal Open Goal: in_progress, 1/2 SC — no live manager
closable goal Closed Goal: in_progress, 1/2 SC — no live manager" "$out"

echo
if [ "$FAILS" -gt 0 ]; then
  echo "session-close-goals: $FAILS/$CASES failed"
  exit 1
fi
echo "session-close-goals: $CASES/$CASES passed"
