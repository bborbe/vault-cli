---
status: prompted
approved: "2026-11-02T21:56:12Z"
generating: "2026-11-02T22:08:40Z"
prompted: "2026-11-02T22:08:40Z"
branch: dark-factory/bug-goal-topic-bind-drops-mid-turn-session
---

## Summary

- A goal or topic `work-on` checks that the page has no session owner, then runs a headless turn that can last up to 30 minutes.
- If another session claims the page during that turn, the bind overwrites it when the turn ends and records it nowhere.
- Goal and topic pages have no session history at all, so the displaced session cannot be recorded even in principle.
- The task path already keeps a history of every session that worked it; goals and topics have no equivalent.
- The fix keeps the session that claimed the page as its owner and records the binding session in the page's history — the task path's ownership rule, with a narrower append rule.

## Problem

A goal or topic `work-on` reads the page, sees `claude_session_id` empty, starts a headless turn,
and only after the turn exits re-reads the page and stamps its own session id
(`persistGoalSessionID` in `pkg/ops/goal_workon.go`, `persistTopicSessionID` in
`pkg/ops/topic_workon.go`). Both persist functions overwrite the field unconditionally. The
turn runs for up to `sessionTurnTimeout` (30m), and other writers stamp the same field in that
window: a second concurrent `goal work-on`, an interactive `/vault-cli:work-on-goal` whose
assistant sets `claude_session_id` on an empty goal through `goal set`, or — most often — the
headless child itself, whose own `/vault-cli:work-on-goal` session-connect stamps its own id
(`agents/work-on-goal-assistant.md:92-109`). When the bind's persist
lands, that session is overwritten. Goal and topic frontmatter carry no `metrics_sessions`
field, so nothing records it, and no reader can find it afterwards.

## Goal

Goal and topic binds take the task path's ownership rule: stamp `claude_session_id` only when
it is still empty at persist time (the append rule differs — see Assumptions). When the persist finds the field holding a different non-empty
session, that session keeps the stamp and the binding session is recorded as a `metrics_sessions`
entry (`session_id` + `started_at`). A persist that finds the field empty stamps its own id and
adds no entry; one that finds its own id writes nothing new. `work-on` still returns the binding
session's id to its caller in every case.

## Non-goals

- `vault-cli goal set` / `topic set claude_session_id` — a separate writer, tracked on its own; a
  setter-level append would also fire on the Vault UI's display-name→uuid resolve.
- The task path (`persistSessionAndMetrics`, `task set claude_session_id`) — fixed by spec 063.
- Teaching `session-self-exit.py` or any close path to resolve a goal or topic anchor.
- Backfilling pages displaced before this fix.
- Refusing a caller-supplied scalar `metrics_sessions` through `goal set` / `topic set` — the task
  path's refusal (`pkg/ops/metrics_session_write.go`) has no goal/topic twin yet; recorded here as
  a known gap.

## Reproduction

A stub `claude_script` stands in for the headless turn so the reproduction needs no real
Claude session. Run against a scratch vault so no real page is touched.

```bash
R=/tmp/vc-goal-rebind-repro
rm -rf $R; mkdir -p $R/vault/goals $R/vault/tasks $R/vault/topics $R/bin

cat > $R/config.yaml <<EOF
current_user: bborbe
default_vault: scratch
vaults:
    scratch:
        path: $R/vault
        name: scratch
        tasks_dir: tasks
        goals_dir: goals
        topics_dir: topics
        claude_script: $R/bin/stub-claude
EOF

cat > $R/bin/stub-claude <<'EOF'
#!/bin/bash
# A headless turn during which another session (X) stamps the goal, then a valid result.
SID=""; while [ $# -gt 0 ]; do [ "$1" = "--session-id" ] && SID="$2"; shift; done
G="/tmp/vc-goal-rebind-repro/vault/goals/Scratch Rebind Goal.md"
vault-cli --config /tmp/vc-goal-rebind-repro/config.yaml --vault scratch \
  goal set "Scratch Rebind Goal" claude_session_id XXXX-interactive >/dev/null
echo "mid-turn (session $SID): $(grep '^claude_session_id' "$G")" >> /tmp/vc-goal-rebind-repro/stub.log
printf '{"type":"result","session_id":"%s","num_turns":1,"is_error":false,"result":"ok"}\n' "$SID"
EOF
chmod +x $R/bin/stub-claude

printf -- '---\nstatus: in_progress\nphase: planning\nassignee: bborbe\n---\nTags: [[Goal]]\n\n---\nScratch goal.\n' \
  > "$R/vault/goals/Scratch Rebind Goal.md"

vault-cli --config $R/config.yaml --vault scratch goal work-on "Scratch Rebind Goal" --mode headless
cat $R/stub.log
grep -E '^(claude_session_id|metrics_sessions)' "$R/vault/goals/Scratch Rebind Goal.md"
grep -c XXXX "$R/vault/goals/Scratch Rebind Goal.md"
```

