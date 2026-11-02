#!/usr/bin/env bash
#
# Tests for session-self-exit.py — every branch that decides whether a worker
# may end its own process. The positive case is the only one that may kill a
# pane; each negative pins a condition that must refuse, because a wrong exit
# ends work in flight.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd) || exit 2
cd "$ROOT" || exit 2

SCRIPT=scripts/session-self-exit.py
SID=11111111-2222-3333-4444-555555555555

# Bind an ephemeral port instead of a fixed one. A hardcoded port collides with
# anything else already listening on the host, and the collision is silent: the
# stub server fails to bind, the test then reads whatever *is* on that port, and
# every "clear gate" case reads the 404 as "store unreadable". Observed
# 2026-10-20 — two unrelated processes held 18099 and `make test` was red on the
# host while green in CI.
free_port() {
	python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()'
}
PORT=$(free_port)
STORE_URL="http://127.0.0.1:$PORT"

TMP=$(mktemp -d)
trap 'kill "${SERVER_PID:-}" 2>/dev/null; wait "${SERVER_PID:-}" 2>/dev/null; rm -rf "$TMP"' EXIT

TASKS="$TMP/tasks"
SESSIONS="$TMP/sessions"
STATE="$TMP/attention"
mkdir -p "$TASKS" "$SESSIONS" "$STATE" "$TMP/store/api/1.0"

# A live tab session: the registry entry is what proves it owns a pane.
cat > "$SESSIONS/4242.json" <<EOF
{"pid": 4242, "sessionId": "$SID", "kind": "interactive"}
EOF

task() { # task <status> <phase> <open-box-count>
	local status=$1 phase=$2 boxes=${3:-0} i
	{
		echo "---"
		echo "status: $status"
		echo "phase: $phase"
		echo "claude_session_id: $SID"
		echo "---"
		echo ""
		echo "# Tasks"
		echo ""
		for ((i = 0; i < boxes; i++)); do echo "- [ ] open"; done
		echo "- [x] done"
	} > "$TASKS/Anchor.md"
}

handed_off_task() { # handed_off_task <status> <phase> <current-owner-sid>
	local status=$1 phase=$2 owner=$3
	{
		echo "---"
		echo "status: $status"
		echo "phase: $phase"
		echo "claude_session_id: $owner"
		echo "metrics_sessions:"
		echo "    - session_id: $SID"
		echo "      started_at: \"2026-10-20T11:51:47.321573+02:00\""
		echo "    - session_id: $owner"
		echo "      started_at: \"2026-10-20T11:52:00.000000+02:00\""
		echo "---"
		echo ""
		echo "# Tasks"
		echo ""
		echo "- [ ] open"
	} > "$TASKS/Anchor.md"
}

other_owner_task() { # other_owner_task <owner-sid> — no handoff record for $SID
	{
		echo "---"
		echo "status: in_progress"
		echo "phase: execution"
		echo "claude_session_id: $1"
		echo "metrics_sessions:"
		echo "    - session_id: $1"
		echo "      started_at: \"2026-10-20T11:52:00.000000+02:00\""
		echo "---"
		echo ""
		echo "# Tasks"
		echo ""
		echo "- [ ] open"
	} > "$TASKS/Anchor.md"
}

deferred_task() { # deferred_task <status> <phase> <defer-date> — the park shape
	local status=$1 phase=$2 defer=$3
	{
		echo "---"
		echo "status: $status"
		echo "phase: $phase"
		echo "claude_session_id: $SID"
		echo "defer_date: \"$defer\""
		echo "---"
		echo ""
		echo "# Tasks"
		echo ""
		echo "- [ ] open"
	} > "$TASKS/Anchor.md"
}

cards() { # cards <json>
	printf '%s' "$1" > "$TMP/store/api/1.0/attention"
}

# The hook's per-session mint log, which carries the provenance the store drops.
# `mint <item-id> <kind> <event>` writes one `open` line for it.
mint() {
	printf '{"type":"open","item_id":"%s","kind":"%s","event":"%s"}\n' "$1" "$2" "$3" \
		>> "$STATE/$SID.events.jsonl"
}

mint_reset() {
	rm -f "$STATE/$SID.events.jsonl"
}

# `mint_raw` writes a literal line, so a fixture can carry a record captured from
# a real `<sid>.events.jsonl` rather than one this test invented. `mint` above is
# the three-key shorthand; this is the fidelity one.
mint_raw() {
	printf '%s\n' "$1" >> "$STATE/$SID.events.jsonl"
}

