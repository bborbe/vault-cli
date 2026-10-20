---
status: approved
approved: "2026-10-20T14:08:55Z"
branch: dark-factory/bug-take-over-bind-drops-outgoing-session
---

## Summary

- Re-binding a task's `claude_session_id` to a new session overwrites the old owner and records it nowhere.
- The dropped session then matches no task, so no exit or close path can ever resolve it.
- `vault-cli work-on` already records every session it touches; only the direct `task set` path does not.
- The Vault UI reaches the same path, so a take-over started from the UI loses the owner too.
- Observed in production: session `60573438-cfc7-4b40-968f-065669208a4b` was displaced and stranded.

## Problem

A session's only link back to its work is the task frontmatter. `session-self-exit.py` and
`close-worker.py` both resolve a session by matching it against a task — the current
`claude_session_id` or a `metrics_sessions` entry. When a take-over overwrites
`claude_session_id` without recording the previous value, the outgoing session matches
nothing: `session-self-exit.py` refuses it with `no task carries claude_session_id <sid>` and
`close-worker.py` with `no task file carries this session id as claude_session_id`. The session
holds its spawn slot and cannot be closed by any manager.

`work-on` does not have this defect: it stamps the owner only when the field is empty and
always appends the incoming id to `metrics_sessions`. The direct set path has no equivalent.

## Goal

A `task set <task> claude_session_id <B>` that replaces a non-empty value with a different
value leaves the previous owner recorded on that task, so the exit and close paths can still
resolve it. A first bind onto an unowned task keeps its current behaviour, and the deliberate
clears are unchanged — the completion clear in `pkg/ops/complete.go` and the spawn-failure
clear in `work-on` both intend to drop the owner, not preserve it.

## Reproduction

Run against a scratch vault so no real task is touched.

```bash
rm -rf /tmp/vc-takeover-repro
mkdir -p /tmp/vc-takeover-repro/tasks

cat > /tmp/vc-takeover-repro/config.yaml <<'EOF'
current_user: bborbe
default_vault: scratch
vaults:
    scratch:
        path: /tmp/vc-takeover-repro
        name: scratch
        tasks_dir: tasks
EOF

printf -- '---\nstatus: in_progress\nphase: planning\nassignee: bborbe\n---\nTags: [[Task]]\n\n---\nScratch task.\n' \
  > "/tmp/vc-takeover-repro/tasks/Scratch Take-Over Repro.md"

CFG=/tmp/vc-takeover-repro/config.yaml
T="Scratch Take-Over Repro"

# 1. first bind: A
vault-cli --config $CFG task set "$T" claude_session_id AAAA --vault scratch
vault-cli --config $CFG task get "$T" metrics_sessions --vault scratch --output json

# 2. take over to B
vault-cli --config $CFG task set "$T" claude_session_id BBBB --vault scratch
vault-cli --config $CFG task get "$T" claude_session_id --vault scratch --output json
vault-cli --config $CFG task get "$T" metrics_sessions --vault scratch --output json
```

Observed on 2026-10-20, vault-cli `0.174.1`:

```
=== step 1: bind A ===
✅ Set claude_session_id=AAAA on: Scratch Take-Over Repro
{  "key": "metrics_sessions",  "name": "Scratch Take-Over Repro",  "value": ""}
=== step 2: take over to B ===
✅ Set claude_session_id=BBBB on: Scratch Take-Over Repro
{  "key": "claude_session_id",  "name": "Scratch Take-Over Repro",  "value": "BBBB"}
{  "key": "metrics_sessions",  "name": "Scratch Take-Over Repro",  "value": ""}
```

`AAAA` appears nowhere on the task after step 2.

Production instance of the same write, commit `9b0c4f5fc5` on the Personal vault:

```
-claude_session_id: 60573438-cfc7-4b40-968f-065669208a4b
+claude_session_id: bfd1ac2c-356d-42ab-b8fe-437c7a30ad18
+metrics_sessions:
+    - session_id: bfd1ac2c-356d-42ab-b8fe-437c7a30ad18
```

