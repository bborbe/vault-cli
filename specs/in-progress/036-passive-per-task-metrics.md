---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-08-24T18:14:35Z"
generating: "2026-08-24T18:15:19Z"
prompted: "2026-08-24T18:21:53Z"
verifying: "2026-08-24T20:08:59Z"
branch: dark-factory/passive-per-task-metrics
---

## Summary

- Passive per-task time and interaction metrics recorded into task frontmatter by the existing work-on-task and complete-task flows — zero extra operator steps, zero new flags.
- Start signal: each work-on-task run appends one session entry (session id + start timestamp) to the task's `metrics_sessions` list; entries accumulate across sessions and are never overwritten.
- End signal: complete-task writes the completion timestamp and recomputes the interaction count from the task's Claude session logs; completing a recurring task archives the finished cycle and resets the accumulator so the next cycle measures fresh.
- A task completed without a work-on anchor still gets an end timestamp; a missing start means "unknown", never zero, and never blocks completion.
- No new service, no new datastore — the metrics are plain frontmatter fields that vault-cli already round-trips intact, so "human time per recurring task" becomes computable from stored data.

## Problem

The measure half of KR2 of the objective "Move Recurring Work Outside the Loop" has been reported missing in every weekly review for four consecutive weeks (W28–W31). Its north star — "human time per recurring task ↓" — is uncomputable because no per-task time or interaction data is captured anywhere. The objective itself states the rule: you cannot optimize what you do not measure. The metrics were deliberately decoupled from the offload half: they were never meant to pick the first offload target (that shipped already), they rank the next targets — and that work is blocked on data that does not exist. The scope is explicitly passive: capture start, end, and interaction count via the existing vault-cli skill flow, with no separate datastore if the task frontmatter will do.

## Goal

After this work, every task that goes through the vault-cli work-on and complete flows accumulates passive metrics in its own frontmatter: a `metrics_sessions` list with one `{session_id, started_at}` entry per work-on run, a `metrics_completed_at` end timestamp, a `metrics_interaction_count` derived from the task's Claude session logs, and — for recurring tasks — a `metrics_cycles` archive of each finished cycle. All of this lands without any extra operator step, survives every vault-cli frontmatter round-trip, and makes "human time per recurring task" (completed minus earliest started, plus interaction count) computable from the stored data.

## Non-goals

- Do NOT rank, recommend, or offload any recurring task — that is the offload half of KR2, delivered separately.
- Do NOT build a dashboard, report command, or visualization for the metrics — the stored fields are the contract; north-star arithmetic is a consumer-side or verification-step computation, not a shipped feature.
- Do NOT backfill metrics for already-completed tasks.
- Do NOT create a new service, datastore, or Prometheus sink — frontmatter only.
- Do NOT enforce the work-on anchor (it stays a convention) and do NOT add any opt-out or disable flag for metrics recording — passive recording is the invariant; a future consumer that needs variation gets a separate spec.
- Do NOT store per-session detail inside `metrics_cycles` — each archived cycle is the aggregate `{started_at, completed_at, interaction_count}`; session-level history stays in the active accumulator.
- Do NOT change `claude_session_id` semantics — existing behavior (including its clearing on recurring completion) is preserved.
- Do NOT add a new E2E scenario — the behavior is reachable by unit tests with fixture session logs plus the operator-executable verification rung; existing scenarios already cover the work-on and complete flows.

## Acceptance Criteria