# The 18 keys the Stop hook actually writes, captured 2026-11-02 from a live log
# with the values replaced. A record missing any of them would still satisfy
# `mint`, which is exactly the drift this fixture exists to catch.
real_closer() { # real_closer <item-id> <cleared-by-event>
	{
		printf '{"item_id":"%s","session_id":"%s","kind":"question","tool_name":"",' "$1" "$SID"
		printf '"detail":"👤 You: pick — 1. one · 2. two","context":"","options":[{"label":"one"}],'
		printf '"answer_cardinality":"single","questions":[],"pane":777,"cwd":"/tmp",'
		printf '"transcript":"/tmp/t.jsonl","host":"host","agent_id":"","ts":1762000000.0,'
		printf '"event":"Stop","cleared_by":{"event":"%s"},"type":"open"}\n' "$2"
	} >> "$STATE/$SID.events.jsonl"
}

# Park instants are computed, never hardcoded: a literal date is in the future
# when it is written and in the past a month later, which turns the positive
# case red on a calendar boundary with no code change.
FUTURE=$(python3 -c 'from datetime import datetime, timedelta, timezone; print((datetime.now(timezone.utc) + timedelta(hours=1)).isoformat())')
PAST=$(python3 -c 'from datetime import datetime, timedelta, timezone; print((datetime.now(timezone.utc) - timedelta(hours=1)).isoformat())')
TOMORROW=$(python3 -c 'from datetime import date, timedelta; print(date.today() + timedelta(days=1))')
# The same instant without an offset: ambiguous by construction, so it must refuse
# whatever the calendar says. Computed for the same reason as FUTURE/PAST.
NAIVE=$(python3 -c 'from datetime import datetime, timedelta, timezone; print((datetime.now(timezone.utc) + timedelta(hours=1)).strftime("%Y-%m-%dT%H:%M:%S"))')
# The `Z` form is what a vault-cli-rewritten date scalar carries; pin it so a
# regression in the designator handling cannot pass unnoticed.
FUTURE_Z=$(python3 -c 'from datetime import datetime, timedelta, timezone; print((datetime.now(timezone.utc) + timedelta(hours=1)).strftime("%Y-%m-%dT%H:%M:%SZ"))')

pass=0
fail=0

run() { # run <want-exit> <label> [extra env...]
	local want=$1 label=$2
	shift 2
	local got=0
	env -i PATH="$PATH" \
		SESSION_ID="$SID" SESSIONS_DIR="$SESSIONS" TASKS_ROOTS="$TASKS" \
		WEZTERM_PANE=777 ATTENTION_STORE_URL="$STORE_URL" ATTENTION_STATE_DIR="$STATE" "$@" \
		python3 "$SCRIPT" --dry-run > "$TMP/out" 2>&1 || got=$?
	if [ "$got" -eq "$want" ]; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ $label: want exit $want, got $got — $(cat "$TMP/out")" >&2
	fi
}

run_msg() { # run_msg <want-exit> <label> <expected-substring> [extra env...]
	local want=$1 label=$2 want_msg=$3
	shift 3
	local got=0
	env -i PATH="$PATH" \
		SESSION_ID="$SID" SESSIONS_DIR="$SESSIONS" TASKS_ROOTS="$TASKS" \
		WEZTERM_PANE=777 ATTENTION_STORE_URL="$STORE_URL" ATTENTION_STATE_DIR="$STATE" "$@" \
		python3 "$SCRIPT" --dry-run > "$TMP/out" 2>&1 || got=$?
	if [ "$got" -eq "$want" ] && grep -qF "$want_msg" "$TMP/out"; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ $label: want exit $want and '$want_msg', got $got — $(cat "$TMP/out")" >&2
	fi
}

run_live() { # run_live <want-exit> <label> [extra env...] — exercises the kill path, no --dry-run
	local want=$1 label=$2
	shift 2
	local got=0
	env -i PATH="$TMP/bin:$PATH" \
		SESSION_ID="$SID" SESSIONS_DIR="$SESSIONS" TASKS_ROOTS="$TASKS" \
		WEZTERM_PANE=777 ATTENTION_STORE_URL="$STORE_URL" ATTENTION_STATE_DIR="$STATE" "$@" \
		python3 "$SCRIPT" > "$TMP/out" 2>&1 || got=$?
	if [ "$got" -eq "$want" ]; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ $label: want exit $want, got $got — $(cat "$TMP/out")" >&2
	fi
}

