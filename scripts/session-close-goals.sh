#!/usr/bin/env bash
#
# Print the open-goal status lines `session-close` Phase 4.5 owes the operator,
# so the closer pastes them instead of composing them.
#
# Phase 4.5 decides per touched goal whether a decision is owed: a goal with an
# open task is report-only (one information line), a closable goal routes to its
# live manager or to the operator. The DECISION is conversational — only the
# session knows which goals it touched. Everything after that decision is
# mechanical: read `status`, count SC, find a manager, probe it. This script owns
# the mechanical half.
#
# Why it exists: the information line was a free sentence in Phase 9's summary,
# and a session can omit a sentence. Measured 2026-10-20 — session `44db8e09`
# ran the goal-status lookup itself, received `status=in_progress`, and printed a
# closer with no goal line at all. Neither the new report-only line nor the old
# outstanding item appeared. A line a session must COMPOSE is droppable; a line a
# script PRINTS and the closer must reproduce is checkable.
#
# Usage:  session-close-goals.sh <vault-root> --goal <title> [--goal <title> ...]
# Exits:  0 = printed (possibly nothing — no in-scope goal is a normal answer)
#         2 = usage or input error
#
# Definitions this script commits to, because each is a place the answer could
# silently drift:
#   1. Only `in_progress` goals are reported. Every other status is the managing
#      session's own write and is not this session's business.
#   2. "Open task" means a task listed under the goal's `# Tasks` whose file
#      status is not `completed`. `aborted` counts as open here: the goal is not
#      drained, so a decision is still owed about the goal itself.
#   3. Manager resolution is goal page first, then a topic page whose `## Goals`
#      section lists the goal. It never falls through to the Fleet Manager —
#      that chain answers "who owns this task", not "who can be told a goal is
#      closable".
#   4. The id is a LEAD. Only `session-liveness.py --check` exit 0 counts as a
#      live manager; 1/2/3/4 and no id all read `no live manager`. An
#      indeterminate id is never printed as a manager.
#   5. `n/m` is ticked SC over total SC, read from the goal's
#      `# Success Criteria` section.
#
# Output — exactly one line per in-scope goal, in input order:
#   goal <title>: in_progress, <n>/<m> SC — managed by <manager>
#   goal <title>: in_progress, <n>/<m> SC — no live manager
#   closable goal <title>: in_progress, <n>/<m> SC — managed by <manager>
#   closable goal <title>: in_progress, <n>/<m> SC — no live manager
#
# The `closable ` prefix is load-bearing: Phase 4.5 routes a closable goal to its
# manager or the operator, and only the open-goal form is report-only. A script
# that printed one form for both would erase the distinction the phase exists for.

set -euo pipefail

GOALS_DIR="24 Goals"
TOPICS_DIR="23 Topics"
VAULT=""

usage() {
  echo "usage: session-close-goals.sh <vault-root> --goal <title> [--goal <title> ...]" >&2
  exit 2
}

