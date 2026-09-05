#!/usr/bin/env bash
#
# Does today's daily note contain a "What happened today" entry for a given
# task/goal title? Used by /vault-cli:session-close Phase 7 (representation
# check) and, transitively, by /vault-cli:sync-progress Phase 2's
# "record already exists" exception.
#
# Usage:   daily-note-has-entry.sh <daily-note-path> <task-or-goal-title>
# Exits:   0 = an entry referencing the title exists
#          1 = no such entry (caller should flag / write the entry)
#          2 = usage or input error (file missing) — NOT the same as "absent"
#
# Why a script and not an inline snippet: this check has shipped three separate
# bugs (hardcoded heading level, unescaped regex metacharacters in the title,
# and an unterminated range + unanchored grep). Every one was invisible to
# reading and obvious to running. It lives here so `make test` can exercise it.
#
# The three properties the check must hold, each a past bug:
#   1. Heading level is discovered, not assumed — vaults use `#` or `##`.
#   2. The title is regex-escaped — titles contain `(`, `)`, `-`, `.`.
#   3. The section range ends at the next same-or-higher-level heading, and the
#      match is anchored to a `### ` entry heading. Otherwise a wikilink
#      anywhere later in the file (e.g. a checkbox in another session's entry)
#      counts as representation and the check passes on zero entries.
#
# Wikilink forms that must match (over-matching is the correct bias — a false
# pass hides a missing record, a false flag costs one glance):
#   bare           [[Title]]
#   aliased        [[Title|runbook]]
#   heading        [[Title#Some Section]]
#   path-prefixed  [[23 Goals/90 Completed/Title|alias]]
# The path-prefixed form does not even begin with the title, so anchoring on
# `[[<title>` fails — hence the optional `([^]|#]*/)?` path group.

set -euo pipefail

if [ "$#" -ne 2 ]; then
	echo "usage: $(basename "$0") <daily-note-path> <task-or-goal-title>" >&2
	exit 2
fi

DAILY=$1
TITLE=$2

if [ ! -f "$DAILY" ]; then
	echo "❌ daily note not found: $DAILY" >&2
	exit 2
fi

# Escape regex metacharacters — titles routinely contain them, e.g.
# "Cleanup Email Inbox (Personal) - 2026-09-05".
TITLE_RE=$(printf '%s' "$TITLE" | sed 's/[][\.^$*+?(){}|\/]/\\&/g')

# Range: start at the "What happened today" heading at whatever level it uses,
# end at the next heading of the SAME OR HIGHER level. A naive `/^#+ /{f=0}`
# terminator is wrong — entries are `###` and match it, closing the range at the
# first entry.
# Match: anchored to `^### ` so only an entry heading counts, allowing an
# optional folder path prefix and an optional `|alias` / `#heading` suffix.
# NOT a pipeline: `awk ... | grep -q` is broken under `set -o pipefail`.
# `grep -q` exits at the first match; on a real daily note awk still has
# hundreds of lines to write, takes SIGPIPE, and dies 141. pipefail promotes
# that to the pipeline's status, so a title that IS present reports absent.
# Invisible in tests — small fixtures let awk finish before grep short-circuits,
# so the bug only appears above the pipe-buffer threshold, i.e. only in real use.
# Also shell-dependent: ugrep (interactive zsh) does not short-circuit the same
# way as BSD grep (the bash this script runs under), so it reproduces only here.
SECTION=$(awk 'match($0,/^#+/) && $0 ~ /^#+ What happened today/ {lvl=RLENGTH; f=1; next}
               f && match($0,/^#+/) && RLENGTH<=lvl {f=0}
               f' "$DAILY")

if grep -qE "^#{3} \[\[([^]|#]*/)?${TITLE_RE}([|#][^]]*)?\]\]" <<<"$SECTION"; then
	exit 0
fi

exit 1
