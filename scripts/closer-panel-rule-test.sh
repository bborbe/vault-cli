#!/usr/bin/env bash
#
# Pins the task-boundary closer panel in the two commands that emit one, and the
# three conditions under which it must NOT be emitted.
#
# The rule has no code: it lives in the prose of `complete-task` step 2e and
# `sync-progress` Phase 6, so a silent edit to that prose is a behaviour
# regression only a presence test can catch. What is pinned:
#
#   1. The panel is PRESENT and carries the self-run close — `👤 You: nothing`
#      followed by `⏰ Next: /vault-cli:session-close — this session runs it now`.
#      An absence-only pin would pass on a deleted panel, which is the defect
#      the panel exists to prevent, so the positive half is the load-bearing one.
#
#   2. The operator-approval form of that close is GONE from commands/, agents/
#      and docs/. CHANGELOG.md is outside that scope on purpose: a released entry
#      may quote the old line to describe what changed, and failing the build for
#      a changelog that records history is not what this pin is for. (No entry
#      quotes it today; the exclusion is for the next one that has to.)
#
#   3. The three skip conditions are present, because each one guards a recursion
#      or a false `⚪ DONE` that the panel alone would introduce:
#        - recurring completion (status stays in_progress → mode 3)
#        - nested caller via complete-task (`sync-progress` Phase 4b → --non-interactive)
#        - nested caller via sync-progress (`session-close` Phase 2, `prepare-compact`,
#          `task-status` → --nested). Every caller that continues after the sync must
#          pass it, or a completion inside the sync self-closes the calling command.
#
# The pattern leads with `[[:space:]]*` because `complete-task.md`'s panel is
# indented inside its step, while `sync-progress.md`'s is flush left. An anchored
# `^⏰` matches the second file only.
#
# Self-checks run on throwaway copies: delete the panel from one carrier and
# require panel_present to say no; restore the approval line into the other and
# require the negative scan to find it; strip one skip clause and require its
# predicate to say no — so no check can pass vacuously.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 2

CARRIERS=(
	commands/complete-task.md
	commands/sync-progress.md
)

PANEL_YOU='[[:space:]]*👤 You: nothing'
PANEL_NEXT='⏰ Next: /vault-cli:session-close — this session runs it now'
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

# panel_present <file> -> "present" when the two panel lines are contiguous.
panel_present() {
	if grep -A1 "$PANEL_YOU" "$1" 2>/dev/null | grep -qF "$PANEL_NEXT"; then
		echo present
	else
		echo absent
	fi
}

# has <file> <fixed-string> -> "yes" when the clause is present.
has() {
	if grep -qF -- "$2" "$1" 2>/dev/null; then echo yes; else echo no; fi
}

# lines_of <string> -> the string's line count, or "grep-error" when grep itself
# failed. `grep -rn` exits 0 on matches, 1 on none, >1 on a real error; piping
# through `2>/dev/null | wc -l` would turn every error into a clean 0 — the
# vacuous pass this file exists to prevent, turned on itself.
lines_of() {
	printf '%s\n' "$1" | sed '/^$/d' | wc -l | tr -d ' '
}

# banned_hits <dir> -> count of operator-approval-form lines, "no-dir" when the
# target is not a directory, or "grep-error". No `--exclude-dir`: BusyBox grep
# rejects the flag, and the three scan targets carry no `.git` to exclude.
banned_hits() {
	if [ ! -d "$1" ]; then
		echo no-dir
		return
	fi
	out=$(grep -rn "$BANNED" "$1")
	rc=$?
	[ "$rc" -gt 1 ] && { echo grep-error; return; }
	lines_of "$out"
}

for f in "${CARRIERS[@]}"; do
	check "$f carries the self-run closer panel" "present" "$(panel_present "$f")"
done

for d in commands agents docs; do
	check "no operator-approval close remains in $d/" "0" "$(banned_hits "$d")"
done

check "complete-task skips the closer for recurring completions" "yes" \
	"$(has commands/complete-task.md '**Recurring tasks.**')"
check "complete-task points at the canonical recurring probe" "yes" \
	"$(has commands/complete-task.md 'Probe as [`sync-progress.md` Phase 6](sync-progress.md) defines it')"