- [ ] Running the standard `vault-cli task work-on "<task>"` on a task in a scratch vault (no new flags, no extra steps) leaves the task file with a `metrics_sessions` entry whose `session_id` matches the task's `claude_session_id` and whose `started_at` is within ±2 minutes of the command's wall-clock time. Evidence: `grep -c '^metrics_sessions:' <task>.md` returns ≥1; the `session_id:` value under `metrics_sessions` equals the `grep '^claude_session_id:' <task>.md` value; `date -u` comparison within ±2 min. Vault/file artifact + state transition.
- [ ] Running `vault-cli task work-on` a second time on the same task leaves `metrics_sessions` with two entries and the first entry's `session_id` and `started_at` values unchanged from the first run. Evidence: `grep -c 'session_id:' <task>.md` returns 1 after the first run and 2 after the second; the first-run values are still present after the second run. Vault/file artifact (state-transition delta).
- [ ] After `vault-cli task complete` on a worked, non-recurring task, the file contains `metrics_completed_at` with an ISO timestamp and `metrics_interaction_count` with a non-negative integer. Evidence: `grep '^metrics_completed_at:' <task>.md` returns ≥1 line; `grep -E '^metrics_interaction_count: [0-9]+$' <task>.md` returns ≥1 line. Vault/file artifact.
- [ ] The stored `metrics_interaction_count` equals the total number of `type: "user"` entries across all of the task's recorded session JSONL files; a session whose JSONL file is missing or unreadable contributes 0 and does not fail completion. Evidence: `go test ./pkg/ops/ -run <NewInteractionCountTest> -v` exits 0 with fixture session files of N and M user turns asserting stored count = N+M, plus mutation check — removing the counting call makes that test fail — and `grep '^metrics_interaction_count:'` on a fixture-based completed task shows the expected integer. Exit code + file content.
- [ ] After `vault-cli task complete` on a recurring task that had been worked on, the file contains a `metrics_cycles` entry with `started_at`, `completed_at`, and `interaction_count`, and the active accumulator is gone. Evidence: `grep '^metrics_cycles:' <task>.md` returns ≥1 line; `grep -c '^metrics_sessions:'` returns 0; `grep -c '^metrics_completed_at:'` returns 0; `grep -c '^metrics_interaction_count:'` returns 0; `grep '^status:'` still shows `in_progress`. File content + negative evidence.
- [ ] `vault-cli task complete` on a task that never ran work-on exits 0, writes `metrics_completed_at`, and writes NO `metrics_interaction_count`. Evidence: exit code 0; `grep '^metrics_completed_at:' <task>.md` returns ≥1 line; `grep -c '^metrics_interaction_count:' <task>.md` returns 0. Exit code + negative evidence.
- [ ] Ship readiness: `make precommit` exits 0; CHANGELOG.md has a `## Unreleased` bullet describing the passive per-task metrics; `commands/work-on-task.md` and `commands/complete-task.md` each state that the metrics fields are written passively by vault-cli and not to be hand-edited. Evidence: exit code; `awk '/^## Unreleased/,/^## v/' CHANGELOG.md | grep -ci metrics` returns ≥1; `grep -c 'metrics_sessions\|metrics_completed_at\|metrics_interaction_count\|metrics_cycles' commands/work-on-task.md` returns ≥1 and the same for `commands/complete-task.md`. Exit code + file content.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint + format + generate + test + checks, exits 0
- `make test` — full suite, exits 0
- `go test ./pkg/ops/ -run <NewInteractionCountTest> -v` — focused fixture test, exits 0
- `grep -rn 'metrics_sessions\|metrics_completed_at\|metrics_interaction_count\|metrics_cycles' pkg/ | wc -l` — returns ≥1 (targeted assertion the change landed)
- `awk '/^## Unreleased/,/^## v/' CHANGELOG.md | grep -ci metrics` — returns ≥1

### Operator-executable (runs on the host after PR merge, spec verification ladder)

Genuinely needed: the end-to-end proof that the fields land passively and the north-star is computable requires a real vault, a real `work-on-task`/`complete-task` run, and real (or planted) Claude session logs on the host.

- `go build -o /tmp/new-vault-cli .` — fresh binary from HEAD
- Against a scratch vault with one test task, run `vault-cli task work-on "<task>"` then `vault-cli task complete "<task>"` using the fresh binary; confirm `metrics_sessions`, `metrics_completed_at`, `metrics_interaction_count` land in the task file with no extra manual step, and that "human time" (completed minus earliest started) plus interaction count are computable from the stored fields — demonstrated on at least one real task.

