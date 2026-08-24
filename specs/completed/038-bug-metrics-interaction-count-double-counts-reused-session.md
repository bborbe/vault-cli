---
status: completed
approved: "2026-08-24T20:50:03Z"
generating: "2026-08-24T20:50:03Z"
prompted: "2026-08-24T21:00:10Z"
verifying: "2026-08-24T21:04:18Z"
completed: "2026-08-24T21:30:12Z"
branch: dark-factory/bug-metrics-interaction-count-double-counts-reused-session
---

## Summary

- `metrics_interaction_count` is inflated whenever a task is worked on more than once against the same Claude session.
- Two work-on runs that reuse one `claude_session_id` produce two `metrics_sessions` entries with the same id; the counter sums `type:"user"` entries per entry with no dedup, so the shared session's turns are counted twice.
- Verified in the spec-036 verification run: stored count **24** vs ground truth **12** `type:"user"` entries in the single distinct session file.
- Unit tests pass (6/6) but never exercise duplicate session ids, so the defect shipped green.
- The fix dedupes session ids before counting and adds a regression test asserting `Count(s1, s1) == Count(s1)`.

## Problem

The passive per-task interaction metric is supposed to measure how many human interactions a task consumed. When the AC2 standard flow reuses an existing Claude session on the second work-on run, the same real session is recorded twice in `metrics_sessions`. The counter then reads the same JSONL file twice and reports double the actual human turns. The metric is a ranking input for the recurring-work offload objective (spec-036's north star); an inflated count corrupts the ranking data for any task touched twice in one session — which is the common case, because reusing a session is the designed path, not an edge case.

## Reproduction

Observed 2026-08-24 during spec-036 verification. Tool version `vault-cli 0.115.0` (HEAD `441e2ff`); the defective counting code landed in commit `42cd4ff` (spec-036 complete-end-hook).

Smallest reproducing flow — the AC2 standard sequence on one task in a scratch vault (`/private/tmp/vault-cli-verify`):

```bash
# 1. First work-on: starts a Claude session, records it
vault-cli task work-on "<task>"
# 2. Second work-on on the SAME task: the running session is reused (cached
#    early-return in the work-on path), so the SAME claude_session_id is
#    recorded again — metrics_sessions now holds two entries with one id
vault-cli task work-on "<task>"
# 3. Complete: recomputes metrics_interaction_count by iterating every
#    metrics_sessions entry and summing type:"user" turns per entry
vault-cli task complete "<task>"
```

Observed evidence (verbatim):

- Task file after the double work-on: `metrics_sessions` contains two entries whose `session_id:` values are identical (the reused `claude_session_id`). Two work-on runs, two entries, one distinct session — per parent spec AC2 the accumulation is correct and intended.
- Stored metric after complete: `metrics_interaction_count: 24`.
- Ground truth — the single distinct session file:

```
75 lines total
~/.claude/projects/-private-tmp-vault-cli-verify/85b2b3b6-527b-4753-8716-6fe636a265b4.jsonl
grep -c '"type":"user"' <file>   → 12
```

- `go test ./pkg/ops/` passes 6/6 — the existing InteractionCounter specs cover sum-across-sessions, missing files, malformed lines, non-user types, unsafe ids, and large lines, but no spec passes a duplicated id.

## Expected vs Actual

**Expected** — per parent spec `specs/in-progress/036-passive-per-task-metrics.md`, Desired Behavior item 3 (line 71): "for each **distinct** `session_id` recorded in `metrics_sessions`, count the `type: "user"` entries in that session's Claude Code JSONL log; the stored count is the sum". And Acceptance Criterion 4 (line 45): the stored `metrics_interaction_count` equals the total number of `type: "user"` entries across the task's recorded session JSONL files. The task has one distinct session; that session holds 12 user turns; the stored count must be **12**.

**Actual** — the stored count is **24**. `Count()` iterates the two `metrics_sessions` entries independently and reads the same JSONL file twice. Each entry is a real recorded session entry (accumulation is by design), but the counter does not honor the "distinct session_id" contract, so the shared session's 12 user turns are summed twice.

## Workaround

None. The two `metrics_sessions` entries are deliberately preserved by parent spec AC2 (accumulate, never overwrite), so the inflated count persists until the next complete on a task that is no longer double-worked. There is no operator-side edit that fixes the metric without corrupting the accumulator. The metric must be corrected at the counter.

## Why this is a bug

The parent spec explicitly names the counting contract: Desired Behavior item 3 says "for each **distinct** `session_id`" (`specs/in-progress/036-passive-per-task-metrics.md:71`). The implementation violates its own spec's wording — `metricsSessionIDs()` copies every recorded id verbatim and `Count()` sums per entry with no dedup, so the "distinct" qualifier is silently dropped. This is a documented-contract-versus-reality mismatch (bug class per `dark-factory/docs/bug-workflow.md`), with verified evidence: 24 stored vs 12 ground truth. It is also a silent wrong-number failure — no error, no warning, just an inflated metric feeding the offload-ranking north star.

## Goal

After this fix, `metrics_interaction_count` reflects the actual human interaction total: a session id appearing more than once in `metrics_sessions` is counted exactly once, and the stored count for a double-worked task equals the ground-truth user-turn total of the single underlying session. All other counting semantics (distinct-session summation, missing-file→0, unsafe-id rejection) are unchanged.

## Acceptance Criteria

- [ ] A new fixture test asserts the dedup contract directly: with a session `s1` containing 3 `type:"user"` turns, `Count(ctx, []string{"s1", "s1"})` returns 3 and equals `Count(ctx, []string{"s1"})` — evidence: `go test ./pkg/ops/ -run InteractionCounter -v` exits 0 (exit code).
- [ ] Dedup holds when a duplicate is interleaved with a distinct session: with `s1` = 3 turns and `s2` = 2 turns, `Count(ctx, []string{"s1", "s2", "s1"})` returns 5 (each distinct session once, order-independent) — evidence: `go test ./pkg/ops/ -run InteractionCounter -v` exits 0 (exit code).
- [ ] Dedup changes none of the existing guard semantics: `Count(ctx, []string{"missing", "missing"})` returns 0 (missing file contributes 0 whether duplicated or not), and every unsafe id in the existing table (`../x`, `a/b`, `a\b`, `..`, `""`) still returns 0 — evidence: `go test ./pkg/ops/ -run InteractionCounter -v` exits 0 (exit code).
- [ ] Regression lock (mutation check): temporarily removing the dedup from the counter makes the new duplicate-id spec fail, and restoring it makes the suite pass again — evidence: `go test ./pkg/ops/ -run InteractionCounter -v` exits non-zero with the dedup removed, exits 0 with it restored (exit code, both directions).
- [ ] The pre-existing six InteractionCounter specs pass unchanged (no test modified, only new ones added) — evidence: `go test ./pkg/ops/ -run InteractionCounter -v` exits 0 listing all six original spec names plus the new dedup specs (exit code).
- [ ] **Operator-executable:** the original reproduction no longer reproduces — replay the double work-on + complete sequence from the Reproduction section with a freshly built binary against a scratch vault; the task's `metrics_interaction_count` equals the ground-truth user-turn count of the single distinct session file, NOT the doubled value (24 in the historical repro — a fresh replay's exact number differs with the fresh session, the invariant is the equality). Evidence: `grep '^metrics_interaction_count:' <task>.md` returns a value equal to `grep -c '"type":"user"' ~/.claude/projects/<encoded-project-dir>/<session-id>.jsonl` (file content, before/after framing).
- [ ] Ship readiness: `make precommit` exits 0 — evidence: exit code.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint + format + generate + test + checks, exits 0
- `make test` — full suite, exits 0
- `go test ./pkg/ops/ -run InteractionCounter -v` — exits 0; the new duplicate-id specs are present in the output and assert dedup

