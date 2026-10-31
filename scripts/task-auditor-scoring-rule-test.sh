#!/usr/bin/env bash
#
# Pins the task-auditor's arithmetic scoring rule to the file that enforces it.
#
# The rule has NO code. It lives in the prose the auditor reads, so the prose IS
# the implementation and a silent edit to it is a behaviour regression — the same
# reasoning scripts/necessity-rule-test.sh gives for its own rule, and the reason
# this is a presence test rather than a fixture: nothing else can catch the rule
# being dropped.
#
# The rule exists because a band-only rubric produced two different scores from one
# byte-identical file. Each marker below is a fact that band-first scoring cannot
# satisfy, so presence is both necessary and cheap to assert:
#
#   - the section heading names arithmetic and forbids a band picked first;
#   - the −2.0 term is the adversarial-laziness rule §14 already stated, listed
#     here so the derivation is the file's ONLY scoring rule (two competing
#     derivations is the defect this rule set removes);
#   - the clamp bounds the total;
#   - the output template must carry the derivation slot, or an auditor following
#     <output_format> literally emits a bare number and defeats the rule;
#   - the retired `**Scoring guidance**` heading must stay gone.
#
# The self-check at the end strips one marker from a throwaway copy and requires
# that copy to be reported: a check that reported nothing would satisfy every
# presence assertion without measuring anything.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

AGENT=agents/task-auditor.md

# Each marker is a fact band-first scoring cannot satisfy.
MARKERS=(
	'Scoring — arithmetic over your own findings, never a band picked first.'
	'each **Critical Issue** you reported: **−1.0**'
	'each **MAJOR** finding you reported, **except a DoD gap — the DoD term below covers that one**: **−0.5**'
	'each **adversarial-laziness FAIL**: **−2.0**'
	'each **Task Scope Fit smell**, **per smell**, once 3+ apply: **−0.5**'
	'**shipping-class or complex** task **missing** DoD: **−1.0**; **weak** DoD: **−0.5**; a **simple non-shipping** task without DoD: **0**'
	'Clamp to **[1, 10]**'
	'A report carrying any Critical Issue cannot land in `9-10`'
	'One defect, one deduction.'
	'Smell 8 is the exception'
	'A smell 8 that this term does not count'
	'| 9-10 | Excellent |'
	'| 7-8 | Good |'
	'| 5-6 | Needs Improvement |'
	'| 3-4 | Needs Improvement |'
	'| 1-2 | Significant Issues |'
	'**Score**: X/10 (<derivation>)'
)

# The heading this rule replaced. Its return would mean band-first scoring came
# back, and every MARKERS assertion above would still pass.
RETIRED='**Scoring guidance**'

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

# missing_markers <file> — how many of MARKERS the file does not carry, plus one
# if the retired heading is back. A count rather than a boolean so the self-check
# can assert an exact expected number.
missing_markers() {
	local f=$1 n=0 m
	[ -f "$f" ] || { echo $(( ${#MARKERS[@]} + 1 )); return; }
	for m in "${MARKERS[@]}"; do
		# NOT `grep -q`: under `set -o pipefail` a short-circuiting grep can report
		# a present marker as missing. Read to EOF instead.
		grep -F -- "$m" "$f" >/dev/null || n=$((n + 1))
	done
	grep -F -- "$RETIRED" "$f" >/dev/null && n=$((n + 1))
	echo "$n"
}

check "real agent carries every scoring marker" 0 "$(missing_markers "$AGENT")"

# Self-check: strip the −2.0 term from a throwaway copy. Exactly one marker must
# go missing — a checker that reported 0 here measures nothing, and one that
# reported everything is not reading the file it was handed.
tmp=$(mktemp) || { echo 'task-auditor-scoring-rule FAILED: mktemp failed' >&2; exit 2; }
trap 'rm -f "$tmp"' EXIT
grep -vF -- 'each **adversarial-laziness FAIL**: **−2.0**' "$AGENT" > "$tmp"
check "self-check: a stripped marker is detected" 1 "$(missing_markers "$tmp")"

# Self-check: the retired heading must be reported when reintroduced.
tmp2=$(mktemp) || { echo 'task-auditor-scoring-rule FAILED: mktemp failed' >&2; exit 2; }
trap 'rm -f "$tmp" "$tmp2"' EXIT
{ cat "$AGENT"; echo "$RETIRED"; } > "$tmp2"
check "self-check: a reintroduced band heading is detected" 1 "$(missing_markers "$tmp2")"

if [ "$fail" -ne 0 ]; then
	echo "task-auditor-scoring-rule FAILED: $fail of $((pass + fail)) checks" >&2
	exit 1
fi

echo "task-auditor-scoring-rule ok: $pass checks"