## Desired Behavior

1. work-on-task appends one `{session_id, started_at}` entry to the task's `metrics_sessions` frontmatter field, preserving every prior entry; the appended `session_id` is the session started by this run and `started_at` is the current timestamp from the shared injected clock. (fires AC1, AC2)
2. complete-task on a non-recurring task writes `metrics_completed_at` (current timestamp) and recomputes and writes `metrics_interaction_count`, all in the same write as the task's status and completed-date update. (fires AC3)
3. Interaction count derivation: for each distinct `session_id` recorded in `metrics_sessions`, count the `type: "user"` entries in that session's Claude Code JSONL log under the operated vault's encoded project directory; the stored count is the sum; a missing, unreadable, or malformed session file contributes 0 and never fails the operation. (fires AC4)
4. complete-task on a recurring task archives the finished cycle as a `{started_at, completed_at, interaction_count}` entry in `metrics_cycles`, then removes the active accumulator fields (`metrics_sessions`, `metrics_completed_at`, `metrics_interaction_count`) so the next cycle measures fresh; status stays `in_progress` and `claude_session_id` is cleared per existing behavior. (fires AC5)
5. No-anchor fallback: a task with no `metrics_sessions` still completes normally — `metrics_completed_at` is written, `metrics_interaction_count` stays absent (unknown, never zero), no new error or warning, no enforcement. (fires AC6)
6. The metrics fields round-trip intact through every vault-cli frontmatter write; the work-on-task and complete-task command docs state that these fields are written passively by vault-cli and must not be hand-edited; CHANGELOG gains a `## Unreleased` bullet. (fires AC2, AC7)

## Constraints

- Metrics land only in the task frontmatter fields `metrics_sessions`, `metrics_completed_at`, `metrics_interaction_count`, and `metrics_cycles`. No new service, no new datastore, no Prometheus sink.
- All timestamps come from the injected `libtime.CurrentDateTime` clock (the testable seam), in ISO 8601 with timezone offset, consistent with the existing date-time fields.
- Accumulate, never overwrite: work-on-task must not truncate or replace prior `metrics_sessions` entries; the only path that clears the active accumulator is recurring-task completion.
- Frozen hook sites (resolved design decisions): the work-on path after the task write (`pkg/ops/workon.go`, ~line 95) and the complete path after the task write (`pkg/ops/complete.go`, ~line 116 non-recurring, ~line 228 recurring). The `session_id` in a work-on entry is the session started by that run, which is known only after the session starts — exact placement inside the frozen work-on path is the implementer's choice.
- Existing behavior must not regress: spec 015 (recurring completion clears `claude_session_id`), the incomplete-checkbox guard, daily-note and goal roll-up, and status/phase semantics. All existing tests in `pkg/ops` continue to pass. The `NumTurns` value read at session start is unrelated to the interaction count and unchanged.
- vault-cli reads Claude Code session logs read-only; it never writes to `~/.claude`.
- `make precommit` passes. Version alignment is owned by the autoRelease releaser (`.maintainer.yaml autoRelease: true`, github-releaser-agent on merge; `.dark-factory.yaml autoRelease: false`, daemon never tags): add a `## Unreleased` bullet only; do NOT hand-bump the three plugin JSON version fields or create a tag.

## Failure Modes