### Operator-executable (runs on the host after merge, spec verification ladder)

Genuinely needed: the bug is a runtime-symptom defect (wrong stored number), so the operator rung must replay the original reproduction against the built binary — test-only evidence does not prove the bug is gone.

- `go build -o /tmp/new-vault-cli .` — fresh binary from HEAD
- Against a scratch vault, run the Reproduction sequence (work-on, work-on, complete) using the fresh binary; confirm `metrics_interaction_count` in the task file equals the ground-truth user-turn count of the single distinct session (`grep -c '"type":"user"' <session.jsonl>`), i.e. 12 not 24.

## Desired Behavior

1. The interaction counter treats a session id that appears more than once in `metrics_sessions` as one session: it counts that session's user turns exactly once, regardless of how many work-on runs recorded the same id. (fires AC1, AC2)
2. The dedup is order- and position-independent — a duplicate anywhere in the recorded id list (adjacent, non-adjacent, repeated N times) contributes the session's count once and never more. (fires AC2)
3. All other counting semantics are preserved verbatim: distinct sessions still sum; a missing, unreadable, or malformed session file still contributes 0 and never fails completion; an unsafe id still contributes 0 and never builds a path. (fires AC3)
4. complete-task writes a `metrics_interaction_count` for a double-worked task that equals the ground-truth user-turn total of the underlying session, so the stored metric matches reality instead of double-counting. (fires AC6)

