---
status: draft
---

# Scenario 006: a fleet-spawned run lands a metrics session

Validates that a real fleet spawn through `/vault-cli:work-on-task` records its session in the task's `metrics_sessions`, and preserves the entries already there.

## Why a scenario

The behaviour lives in `agents/work-on-task-assistant.md` § Session connect, a Claude Code agent definition with no test harness, executed only by a real LLM in a real spawned worker behind `mcp__supervisor__spawn_agent`. An integration test can exercise the verb; it can never exercise the call site firing. `scripts/metrics-append-call-site-test.sh` pins the invocation's presence, not that it fires — the script proves the artifact names the invocation, never that a spawned worker reaches the branch and runs it.

It is load-bearing for the fleet journey: the fresh-start fleet spawn is the manager layer's core journey, and metrics parity for it is the entire point of the change this scenario walks. `scenarios/005` covers `work-on` resume and does not cover this.

The regression risk is concrete: an edit to § Session connect silently drops the invocation and fleet runs vanish from metrics again, exactly as they are absent today. No unit test, no integration test and no call-site script can see that.

**This scenario is not sandboxed, by necessity.** The spawn uses the **installed** plugin and the **live** vault, so a temp-vault sandbox would be fiction. It mutates two clearly-named throwaway fixtures and today's daily note, and both are cleaned up below. The counterpart to `scenarios/005`'s TTY warning: this walk needs no TTY, but it does need the vault directory to be already trusted by Claude Code, because the first-run trust prompt would be an approval turn inside the spawned worker.

## Setup

```bash
VAULT_NAME=personal   # pinned explicitly; config list exposes no default marker
VAULT=$(vault-cli config list --output json | python3 -c "import json,sys; print(next(v['path'] for v in json.load(sys.stdin) if v['name']=='$VAULT_NAME'))")
TASKS_DIR=$(vault-cli config list --output json | python3 -c "import json,sys; print(next(v.get('tasks_dir') or '24 Tasks' for v in json.load(sys.stdin) if v['name']=='$VAULT_NAME'))")
PLUGIN_VER=$(claude plugin list | grep -A1 'vault-cli@vault-cli' | awk '/Version/{print $2}')
WINDOW_START=$(date -u +%Y-%m-%dT%H:%M:%SZ)
FIXTURE1="$VAULT/$TASKS_DIR/Scenario 006 Fixture A.md"
FIXTURE2="$VAULT/$TASKS_DIR/Scenario 006 Fixture B.md"
N=2   # the number of metrics_sessions entries authored into fixture B below
echo "VAULT=$VAULT TASKS_DIR=$TASKS_DIR PLUGIN_VER=$PLUGIN_VER WINDOW_START=$WINDOW_START N=$N"
echo "FIXTURE1=$FIXTURE1"; echo "FIXTURE2=$FIXTURE2"   # note both paths; you need them after the spawns
```

Fixture A — no `claude_session_id`, no `metrics_sessions`:

```bash
cat > "$FIXTURE1" <<'EOF'
---
page_type: task
status: in_progress
priority: 2
---
Tags: [[Task]]

---
Throwaway fixture for scenario 006. Delete after the walk.

# Success Criteria

- [ ] The spawn lands exactly one metrics_sessions entry.

# Tasks

- [ ] Run /vault-cli:work-on-task

# Definition of Done

- [ ] Entry present with the spawned session id.
EOF
```

Fixture B — no `claude_session_id`, and a `metrics_sessions` block holding `N` entries authored in the writer's own shape (a 4-space-indented block sequence, `started_at` quoted) so the "byte-identical before and after" comparison is against a canonical baseline, not against a re-serialised one:

```bash
cat > "$FIXTURE2" <<'EOF'
---
metrics_sessions:
    - session_id: 11111111-1111-4111-8111-111111111111
      started_at: "2026-09-01T08:00:00Z"
    - session_id: 22222222-2222-4222-8222-222222222222
      started_at: "2026-09-02T08:00:00Z"
page_type: task
priority: 2
status: in_progress
---
Tags: [[Task]]

---
Throwaway fixture for scenario 006. Delete after the walk.

# Success Criteria

- [ ] The spawn adds one entry and preserves the two already there.

# Tasks

- [ ] Run /vault-cli:work-on-task

# Definition of Done

- [ ] N+1 entries, the original N byte-identical.
EOF
```