[ $# -ge 3 ] || usage
VAULT="$1"; shift
[ -d "$VAULT" ] || { echo "error: vault root not a directory: $VAULT" >&2; exit 2; }

TITLES=()
while [ $# -gt 0 ]; do
  case "$1" in
    --goal)
      [ $# -ge 2 ] || usage
      TITLES+=("$2"); shift 2 ;;
    *) usage ;;
  esac
done
[ ${#TITLES[@]} -gt 0 ] || usage

# `SESSION_LIVENESS` exists for the test suite: a fixture must be able to decide
# the probe's verdict, and the shipped path resolves to whatever the operator's
# machine happens to have installed. Unset in every real run.
LIVENESS="${SESSION_LIVENESS:-$(ls -d "$HOME"/.claude/plugins/cache/claude-supervisor/*/*/scripts 2>/dev/null | sort -V | tail -1)/session-liveness.py}"

field() { awk -v k="$1" 'index($0,k":")==1{sub(/^[^:]*:[[:space:]]*/,"");gsub(/^"|"$/,"");print;exit}' "$2"; }

sc_counts() {
  awk '
    /^# /{sec=substr($0,3); gsub(/^[ \t]+|[ \t]+$/,"",sec)}
    sec=="Success Criteria" && /^[ \t]*-[ \t]*\[[ x]\]/{
      total++
      if ($0 ~ /^[ \t]*-[ \t]*\[x\]/) ticked++
    }
    END{printf "%d/%d", ticked+0, total+0}
  ' "$1"
}

open_tasks() {
  awk '
    /^# /{sec=substr($0,3); gsub(/^[ \t]+|[ \t]+$/,"",sec)}
    sec=="Tasks"{
      line=$0
      while (match(line, /\[\[[^]]+\]\]/)) {
        t=substr(line, RSTART+2, RLENGTH-4)
        sub(/\|.*/,"",t); sub(/#.*/,"",t); sub(/^.*\//,"",t)
        print t
        line=substr(line, RSTART+RLENGTH)
      }
    }
  ' "$1" | awk '!seen[$0]++'
}

task_status() {
  local f="$VAULT/25 Tasks/$1.md"
  [ -f "$f" ] || { echo ""; return; }
  field status "$f"
}

# manager_id_for <goal-title> — goal page, then a topic page listing the goal.
manager_id_for() {
  local title="$1" f id
  f="$VAULT/$GOALS_DIR/$title.md"
  if [ -f "$f" ]; then
    id="$(field manager_session_id "$f")"
    if [ -n "$id" ]; then printf '%s' "$id"; return; fi
  fi
  while IFS= read -r tf; do
    [ -f "$tf" ] || continue
    awk -v want="$title" '
      /^## /{sec=substr($0,4); gsub(/^[ \t]+|[ \t]+$/,"",sec)}
      sec=="Goals" && index($0, want)>0 {found=1; exit}
      END{exit found?0:1}
    ' "$tf" || continue
    id="$(field manager_session_id "$tf")"
    if [ -n "$id" ]; then printf '%s' "$id"; return; fi
  done < <(find "$VAULT/$TOPICS_DIR" -name '*.md' 2>/dev/null)
}

# manager_name <id> — the registry name, or empty when the id is not live.
manager_name() {
  local id="$1"
  [ -n "$id" ] && [ -f "$LIVENESS" ] || return 0
  python3 "$LIVENESS" --check "$id" >/dev/null 2>&1 || return 0
  python3 "$LIVENESS" --list --json 2>/dev/null | python3 -c '
import json,sys
want=sys.argv[1]
try: d=json.load(sys.stdin)
except Exception: raise SystemExit(0)
rows = d if isinstance(d,list) else d.get("sessions", d)
if isinstance(rows,dict): rows=list(rows.values())
for r in rows:
    if r.get("sessionId")==want:
        print(r.get("name") or want); break
' "$id"
}

for title in "${TITLES[@]}"; do
  gf="$VAULT/$GOALS_DIR/$title.md"
  [ -f "$gf" ] || continue
  [ "$(field status "$gf")" = "in_progress" ] || continue

  sc="$(sc_counts "$gf")"
  opens=0
  while IFS= read -r t; do
    [ -n "$t" ] || continue
    [ "$(task_status "$t")" = "completed" ] || opens=$((opens+1))
  done < <(open_tasks "$gf")

  mid="$(manager_id_for "$title")"
  mname="$(manager_name "$mid")"

  if [ "$opens" -gt 0 ]; then prefix="goal"; else prefix="closable goal"; fi
  if [ -n "$mname" ]; then
    printf '%s %s: in_progress, %s SC — managed by %s\n' "$prefix" "$title" "$sc" "$mname"
  else
    printf '%s %s: in_progress, %s SC — no live manager\n' "$prefix" "$title" "$sc"
  fi
done