Observed on 2026-11-02, vault-cli `v0.186.0`:

```
session_id: 6fe368aa-d533-4a60-acd9-6314d9af5b15
mid-turn (session 6fe368aa-d533-4a60-acd9-6314d9af5b15): claude_session_id: XXXX-interactive
claude_session_id: 6fe368aa-d533-4a60-acd9-6314d9af5b15
0
```

The stub's log proves `XXXX-interactive` was on the page mid-turn; after the bind it appears
nowhere. A second `goal work-on` on the now-owned goal invokes the stub 0 times and leaves the
stamp unchanged — the existing-owner guard in `handleClaudeSession` — so the loss happens only
inside the turn window.

## Expected vs Actual

**Expected.** No session is lost, and the page keeps naming the session that claimed it.
`docs/work-on-session-lifecycle.md:65` states "**The entry accumulates; it never replaces.**", and
`:100-101` defines the sessions tied to a task as `claude_session_id` **and** every
`metrics_sessions` id; this spec extends that union to goals and topics. The task path's
`persistSessionAndMetrics` (`pkg/ops/workon.go`) stamps the owner only when the field is empty. `claude_session_id` is what
`/supervisor:open` and the Vault UI resume, so it must not move to a turn that has already
finished.

**Actual.** The displaced session is overwritten and recorded nowhere; goal and topic pages have
no `metrics_sessions` at all.

## Why this is a bug

The persist was designed (spec 041) to land only after the turn, so an id on disk means a
resumable session. That moved the write to the end of a long window but kept the
unconditional overwrite, so any write inside the window is silently lost. A session that matches
no page record cannot be found by anything that resolves sessions from the vault.

## Acceptance Criteria

`$R`, the stub and the scratch goal are those from `## Reproduction`; the topic cases use a
scratch topic page under `$R/vault/topics/` and `topic set` / `topic work-on` in the stub.

