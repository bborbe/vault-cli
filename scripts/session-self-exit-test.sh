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
PORT=18099
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

cards '[]'
python3 -m http.server "$PORT" --bind 127.0.0.1 --directory "$TMP/store" >/dev/null 2>&1 &
SERVER_PID=$!
for _ in $(seq 1 50); do
	curl -sf "$STORE_URL/api/1.0/attention" >/dev/null 2>&1 && break
	sleep 0.1
done

# --- positive path
task completed done 0
run 0 "completed + phase done + zero boxes exits"

task aborted planning 3
run 0 "aborted exits even with open boxes"

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