## Constraints

- Parent spec 036 semantics must not regress: AC3 (`metrics_completed_at` written; `metrics_interaction_count` a non-negative integer), AC5 (recurring archive to `metrics_cycles` then reset of the active accumulator; `claude_session_id` cleared per existing behavior), AC6 (no-anchor task completes with the count field absent, never zero).
- `metrics_sessions` accumulation is unchanged: two work-on runs still append two entries (parent AC2). The dedup lives on the read side (the counter), never on the write side — the accumulator must not start suppressing entries.
- Best-effort missing-file→0 contract unchanged: a session whose JSONL is missing or unreadable contributes 0 and does not fail completion.
- Read-only contract on `~/.claude` unchanged: the fix reads session logs only, never writes.
- Path-traversal guard unchanged: the `isSafeSessionID` rules (reject empty, `.`, `..`, path separators) stay; an unsafe id contributes 0 and no path is ever built for it.
- The fix is confined to the interaction counter (`pkg/ops/interaction_count.go`, `Count()` and/or `metricsSessionIDs()`); no change to domain structs, frontmatter serialization, or the work-on/complete hooks.
- All existing tests in `pkg/ops` continue to pass; the six original InteractionCounter specs are not modified.
- Dedup must be bounded (set-based, one pass) — a task worked on many times in one session produces a long id list, and counting must complete in linear time.
- `make precommit` passes. This is a bug fix on `kind: bug`; CHANGELOG and version alignment follow the autoRelease releaser as in parent spec 036 (add a `## Unreleased` bullet only; do not hand-bump versions).

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Same session id recorded N times (task worked on N times in one session) | Session counted exactly once; stored count never exceeds the distinct-session ground truth | Re-run `vault-cli task complete`; confirm `grep '^metrics_interaction_count:' <task>.md` equals `grep -c '"type":"user"' <session>.jsonl` |
| Duplicate ids interleaved with distinct ids | Each distinct session counted once, order-independent | None needed — covered by AC2 |
| Malformed or empty session id in `metrics_sessions` | Contributes 0, no panic; dedup never crashes on a malformed id | None needed — unchanged guard behavior (AC3) |
| Missing JSONL for a duplicated id | Contributes 0 both before and after dedup; completion succeeds | None needed — best-effort contract unchanged |
| Very long `metrics_sessions` list | Dedup is a single set pass; counting completes in linear time | None needed — bounded, no unbounded memory growth |

Concurrency is not applicable: dedup is in-memory within a single `Count()` call and touches no shared state. No network I/O is involved (local file reads only), so rate limiting is not applicable.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Dedup session ids in the interaction counter (`Count()`/`metricsSessionIDs()`) + new duplicate-id fixture specs (adjacent duplicate, interleaved duplicate, duplicated-missing, mutation check) | 1, 2, 3 | 1-5, 7 | — |
| 2 | No code prompt — operator rung replays the Reproduction sequence against the built binary and confirms stored count 12 not 24 | 4 | 6 | prompt 1 |

Rationale: prompt 1 is the entire code change — a single-layer, single-function dedup plus the regression specs that pin it (including the mutation check that fails when the dedup is removed). The end-to-end replay is not a prompt; it is the operator-executable verification rung that bug workflow demands for a runtime-symptom defect.
