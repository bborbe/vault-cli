#!/usr/bin/env bash
#
# Tests for update-topic-goal-entries.sh — the `## Goals` entry rule behind
# /vault-cli:update-topic's membership diff.
#
# Each case pins a rule where the obvious implementation is wrong, so a
# regression fails here instead of silently proposing the wrong goals:
#
#   - THE SECTION IS LEVEL-RELATIVE. `## Goals` must run past `### Lever A` and
#     stop only at the next `#` or `##`. Bounding at any heading reads 2 entries
#     on the real Development Pipeline Throughput page, whose other 29 live in
#     `### Lever A/B/C/D` subsections, and reports those 29 as missing. That
#     regression shipped in v0.178.0 and this case is what pins it shut.
#   - a DEEPER heading does not end the section, and a heading at the Goals
#     heading's own level does. Both directions are asserted, because a bound
#     that is too wide imports the next section's entries and one that is too
#     narrow drops the grouped ones.
#   - DECLARATION, NOT MENTION. A numbered list item, a bold prose line and a
#     wikilink under a later section are not entries. The real page carries all
#     three, one line away from genuine entries.
#   - the STATUS TOKEN is the first backticked span that is a known goal status,
#     never the first backticked span. The real page has an entry reading
#     `— SC5 lever on `intake/spec`, `in_progress` (phase `planning`)`, where the
#     first span is a flow-stage name; taking it would make the caller rewrite
#     the wrong token. A phase value (`planning`) is likewise not a status.
#   - an entry with NO status token reports `none` and must not be given one —
#     the caller leaves those lines untouched rather than inventing a status.
#   - a page with no Goals heading exits 3. Printing nothing would be
#     indistinguishable from an empty section, and the caller would then propose
#     adding every tagging goal on disk.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 2

SCRIPT="$ROOT/scripts/update-topic-goal-entries.sh"
FAILS=0
CASES=0

ok()   { CASES=$((CASES+1)); printf '  ok   %s\n' "$1"; }
fail() { CASES=$((CASES+1)); FAILS=$((FAILS+1)); printf '  FAIL %s\n     expected: %s\n     actual:   %s\n' "$1" "$2" "$3"; }

expect() { # name expected actual
  if [ "$2" = "$3" ]; then ok "$1"; else fail "$1" "$2" "$3"; fi
}

WORK=$(mktemp -d) || exit 2
trap 'rm -rf "$WORK"' EXIT

# The real page's shape: two entries above the first grouping heading, the rest
# inside `### Lever` subsections, a deeper `####` sub-grouping, a numbered list
# item and a bold prose line inside a Lever, and a later section whose bullets
# and sub-headings must stay out of the set.
PAGE1="$WORK/23 Topics/Development Pipeline Throughput.md"
mkdir -p "$WORK/23 Topics"
cat > "$PAGE1" <<'EOF'
---
page_type: topic
---
Tags: [[Topic]]

---

- [[Before The Section]]

## Goals

- [[Top Entry]] — SC1
- [[Token Entry]] — `in_progress` (phase `planning`)

### Lever A — attention

- [[Lever A Entry]] — `next`
- [[Flow Stage Entry]] — SC5 lever on `intake/spec`, `in_progress` (phase `planning`); reworded
- [[Phase Only Entry]] — SC1 (phase `planning`)

#### Sub-grouping

- [[Deep Entry]] — `backlog`

### Lever C — capacity

- [[Lever C Entry]] — `completed` 2026-10-21 — a trailing clause
- **Binding now (2026-10-19):** the Mac's CPU, load 80–117
  1. [[Numbered Item]] — not a declaration

# Related

- [[Related Entry]]

### Related Sub

- [[Related Sub Entry]]
EOF

expect "level-relative bound: grouped entries are read, later sections are not" \
  "$(printf '%s\t%s\n' \
      "Top Entry" none \
      "Token Entry" in_progress \
      "Lever A Entry" next \
      "Flow Stage Entry" in_progress \
      "Phase Only Entry" none \
      "Deep Entry" backlog \
      "Lever C Entry" completed)" \
  "$(bash "$SCRIPT" "$PAGE1")"

# A `# Goals` page ends at the next `#` — the same rule one level up.
PAGE2="$WORK/h1.md"
cat > "$PAGE2" <<'EOF'
# Goals

- [[H1 Entry]] — `in_progress`

# Other

- [[Outside Entry]]
EOF

expect "H1 Goals section stops at the next H1" \
  "$(printf '%s\t%s\n' "H1 Entry" in_progress)" \
  "$(bash "$SCRIPT" "$PAGE2")"

# A grouped page whose entries all carry tokens still reports the count.
PAGE3="$WORK/grouped.md"
cat > "$PAGE3" <<'EOF'
## Goals

### Group

- [[Grouped One]] — `next`
- [[Grouped Two]] — `backlog`

## Next Section

- [[Excluded]]
EOF

expect "a Goals section with no entries above its first grouping is still read" \
  "$(printf '%s\t%s\n' "Grouped One" next "Grouped Two" backlog)" \
  "$(bash "$SCRIPT" "$PAGE3")"

# Error paths. An absent section must not read as an empty one.
PAGE4="$WORK/no-goals.md"
printf '# Body\n\n- [[Not An Entry]]\n' > "$PAGE4"

bash "$SCRIPT" "$PAGE4" >/dev/null 2>&1
expect "no Goals heading exits 3" "3" "$?"

bash "$SCRIPT" >/dev/null 2>&1
expect "no arguments exits 2" "2" "$?"

bash "$SCRIPT" "$WORK/does-not-exist.md" >/dev/null 2>&1
expect "missing page exits 2" "2" "$?"

echo
if [ "$FAILS" -gt 0 ]; then
  echo "update-topic-goal-entries: $FAILS/$CASES failed"
  exit 1
fi
echo "update-topic-goal-entries: $CASES/$CASES passed"