- [ ] **AC1 — a goal keeps the claimant and records the binder.** After the reproduction, the goal file reads `claude_session_id: XXXX-interactive`, and `grep -c -- '- session_id: <printed id>' "<goal file>"` returns 1, where `<printed id>` is the `session_id:` line `goal work-on` printed. ⚠️ Anchor on the list-item form `- session_id:` so the `claude_session_id:` line cannot match.
- [ ] **AC2 — the entry records turn start.** With `sleep 2` added at the top of the stub and the stub logging `date -u +%Y-%m-%dT%H:%M:%SZ` before its `set`, the AC1 entry's `started_at` parses as RFC3339, is not `0001-01-01T00:00:00Z`, is at or after `date -u` taken immediately before the `goal work-on` call, and is **before** the stub's logged mid-turn time — i.e. it is the turn-start time, not the persist time.
- [ ] **AC3 — a topic does the same.** Seed `printf -- '---\nstatus: in_progress\nphase: planning\nassignee: bborbe\n---\nTags: [[Topic]]\n\n---\nScratch topic.\n' > "$R/vault/topics/Scratch Rebind Topic.md"`, point the stub's mid-turn write at `topic set "Scratch Rebind Topic" claude_session_id XXXX-interactive`, and run `topic work-on "Scratch Rebind Topic" --mode headless`: the topic file reads `claude_session_id: XXXX-interactive` and carries one `- session_id: <printed id>` entry.
- [ ] **AC4 — an empty field at persist time is stamped as today and adds nothing.** With the stub's mid-turn `set` removed, a goal and a topic bind each leave `claude_session_id` equal to the printed id and no `metrics_sessions` key: `! grep -q '^metrics_sessions:' "<file>"` succeeds for both.
- [ ] **AC5 — the claimant is not appended.** After AC1, `grep -c -- '- session_id: XXXX-interactive' "<goal file>"` returns 0 — it is the owner, not history.
- [ ] **AC6 — existing entries are preserved.** On a goal file pre-seeded with one `metrics_sessions` entry, after an AC1-shaped bind `awk '/^    - session_id:/{print NR": "$0}' "<goal file>"` prints the seeded id on the lower line number and `<printed id>` on the next.
- [ ] **AC7 — the caller still gets its id.** In AC1, `goal work-on` prints `session_id: <printed id>` (the binding session), not `XXXX-interactive`.
- [ ] **AC8 — the binder's own mid-turn stamp adds nothing.** With the stub's mid-turn write changed to `goal set … claude_session_id "$SID"` (its own id, as the child's session-connect does), the goal file reads `claude_session_id: <printed id>` and `! grep -q '^metrics_sessions:' "<goal file>"` succeeds.
- [ ] **AC9 — regression tests lock it.** New tests drive `persistGoalSessionID` and `persistTopicSessionID` with a stored page whose `claude_session_id` differs from the incoming id and assert the stored id is kept and the incoming id lands in `metrics_sessions`; they fail against the pre-fix code. `make test` and `make precommit` both exit 0.
- [ ] **AC10 — the docs match.** `docs/work-on-session-lifecycle.md` § Post-exit write ordering states that the goal and topic persist stamps only an empty field and otherwise records the binding session in `metrics_sessions`; `grep -n 'metrics_sessions' docs/work-on-session-lifecycle.md` returns a line naming the goal path.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `ROOTDIR=/workspace make test` — exits 0
- `ROOTDIR=/workspace make precommit` — exits 0
- the AC9 regression tests run alone — exit 0

### Operator-executable (runs on the host against a freshly built binary)

- The `## Reproduction` block with `vault-cli` resolving to `/tmp/new-vault-cli` (built from the
  branch): the stub log still shows `XXXX-interactive` mid-turn, and AC1–AC5 hold on the goal and
  topic files, and AC2 and AC8 hold with their stub variants.

## Desired Behavior

1. A goal persist that re-reads a page whose `claude_session_id` is a different non-empty id
   leaves that id in place and appends `{session_id: <its own id>, started_at: <turn start>}` to
   the page's `metrics_sessions`, in one write. `<turn start>` is the clock read captured before
   `StartSession`, as the task path captures `startedAt` before the spawn.
2. The topic persist does the same on topic pages.
3. A persist that finds the field empty stamps its own id and appends nothing; one that finds
   its own id appends nothing.
4. Both persist functions still return the binding session's id to the caller.
5. Goal and topic frontmatter gain a `metrics_sessions` reader and appender with the same entry
   shape and normalisation as the task's (`pkg/domain/task_frontmatter_metrics.go`).

## Assumptions

- The claimant keeps the stamp. `claude_session_id` is what `/supervisor:open` and the Vault UI
  resume; a headless turn that has already finished is a transient writer, not an owner. Moving
  the stamp to it would break resume for the session a human is working in. This is the task
  path's rule (`persistSessionAndMetrics`), so both anchors behave the same on ownership under the race.
- The rule is "stamp only if empty", which never inspects whether the claimant is alive. The
  reproduction's claimant `XXXX-interactive` is a fixture id written by the stub, not a live
  session, so it measures the record shape and the stamp-preservation rule; the claim that the
  kept owner is the *live* session is asserted from the writers that stamp mid-turn (an
  interactive `/work-on-goal`, a concurrent `work-on`), not measured.
- **The append rule deliberately differs from tasks.** Tasks always append the incoming id because
  `work-on` is their metrics producer; here only a binder that finds another owner is appended, so
  an uncontended goal or topic gains no new key (AC4, AC8). The task path also persists before the
  spawn and has no 30-minute window; the goal path keeps its post-exit persist (spec 041).
- **A losing TTY caller still resumes its own session.** On the interactive branch
  (`goal_workon.go`, `topic_workon.go`) the persisted return value feeds `ResumeSession`; a caller
  that loses the race resumes the binding session, which the page no longer names as owner. That
  is intended — the caller continues its own turn — and a later `work-on` on the page resumes the
  claimant through the existing-owner early return. The persist functions must not return the
  stored owner instead.
- The id is taken verbatim, with no UUID validation — the reproduction uses `XXXX-interactive`.

## Constraints

- No new CLI verb or flag.
- `handleClaudeSession`'s existing-owner early return is unchanged.
- The persist stays post-exit (spec 041): the id is written only after the turn validates.
- One write per persist, re-read first, exactly as today.
- The task path, `goal set` / `topic set`, and `task append-metrics-session` are unchanged.
- Errors wrap with `github.com/bborbe/errors`; tests use Ginkgo/Gomega.
- `NewGoalWorkOnOperation` and `NewTopicWorkOnOperation` take an injected `libtime.CurrentDateTime`,
  as `NewWorkOnOperation` does, wired at their `pkg/cli/cli.go` call sites, so tests pin the
  turn-start time.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Re-read fails | persist returns an empty id and writes nothing — unchanged | re-run `work-on` once the page is readable |
| `metrics_sessions` holds a malformed entry | well-formed entries kept in order, new entry appended; normalisation as on tasks | none — the original stays in git history |
| Two binds race to persist | each re-reads then writes; in the re-read→write gap the last write wins, so an entry can be lost — the same accepted race as `work-on-session-lifecycle.md:106-109` | not recovered; re-running `work-on` re-records only its own session |
| Write fails after the re-read | persist returns an empty id and nothing lands — unchanged | re-run `work-on` |
| Stored id equals the binding id | nothing appended, nothing re-stamped | none |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `metrics_sessions` reader/appender on goal and topic frontmatter, unit-tested (order kept, malformed entries dropped) | 5 | 9 (unit half) | — |
| 2 | Goal and topic persist: stamp only if empty, else append at turn start; clock injection; tests, docs | 1–4 | 1–8, 9, 10 | prompt 1 |

Rationale: prompt 1 adds the record shape; prompt 2 is the behaviour that uses it.
