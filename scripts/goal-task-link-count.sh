#!/usr/bin/env bash
#
# Count task -> goal declarations that have no matching entry in the goal's
# `# Tasks` list.
#
# A task declares its parent in `goals:` frontmatter; the goal declares its
# children in a `# Tasks` checkbox list. The two directions are written by
# different code paths and drift silently. Measured 2026-09-19 in the Personal
# vault: 112 of 256 non-terminal declarations had no entry on the goal side.
#
# The drift is not cosmetic. `vault-cli task complete` flips the goal's checkbox
# for the completed task; with no checkbox it emits
# `checkbox not found for task %s in goal %s` (pkg/ops/complete.go) and the
# goal's progress silently stops tracking. Every manager sweep that cross-checks
# the two directions reports each miss as a finding, so at 44% the finding is
# noise and trains its reader to skim.
#
# Usage:  goal-task-link-count.sh <vault-root> [--tasks-dir D] [--goals-dir D]
# Exits:  0 = counted (always — a non-zero missing count is data, not an error)
#         2 = usage or input error
#
# The figures are DERIVED, never hardcoded. The vault root is the input and both
# totals come from parsing `goals:` frontmatter cross-checked against each goal's
# `# Tasks` membership. Running against a fixture whose true answer differs from
# the real vault's is what proves it measures rather than recites — see
# goal-task-link-count-test.sh.
#
# Definitions this script commits to, because each is a place the count could
# silently drift:
#   1. "Declaration" is one (task, goal) pair — a task naming two goals is two.
#   2. Terminal tasks are excluded. `completed` / `aborted` are done declaring;
#      counting them would inflate the denominator with rows nothing can fix.
#   3. A declaration whose goal file does not exist is EXCLUDED from the counts
#      and reported separately as `dangling parent`. It is a different defect
#      with a different remedy: neither candidate fix for this count applies,
#      since no link can be backfilled into a file that is not there, and a
#      rendered view of a missing goal renders nothing. Folding those into the
#      headline makes one number describe two problems while only one is fixable.
#   4. Wikilinks are normalised: `|alias`, `#heading` and any `dir/` prefix are
#      stripped, so `[[25 Tasks/Foo|bar]]` and `[[Foo]]` are the same title.
#   5. Membership is the UNION of every `# Tasks` section in a goal file. A goal
#      with a duplicated `# Tasks` heading exists in the corpus (found
#      2026-09-19: `Automate General Alerts Monitoring`), and a task listed under
#      either one is declared. Reading only the first heading — the obvious
#      implementation — silently moves 2 declarations from linked to missing and
#      changes the headline by 2, so the rule is stated rather than implied.
#
# The headline is rule-sensitive: on the 2026-09-19 corpus, rule 3 alone moves it
# by 14. A figure quoted without its rule set is therefore not reproducible, which
# is why the rules are printed on every run rather than documented once here.
#
# Two awk passes over the whole corpus, not one process per file: at ~2400 task
# files the per-file form spends most of its time in fork().

set -euo pipefail

TASKS_DIR="25 Tasks"
GOALS_DIR="24 Goals"
VAULT=""

usage() {
	echo "usage: $(basename "$0") <vault-root> [--tasks-dir D] [--goals-dir D]" >&2
}

while [ "$#" -gt 0 ]; do
	case "$1" in
	--tasks-dir)
		[ "$#" -ge 2 ] || { usage; exit 2; }
		TASKS_DIR=$2
		shift 2
		;;
	--goals-dir)
		[ "$#" -ge 2 ] || { usage; exit 2; }
		GOALS_DIR=$2
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	-*)
		echo "unknown option: $1" >&2
		usage
		exit 2
		;;
	*)
		if [ -n "$VAULT" ]; then
			echo "unexpected extra argument: $1" >&2
			usage
			exit 2
		fi
		VAULT=$1
		shift
		;;
	esac
done