| Trigger | Expected behavior | Detection | Reversibility | Concurrency | Recovery |
|---------|-------------------|-----------|---------------|-------------|----------|
| Session JSONL missing, pruned, or unreadable | That session contributes 0; completion succeeds; count reflects only readable sessions | `metrics_interaction_count` lower than the session's actual user-turn count; compare against `~/.claude` | Reversible | — | Documented best-effort contract; operator accepts, or re-runs complete if the file reappears |
| Crash mid-operation (work-on between task write and session start; complete before its single write) | No partial metrics state — all metrics mutations for one operation land in one frontmatter write; a crash before it leaves the pre-operation file | Expected fields absent from frontmatter after re-running | Reversible | — | Re-run the command; work-on re-appends a fresh entry |
| Clock skew (manual clock change or NTP jump between start and end) | Raw timestamps stored as-is, no validation, no deduction; computed duration may be implausible or negative | North-star consumer sees a negative or implausible duration | Reversible (data-quality only) | — | Consumer filters implausible durations; single-host clock limits exposure to manual changes |
| Concurrent sessions work-on or complete the same task (git-synced worktree, sibling agents) | Last writer wins on the file; a concurrent metrics append can be lost | Task frontmatter lacks an expected entry | Reversible | Real case — two agents writing the same task file; obsidian-git auto-syncs on schedule | Re-run the operation; accepted limitation of git-synced markdown (no locking); metrics are derived data |
| Malformed or legacy `metrics_*` values (hand-edited vault file) | Treated as absent; work-on starts a fresh accumulator; complete writes end fields; no crash | Inspect task frontmatter | Reversible | — | Re-run the operation |
| Very large session JSONL (resource exhaustion) | Streamed parse with bounded memory; counting completes | None | — | — | — |
| `~/.claude/projects` directory absent or permission-denied | All sessions contribute 0; completion succeeds | `metrics_interaction_count` present and low or zero | Reversible | — | Documented best-effort; operator restores session access |

Rate limiting / throttling is not applicable: the metrics path makes no network calls — all reads are local files.

## Security / Abuse Cases

Vault files are user-controlled input, so `metrics_*` fields and session IDs must never be trusted as code or as safe path components.

- Path traversal: session IDs from the task are resolved to file paths under the operated vault's encoded project directory. A session ID containing a path separator (`/`, `\`, `..`) must be rejected and contribute 0, so it cannot escape the directory or read arbitrary files.
- Untrusted JSON on disk: session log lines that fail to parse are skipped, never panic; only entries with `type: "user"` count.
- Read-only contract on `~/.claude`: vault-cli never writes, deletes, or follows symlinks outside the operated vault's encoded project directory when reading session logs.
- Malformed metrics values never block an operation — they are treated as absent.
- No retry loops: session-log reading is bounded and non-retrying, so a large or slow file cannot hang completion indefinitely.

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Frontmatter field helpers: typed serialization + round-trip for `metrics_sessions` (list of `{session_id, started_at}`), `metrics_completed_at` (timestamp), `metrics_interaction_count` (int), `metrics_cycles` (list of aggregates) | 6 | 2, 7 (part) | — |
| 2 | work-on start hook: append `{session_id, started_at}` to `metrics_sessions` in the work-on path, accumulate, using the injected clock | 1 | 1, 2 | prompt 1 |
| 3 | complete end hook + interaction derivation: write `metrics_completed_at`; count `type: "user"` entries across recorded session JSONL (best-effort, 0 per missing file); recurring path archives the cycle to `metrics_cycles` then resets the accumulator | 2, 3, 4, 5 | 3, 4, 5, 6 | prompts 1, 2 |
| 4 | Plugin surface + release: document passive fields in `commands/work-on-task.md` and `commands/complete-task.md`, add the `## Unreleased` CHANGELOG bullet | 6 | 7 | prompts 2, 3 (docs describe shipped behavior) |

Rationale: prompt 1 establishes the frontmatter contract (serialization/round-trip) that both hooks depend on; prompt 2 needs the helpers and the session ID from the work-on flow; prompt 3 is the largest surface (end hook + parser + recurring archive) and builds on 1 and 2; prompt 4 is docs + changelog last so it describes behavior that already shipped. No cycles — each prompt depends only on earlier ones.

## Do-Nothing Option

Not doing this leaves the measure half of KR2 un-instrumented: "human time per recurring task" stays uncomputable, the metric keeps showing up as a miss in weekly reviews, and the offload half loses its instrument for ranking future offload targets — the objective claims leverage with no data behind it. The cost of doing it is small (frontmatter-only, no new infrastructure, passive hooks), so the cost of leaving the gap is not acceptable.
