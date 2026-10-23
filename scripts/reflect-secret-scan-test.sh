#!/usr/bin/env bash
#
# Pins `scripts/reflect-secret-scan.sh` — the mechanical half of
# `commands/reflect.md`'s NO-ASK sensitivity scan.
#
# The scan is the only thing standing between a NO-ASK reflect run and the
# vault, so it needs a probe that can fail. A prose list cannot be probed at
# all: a reworded list reads identically whether the scan works or not, which
# is exactly how the review found it both over- and under-broad at once.
#
# Both directions are asserted, and that is the point of this script. A fixture
# that only proved "secrets are caught" would pass for a scan that rejects
# everything — which under NO-ASK silently drops every legitimate learning with
# nobody to notice. A fixture that only proved "prose is kept" would pass for a
# scan that catches nothing. Neither direction alone distinguishes a working
# scan from a broken one; together they do.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and exercise the wrong tree.
cd "$ROOT" || exit 2

SCAN=scripts/reflect-secret-scan.sh

pass=0
fail=0

check() { # check <label> <want> <got>
	local label=$1 want=$2 got=$3
	if [ "$want" = "$got" ]; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ $label: want [$want], got [$got]" >&2
	fi
}

# scan <text> — the scan's own one-line verdict for <text>.
scan() {
	printf '%s' "$1" | bash "$SCAN" 2>/dev/null
}

# --- REJECTED: every documented class must actually fire.
# A class named in the prose but never exercised here is a class nobody has
# seen work. The fixture value is deliberately fake — this file is committed.
check "private key block is rejected" "secret-scan: rejected (private-key)" \
	"$(scan '-----BEGIN RSA PRIVATE KEY-----')"
check "AWS access key id is rejected" "secret-scan: rejected (aws-key-id)" \
	"$(scan 'creds AKIAIOSFODNN7EXAMPLE here')"
check "GitHub token is rejected" "secret-scan: rejected (github-token)" \
	"$(scan 'token ghp_abcdefghijklmnopqrstuvwxyz0123')"
check "sk- token at a boundary is rejected" "secret-scan: rejected (api-key)" \
	"$(scan 'key sk-projabcdefghijklmnop1234 end')"
check "Slack token is rejected" "secret-scan: rejected (slack-token)" \
	"$(scan 'hook xoxb-1234567890-abcdefghij')"
check "Bearer token is rejected" "secret-scan: rejected (bearer-token)" \
	"$(scan 'header Bearer abcdefghijklmnopqrstuvwxyz012345')"
check "connection string with credentials is rejected" "secret-scan: rejected (connection-string)" \
	"$(scan 'db postgres://appuser:s3cretpw@db.internal:5432/orders')"
check "assignment-shaped password is rejected" "secret-scan: rejected (key-assignment)" \
	"$(scan 'config password=hunter22 applied')"
check "JWT is rejected" "secret-scan: rejected (jwt)" \
	"$(scan 'auth eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U')"
check "base64 blob is rejected" "secret-scan: rejected (base64-blob)" \
	"$(scan 'blob dGhpcyBpcyBhIHRlc3QgYmxvYiB0aGF0IGlzIGxvbmcgZW5vdWdo')"

# --- KEPT: the false positives the review named must not fire.
# `sk-` is a token PREFIX. `task-`, `risk-` and `disk-` all contain it, and a
# bare `sk-` rejects ordinary prose about them — the over-broad half of the
# finding. This fixture is the exact string class that finding described.
# The token after `sk-` must be LONG enough to satisfy the `{16,}` length
# bound on its own, or this fixture passes for the wrong reason — it would then
# discriminate on length and leave the anchor itself unprobed. Verified
# 2026-10-23: with the anchor stripped to a bare `sk-`, the earlier short form
# ("task-runner") still read clean, so it proved nothing about the anchor.
check "task-/risk-/disk- prose is kept" "secret-scan: clean" \
	"$(scan 'the task-runner-handled-risk-free-disk-cleanup-this-week')"
# A 40-hex git SHA is a valid base64 run. Conversations cite SHAs constantly,
# so the base64 class must exclude pure hex or every commit reference is
# rejected — the reason that class is a post-filter and not an ERE.
check "a 40-hex git SHA is kept" "secret-scan: clean" \
	"$(scan 'merged in 4fe7320abc123def456789012345678901234567 today')"
# Prose ABOUT the key shapes must not fire on itself — otherwise the rule
# document cannot mention the rule.
check "prose about a token= field is kept" "secret-scan: clean" \
	"$(scan 'the token= field is documented in the guide')"
check "prose about Bearer tokens is kept" "secret-scan: clean" \
	"$(scan 'we discussed Bearer tokens in general terms')"

# --- the class list the sync pin consumes
# `--classes` is what `scripts/mode-membership-rule-test.sh` reads to require
# that every implemented class is also named in `commands/reflect.md`'s
# fallback prose. A class that vanishes from this list silently drops its pin.
check "--classes prints all ten classes" "10" \
	"$(bash "$SCAN" --classes | wc -l | tr -d ' ')"
check "--classes names base64-blob" "found" \
	"$(bash "$SCAN" --classes | grep -xF 'base64-blob' >/dev/null && echo found)"

echo "reflect-secret-scan: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