if [ -z "$VAULT" ]; then
	usage
	exit 2
fi

TASKS="$VAULT/$TASKS_DIR"
GOALS="$VAULT/$GOALS_DIR"

for d in "$TASKS" "$GOALS"; do
	if [ ! -d "$d" ]; then
		echo "❌ directory not found: $d" >&2
		exit 2
	fi
done

shopt -s nullglob
TASK_FILES=("$TASKS"/*.md)
GOAL_FILES=("$GOALS"/*.md)
shopt -u nullglob

if [ "${#TASK_FILES[@]}" -eq 0 ] || [ "${#GOAL_FILES[@]}" -eq 0 ]; then
	echo "❌ no .md files under $TASKS or $GOALS" >&2
	exit 2
fi

# --- pass 1: task file -> status -------------------------------------------
# Status gates the denominator (definition 2). Printed when the frontmatter
# closes, and again at END for the final file — BSD awk has no ENDFILE, so the
# last file has no "next file" to flush it.
task_status() {
	awk '
		FNR == 1 {
			if (NR > 1) print prev "\t" st
			prev = FILENAME; st = ""; fm = 0
		}
		FNR == 1 && /^---[[:space:]]*$/ { fm = 1; next }
		fm && /^---[[:space:]]*$/ { fm = 0; next }
		fm && /^status:/ { st = $0; sub(/^status:[[:space:]]*/, "", st) }
		END { if (prev != "") print prev "\t" st }
	' "$@"
}

