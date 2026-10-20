---
status: draft
spec: [063-bug-take-over-bind-drops-outgoing-session]
created: "2026-10-20T14:46:00Z"
branch: dark-factory/bug-take-over-bind-drops-outgoing-session
---

# Record the displaced owner when a take-over re-binds a task's session id (spec 063, prompt 1 of 1)

<!--
RESOLVED BY THE REVIEWER (2026-10-20): the spec's Failure Modes table claimed a
malformed existing `metrics_sessions` entry is "preserved verbatim", which the shared
writer cannot deliver. The spec row was corrected instead — it now records that an
entry with no `session_id` is dropped and a non-list value is normalized, matching the
`work-on` path and the public `append-metrics-session` verb. Reuse
`domain.TaskFrontmatter.AppendMetricsSession` as specified below; do not add a
raw-preserving writer.
-->

<summary>
- A take-over that re-binds a task to a new session now leaves the previous owner recorded on that task.
- The previous owner is appended to the same history field the exit and close paths already read, so a displaced session can still resolve its task and end.
- The new session id is still the one stored as the task's current owner.
- A first bind onto a task that has no owner records nothing extra.
- Re-binding the same session id records nothing.
- Deliberately clearing the owner — completion, spawn failure, and the explicit clear verb — keeps its current behaviour and records no history.
- Existing history entries keep their order and the new entry is appended last.
- Every caller of the set command inherits the behaviour, including the Vault UI.
- The session-lifecycle documentation no longer says the set command always refuses the history field.
- A regression test binds one session then another and fails against the old behaviour.
</summary>

<objective>
Make `vault-cli task set <task> claude_session_id <new>` append the displaced, different, non-empty owner to `metrics_sessions` before storing the new id, in the same single write. The exit and close paths resolve a session by matching `claude_session_id` or any `metrics_sessions` id; without the append a taken-over session matches nothing and can never exit or be closed. First binds, same-value re-binds, and the deliberate clears must keep their current behaviour.
</objective>

<context>
Read `CLAUDE.md` for project conventions. Then read these files fully, in this order:

