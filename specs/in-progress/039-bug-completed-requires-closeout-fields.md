---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-08-25T07:32:00Z"
generating: "2026-08-25T07:32:00Z"
prompted: "2026-08-25T07:43:46Z"
verifying: "2026-08-25T08:17:41Z"
branch: dark-factory/bug-completed-requires-closeout-fields
---

## Summary

- vault-cli currently requires `aborted_reason` + `gate_successor` on BOTH the `aborted` AND the `completed` transition — spec 037 over-applied the close-out guard to a status that is not a close-out.
- `completed` means all success criteria were met: nothing was abandoned, no risk gate was retired early, so forcing an "aborted reason" on it is semantically wrong and blocks the most common terminal transition.
- The fix splits the guard by status: `completed` requires neither field (task and goal, via `task complete`, `goal complete`, `task set ... status completed`, `goal set ... status completed`, and the `task update` checkbox sync); `aborted` keeps both requirements exactly as spec 037 locked them.
- `--reason` / `--gate-successor` stay accepted on `completed` transitions and are still persisted when supplied — they become optional, never required.
- `docs/task-writing.md` § Lifecycle, the CLI help text, and the spec-037 record are updated so no documentation claims completion requires the fields.

## Problem

The close-out guard introduced by spec 037 treats `aborted` and `completed` as one class: `pkg/domain/closeout.go` freezes `closeOutFields = ["aborted_reason", "gate_successor"]`, and both `SetStatus` chokepoints (`pkg/domain/task_frontmatter.go:183-201`, `pkg/domain/goal_frontmatter.go:144-160`) reject either status unless both fields are present. But `completed` is not a close-out in the spec-037 sense — the whole point of that spec was that an *abandoned* task must not silently retire a risk gate. A task whose success criteria are all met was not abandoned; `aborted_reason` (a field whose name means "why the work was abandoned") is nonsense on it, and every normal completion now fails unless the operator invents a reason and a successor. Completion is the most common terminal transition (every recurring task, every daily flow), so the guard regressed the primary happy path. The decision to relax `completed` is recorded in the vault task "Fix aborted_reason Required on Task Completion (vault-cli)" (planning) — it supersedes the 037 constraint that `completed` also carries `aborted_reason`.

## Reproduction

Tool version: vault-cli built from current HEAD (source `0881b68`, released `v0.116.1`). Fresh scratch vault: `cp -r example /tmp/...`, `sed` the `__VAULT_PATH__` placeholder in the copied `config.yaml` to the temp vault path.

```bash
# A) task complete WITHOUT reason — expected success, actually rejected
./vault-cli --config <cfg> task complete "Simple Task"; echo "exit=$?"
# exit=1
# Error: set status: cannot set status "completed": missing close-out field(s) aborted_reason, gate_successor; a close-out must record why the work is being closed out (aborted_reason), consider what it owns (trigger / gate / threshold / recurring check), and name where that risk moves (gate_successor, or "none" when nothing is inherited): validation error

# B) task set status completed WITHOUT fields — expected success, actually rejected
./vault-cli --config <cfg> task set "Simple Task" status completed; echo "exit=$?"
# exit=1
# Error: set field: cannot set status "completed": missing close-out field(s) aborted_reason, gate_successor; ...
# Try: vault-cli task set "Simple Task" status completed --reason "<text>" --gate-successor "<successor|none>"

# C) task set status aborted WITHOUT fields — correctly rejected, must stay rejected
./vault-cli --config <cfg> task set "Simple Task" status aborted; echo "exit=$?"
# exit=1  (this is the intended, preserved behavior)

# file state after A-C: status: todo (no write happened)

# D) goal set status completed WITHOUT fields — expected success, actually rejected
./vault-cli --config <cfg> goal set "Example Goal" status completed; echo "exit=$?"
# exit=1
# Error: set field "status": cannot set status "completed": missing close-out field(s) aborted_reason, gate_successor; ...
```

## Expected vs Actual

