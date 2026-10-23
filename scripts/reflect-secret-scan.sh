#!/usr/bin/env bash
#
# The mechanical half of `commands/reflect.md`'s NO-ASK sensitivity scan.
#
# `reflect --non-interactive` writes KB pages with nobody reviewing the learning
# list first, so the sensitivity check the operator would have made has to be
# carried by the command itself. Prose alone cannot carry it: a reworded list
# passes review while the scan still misfires, and the misfire is bad in BOTH
# directions — a false positive silently drops a legitimate learning with no
# operator to catch it, a false negative publishes a secret into the vault.
#
# This script is the single executable source of the pattern set. It reads the
# drafted learning text on stdin and prints ONE class name. It never prints the
# matched value: doing so would put the secret into the very transcript the scan
# exists to keep it out of.
#
#   echo "$text" | scripts/reflect-secret-scan.sh
#   → secret-scan: clean                     exit 0
#   → secret-scan: rejected (aws-key-id)     exit 1
#
#   scripts/reflect-secret-scan.sh --classes
#   → one class name per line, for the sync pin that keeps the prose fallback
#     in `commands/reflect.md` from drifting away from this list
#
# `commands/reflect.md` names this script and keeps a prose copy of the classes
# as the fallback for the case `commands/session-close.md` already documents:
# the plugin-cache glob resolving to nothing, where a bare `scripts/…` path is
# unreachable from a vault cwd. `scripts/mode-membership-rule-test.sh` pins the
# two lists together, so the script cannot gain a class the prose never names.
#
# Run from repo root (Makefile target `test` exercises it).

set -uo pipefail

# --- class definitions
#
# Each entry is `<class>|<ERE>`. Two anchors are load-bearing and both exist
# because the unanchored form over-matched in review:
#
#   `sk-` is a TOKEN PREFIX, never a substring. `task-`, `risk-` and `disk-`
#   all contain `sk-`, so a bare `sk-` rejects ordinary prose about task and
#   risk handling — the false positive found on `commands/reflect.md:29`.
#
#   the key-assignment shapes require a key at a token boundary, `=`, and a
#   value — 8+ non-space characters bare, or 4+ inside quotes so a value
#   containing spaces (`password="my secret pass"`) is not missed. Prose about a
#   `token=` field has nothing after the `=`, so it never fires — the other half
#   of that finding.
#
#   `bearer-token` matches the auth scheme in ANY casing, not just the canonical
#   `Bearer`: HTTP auth schemes are case-insensitive, so `authorization: bearer
#   <token>` is equally valid and a case-sensitive literal scans it clean.
CLASSES=(
	'private-key|-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----'
	'aws-key-id|(^|[^A-Za-z0-9])(AKIA|ASIA)[0-9A-Z]{16}'
	'github-token|(ghp_|gho_|ghu_|ghs_|ghr_|github_pat_)[A-Za-z0-9_]{20,}'
	'api-key|(^|[^A-Za-z0-9])sk-[A-Za-z0-9_-]{16,}'
	'stripe-key|(^|[^A-Za-z0-9])(sk|rk)_(live|test)_[A-Za-z0-9]{16,}'
	'slack-token|xox[baprs]-[A-Za-z0-9-]{10,}'
	'bearer-token|[Bb][Ee][Aa][Rr][Ee][Rr][[:space:]]+[A-Za-z0-9._~+/-]{20,}'
	'connection-string|[a-z][a-z0-9+.-]*://[^[:space:]/:@]+:[^[:space:]/@]+@'
	"key-assignment|(^|[^A-Za-z0-9_])(password|passwd|token|secret|api[_-]?key)[[:space:]]*=[[:space:]]*(\"[^\"]{4,}\"|'[^']{4,}'|[^[:space:]]{8,})"
	'jwt|eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}'
)

# base64-blob cannot be an ERE in the table above. A 40-hex git SHA is a valid
# base64 run, and conversations cite SHAs constantly, so a plain character-class
# match would reject every commit reference. The rule is therefore "a >= 40-char
# run of the base64 alphabet that is NOT a pure hex digest" — which is what a
# random key encodes to, and what a SHA does not.
base64_blob() {
	printf '%s' "$text" | grep -oE '[A-Za-z0-9+/]{40,}={0,2}' |
		grep -vE '^[0-9a-fA-F]+$' | head -1
}

if [ "${1:-}" = "--classes" ]; then
	for entry in "${CLASSES[@]}"; do
		printf '%s\n' "${entry%%|*}"
	done
	printf '%s\n' 'base64-blob'
	exit 0
fi

# Fail CLOSED on a read error. `text=$(cat)` alone leaves `text` empty when
# stdin cannot be read, and an empty string scans clean — the scan would report
# the one verdict that lets an unreviewed write through, on the one path where
# nothing was actually inspected.
if ! text=$(cat); then
	printf 'secret-scan: rejected (unreadable-input)\n'
	exit 1
fi

for entry in "${CLASSES[@]}"; do
	class=${entry%%|*}
	pattern=${entry#*|}
	# `grep -E … >/dev/null`, NOT `grep -q`: under `set -o pipefail` a `-q`
	# exits at the first match, the producer's remaining write takes SIGPIPE
	# (141), and the pipeline reports a PRESENT class as absent. Same trap the
	# membership checks in `scripts/mode-membership-rule-test.sh` document.
	if printf '%s' "$text" | grep -E -- "$pattern" >/dev/null 2>&1; then
		printf 'secret-scan: rejected (%s)\n' "$class"
		exit 1
	fi
done

if [ -n "$(base64_blob)" ]; then
	printf 'secret-scan: rejected (base64-blob)\n'
	exit 1
fi

printf 'secret-scan: clean\n'
exit 0