- [ ] `WINDOW_START`, `PLUGIN_VER`, `VAULT`, `TASKS_DIR` and `N` are recorded in the result — a result without the plugin version and the window start cannot be compared against another walk, and the window is what makes the timestamp assertion decidable
- [ ] `grep -c 'claude_session_id' "$FIXTURE1"` returns `0` and `grep -c 'metrics_sessions' "$FIXTURE1"` returns `0`
- [ ] `grep -c 'claude_session_id' "$FIXTURE2"` returns `0` and `grep -c -- '- session_id:' "$FIXTURE2"` returns `$N`
- [ ] The vault directory is already trusted by Claude Code — otherwise the first-run "Do you trust the files in this folder?" gate is an approval turn **inside the spawned worker** and breaks the walk. **Prove it, don't assume it**: launch `claude` once in `$VAULT` and confirm no trust prompt appears, *in the same environment the walk will run in*. This is the single most likely cause of a false FAIL on this scenario

## Action

- [ ] `mcp__supervisor__spawn_agent(prompt='/vault-cli:work-on-task "Scenario 006 Fixture A"', …)` — **the prompt string unmodified, nothing chained onto it**. Record the session id `spawn_agent` reported for the run
- [ ] After fixture A's run completes, `mcp__supervisor__spawn_agent(prompt='/vault-cli:work-on-task "Scenario 006 Fixture B"', …)` — again the prompt string unmodified. Record the session id `spawn_agent` reported for this run too
- [ ] The verifier issues **no manual invocation of the verb** at any point in the window — no `vault-cli task append-metrics-session` by hand, on either fixture, before or after the spawns

The two exclusions together are what makes the landing entry attributable to the call site. Chaining the verb onto the spawn prompt is excluded by the unmodified prompt string; running it by hand is excluded by the verifier issuing none. Drop either and a passing walk proves nothing about § Session connect.

## Expected

Assert against the files on disk, not against scrollback.

- [ ] Fixture A carries **exactly one** `metrics_sessions` entry — `grep -c -- '- session_id:' "$FIXTURE1"` returns `1`
- [ ] That entry's `session_id` equals the id `spawn_agent` reported for fixture A's run
- [ ] That entry's `started_at` falls inside the spawn window — `≥` the spawned transcript's first timestamp and `≤` its last. Read both timestamps from the worker's own transcript; a clock-skewed host makes this the failing assertion while the entry itself stays valid
- [ ] Fixture B carries `N + 1` entries — `grep -c -- '- session_id:' "$FIXTURE2"` returns `3` with `N=2`
- [ ] The original `N` entries of fixture B are **byte-identical** to their pre-run form — compare the first two `- session_id:` blocks against the baseline captured in Setup, not a re-read of the same file
- [ ] Call-site provenance is an **artifact** check, not a log-absence one. Session-connect runs inside the worker's own session, so the invocation legitimately appears in that transcript and its presence there proves nothing. The check is instead against the **released** plugin in the load path: `grep -n 'append-metrics-session' ~/.claude/plugins/cache/vault-cli/vault-cli/$PLUGIN_VER/agents/work-on-task-assistant.md` returns `≥1` line, and that line sits **inside § Session connect**
- [ ] Note in the result that `status:` here flips from `draft` to `active` **only after the first successful walk**, and that the flip happens on the operator rung — not as part of authoring this file

## Cleanup

```bash
rm -f "$FIXTURE1" "$FIXTURE2"
# remove both fixtures' tracking lines from today's daily note
grep -n 'Scenario 006 Fixture' "$VAULT/60 Periodic Notes/Daily/$(date +%F).md"
```

- [ ] Both fixture files are removed, and the paths recorded in Setup are kept in the result so the operator can find them if a cleanup step was missed
- [ ] The fixtures' tracking lines are gone from today's daily note — the spawn's `/vault-cli:work-on-task` run adds a checkbox for each fixture, and both must be removed