# A failing `wezterm` proves the kill path reports a refusal rather than claiming success.
mkdir -p "$TMP/bin"
printf '#!/bin/sh\nexit 1\n' > "$TMP/bin/wezterm"
chmod +x "$TMP/bin/wezterm"

cards '[]'
python3 -m http.server "$PORT" --bind 127.0.0.1 --directory "$TMP/store" >/dev/null 2>&1 &
SERVER_PID=$!
server_up=0
for _ in $(seq 1 50); do
	if curl -sf "$STORE_URL/api/1.0/attention" >/dev/null 2>&1; then
		server_up=1
		break
	fi
	sleep 0.1
done
if [ "$server_up" -ne 1 ]; then
	echo "❌ stub attention store never answered on $STORE_URL" >&2
	exit 2
fi

# --- positive path
task completed done 0
run 0 "completed + phase done + zero boxes exits"

task aborted planning 3
run 0 "aborted exits even with open boxes"

# --- positive path: handed off to another session
OTHER=99999999-8888-7777-6666-555555555555
handed_off_task in_progress execution "$OTHER"
run_msg 0 "handed-off session exits while the task is still in progress" "SELF-EXIT: exit — handed off to $OTHER"

handed_off_task completed done "$OTHER"
run_msg 0 "handed-off session exits after the task goes terminal" "SELF-EXIT: exit — handed off to $OTHER"

# --- negative path: handed off but holding an open card
cards "[{\"item_id\":\"abc\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
handed_off_task in_progress execution "$OTHER"
run 10 "handed-off session with an open attention card refuses"
cards '[]'

# --- negative path: another owner and no handoff record for this session
other_owner_task "$OTHER"
run 10 "a task owned elsewhere with no handoff record refuses"

# --- negative path: the id appears only in the task body, never in frontmatter
{
	echo "---"
	echo "status: completed"
	echo "phase: done"
	echo "claude_session_id: $OTHER"
	echo "---"
	echo ""
	echo "# Tasks"
	echo ""
	echo "- [x] done"
	echo ""
	echo "Session $SID reviewed this task."
} > "$TASKS/Anchor.md"
run 10 "an id present only in the task body refuses"

# --- negative paths: the task is not terminal
task completed planning 0
run 10 "completed but phase is not done"

task completed done 2
run 10 "completed but open boxes remain"

task hold done 0
run 10 "hold is parked, not finished"

task in_progress execution 1
run_msg 10 "in_progress refuses" "task status is in_progress, not completed or aborted"

# --- positive path: the park shape — in_progress with a future defer_date
deferred_task in_progress execution "$FUTURE"
run_msg 0 "in_progress with a future defer_date exits" "SELF-EXIT: exit — Anchor.md is parked until $FUTURE"

# --- negative path: a headless SDK query has no pane of its own
# It inherits its spawner's $WEZTERM_PANE, so the park branch must not accept it
# even though a registry entry exists — exiting would close the spawner's tab.
printf '{"pid": 4242, "sessionId": "%s", "kind": "interactive", "entrypoint": "sdk-cli"}\n' "$SID" > "$SESSIONS/4242.json"
deferred_task in_progress execution "$FUTURE"
run_msg 10 "a headless SDK query with a parked task refuses" "no pane of its own"
printf '{"pid": 4242, "sessionId": "%s", "kind": "interactive"}\n' "$SID" > "$SESSIONS/4242.json"

# --- negative paths: a defer_date that is not a future instant
deferred_task in_progress execution "$PAST"
run_msg 10 "in_progress with an elapsed defer_date refuses" "has already elapsed"

deferred_task in_progress execution "$TOMORROW"
run_msg 10 "in_progress with a date-only defer_date refuses" "is not a future instant"

deferred_task in_progress execution "not-a-date"
run_msg 10 "in_progress with an unparseable defer_date refuses" "is not a future instant"

deferred_task in_progress execution "$NAIVE"
run_msg 10 "in_progress with an offset-less defer_date refuses" "is not a future instant"

deferred_task in_progress execution "$FUTURE_Z"
run_msg 0 "a Z-suffixed future defer_date exits" "is parked until $FUTURE_Z"

# --- a deferred task still cannot exit through an open gate
deferred_task in_progress execution "$FUTURE"
cards "[{\"item_id\":\"abc\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "a parked task with an open attention card refuses"
cards '[]'

# --- negative path: an open gate
task completed done 0
cards "[{\"item_id\":\"abc\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "open attention card for this session refuses"

cards "[{\"item_id\":\"abc\",\"producer_id\":\"someone-else\",\"state\":\"open\"}]"
run 0 "another session's card does not block"