**Expected** — per the vault task decision (Fix aborted_reason Required on Task Completion): a `completed` transition requires neither `aborted_reason` nor `gate_successor`. `vault-cli task complete "Simple Task"` (success criteria met, no fields) exits 0 and writes `status: completed` with no `aborted_reason` / `gate_successor` in the frontmatter; `goal set ... status completed` behaves the same. The `aborted` transition keeps both requirements from spec 037 (ACs and Desired Behavior of `specs/completed/037-mandatory-abort-reason-and-gate-successor.md`).

**Actual** — `completed` is gated identically to `aborted`: every completion without the two fields exits non-zero with the "missing close-out field(s)" error and nothing is written (reproduction steps A, B, D). The `aborted` rejection (step C) is correct and must be preserved.

## Workaround

Until the fix lands, completion works only by supplying the fields: `vault-cli task complete "Simple Task" --reason "all criteria met" --gate-successor none` (same for `goal complete` and the `set ... status completed` routes). This is the exact over-application the fix removes.

## Why this is a bug

Spec 037's own framing was risk-retirement on *abandonment*: a task aborted at `phase: todo` that owned a live ORB trigger silently killed it. That concern does not extend to `completed` — completing means the work and its gates were satisfied, not retired. Forcing `aborted_reason` on `completed` (a field literally named for the abort path) contradicts the vault's completion semantics (docs/task-writing.md § Lifecycle: "completed — All success criteria met") and blocks every normal completion, not an edge case. The over-application is a regression introduced by spec 037; the boss decision recorded in the vault task ("Fix aborted_reason Required on Task Completion") restores `completed` to its pre-037 behavior while keeping the 037 guard exactly on `aborted`.

## Goal

After this work, no code path in vault-cli requires `aborted_reason` or `gate_successor` for a `completed` transition: `task complete`, `goal complete`, `task set <name> status completed`, `goal set <name> status completed`, and the `task update` checkbox sync all succeed without the fields and persist `status: completed` with neither field added. The `aborted` transition is unchanged from spec 037 — both fields still required, same actionable error, rejection before any write. Supplying `--reason` / `--gate-successor` on a completion still persists them (optional, never required). Docs, CLI help text, and the spec-037 record no longer claim completion requires the fields.

## Acceptance Criteria

**Scenario coverage: none.** This is a local CLI behavior change reachable by unit + integration tests and replayed live on a scratch vault at the operator rung; no Docker, cluster, or external service is involved, so the four conditions for a scenario do not hold.