# --- pass 2: task file -> declared goal title ------------------------------
# One line per declaration, so a task naming two goals yields two lines.
task_goals() {
	awk '
		function norm(s) {
			sub(/\|.*/, "", s)   # [[Title|alias]]
			sub(/#.*/, "", s)    # [[Title#Heading]]
			sub(/.*\//, "", s)   # [[dir/Title]]
			return s
		}
		FNR == 1 { prev = FILENAME; fm = 0; g = 0 }
		FNR == 1 && /^---[[:space:]]*$/ { fm = 1; next }
		fm && /^---[[:space:]]*$/ { fm = 0; g = 0; next }
		fm && /^goals:/ { g = 1; next }
		fm && g && /^[[:space:]]+-/ {
			line = $0
			if (match(line, /\[\[[^]]+\]\]/)) {
				print prev "\t" norm(substr(line, RSTART + 2, RLENGTH - 4))
			}
			next
		}
		fm && g { g = 0 }
	' "$@"
}

# --- pass 3: goal file -> titles listed under `# Tasks` ---------------------
# Section-scoped: a wikilink anywhere else in the goal (Related, Impact, a
# neighbouring section) must not count as membership. Every `[[...]]` on a line
# is taken, not just the first, so `- [ ] [[A]] and [[B]]` yields both.
goal_links() {
	awk '
		function norm(s) {
			sub(/\|.*/, "", s)
			sub(/#.*/, "", s)
			sub(/.*\//, "", s)
			return s
		}
		FNR == 1 { prev = FILENAME; s = 0 }
		/^# Tasks[[:space:]]*$/ { s = 1; next }
		s && /^# / { s = 0; next }
		s {
			line = $0
			while (match(line, /\[\[[^]]+\]\]/)) {
				print prev "\t" norm(substr(line, RSTART + 2, RLENGTH - 4))
				line = substr(line, RSTART + RLENGTH)
			}
		}
	' "$@"
}

status_tsv=$(task_status "${TASK_FILES[@]}")
goals_tsv=$(task_goals "${TASK_FILES[@]}")
links_tsv=$(goal_links "${GOAL_FILES[@]}")

# --- join ------------------------------------------------------------------
# Single awk over the three streams, and the ORDER MATTERS: membership (`L`)
# must be read before declarations (`G`) are tested against it. Emitting G first
# leaves gfile/member empty and reports every declaration as missing — which is
# exactly the shape of the defect being measured, so it fails silently green.
#
# Everything is sorted before printing: awk array iteration order is
# unspecified, and "run twice, same result" is a property this script exists to
# demonstrate.
emit() { # emit <prefix> <tsv>  — skips empty streams, whose printf would emit a blank line
	[ -n "$2" ] || return 0
	printf '%s\n' "$2" | sed "s/^/$1\t/"
}

result=$(
	{
		# F: the goal files that EXIST. Emitted independently of their links —
		# deriving existence from the link stream makes a goal with an empty
		# `# Tasks` section indistinguishable from a goal file that is absent,
		# and mislabels every declaration to it as the wrong defect.
		emit F "$(printf '%s\n' "${GOAL_FILES[@]}")"
		emit L "$links_tsv"
		emit S "$status_tsv"
		emit G "$goals_tsv"
	} | awk -F'\t' '
		function base(p) { n = split(p, a, "/"); return a[n] }
		$1 == "S" { st[$2] = $3; next }
		$1 == "F" {
			gname = base($2); sub(/\.md$/, "", gname); gfile[gname] = 1; next
		}
		$1 == "L" {
			gname = base($2); sub(/\.md$/, "", gname)
			member[gname "\t" $3] = 1; next
		}
		$1 == "G" {
			f = $2; g = $3
			# terminal tasks do not declare (rule d)
			if (st[f] == "completed" || st[f] == "aborted") next
			title = base(f); sub(/\.md$/, "", title)
			# rule (a): a dangling parent is a DIFFERENT defect with a different
			# remedy — no link can be backfilled into a file that is not there —
			# so it is counted separately and kept out of this denominator.
			if (!(g in gfile)) { dangling++; next }
			total++
			if ((g "\t" title) in member) { linked++; next }
			missing++; bygoal[g]++
		}
		END {
			printf "total\t%d\n", total
			printf "linked\t%d\n", linked
			printf "missing\t%d\n", missing
			printf "dangling\t%d\n", dangling
			for (g in bygoal) printf "goal\t%d\t%s\n", bygoal[g], g
		}
	' | sort -t$'\t' -k1,1
)

val() { printf '%s\n' "$result" | awk -F'\t' -v k="$1" '$1==k {print $2; exit}'; }

total=$(val total)
linked=$(val linked)
missing=$(val missing)
dangling=$(val dangling)

total=${total:-0}
linked=${linked:-0}
missing=${missing:-0}
dangling=${dangling:-0}

pct="0.0"
if [ "$total" -gt 0 ]; then
	pct=$(awk -v m="$missing" -v t="$total" 'BEGIN { printf "%.1f", 100 * m / t }')
fi

# The numbers are only meaningful next to the rules that produced them: the
# headline moves by 14 on rule (a) alone, so a figure quoted without its rule
# set is not reproducible. Printed every run, not documented once.
cat <<'RULES'
counting rules (canonical, decided 2026-09-19):
  a. a declaration naming a goal file that does NOT exist is EXCLUDED from the
     counts below and reported separately as `dangling parent` — it is a
     different defect with a different remedy, since no link can be backfilled
     into a file that is not there
  b. membership is the UNION of every `# Tasks` section in the goal file
  c. a wikilink outside `# Tasks` is a mention, not a declaration
  d. terminal tasks (status `completed` / `aborted`) are excluded entirely
  e. `|alias`, `#heading` and `dir/` wikilink forms all resolve to one title
RULES

echo
echo "vault:        $VAULT"
echo "tasks dir:    $TASKS_DIR"
echo "goals dir:    $GOALS_DIR"
echo "declarations: $total"
echo "linked:       $linked"
echo "missing:      $missing (${pct}%)"
echo "dangling parent (excluded by rule a): $dangling"
echo
echo "missing per goal:"

printf '%s\n' "$result" |
	awk -F'\t' '$1=="goal" {printf "%5d  %s\n", $2, $3}' |
	sort -rn -k1,1 -k2,2

exit 0
