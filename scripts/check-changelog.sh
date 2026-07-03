#!/usr/bin/env bash
#
# Verifies CHANGELOG.md structure. The "# Changelog" title and the
# "All notable changes…" preamble must precede every "## " section
# (## Unreleased or ## vX.Y.Z). Guards against a section inserted above
# the preamble — the malformed shape a changelog edit can otherwise
# introduce without any other check catching it.
#
# Exits non-zero with a clear message on violation.
# Run from repo root (Makefile target `check-changelog`).

set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

FILE=CHANGELOG.md

# `|| true` is required: under `set -euo pipefail` a no-match grep in a
# command substitution would abort the script before the emptiness checks run.
PREAMBLE_LINE=$(grep -n -m1 '^All notable changes to this project' "$FILE" | cut -d: -f1 || true)
FIRST_SECTION_LINE=$(grep -n -m1 '^## ' "$FILE" | cut -d: -f1 || true)

if [ -z "$PREAMBLE_LINE" ]; then
	echo "❌ CHANGELOG structure: missing preamble line 'All notable changes to this project…' in $FILE" >&2
	exit 1
fi

if [ -n "$FIRST_SECTION_LINE" ] && [ "$FIRST_SECTION_LINE" -lt "$PREAMBLE_LINE" ]; then
	echo "❌ CHANGELOG structure: a '## ' section at line $FIRST_SECTION_LINE appears before the preamble (line $PREAMBLE_LINE) in $FILE" >&2
	echo "   Expected order: '# Changelog' → preamble → '## Unreleased' → '## vX.Y.Z' (newest first)." >&2
	exit 1
fi

echo "CHANGELOG structure OK"
