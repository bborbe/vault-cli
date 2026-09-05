#!/usr/bin/env bash
#
# Tests for daily-note-has-entry.sh. Each case pins one of the three bugs this
# check has historically shipped, so a regression fails here instead of silently
# passing a session-close.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing here: this script runs under `set -uo pipefail`
# WITHOUT `-e` (it must survive non-zero exits from the script under test), so a
# failed cd would otherwise continue and run the cases from the wrong directory.
cd "$ROOT" || exit 2

SCRIPT=scripts/daily-note-has-entry.sh
H1=scripts/testdata/daily-note-h1.md
H2=scripts/testdata/daily-note-h2.md

pass=0
fail=0

expect() { # expect <want-exit> <label> <file> <title>
	local want=$1 label=$2 file=$3 title=$4 got=0
	bash "$SCRIPT" "$file" "$title" >/dev/null 2>&1 || got=$?
	if [ "$got" -eq "$want" ]; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ $label: want exit $want, got $got" >&2
	fi
}

# --- present / absent baseline
expect 0 "entry present"                       "$H1" "Entry Present Task"
expect 1 "task absent entirely"                "$H1" "Nonexistent Task"

# --- bug 3a: unterminated range ran to EOF, so a later section counted
expect 1 "### heading in a LATER section"      "$H1" "After Section Task"

# --- bug 3b: unanchored grep matched a wikilink on any line
expect 1 "checkbox before the section"         "$H1" "Checkbox Only Task"
expect 1 "checkbox inside another entry"       "$H1" "Checkbox Inside Entry Task"
expect 1 "body-text mention, not an entry"     "$H1" "Body Mention Task"

# --- bug 2: unescaped regex metacharacters in the title
expect 0 "title with parens and dashes"        "$H1" "Cleanup Email Inbox (Personal) - 2026-09-05"

# --- bug 1: heading level assumed; vaults use # or ##
expect 0 "h2 section heading"                  "$H2" "H2 Vault Task"
expect 1 "later section under h2"              "$H2" "After H2 Section Task"

# --- wikilink forms the script's header claims to match
expect 0 "aliased  [[T|alias]]"                "$H1" "Aliased Form Task"
expect 0 "heading  [[T#Section]]"              "$H1" "Heading Form Task"
expect 0 "path-prefixed [[dir/T|alias]]"       "$H1" "Path Prefixed Task"

# --- input errors are distinct from "absent"
expect 2 "missing daily note"                  "scripts/testdata/nope.md" "Anything"

echo "daily-note-has-entry: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
