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
mkdir -p "$TASKS" "$SESSIONS" "$TMP/store/api/1.0"

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

cards() { # cards <json>
	printf '%s' "$1" > "$TMP/store/api/1.0/attention"
}

pass=0
fail=0

run() { # run <want-exit> <label> [extra env...]
	local want=$1 label=$2
	shift 2
	local got=0
	env -i PATH="$PATH" \
		SESSION_ID="$SID" SESSIONS_DIR="$SESSIONS" TASKS_ROOTS="$TASKS" \
		WEZTERM_PANE=777 ATTENTION_STORE_URL="$STORE_URL" "$@" \
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
		WEZTERM_PANE=777 ATTENTION_STORE_URL="$STORE_URL" "$@" \
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
		WEZTERM_PANE=777 ATTENTION_STORE_URL="$STORE_URL" "$@" \
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
run 10 "in_progress refuses"

# --- negative path: an open gate
task completed done 0
cards "[{\"item_id\":\"abc\",\"producer_id\":\"$SID\",\"state\":\"open\"}]"
run 10 "open attention card for this session refuses"

cards "[{\"item_id\":\"abc\",\"producer_id\":\"someone-else\",\"state\":\"open\"}]"
run 0 "another session's card does not block"

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
