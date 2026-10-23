#!/usr/bin/env bash
#
# Pins the task-boundary closer panel in the two commands that emit one.
#
# The rule has no code: it lives in the prose of `complete-task` step 2e and
# `sync-progress` Phase 6, so a silent edit to that prose is a behaviour
# regression only a presence test can catch. Two facts are pinned:
#
#   1. The panel is PRESENT and carries the self-run close — `👤 You: nothing`
#      followed by `⏰ Next: /vault-cli:session-close — this session runs it now`.
#      An absence-only pin would pass on a deleted panel, which is the defect
#      the panel exists to prevent, so the positive half is the load-bearing one.
#
#   2. The operator-approval form of that close is GONE from commands/, agents/
#      and docs/. CHANGELOG.md is deliberately outside that scope — it quotes
#      the old line as history.
#
# The pattern leads with `[[:space:]]*` because `complete-task.md`'s panel is
# indented inside its step, while `sync-progress.md`'s is flush left. An anchored
# `^⏰` matches the second file only.
#
# Two self-checks, run on throwaway copies: delete the panel from one carrier and
# require panel_present to say no; restore the approval line into the other and
# require the negative scan to find it — so neither check can pass vacuously.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 2

CARRIERS=(
	commands/complete-task.md
	commands/sync-progress.md
)

PANEL_NEXT='[[:space:]]*⏰ Next: /vault-cli:session-close — this session runs it now'
PANEL_YOU='[[:space:]]*👤 You: nothing'
BANNED='approve: /vault-cli:session-close'

pass=0
fail=0

check() { # check <label> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   %s\n' "$1"
		pass=$((pass + 1))
	else
		printf 'FAIL %s\n     expected: %s\n     actual:   %s\n' "$1" "$2" "$3"
		fail=$((fail + 1))
	fi
}

# panel_present <file> -> "present" when both panel lines are contiguous.
panel_present() {
	if grep -A1 "$PANEL_YOU" "$1" 2>/dev/null | grep -qF '⏰ Next: /vault-cli:session-close — this session runs it now'; then
		echo present
	else
		echo absent
	fi
}

# banned_hits <dir> -> count of operator-approval-form lines.
banned_hits() {
	grep -rn --exclude-dir=.git "$BANNED" "$1" 2>/dev/null | wc -l | tr -d ' '
}

for f in "${CARRIERS[@]}"; do
	check "$f carries the self-run closer panel" "present" "$(panel_present "$f")"
done

check "no operator-approval close remains in commands/" "0" "$(banned_hits commands)"
check "no operator-approval close remains in agents/" "0" "$(banned_hits agents)"
check "no operator-approval close remains in docs/" "0" "$(banned_hits docs)"

# Self-checks: prove each predicate can fail.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/stripped" "$TMP/restored"
grep -v '⏰ Next: /vault-cli:session-close — this session runs it now' \
	"${CARRIERS[0]}" >"$TMP/stripped/$(basename "${CARRIERS[0]}")"
check "self-check: a deleted panel reads absent" "absent" \
	"$(panel_present "$TMP/stripped/$(basename "${CARRIERS[0]}")")"

sed -E "s|^[[:space:]]*👤 You: nothing\$|👤 You: $BANNED|" "${CARRIERS[1]}" \
	>"$TMP/restored/$(basename "${CARRIERS[1]}")"
check "self-check: a restored approval line is caught" "1" \
	"$(banned_hits "$TMP/restored")"

echo "closer-panel-rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
