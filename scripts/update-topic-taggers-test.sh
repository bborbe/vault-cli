#!/usr/bin/env bash
#
# Tests for update-topic-taggers.sh — the membership rule behind
# /vault-cli:update-topic's `## Goals` diff.
#
# Each case pins a rule where the obvious implementation is wrong, so a
# regression fails here instead of silently proposing the wrong goals:
#
#   - the goals folder carries a SPACE ("24 Goals", the vault's real name). An
#     unquoted `$GOALS_DIR/*.md` glob word-splits into `24` and `Goals/*.md` and
#     the scan reads nothing — this is the case that made the first version
#     silently produce an empty diff on the standard layout.
#   - membership reads the `topics:` FIELD, not the file. A goal whose `title:`
#     or body names the topic is not a member; a whole-file grep reports it.
#   - a whole element, not a substring: topic "German" must not match
#     `[[German-Spanish]]`.
#   - the bare list form `topics: [Spanish]` and the legacy singular `topic:`
#     are both real frontmatter shapes in this vault.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 2

SCRIPT="$ROOT/scripts/update-topic-taggers.sh"
FAILS=0
CASES=0

ok()   { CASES=$((CASES+1)); printf '  ok   %s\n' "$1"; }
fail() { CASES=$((CASES+1)); FAILS=$((FAILS+1)); printf '  FAIL %s\n     expected: %s\n     actual:   %s\n' "$1" "$2" "$3"; }

expect() { # name expected actual
  if [ "$2" = "$3" ]; then ok "$1"; else fail "$1" "$2" "$3"; fi
}

WORK=$(mktemp -d) || exit 2
trap 'rm -rf "$WORK"' EXIT

GOALS="$WORK/24 Goals"
mkdir -p "$GOALS"

cat > "$GOALS/Wikilink Member.md" <<'EOF'
---
status: in_progress
topics:
    - '[[Development Pipeline Throughput]]'
---
# Goal
EOF

cat > "$GOALS/Other Topic.md" <<'EOF'
---
status: in_progress
topics:
    - '[[Isolated Agent Workers]]'
---
# Goal
EOF

cat > "$GOALS/Title Only.md" <<'EOF'
---
status: in_progress
title: Development Pipeline Throughput
topics:
    - '[[Something Else]]'
---
# Goal
EOF

cat > "$GOALS/Body Mention.md" <<'EOF'
---
status: in_progress
topics:
    - '[[Something Else]]'
---
# Goal

This goal is tracked on [[Development Pipeline Throughput]] but does not tag it.
EOF

cat > "$GOALS/German.md" <<'EOF'
---
status: in_progress
topics:
    - '[[German]]'
---
# Goal
EOF

cat > "$GOALS/German-Spanish.md" <<'EOF'
---
status: in_progress
topics:
    - '[[German-Spanish]]'
---
# Goal
EOF

cat > "$GOALS/Bare List.md" <<'EOF'
---
status: in_progress
topics: [Spanish]
---
# Goal
EOF

cat > "$GOALS/Legacy Singular.md" <<'EOF'
---
status: in_progress
topic: '[[Legacy Topic]]'
---
# Goal
EOF

echo "update-topic-taggers:"

expect "field-scoped wikilink membership, spaced folder, others excluded" \
  "Wikilink Member" \
  "$(bash "$SCRIPT" "$GOALS" "Development Pipeline Throughput")"

expect "title and body mentions are not members" \
  "" \
  "$(bash "$SCRIPT" "$GOALS" "Nothing Tags This")"

expect "whole element, not substring — German does not match German-Spanish" \
  "German" \
  "$(bash "$SCRIPT" "$GOALS" "German")"

expect "Spanish matches the bare list form, not German-Spanish" \
  "Bare List" \
  "$(bash "$SCRIPT" "$GOALS" "Spanish")"

expect "legacy singular topic: key is read" \
  "Legacy Singular" \
  "$(bash "$SCRIPT" "$GOALS" "Legacy Topic")"

expect "absent topic yields no output" \
  "" \
  "$(bash "$SCRIPT" "$GOALS" "No Such Topic")"

# A missing directory must fail loudly, never report an empty membership set:
# an empty diff reads as "page matches disk" and would hide a wrong folder.
bash "$SCRIPT" "$WORK/does-not-exist" "Anything" >/dev/null 2>&1
expect "missing goals dir exits non-zero" "2" "$?"

echo
if [ "$FAILS" -gt 0 ]; then
  echo "update-topic-taggers: $FAILS/$CASES failed"
  exit 1
fi
echo "update-topic-taggers: $CASES/$CASES passed"