# --- the closer-report carve-out
#
# A `Stop` closer is minted as a `question` with `cleared_by: {"event": "SessionEnd"}`,
# so only the session's own end clears it — refusing on it is a deadlock. The store
# cannot tell it from an ask (the watcher forwards neither `event` nor `cleared_by`),
# so the gate joins on `dedup_key` → the hook's own log.
task completed done 0

mint_reset
mint hookcloser question Stop
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"hookcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run_msg 0 "a Stop closer report does not block the exit" "1 closer report(s) ignored"

mint hookask question PreToolUse
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"hookcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"},
       {\"item_id\":\"store2\",\"dedup_key\":\"hookask\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "a real AskUserQuestion card still refuses alongside a closer"

mint_reset
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"hookcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "an unreadable mint log refuses (fail-closed)"

# A closer line must not excuse an item the log does not know about.
mint hookcloser question Stop
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"unknown-id\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "an item with no mint record refuses"

# --- fixture fidelity: the full captured record shape, and the older mint form
#
# `mint` writes only the three keys the implementation reads, so it agrees with
# the implementation by construction. These carry the whole captured key set.
mint_reset
real_closer realcloser SessionEnd
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"realcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run_msg 0 "a full captured closer record does not block the exit" "1 closer report(s) ignored"

# Older mints in the same append-only log declare `UserPromptSubmit`, not
# `SessionEnd`. The predicate keys on `event`, so both must be skipped.
mint_reset
real_closer oldcloser UserPromptSubmit
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"oldcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run_msg 0 "an older UserPromptSubmit closer record does not block either" "1 closer report(s) ignored"

# --- corruption: the gate must refuse by name, never die with a traceback
mint_reset
mint_raw 'not json at all'
real_closer realcloser SessionEnd
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"realcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run_msg 0 "a malformed line is skipped, not fatal" "1 closer report(s) ignored"

mint_reset
mint_raw 'null'
mint_raw '123'
mint_raw '[1,2]'
real_closer realcloser SessionEnd
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"realcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run_msg 0 "valid-JSON non-object lines are skipped, not fatal" "1 closer report(s) ignored"

# A hook killed mid-write leaves a truncated multi-byte sequence.
mint_reset
printf '\xe2\x82\n' >> "$STATE/$SID.events.jsonl"
real_closer realcloser SessionEnd
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"realcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run_msg 0 "a truncated UTF-8 line is skipped, not fatal" "1 closer report(s) ignored"

mint_reset
mint_raw 'null'
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"store1\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "a log with no usable record still refuses"

# The store is the other unguarded input: an object or array `dedup_key` is
# unhashable, and an uncaught TypeError would be a traceback, not a refusal.
mint_reset
real_closer realcloser SessionEnd
cards "[{\"item_id\":\"store1\",\"dedup_key\":{\"a\":1},\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "an unhashable dedup_key refuses by name, not with a traceback"

cards "[{\"item_id\":\"store1\",\"dedup_key\":[\"a\"],\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "an array dedup_key refuses by name, not with a traceback"

# The production default path, never otherwise exercised: with ATTENTION_STATE_DIR
# empty and a HOME carrying no attention state, the gate must still fail closed.
cards "[{\"item_id\":\"store1\",\"dedup_key\":\"realcloser\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
mkdir -p "$TMP/home"
run 10 "the default state dir with no mint log refuses" ATTENTION_STATE_DIR= HOME="$TMP/home"

mint_reset
cards '[]'

# --- negative path: the pane id is not a number
task completed done 0
run 10 "non-numeric pane id refuses" WEZTERM_PANE=abc

# --- negative path: kill-pane fails on a live run (no --dry-run)
task completed done 0
run_live 10 "a failing kill-pane reports a refusal, not success"

# --- negative path: the store cannot be read
task completed done 0
run 10 "unreachable attention store refuses" ATTENTION_STORE_URL="http://127.0.0.1:1"

# --- negative path: headless (no registry entry)
task completed done 0
mv "$SESSIONS/4242.json" "$TMP/4242.json.bak"
run 10 "headless session (no registry entry) refuses"
mv "$TMP/4242.json.bak" "$SESSIONS/4242.json"

# --- negative path: no anchor task
rm -f "$TASKS/Anchor.md"
run 10 "no task carrying the session id refuses"

if [ "$fail" -ne 0 ]; then
	echo "session-self-exit: $pass passed, $fail failed" >&2
	exit 1
fi
echo "session-self-exit: $pass passed"
