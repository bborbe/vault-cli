#!/usr/bin/env bash
#
# Pins the create-time `role:` / `mode:` write to the artifacts that carry it.
#
# The rule has no code: both task-writing agents read it from prose, so a silent
# edit is a behaviour regression only a presence test can catch. Each carrier
# must (a) cite the role × mode matrix by its installed path and (b) name both
# fields as written ALWAYS. The matrix itself lives in claude-supervisor
# (docs/role-mode-matrix.md) and is deliberately not restated here.
#
# Self-check on a throwaway copy: strip the citation from one carrier and
# require the scan to report it, so the scan cannot pass vacuously.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 2

CITE='claude-supervisor/docs/role-mode-matrix.md'
CARRIERS=(agents/task-creator.md agents/task-writer.md commands/create-task.md)
AGENTS=(agents/task-creator.md agents/task-writer.md)

scan() {
	local dir=$1 rc=0 f
	[ "${#CARRIERS[@]}" -eq 3 ] || { echo "role-mode-matrix: expected 3 carriers, got ${#CARRIERS[@]}"; return 1; }
	for f in "${CARRIERS[@]}"; do
		[ -r "$dir/$f" ] || { echo "role-mode-matrix: $dir/$f is missing or unreadable"; rc=1; continue; }
		grep -qF "$CITE" "$dir/$f" || { echo "role-mode-matrix: $f does not cite $CITE"; rc=1; }
	done
	for f in "${AGENTS[@]}"; do
		grep -qF '`role: <agent|human>` and `mode: <headless|interactive>` — ALWAYS' "$dir/$f" \
			|| { echo "role-mode-matrix: $f does not write role/mode ALWAYS"; rc=1; }
	done
	return $rc
}

scan "$ROOT" || exit 1

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/agents" "$TMP/commands"
for f in "${CARRIERS[@]}"; do cp "$f" "$TMP/$f" || exit 2; done
clean=$(scan "$TMP") || { echo "role-mode-matrix: self-check failed — the unmodified copy did not scan clean:"; echo "$clean"; exit 1; }
sed -i.bak "s#$CITE#REMOVED#g" "$TMP/agents/task-writer.md"
out=$(scan "$TMP")
if [ "$out" != "role-mode-matrix: agents/task-writer.md does not cite $CITE" ]; then
	echo "role-mode-matrix: self-check failed — a stripped citation was not reported"
	exit 1
fi

echo "role-mode-matrix rule OK"