- `specs/in-progress/063-bug-take-over-bind-drops-outgoing-session.md` — Goal, Acceptance Criteria, Assumptions, Constraints, and Failure Modes. This prompt covers all of AC1–AC8.
- `pkg/ops/frontmatter.go` — `frontmatterSetOperation.Execute` and the refusals it runs before any mutation (`blockedBySetRefusal`, `metricsSessionsWriteRefusal`, `todoPlanningSetRefusal`, `flagProvenanceSetRefusal`). Note the existing `previousAssignee := task.Assignee()` pattern: read the prior value before mutating, then compose every field into the single `WriteTask` at the end.
- `pkg/ops/workon.go` — `persistSessionAndMetrics`. This is the intended contract on the other path: stamp `claude_session_id` only when empty, always append the incoming id. Do **not** change it.
- `pkg/ops/metrics_session_append.go` — the public `append-metrics-session` verb. It validates a UUID before reading the task. The internal append must **not** route through it: the spec deliberately uses `AAAA` / `BBBB`, which that verb refuses.
- `pkg/domain/task_frontmatter_metrics.go` — `MetricsSessions()` and `AppendMetricsSession(entry MetricsSession)`. The latter ignores an empty `SessionID` and appends without deduplicating.
- `pkg/domain/metrics.go` — the `MetricsSession` struct: `SessionID string` and `StartedAt libtime.DateOrDateTime`.
- `pkg/domain/task_frontmatter.go` — `ClaudeSessionID()` and `SetField`'s `"claude_session_id"` arm.
- `pkg/ops/metrics_session_write.go` — the absolute refusal for caller-supplied `metrics_sessions` writes. Leave it untouched.
- `pkg/ops/frontmatter_test.go` — the existing `Describe("FrontmatterSetOperation", …)` setup, especially the `"setting claude_session_id field"` context. Its default task has no `claude_session_id`, so it stays a first-bind case.
- `pkg/ops/metrics_session_append_test.go` — the clock-pinning pattern: `libtime.NewCurrentDateTime()` + `SetNow(libtimetest.ParseDateTime(...))`, and the typed-entry assertions.
- `integration/cli_test.go` — `createTempVault`, and the `Describe("task append-metrics-session", …)` block. Its local helpers (`runEntityCommand`, `readFile`, frontmatter parsing, `- session_id:` counting) are closure-scoped; declare your own in the new file.
- `docs/work-on-session-lifecycle.md` — the paragraph beginning `**The generic write verbs refuse the field.**` and the union statement at `**Per-entry removal is what the shared-session rule requires.**`

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap` / `errors.Errorf` from `github.com/bborbe/errors`; never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external test packages, `gexec` session assertions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and the required `fix:` prefix.
</context>

<requirements>
1. **Add one unexported helper** to `pkg/ops/frontmatter.go`, immediately after `frontmatterSetOperation.Execute` and before `writeTaskCloseOutFieldsIfCloseOut`. Use exactly this signature and behaviour:

   ```go
   // appendDisplacedSession appends the outgoing owner to metrics_sessions when a
   // claude_session_id write replaces a different non-empty id. The exit and close
   // paths resolve a session by matching claude_session_id or any metrics_sessions
   // entry, so the displaced id must stay on the task or it can never be resolved
   // again. A first bind (no previous owner) and a re-bind to the same value append
   // nothing. The incoming id is deliberately not appended: work-on and the
   // session-connect append are the writers that establish ownership, and appending
   // both would double-count the incoming session.
   func (o *frontmatterSetOperation) appendDisplacedSession(task *domain.Task, key, value string) {
       previous := task.ClaudeSessionID()
       if key != "claude_session_id" || previous == "" || previous == value {
           return
       }
       task.AppendMetricsSession(domain.MetricsSession{
           SessionID: previous,
           StartedAt: libtime.DateOrDateTime(o.currentDateTime.Now().Time()),
       })
   }
   ```

   `domain` and `libtime` are already imported in `pkg/ops/frontmatter.go`. Do not add imports and do not add a new exported symbol.

2. **Call the helper once, before any mutation of the session id.** In `frontmatterSetOperation.Execute`, immediately after `previousAssignee := task.Assignee()` and before `writeTaskCloseOutFieldsIfCloseOut`, add:

   ```go
   o.appendDisplacedSession(task, key, value)
   ```

   The existing `task.SetField(ctx, key, value)` later in the function stores the new value, and the existing single `o.taskStorage.WriteTask(ctx, task)` persists both changes together. Do **not** add a second `WriteTask` and do **not** move the append after the write.

3. **Preserve the existing refusal and clear behaviour.**
   - `metricsSessionsWriteRefusal` keeps refusing a caller-supplied `metrics_sessions` value on `task set` / `task add` / `task remove`. The append is keyed on `claude_session_id`, so it never touches that path.
   - `frontmatterClearOperation` (`vault-cli task clear <task> claude_session_id`) is unchanged: it deletes the key and appends nothing.
   - `persistSessionAndMetrics` in `pkg/ops/workon.go` is unchanged: stamp only when empty, always append the incoming id.
   - The completion clear in `pkg/ops/complete.go` and the spawn-failure clear in `work-on` are unchanged and never route through this operation.
   - `task set <task> claude_session_id ""` is a replacement like any other: it appends the outgoing id before storing the empty value.

4. **Add unit specs** in a new file `pkg/ops/frontmatter_session_takeover_test.go`, package `ops_test`, with the BSD header (`Copyright (c) 2026 Benjamin Borbe`). Wrap them in `var _ = Describe("FrontmatterSetOperation session take-over", func() { ... })`. Use this setup:
   - `mockTaskStorage := &mocks.TaskStorage{}`.
   - A pinned clock: `pinned := libtime.NewCurrentDateTime(); pinned.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))`.
   - `ops.NewFrontmatterSetOperation(mockTaskStorage, pinned, ops.NewEscalationPublisher("", "", &mocks.NotificationSenderFactory{}), "personal", "25 Tasks")`.
   - A task `domain.NewTask(map[string]any{"status": "in_progress"}, domain.FileMetadata{Name: "Alpha"}, domain.Content(""))`, returned by `FindTaskByNameReturns`; `WriteTaskReturns(nil)`.
   - A `bind := func(value string) error { return setOp.Execute(ctx, "/vault", "Alpha", "claude_session_id", value, "", "", "", false) }` helper.

   Use the non-UUID ids `"AAAA"` and `"BBBB"` from the spec, so the specs also prove the internal append does not go through the UUID-validating verb. Cover:
   - **Take-over (AC1/AC3/AC4):** bind `AAAA`, then bind `BBBB`. `task.ClaudeSessionID()` is `"BBBB"`; `task.MetricsSessions()` has length 1; entry 0 is `SessionID == "AAAA"` and `StartedAt == libtime.DateOrDateTime(libtimetest.ParseDateTime("2026-03-03T12:00:00Z").Time())`; `WriteTaskCallCount() == 2` (one write per command).
   - **First bind (AC2):** bind `BBBB` on an unowned task. `MetricsSessions()` is empty and the write count is 1.
   - **Same value (AC5):** bind `AAAA` twice. `MetricsSessions()` is empty.
   - **Empty replacement:** bind `AAAA`, then bind `""`. `ClaudeSessionID()` is `""` and `MetricsSessions()` has exactly `"AAAA"`.
   - **Order preserved:** seed the task with `claude_session_id: "AAAA"` and a pre-existing `metrics_sessions` entry `"OLDER"` with a distinct timestamp; bind `"BBBB"`. The entries are `["OLDER", "AAAA"]` in that order and the older timestamp is unchanged.
   - **Narrowness:** with `claude_session_id: "AAAA"`, execute `set phase execution`. No `metrics_sessions` entry appears.
   - **Unreadable task (Failure Modes row 1):** `FindTaskByNameReturns(nil, errors.New(ctx, "boom"))`; the bind returns an error containing `"find task"` and `WriteTaskCallCount() == 0`.

5. **Add binary-level specs** in a new file `integration/task_set_session_takeover_test.go`, package `integration_test`, with the BSD header. Wrap them in `var _ = Describe("task set session take-over", func() { ... })`. Use `createTempVault` with a single task `"Alpha"` whose frontmatter is alphabetically ordered, e.g.:

   ```text
   ---
   page_type: task
   status: in_progress
   task_identifier: 11111111-1111-4111-8111-111111111111
   ---
   body line
   ```

   Run the real binary with `exec.Command(binPath, "--config", configPath, "--vault", "test", ...)` through `gexec.Start`, asserting with `Eventually(session).Should(gexec.Exit(N))`. Read the task at `<vaultPath>/Tasks/Alpha.md` and parse its frontmatter with `gopkg.in/yaml.v3` (unmarshalling the whole file fails on the body). Cover:
   - **AC1 + AC3 + AC4:** bind `AAAA`, then bind `BBBB` inside a `start := time.Now()` / `end := time.Now()` window. Afterwards:
     - `task get Alpha claude_session_id --output json` has `value == "BBBB"`.
     - `task get Alpha metrics_sessions --output json` has a `value` containing `session_id:AAAA` and not containing `session_id:BBBB`.
     - On disk, `strings.Count(content, "- session_id: AAAA") == 1` and `strings.Count(content, "- session_id: BBBB") == 0`. The `- ` list-item prefix is load-bearing: a bare `session_id: BBBB` would also match `claude_session_id: BBBB`.
     - The `AAAA` entry's `started_at` is non-empty and is not the zero time. Accept either a `string` or a `time.Time` from the YAML decoder. When it carries a time, it parses as RFC3339 and lies within `[start.Add(-time.Minute), end.Add(time.Minute)]`; the shared time type renders a midnight-UTC instant as a date only (`YYYY-MM-DD`), so accept that shape too and assert it equals `start.UTC().Format("2006-01-02")`.
   - **AC2:** on a fresh vault, bind `BBBB` once. `task get Alpha metrics_sessions --output json` has `value == ""`, and the file contains no `metrics_sessions:` key.
   - **AC5:** bind `AAAA`, then `BBBB`, capture the `metrics_sessions` value, bind `BBBB` again, and assert the value is identical and `strings.Count(content, "- session_id:") == 1`.

   Declare the command runner, file reader, frontmatter parser, and entry lookup as local closures in the new file; the sibling block's helpers are not visible.

6. **Update `docs/work-on-session-lifecycle.md`** (AC8). Replace the paragraph beginning `**The generic write verbs refuse the field.**` with a version that keeps the caller-supplied refusal and adds the internal exception. Use this text:

   ```markdown
   **The generic write verbs refuse a caller-supplied value for the field.** `task set`,
   `task add` and `task remove` refuse a `metrics_sessions` value supplied by the caller,
   before any mutation, with no `--force` bypass, because `set` stores a scalar and `add` /
   `remove` comma-split into a list of scalars — both shapes the dedicated list reader
   discards. That refusal lives in `pkg/ops/metrics_session_write.go`. The one internal
   exception is the `claude_session_id` write: `task set <task> claude_session_id <new>`
   that displaces a different non-empty owner appends that outgoing id to
   `metrics_sessions` before storing the new id, so a taken-over session stays resolvable
   by the exit and close paths. The append is internal to the session-id write, not a
   caller-facing way to write the field.
   ```

   Do not change the union statement later in the document, and do not change `docs/session-liveness.md` (it answers a narrower question keyed on `claude_session_id` alone).

7. **Record the change in `CHANGELOG.md`.** Insert a `## Unreleased` section directly below the preamble (`All notable changes…` and the `* MAJOR / MINOR / PATCH` lines) and above the newest `## vX.Y.Z`, with one bullet:

   ```markdown
   - fix: `vault-cli task set <task> claude_session_id <new>` appends the displaced owner to `metrics_sessions` before writing the new id, so a session handed off by a take-over stays resolvable by `session-self-exit.py` and `close-worker.py`. A first bind and a re-bind to the same value append nothing; the completion and spawn-failure clears are unchanged.
   ```

   Do **not** bump any version string: this repo's release automation owns the four version fields (`CHANGELOG.md` top version, `.claude-plugin/plugin.json`, and both `.claude-plugin/marketplace.json` fields). Add the unreleased bullet only.