- [ ] `make precommit` exits 0 — evidence: exit code
- [ ] Domain unit test (task): `TaskFrontmatter.SetStatus(completed)` on a frontmatter holding neither field succeeds and the map holds `status: completed` with no `aborted_reason` / `gate_successor` keys — evidence: `go test ./pkg/domain/ -run "SetStatus close-out" -v` exits 0 and the new spec asserts the absence (test assertion + exit code)
- [ ] Domain unit test (task): `TaskFrontmatter.SetStatus(aborted)` without both fields still returns a validation error naming the missing field(s) and leaves the frontmatter unchanged; with both fields it succeeds — evidence: `go test ./pkg/domain/ -run "SetStatus close-out" -v` exits 0 (test assertion + exit code, regression lock)
- [ ] Mirror tests for `GoalFrontmatter.SetStatus`: `completed` succeeds without the fields; `aborted` is still gated — evidence: `go test ./pkg/domain/ -run "SetStatus close-out" -v` exits 0 (test assertion + exit code)
- [ ] `make test` exits 0 and the pre-existing ops assertions that a no-field completion errors (`pkg/ops/complete_test.go` "task without close-out fields", `pkg/ops/goal_complete_test.go` "reports the missing close-out fields") are updated to assert success for `completed` while the aborted-path assertions stay — evidence: exit code + test row names
- [ ] **Operator-executable:** `vault-cli task complete "Simple Task"` on a scratch task with all success criteria met and no fields exits 0; `grep -n '^status:' "<vault>/24 Tasks/Simple Task.md"` shows `status: completed`; `grep -n '^aborted_reason:' "<taskfile>"` returns 0 lines and `grep -n '^gate_successor:' "<taskfile>"` returns 0 lines — evidence: exit code + file content (positive + negative)
- [ ] **Operator-executable (regression lock):** `vault-cli task set "Simple Task" status aborted` without fields exits non-zero, stderr contains `aborted_reason`, and `grep -n '^status:' "<taskfile>"` still shows the pre-abort status — evidence: exit code + stderr match + file content
- [ ] **Operator-executable:** `vault-cli goal set "Example Goal" status completed` without fields exits 0 and the goal file frontmatter contains `status: completed` with no `aborted_reason` — evidence: exit code + file content (negative)
- [ ] `docs/task-writing.md` § Lifecycle no longer claims `completed` requires close-out fields: the "Close-out fields" block is scoped to `aborted` only, and `grep -n "Close-out fields" docs/task-writing.md` returns a heading naming `aborted` only — evidence: file content; and `grep -n 'required to complete' pkg/cli/cli.go` returns 0 lines (no help text claims completion requires reason) — evidence: grep (negative)
- [ ] The spec-037 record gains an appended note (below its existing "Verification Result" section, no existing body text modified) stating that `completed` no longer requires `aborted_reason` / `gate_successor` and naming this spec — evidence: `grep -n 'no longer requires\|Superseded' specs/completed/037-mandatory-abort-reason-and-gate-successor.md` returns line ≥ 1 (file content)

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
go test ./pkg/domain/ -run "SetStatus close-out" -v
```

### Operator-executable (runs on the host after PR merge, spec verification ladder)

Genuinely needed: the bug is a runtime-symptom defect (a transition that must now succeed), so bug workflow requires replaying the reproduction from the Reproduction section against a freshly built binary.

```
make build
cp -r example /tmp/vault-cli-closeout-e2e
sed -i '' "s|__VAULT_PATH__|/tmp/vault-cli-closeout-e2e/vault|" /tmp/vault-cli-closeout-e2e/config.yaml
# completed without fields → succeeds (bug gone)
./bin/vault-cli --config /tmp/vault-cli-closeout-e2e/config.yaml task complete "Simple Task" ; echo "exit=$?"   # exit=0
grep -n '^status:' "/tmp/vault-cli-closeout-e2e/vault/24 Tasks/Simple Task.md"                               # status: completed
grep -c '^aborted_reason:' "/tmp/vault-cli-closeout-e2e/vault/24 Tasks/Simple Task.md"                       # 0
grep -c '^gate_successor:' "/tmp/vault-cli-closeout-e2e/vault/24 Tasks/Simple Task.md"                       # 0
# aborted without fields → still rejected (regression lock)
./bin/vault-cli --config /tmp/vault-cli-closeout-e2e/config.yaml task set "Simple Task" status aborted ; echo "exit=$?"   # exit≠0, stderr names aborted_reason
# goal completed without fields → succeeds
./bin/vault-cli --config /tmp/vault-cli-closeout-e2e/config.yaml goal set "Example Goal" status completed ; echo "exit=$?"  # exit=0
```

Assert each `exit=$?` and each `grep` result against the Acceptance Criteria. Note the repro in the Reproduction section used a `task complete` on a task without subtasks, so the incomplete-subtask check does not interfere; on a task with unchecked success criteria the pre-existing incomplete-subtask guard still fires and `--force` is unchanged.

## Desired Behavior

1. **Status-split guard at both chokepoints.** `TaskFrontmatter.SetStatus(completed)` and `GoalFrontmatter.SetStatus(completed)` never consult the close-out fields and always succeed (after status validation); only `aborted` still requires non-empty `aborted_reason` AND `gate_successor`. The guard in `pkg/domain/closeout.go` becomes status-aware — it is consulted for `aborted` only. (fires AC 2, 3, 4)
2. **Completion without fields succeeds.** `task complete <name>`, `task set <name> status completed`, `goal complete <name>`, `goal set <name> status completed`, and the `task update` checkbox sync persist `status: completed` with no `aborted_reason` / `gate_successor` added to the frontmatter. (fires AC 2, 5, 6, 8)
3. **One-step fields stay optional on completion.** When `--reason <text>` / `--gate-successor <name|none>` are supplied on a `completed` transition, they are still persisted with the status in one write — they remain accepted, they just stop being required. (fires AC 5, 6)
4. **`aborted` behavior is preserved verbatim from spec 037.** `task set <name> status aborted` / `goal set <name> status aborted` without both fields fail before any write with the same actionable error naming the missing field(s); with both fields they succeed and persist the fields. `task complete --force` still bypasses only the incomplete-subtask check, never the `aborted` guard. (fires AC 3, 4, 7)
5. **Checkbox-sync completion is no longer gated.** When `task update` would set status to `completed` and the fields are absent, it proceeds and persists `status: completed` — no error, no partial write. (fires AC 5, 8)
6. **Surface text matches reality.** CLI help for the `complete` commands and `docs/task-writing.md` § Lifecycle state that the close-out fields apply to `aborted` only; no help string or doc line claims completion requires a reason. (fires AC 9)
7. **Spec-037 record is annotated, not rewritten.** The completed 037 spec keeps its body and gains an appended note recording that `completed` no longer requires the close-out fields and naming this spec. (fires AC 10)

## Constraints

- Frozen field names: `aborted_reason` (free text) and `gate_successor` (successor name or literal `none`). Both remain **required for `aborted` only**; neither may ever gate a `completed` transition.
- The two `SetStatus` chokepoints are the sole enforcement points; the status split must live there. No route may special-case around them, and no route may re-introduce a `completed` guard (e.g. an update-path check that demands the fields before writing `completed`).
- Spec-037 `aborted` semantics are frozen and unchanged: both fields required, rejection raised before any write via `errors.Wrapf(ctx, validation.Error, ...)` in the existing style, same actionable error text (owned-risk categories + command form).
- `completed` transitions are not backfilled and no migration runs: a task completed without fields stays that way; existing aborted/completed tasks are untouched.
- `--reason` / `--gate-successor` values remain user-supplied Go strings persisted through the existing YAML serializer (special characters quoted) — no raw string concatenation into the file.
- `task complete --force` semantics unchanged (bypasses only the incomplete-subtask check); non-close-out transitions (`in_progress`, `next`, `backlog`, `hold`) unchanged; objectives / themes / visions untouched (the guard never applied there).
- `docs/task-writing.md` § Lifecycle is the live doc and is updated in place. The completed 037 spec record is immutable — only an appended supersession note, never a body rewrite.
- CLI help text must not claim completion requires the fields ("required to complete" wording is removed); the `set`-command help continues to state that the fields are required for the `aborted` transition (which stays true).

## Non-goals

- Renaming `aborted_reason` to a generic `completion_reason` — field names are frozen by spec 037.
- Removing `gate_successor` from the close-out contract entirely — it remains required for `aborted`.
- Backfilling or migrating existing completed/aborted tasks in the vault — no migration runs.
- Changing `task complete --force` subtask-bypass semantics — bypasses only the incomplete-subtask check, never the `aborted` guard.
- Touching non-close-out transitions (`in_progress`, `next`, `backlog`, `hold`) or objectives / themes / visions (the guard never applied there).
- Rewriting the completed 037 spec record's body — immutable; only an appended supersession note.

## Do-Nothing Option

Doing nothing leaves the regression in place: every `task complete`, `goal complete`, and checkbox-sync completion on a task without `aborted_reason` + `gate_successor` fails with "missing close-out field(s)", forcing operators to invent an abort-only reason on a normal completion and polluting completed task files with abort semantics. The workaround (always pass `--reason` / `--gate-successor` on every completion) exists but taxes every finished task and obscures whether a gate was actually handed off. Not acceptable — the bug blocks routine close-out of completed work.

## Failure Modes

This change relaxes a persistent-state gate, so Detection and Reversibility apply. No network I/O (external unavailability / rate limiting n/a), no new timestamp logic (clock skew n/a).

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---------|-------------------|----------|-----------|---------------|-------------|
| Operator runs `task complete` on a task with unchecked success criteria | Existing incomplete-subtask guard fires (unchanged); nothing written | Check the criteria, or re-run with `--force` (unchanged) | exit code | Reversible — no write occurred | n/a |
| Operator runs `task set <name> status aborted` without the fields | Rejected with the actionable error; nothing written (unchanged from 037) | Supply `--reason` / `--gate-successor <name\|none>` and re-run | exit code + stderr | Reversible — no write occurred | n/a |
| Operator completes a task without the fields | Succeeds and writes `status: completed` with no `aborted_reason` / `gate_successor` (the intended fix) | None — this is the new contract; docs and help text state it | file shows `status: completed`, no close-out fields | Irreversible from the CLI's perspective (no backfill) — intended, since nothing was aborted | n/a |
| Existing automation built on the 037 requirement for `completed` | It now succeeds without the fields; passing `--reason` / `--gate-successor` still works unchanged | None — the 037 breaking change for completion is reversed; CHANGELOG entry calls it out | exit code | Reversible — no data lost | n/a |
| Crash mid-complete after the task write but before goal/daily-note updates | Task persists as `completed`; some goal checkboxes can be stale (pre-existing multi-write behavior, unchanged) | Operator re-runs `task complete` (task path not blocked; goal path reports already-completed) | task file + goal file diff | Partial | Two instances completing the same task race on the file write; last write wins — no new race introduced |
| `--reason` / `--gate-successor` text contains YAML-special characters | Serializer quotes the value; file remains valid YAML (unchanged; must be covered by a test) | If a malformed file ever appears, existing `task lint` surfaces it | `task lint` / file parse | Reversible | n/a |
| External writer (hand edit, obsidian-git sync, vault-ui) sets `completed` directly, bypassing the CLI | Persists without the fields — the guard only covers vault-cli writes (unchanged from 037) | Not addressed by this spec | file shows `completed` | Irreversible from the CLI's perspective | n/a |

## Security / Abuse Cases

No new surface introduced by this change; the relaxation removes a gate rather than adding input handling.

- **Input into YAML:** `--reason` / `--gate-successor` remain user-supplied strings persisted as Go strings through the YAML serializer (which quotes special characters) — unchanged from spec 037; a raw concatenation could inject a second frontmatter key, which the existing serializer and tests pin against.
- **No new trust boundary:** the change adds no HTTP, no network, no path resolution, and no new file types; the only files written are the task/goal markdown files vault-cli already owns.
- **Error output:** unchanged static CLI text; no new injection surface.
- **`gate_successor` is not cross-validated** (no task/alert registry, unchanged from 037) — verifiability comes from the field being present, not reference integrity.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Status-split guard at the two `SetStatus` chokepoints + `closeout.go`; domain unit tests updated (completed succeeds without fields, aborted still gated) + ops integration tests flipped (`complete_test.go`, `goal_complete_test.go`, `update_test.go`) | 1, 2, 3, 4, 5 | 1, 2, 3, 4, 5 | — |
| 2 | Docs + surface text: `docs/task-writing.md` § Lifecycle scoped to `aborted`, CLI help strings (no "required to complete"), appended supersession note on the 037 record, CHANGELOG `## Unreleased` entry | 6, 7 | 9, 10 | prompt 1 |
| — | Operator rung: replay the Reproduction sequence against a freshly built binary on a scratch vault (task complete success, aborted still rejected, goal complete success) | — | 6, 7, 8 | prompt 1 |

Rationale: prompt 1 is the entire behavior change — the guard lives at the single domain seam and its tests flip with it, so keeping behavior + tests in one prompt avoids a green-tests-but-wrong-behavior window. Prompt 2 is the documentation/help surface that must not contradict the new behavior; it depends on prompt 1 only because the doc wording mirrors the shipped behavior. The operator rung is bug-workflow verification (replay the repro), not a code prompt.