The incoming session was recorded; the outgoing one was dropped.

## Expected vs Actual

**Expected.** A take-over re-bind leaves the outgoing owner recorded on the task, in the same
field the exit and close paths already read (`metrics_sessions`). `metrics_sessions` is the
established mechanism: `work-on` appends to it, `session-self-exit.py` and `close-worker.py`
both resolve a handed-off session from it, and the sibling fix that shipped in vault-cli
`0.175.1` / supervisor `0.188.2` reads it.

**Actual.** The outgoing owner is dropped. `metrics_sessions` stays empty, so neither reader
can resolve the session and it can never exit or be closed.

## Why this is a bug

The union is documented in `docs/work-on-session-lifecycle.md:96` — "the set of session ids
that collide on a task is read from `claude_session_id` **and** every `metrics_sessions` id" —
and restated in `commands/task-status.md:83`. The exit and close resolvers read that union;
`docs/session-liveness.md` answers the narrower four-state liveness question, which keys on
`claude_session_id` alone. `docs/work-on-session-lifecycle.md` also states that a
`metrics_sessions` entry "accumulates and never replaces". A write that replaces the owner and
appends nothing contradicts both: it makes the union smaller than the history of sessions that
worked the task.

`pkg/ops/workon.go`'s `persistSessionAndMetrics` already implements the intended contract for
the other path — stamp only when empty, always append. The direct set path is the outlier.

## Acceptance Criteria

`$CFG` and `<task>` are the ones from `## Reproduction` (`--config /tmp/vc-takeover-repro/config.yaml --vault scratch`).