8. **Self-check before finishing.** Walk AC1–AC8 of the spec against the diff, then run the full `<verification>` block.
</requirements>

<constraints>
- No new CLI verb and no new flag: the behaviour attaches to the existing `task set` path. `pkg/cli/cli.go` needs no change.
- `metrics_sessions` stays refused through `task set` / `task add` / `task remove` for caller-supplied values. The append is internal to the `claude_session_id` write. Do not touch `pkg/ops/metrics_session_write.go`.
- `work-on`'s `persistSessionAndMetrics` keeps its current shape: stamp only when empty, always append the incoming id. Do not touch `pkg/ops/workon.go`.
- Only the outgoing id is appended. Do not append the incoming id: `work-on` and the session-connect append already record it, and appending both would double-count it.
- Existing well-formed `metrics_sessions` entries are preserved in order; the new entry is appended last. Go through `domain.TaskFrontmatter.AppendMetricsSession` — the same writer the work-on path and the public append verb use. Do not add a raw-preserving writer.
- The internal append takes the previous id verbatim, with no UUID validation. Do not route it through `AppendMetricsSessionOperation`, which refuses `AAAA`.
- The task file is written once per command, not twice.
- Two writers racing on the same task remain last-write-wins; do not add a lock, a re-read, or deduplication. Entries accumulate by design and a duplicate is harmless to both readers.
- `vault-cli task clear <task> claude_session_id` keeps its current behaviour: delete the key, no append.
- Goal and topic set operations are out of scope; only the task path changes.
- `agents/work-on-task-assistant.md` needs no edit: its claim branch already appends the incoming id through `append-metrics-session`, and the new internal append records the outgoing id — different ids, so nothing is double-counted.
- No backfill of `metrics_sessions` onto existing tasks.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `make test` iteratively while implementing, then `make precommit` once at the end. `make precommit` must exit 0.