check "sync-progress carries the canonical recurring probe" "yes" \
	"$(has commands/sync-progress.md 'vault-cli task get "<name>" recurring --output json')"
check "complete-task skips the closer for nested callers" "yes" \
	"$(has commands/complete-task.md '**Nested callers.**')"
check "sync-progress skips the closer when no completion was established" "yes" \
	"$(has commands/sync-progress.md '**Skip the closer entirely** when no completion was established')"
check "sync-progress skips the closer for recurring completions" "yes" \
	"$(has commands/sync-progress.md '**Skip it for a recurring completion too.**')"
check "sync-progress skips the closer when NESTED" "yes" \
	"$(has commands/sync-progress.md '**Skip it entirely when `NESTED=true`.**')"
# The join that makes the guard work: the flag is only a guard because the
# Arguments section turns it into NESTED. Pin the parse and the default, or
# deleting them leaves every caller and skip clause pinned while the guard is dead.
check "sync-progress parses --nested into NESTED=true" "yes" \
	"$(has commands/sync-progress.md 'set `NESTED=true` and strip the token')"
check "sync-progress defaults NESTED=false without the flag" "yes" \
	"$(has commands/sync-progress.md 'Absent the flag, `NESTED=false`')"
check "session-close passes --nested to sync-progress" "yes" \
	"$(has commands/session-close.md 'Skill: vault-cli:sync-progress --nested')"
check "prepare-compact passes --nested to sync-progress" "yes" \
	"$(has commands/prepare-compact.md 'Skill: vault-cli:sync-progress --nested')"
check "task-status passes --nested to sync-progress" "yes" \
	"$(has commands/task-status.md 'Skill: vault-cli:sync-progress --nested')"
check "sync-progress Phase 4b passes --non-interactive to complete-task" "yes" \
	"$(has commands/sync-progress.md 'Skill: vault-cli:complete-task --non-interactive')"

# Absence half: presence checks only pin the callers that exist today. A caller
# added later — in commands/, agents/ or docs/ — would pass silently, so require
# that NO bare invocation survives in any of them.
bare_callers() { # bare_callers [root] — sync-progress invocations without --nested
	root="${1:-.}"
	total=0
	for d in commands agents docs; do
		[ -d "$root/$d" ] || continue
		out=$(grep -rn 'Skill: vault-cli:sync-progress' "$root/$d")
		rc=$?
		[ "$rc" -gt 1 ] && { echo grep-error; return; }
		total=$((total + $(printf '%s\n' "$out" | grep -vF -- '--nested' | sed '/^$/d' | wc -l | tr -d ' ')))
	done
	echo "$total"
}
check "no sync-progress caller omits --nested" "0" "$(bare_callers .)"

# Self-checks: prove each predicate can fail.
TMP=$(mktemp -d)
[ -d "$TMP" ] || { echo "FAIL could not create a temp dir for the self-checks" >&2; exit 2; }
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/stripped" "$TMP/restored" "$TMP/noclause"
grep -vF "$PANEL_NEXT" "${CARRIERS[0]}" >"$TMP/stripped/$(basename "${CARRIERS[0]}")"
check "self-check: a deleted panel reads absent" "absent" \
	"$(panel_present "$TMP/stripped/$(basename "${CARRIERS[0]}")")"

sed -E "s|^[[:space:]]*👤 You: nothing\$|👤 You: $BANNED|" "${CARRIERS[1]}" \
	>"$TMP/restored/$(basename "${CARRIERS[1]}")"
check "self-check: a restored approval line is caught" "1" \
	"$(banned_hits "$TMP/restored")"

grep -vF '**Skip it entirely when `NESTED=true`.**' commands/sync-progress.md \
	>"$TMP/noclause/sync-progress.md"
check "self-check: a stripped skip clause reads absent" "no" \
	"$(has "$TMP/noclause/sync-progress.md" '**Skip it entirely when `NESTED=true`.**')"

check "self-check: a non-directory target is not a vacuous pass" "no-dir" \
	"$(banned_hits "$TMP/does-not-exist")"

mkdir -p "$TMP/bare/commands"
printf 'Skill: vault-cli:sync-progress\n' >"$TMP/bare/commands/x.md"
check "self-check: a caller without --nested is caught" "1" "$(bare_callers "$TMP/bare")"

echo "closer-panel-rule: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