- [ ] **AC1 — a take-over records the outgoing owner.** After binding A then B to a scratch task, `vault-cli --config $CFG task get <task> claude_session_id --vault scratch --output json` returns `BBBB` and `vault-cli --config $CFG task get <task> metrics_sessions --vault scratch --output json` returns a value containing `AAAA`.
- [ ] **AC2 — a first bind records nothing.** On a scratch task with no `claude_session_id`, a single bind to `BBBB` leaves `vault-cli --config $CFG task get <task> metrics_sessions --vault scratch --output json` returning `"value": ""`.
- [ ] **AC3 — the appended entry is well-formed.** The `metrics_sessions` value from AC1 contains `session_id:AAAA` and a `started_at:` timestamp on the same entry, in the shape the CLI already prints for a populated field (`map[session_id:… started_at:…]`). The timestamp is non-zero — `started_at:0001-01-01T00:00:00Z` fails — and falls inside the command's run window.
- [ ] **AC4 — the outgoing id is appended, not the incoming one.** After the A→B take-over, `grep -c -- '- session_id: BBBB' "<task file>"` returns 0 and `grep -c -- '- session_id: AAAA' "<task file>"` returns 1. ⚠️ Anchor on the list-item form: a bare `session_id: BBBB` also matches the `claude_session_id: BBBB` line, so the unanchored pattern reads 1 and fails a correct fix.
- [ ] **AC5 — re-binding the same value appends nothing.** Setting `claude_session_id` to the value it already holds leaves `metrics_sessions` unchanged — `vault-cli --config $CFG task get <task> metrics_sessions --vault scratch --output json` returns the same value before and after.
- [ ] **AC6 — a regression test locks the behaviour.** A new test binds A, binds B, and asserts the outgoing id is in `metrics_sessions`; it fails against the pre-fix binary and passes after. `make test` exits 0, including the existing `persistSessionAndMetrics` tests.
- [ ] **AC7 — the whole suite is clean.** `make precommit` exits 0.
- [ ] **AC8 — the docs match the shipped behaviour.** `docs/work-on-session-lifecycle.md` no longer claims `task set` refuses `metrics_sessions` unconditionally: its sentence names the caller-supplied case as refused and the `claude_session_id` write as appending internally.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make test` — exits 0
- `make precommit` — exits 0
- the regression test named in AC6, run alone — exits 0 (it fails before the fix, so a pass proves the change landed)

### Operator-executable (runs on the host after the fix ships)

- The Reproduction block above, replayed against a freshly built binary: step 2's
  `metrics_sessions` value contains `AAAA`, and step 1's is still `""`.
- `TASKS_ROOTS=/tmp/vc-takeover-repro/tasks SESSIONS_DIR=/tmp/vc-takeover-repro/sessions WEZTERM_PANE=1 session-self-exit.py --session AAAA --dry-run` prints `SELF-EXIT: exit — handed off to BBBB, no open card, pane 1`, where `/tmp/vc-takeover-repro/sessions/4242.json` holds `{"pid": 4242, "sessionId": "AAAA", "kind": "interactive"}`. All three overrides are required: `TASKS_ROOTS` because the script otherwise scans the real vaults, `SESSIONS_DIR` because `owns_pane()` refuses without a registry record for AAAA, and `WEZTERM_PANE` because `main()` refuses a non-numeric pane.

## Desired Behavior

1. `task set <task> claude_session_id <B>` on a task currently owned by `A` appends
   `{session_id: A, started_at: <now>}` to `metrics_sessions` and then sets the field to `B`.
2. The same command on a task with no owner sets the field and appends nothing.
3. Setting the field to the value it already holds appends nothing.
4. Every caller of `task set claude_session_id` inherits the behaviour — the Vault UI binds
   through that same command, so it needs no change of its own.

## Assumptions

- `metrics_sessions` is the right home for the outgoing owner: it is the field the exit and
  close resolvers already read, and `work-on` already appends to it.
- `task set` reads the task before mutating it, so the previous value is available at write
  time (`pkg/ops/frontmatter.go` reads, then `SetField`, then `WriteTask`).
- Only the outgoing id is appended. `work-on` appends the incoming id because it is the writer
  that establishes ownership; a take-over only has to preserve what it displaces. Appending
  both would double-count the incoming session once `work-on` runs.
- The completion clear (`pkg/ops/complete.go`) and the spawn-failure clear in `work-on` are
  deliberate removals and stay out of scope.
- `task set <task> claude_session_id ""` is a replacement like any other: it appends the outgoing
  id before storing the empty value. `vault-cli task clear <task> claude_session_id` is a
  deliberate operator recovery path and is excluded — it deletes the key with no append.
- The internal append takes the previous id verbatim, with no UUID validation. The public
  `append-metrics-session` verb validates a UUID, and the reproduction deliberately uses `AAAA`
  / `BBBB`; routing the internal append through that verb would fail AC1.

## Constraints

- No new CLI verb or flag: the behaviour attaches to the existing `task set` path.
- `metrics_sessions` stays refused through `task set` / `task add` / `task remove` for
  callers. The append is internal to the session-id write, not a new caller-facing verb.
- `work-on`'s `persistSessionAndMetrics` keeps its current shape: stamp only when empty,
  always append the incoming id.
- Existing `metrics_sessions` entries are preserved in order; the new entry is appended.
- The task file is written once per command, not twice.
- `docs/work-on-session-lifecycle.md` ships updated in the same change: its sentence that
  `task set` refuses `metrics_sessions` must name the caller-supplied case as refused and the
  `claude_session_id` write as appending internally (AC8).
- `vault-cli task clear <task> claude_session_id` keeps its current behaviour — delete the key,
  no append. It is a deliberate recovery path, not a take-over.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Task file unreadable | command exits non-zero, file untouched | re-run once the file is readable; no partial state to repair |
| `metrics_sessions` holds a malformed entry | existing entries are preserved verbatim; the new entry is appended | inspect the file; the malformed entry is the operator's to fix |
| Two writers race on the same task | last write wins; both sessions may appear in `metrics_sessions` | entries accumulate by design; a duplicate is harmless to both readers |
| Bind to the same value | no entry appended | none needed |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Append the outgoing owner on a session-id re-bind, with tests | 1–4 | 1–8 | — |

Rationale: one layer (the task frontmatter's session-id write) and one observable behaviour.
A second prompt would have nothing of its own to change.