Targeted runs:

```
go build ./...
go test -mod=mod ./pkg/ops/... -ginkgo.focus="session take-over" -ginkgo.v
go test -mod=mod ./integration/... -ginkgo.focus="session take-over" -ginkgo.v
```

`-ginkgo.focus` with zero matching specs exits 0 while running nothing. After each focused run, confirm the new spec names appear in the output; a log without them is a failure, not a pass.

Evidence greps (run each and confirm the count):

```
grep -c 'func (o \*frontmatterSetOperation) appendDisplacedSession' pkg/ops/frontmatter.go   # 1
grep -c 'o.appendDisplacedSession(task, key, value)' pkg/ops/frontmatter.go                  # 1
grep -c 'task.AppendMetricsSession' pkg/ops/frontmatter.go                                   # 1
grep -c 'func metricsSessionsWriteRefusal' pkg/ops/metrics_session_write.go                  # 1 (untouched)
grep -c 'func persistSessionAndMetrics' pkg/ops/workon.go                                    # 1 (untouched)
grep -c 'FrontmatterSetOperation session take-over' pkg/ops/frontmatter_session_takeover_test.go  # 1
grep -c 'task set session take-over' integration/task_set_session_takeover_test.go           # 1
grep -c -- '- session_id: AAAA' integration/task_set_session_takeover_test.go                # >= 1
grep -c 'caller-supplied value for the field' docs/work-on-session-lifecycle.md              # 1
sed -n '/^## Unreleased/,/^## v/p' CHANGELOG.md | grep -c 'claude_session_id'                # >= 1
```

Syntax and formatting:

```
gofmt -e -l pkg/ops/frontmatter.go pkg/ops/frontmatter_session_takeover_test.go integration/task_set_session_takeover_test.go
```

The command must print nothing. `make precommit` runs `generate` (which recreates `mocks/`; the interface is unchanged, so the output is stable) and `addlicense` (which restores the BSD header on every `.go` file, including the two new test files).
</verification>
