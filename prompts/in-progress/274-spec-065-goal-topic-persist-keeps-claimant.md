---
status: approved
spec: [065-bug-goal-topic-bind-drops-mid-turn-session]
created: "2026-11-02T21:58:52Z"
queued: "2026-11-02T22:08:40Z"
---

# Goal and topic work-on keep a mid-turn claimant and record the binding session

<summary>
- When a goal or topic is claimed by another session while a `work-on` turn is still running, that session keeps ownership of the page afterwards.
- The session that ran the turn is no longer lost: it is recorded in the page's session history with the time its turn started.
- A page nobody claimed during the turn is stamped exactly as before and gains no history entry.
- A page the turn's own session stamped during the turn gains no history entry either.
- Earlier history entries on the page are kept, and the new entry is added last.
- The caller of `work-on` still receives the id of the session that ran the turn, in every case.
- The turn-start time comes from an injected clock, so tests pin it.
- The session-lifecycle documentation and the changelog describe the new rule.
- Regression tests prove the old overwrite is gone for both goals and topics.
</summary>

<objective>
Change `persistGoalSessionID` (`pkg/ops/goal_workon.go`) and `persistTopicSessionID` (`pkg/ops/topic_workon.go`) so they stamp `claude_session_id` only when the re-read page still has it empty; when it holds a different non-empty id, that claimant keeps the stamp and the binding session is appended to `metrics_sessions` as `{session_id, started_at: <turn start>}` in the same single write. Today both persists overwrite unconditionally after a turn of up to 30 minutes, silently erasing whichever session claimed the page meanwhile — and goal/topic pages had no history to record it in (prompt 1 added that record shape). This is spec 065 prompt 2 of 2: Desired Behaviors 1–4, AC1–AC10.
</objective>

<context>
Read `CLAUDE.md`, `docs/development-patterns.md`, and `docs/dod.md`.

Read these files fully before changing anything:

- `specs/in-progress/065-bug-goal-topic-bind-drops-mid-turn-session.md` — Goal, Desired Behavior, Assumptions (the append rule deliberately differs from tasks; a losing TTY caller still resumes its own session), Constraints, Failure Modes, AC1–AC10.
- `pkg/ops/goal_workon.go` — `NewGoalWorkOnOperation(goalStorage storage.GoalStorage, uuidGenerator func() string, starter ClaudeSessionStarter, resumer ClaudeResumer) GoalWorkOnOperation`, the `goalWorkOnOperation` struct, `persistGoalSessionID(ctx context.Context, vaultPath string, goalName string, sessionID string, goalStorage storage.GoalStorage) (string, error)`, and `handleClaudeSession` (existing-owner early return; TTY branch and non-interactive branch each call `StartSession` then `persistGoalSessionID`).
- `pkg/ops/topic_workon.go` — the topic twins: `NewTopicWorkOnOperation(topicStorage storage.TopicStorage, uuidGenerator func() string, starter ClaudeSessionStarter, resumer ClaudeResumer) TopicWorkOnOperation`, `persistTopicSessionID(ctx context.Context, vaultPath string, topicName string, sessionID string, topicStorage storage.TopicStorage) (string, error)` (it stamps through `refreshed.SetField(ctx, "claude_session_id", sessionID)`), and `handleClaudeSession` (reads the owner via `topic.GetField("claude_session_id")`).
- `pkg/ops/workon.go` — the pattern to mirror: `NewWorkOnOperation(..., currentDateTime libtime.CurrentDateTime, uuidGenerator func() string, ...)`, the `currentDateTime libtime.CurrentDateTime` struct field, `persistSessionAndMetrics(ctx, vaultPath, taskName, sessionID string, startedAt libtime.DateOrDateTime, taskStorage storage.TaskStorage) (string, error)`, and the turn-start capture `startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())`. Do **not** change this file.
- `pkg/domain/goal_frontmatter.go` / `pkg/domain/topic_frontmatter.go` — the `MetricsSessions()` / `AppendMetricsSession(entry MetricsSession)` methods prompt 1 added (promoted onto `*domain.Goal` / `*domain.Topic`), and `GoalFrontmatter.ClaudeSessionID()` / `SetClaudeSessionID(v string)`.
- `pkg/domain/metrics.go` — `MetricsSession{SessionID string; StartedAt libtime.DateOrDateTime}`.
- `pkg/ops/export_test.go` — the pattern for exposing unexported symbols to the external `ops_test` package (`var RollupMedian = rollupMedian`).
- `pkg/cli/cli.go` — `createWorkOnGoalCommand` (call `ops.NewGoalWorkOnOperation(goalStore, uuid.NewString, starter, resumer)`) and `createWorkOnTopicCommand` (call `ops.NewTopicWorkOnOperation(topicStore, uuid.NewString, starter, resumer)`). `libtime` is already imported; other commands build `currentDateTime := libtime.NewCurrentDateTime()`.
- `pkg/ops/goal_workon_test.go`, `pkg/ops/topic_workon_test.go`, `pkg/ops/workon_session_writeback_test.go` — every existing constructor call site, the mock setup (`mocks.GoalStorage`, `mocks.TopicStorage`, `mocks.ClaudeSessionStarter`), `pinnedSessionID`, and the real-storage harness (`storage.NewGoalStorage(&storage.Config{...})`, `os.MkdirTemp`).
- `pkg/ops/workon_test.go` — clock pinning: `currentDateTime := libtime.NewCurrentDateTime(); currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))` with `libtimetest "github.com/bborbe/time/test"`.
- `docs/work-on-session-lifecycle.md` — `## Post-exit write ordering`, especially the paragraph beginning `The **goal path** (`pkg/ops/goal_workon.go`) keeps its post-exit ordering unchanged`.
- `CHANGELOG.md` — top of file; there is currently no `## Unreleased` section.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`
</context>

<requirements>
1. **Inject the clock into the goal operation** (`pkg/ops/goal_workon.go`). Change the constructor to mirror `NewWorkOnOperation`'s ordering (storage, clock, uuid generator, …):

   ```go
   func NewGoalWorkOnOperation(
   	goalStorage storage.GoalStorage,
   	currentDateTime libtime.CurrentDateTime,
   	uuidGenerator func() string,
   	starter ClaudeSessionStarter,
   	resumer ClaudeResumer,
   ) GoalWorkOnOperation
   ```

   Add `currentDateTime libtime.CurrentDateTime` to `goalWorkOnOperation` and import `libtime "github.com/bborbe/time"`. The `GoalWorkOnOperation` interface is unchanged (mocks need no regeneration).

2. **Same for topics** (`pkg/ops/topic_workon.go`): `NewTopicWorkOnOperation(topicStorage storage.TopicStorage, currentDateTime libtime.CurrentDateTime, uuidGenerator func() string, starter ClaudeSessionStarter, resumer ClaudeResumer) TopicWorkOnOperation`, plus the struct field. `TopicWorkOnOperation` interface unchanged.

3. **Capture turn start before the turn.** In both `handleClaudeSession` methods, after `sessionID := …uuidGenerator()` and **before** the `if isInteractive` split (so both the TTY and the non-interactive branch use it), add:

   ```go
   startedAt := libtime.DateOrDateTime(g.currentDateTime.Now().Time()) // o.currentDateTime in the topic file
   ```

   Pass `startedAt` to the persist call on **both** branches. The existing-owner early return (`if existing := …; existing != "" { return existing, nil }`) stays exactly as it is and takes no clock read. The persist stays post-exit: it runs only after `StartSession` returns nil, as today.

4. **New goal persist contract.** Change the signature to (parameter order mirrors `persistSessionAndMetrics`):

   ```go
   func persistGoalSessionID(
   	ctx context.Context,
   	vaultPath string,
   	goalName string,
   	sessionID string,
   	startedAt libtime.DateOrDateTime,
   	goalStorage storage.GoalStorage,
   ) (string, error)
   ```

   Behaviour, after the existing re-read (`goalStorage.FindGoalByName`):
   - stored `refreshed.ClaudeSessionID()` is `""` → `refreshed.SetClaudeSessionID(sessionID)`; append nothing.
   - stored equals `sessionID` → change nothing (no re-stamp, no append). This is the binder's own child session-connect case (AC8).
   - stored is a different non-empty id → leave it in place; `refreshed.AppendMetricsSession(domain.MetricsSession{SessionID: sessionID, StartedAt: startedAt})`.
   - Then exactly one `goalStorage.WriteGoal(ctx, refreshed)`, as today.
   - Return `sessionID` (the **binding** session) on success in all three cases — never the stored owner. On the TTY branch this value feeds `ResumeSession`; a caller that lost the race resumes its own session by design (spec Assumptions).
   - Failure paths unchanged: re-read error → `"", errors.Wrap(ctx, err, "re-read goal after claude session")`, nothing written; write error → `"", errors.Wrap(ctx, err, "save session id to goal")`.

   Rewrite the doc comment: it no longer "writes back only the session id"; state the stamp-only-if-empty rule, that a different claimant keeps the stamp and the binder is recorded in `metrics_sessions` at turn start, that the append rule deliberately differs from tasks (an uncontended bind adds no entry), and keep the existing re-read and "empty id on failure" rationale.

5. **New topic persist contract** — `persistTopicSessionID(ctx context.Context, vaultPath string, topicName string, sessionID string, startedAt libtime.DateOrDateTime, topicStorage storage.TopicStorage) (string, error)` with the same three-way rule. Read the stored owner with `refreshed.GetField("claude_session_id")`; stamp an empty field with the existing `refreshed.SetField(ctx, "claude_session_id", sessionID)` (keep its `"save session id to topic"` error wrap); append with `refreshed.AppendMetricsSession(...)`. One `topicStorage.WriteTopic` per persist; same failure paths and return value as the goal persist. Update the doc comment the same way.

6. **Wire the clock at the CLI** (`pkg/cli/cli.go`). In `createWorkOnGoalCommand` and `createWorkOnTopicCommand`, pass `libtime.NewCurrentDateTime()` as the new second argument (a local `currentDateTime := libtime.NewCurrentDateTime()` declared in the `RunE`, as sibling commands do, is equally fine). These are the only production call sites; confirm with `grep -rn 'NewGoalWorkOnOperation(\|NewTopicWorkOnOperation(' --include='*.go' .` and update every hit.

7. **Update every existing test call site** in `pkg/ops/goal_workon_test.go`, `pkg/ops/topic_workon_test.go` and `pkg/ops/workon_session_writeback_test.go` to pass a clock (`libtime.NewCurrentDateTime()` is fine where `started_at` is not asserted). If an existing test asserts that a different non-empty stored id is overwritten, change it to the new contract and note that in your report; no such assertion is known today.

8. **Expose the persist functions to `ops_test`** in `pkg/ops/export_test.go`, following its existing doc-comment style:

   ```go
   // PersistGoalSessionID exposes persistGoalSessionID for the external ops_test package.
   var PersistGoalSessionID = persistGoalSessionID

   // PersistTopicSessionID exposes persistTopicSessionID for the external ops_test package.
   var PersistTopicSessionID = persistTopicSessionID
   ```

9. **Regression tests (AC9)** in a new file `pkg/ops/goal_topic_session_rebind_test.go`, package `ops_test`, BSD header `Copyright (c) 2026 Benjamin Borbe`, wrapped in `var _ = Describe("goal and topic session rebind", func() { … })`. Use a pinned turn-start `T1 = libtimetest.ParseDateTime("2026-03-03T12:00:00Z")` and the claimant id `"XXXX-interactive"` (non-UUID on purpose: the id is taken verbatim). Cover, for **both** goal and topic:

   a. **Persist, mocks — claimant kept, binder appended.** Storage mock's `FindGoalByNameReturns` / `FindTopicByNameReturns` a page with `claude_session_id: XXXX-interactive`; call `ops.PersistGoalSessionID` / `ops.PersistTopicSessionID` with `pinnedSessionID` and `libtime.DateOrDateTime(T1.Time())`. Assert: returned id `== pinnedSessionID`; the written page (`WriteGoalArgsForCall(0)` / `WriteTopicArgsForCall(0)`) still reads `XXXX-interactive`; `MetricsSessions()` has exactly one entry with `SessionID == pinnedSessionID` and `StartedAt` equal to T1; write count 1. These specs must fail against the pre-fix code.
   b. **Persist, mocks — empty field stamped, nothing appended (AC4).** Stored id empty → written id `== pinnedSessionID`, `MetricsSessions()` is `nil`, `Get("metrics_sessions")` is `nil`.
   c. **Persist, mocks — own id, nothing appended (AC8).** Stored id `== pinnedSessionID` → written id unchanged, `MetricsSessions()` is `nil`.
   d. **Persist, mocks — existing entries kept (AC6).** Stored page has `claude_session_id: XXXX-interactive` and one pre-seeded entry `{session_id: "seeded-session", started_at: "2026-01-01T08:00:00Z"}` (generic `[]any{map[string]any{…}}` shape); after persist the entries are `["seeded-session", pinnedSessionID]` in that order and the seeded `started_at` is unchanged.
   e. **Persist, mocks — failure modes.** Re-read error → `""` returned, error contains `"re-read goal after claude session"` / `"re-read topic after claude session"`, write count 0. Write error → `""` returned, error contains `"save session id to goal"` / `"save session id to topic"`.
   f. **Execute, mocks — turn start is captured before the turn (AC2/AC7).** Build the operation with a clock pinned at T1. In `mockStarter.StartSessionStub`, simulate the mid-turn claimant by setting the page the storage mock returns to `claude_session_id: XXXX-interactive` **and** advance the clock to `T2 = libtimetest.ParseDateTime("2026-03-03T12:30:00Z")`. Run `Execute` with `isInteractive = false`. Assert `result.SessionID == pinnedSessionID`, the last written page reads `XXXX-interactive`, and its single `metrics_sessions` entry has `StartedAt` equal to **T1**, not T2. Repeat with `isInteractive = true` and a `mocks.ClaudeResumer`: assert `ResumeSessionCallCount() == 1` and its session-id argument (`ResumeSessionArgsForCall(0)`) is `pinnedSessionID`, not `XXXX-interactive` (spec Assumption: a losing TTY caller still resumes its own session), and the entry's `StartedAt` is again T1.
   g. **Real storage round-trip (boundary).** Seed a goal file under a temp vault (`storage.NewGoalStorage(&storage.Config{GoalsDir: "23 Goals"})`) and a topic file (`storage.NewTopicStorage(&storage.Config{TopicsDir: "23 Topics"})`) whose frontmatter carries `claude_session_id: XXXX-interactive`; call the exported persist; read the file bytes and assert `strings.Count(content, "- session_id: "+pinnedSessionID) == 1`, `strings.Count(content, "- session_id: XXXX-interactive") == 0` (AC5 — the claimant is owner, not history), and `claude_session_id: XXXX-interactive` is still present. The `- ` list-item prefix is load-bearing: a bare `session_id:` also matches `claude_session_id:`. Then re-read through the storage and confirm `MetricsSessions()[0].StartedAt.Time().Equal(T1.Time())`. Also seed an uncontended page (no `claude_session_id`), persist, and assert the file contains no `metrics_sessions:` key.

   Use `os.MkdirTemp` + `DeferCleanup(os.RemoveAll)` for temp vaults.

10. **Document the rule** in `docs/work-on-session-lifecycle.md` § `Post-exit write ordering` (AC10). Replace the two-line paragraph beginning `The **goal path** (`pkg/ops/goal_workon.go`) keeps its post-exit ordering unchanged:` with this text (keep the line breaks — the verification greps one physical line):

    ```markdown
    The **goal and topic paths** (`pkg/ops/goal_workon.go`, `pkg/ops/topic_workon.go`) keep
    their post-exit ordering unchanged: `persistGoalSessionID` / `persistTopicSessionID` run
    only after `StartSession` returns cleanly. Because the turn can last up to the turn
    timeout, another session can claim the page inside that window — a concurrent
    `work-on`, an interactive `/vault-cli:work-on-goal`, or the headless child's own
    session-connect. The persist therefore re-reads the page and stamps `claude_session_id`
    only when it is still empty. When it holds a different non-empty id, that claimant keeps
    the stamp and the goal or topic persist records the binding session in `metrics_sessions`
    (`session_id` plus `started_at`, the turn start captured before `StartSession`) in the
    same single write. When it already holds the binding id, nothing is appended. The append
    rule deliberately differs from the task path: an uncontended goal or topic bind adds no
    `metrics_sessions` entry. `work-on` still returns the binding session's id in every case,
    and on the TTY branch that is the id it resumes. Two binds racing on the persist are
    last-write-wins in the re-read→write gap, the same accepted race as on tasks.
    ```

    Change nothing else in the document.

11. **CHANGELOG.** Insert a `## Unreleased` section below the preamble (`All notable changes…` and the three `* MAJOR / MINOR / PATCH` lines) and above `## v0.186.1` (or above whatever the newest `## vX.Y.Z` is when this runs; if `## Unreleased` already exists, add the bullet there). One bullet:

    ```markdown
    - fix: `goal work-on` / `topic work-on` no longer overwrite a session that claimed the page during the headless turn. The persist stamps `claude_session_id` only when it is still empty; when another session holds it, that session keeps the stamp and the binding session is recorded in the page's `metrics_sessions` with its turn-start time. An uncontended bind is stamped as before and adds no entry.
    ```

    Do not bump any version string — the releaser owns the four version fields.

12. **Self-check.** Before finishing, re-run `<verification>` and confirm every step passes; walk AC1–AC10 of the spec against the change (AC1–AC8 are exercised operator-side against a built binary, but each must be implied by the tests in requirement 9).
</requirements>

<constraints>
- Goal and topic binds take the task path's ownership rule: stamp `claude_session_id` only when it is empty at persist time. The append rule deliberately differs: only a binder that finds a **different** non-empty owner is appended; an empty field or the binder's own id appends nothing.
- `work-on` returns the binding session's id to its caller in every case; the persist functions must not return the stored owner.
- `handleClaudeSession`'s existing-owner early return is unchanged.
- The persist stays post-exit (spec 041): the id is written only after the turn validates.
- One write per persist, re-read first, exactly as today.
- `NewGoalWorkOnOperation` and `NewTopicWorkOnOperation` take an injected `libtime.CurrentDateTime`, as `NewWorkOnOperation` does, wired at their `pkg/cli/cli.go` call sites, so tests pin the turn-start time.
- The task path (`persistSessionAndMetrics`, `pkg/ops/workon.go`), `goal set` / `topic set`, and `task append-metrics-session` are unchanged.
- No new CLI verb or flag.
- The id is taken verbatim, with no UUID validation.
- No backfill of pages displaced before this fix; no change to `session-self-exit.py` or any close path.
- The re-read→write race between two persists is accepted (last write wins); add no lock and no dedup.
- Errors wrap with `github.com/bborbe/errors`; tests use Ginkgo/Gomega with Counterfeiter mocks.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `ROOTDIR=/workspace make test` — must exit 0.

Focused run of the AC9 regression specs (confirm the `goal and topic session rebind` specs appear in the verbose output — a focus matching zero specs exits 0 while running nothing):

```
go test -mod=mod ./pkg/ops/... -ginkgo.focus="goal and topic session rebind" -ginkgo.v
```

Evidence greps:

```
grep -c 'startedAt libtime.DateOrDateTime' pkg/ops/goal_workon.go     # >= 1
grep -c 'startedAt libtime.DateOrDateTime' pkg/ops/topic_workon.go    # >= 1
grep -c 'currentDateTime libtime.CurrentDateTime' pkg/ops/goal_workon.go   # >= 1
grep -c 'currentDateTime libtime.CurrentDateTime' pkg/ops/topic_workon.go  # >= 1
grep -c 'AppendMetricsSession' pkg/ops/goal_workon.go    # >= 1
grep -c 'AppendMetricsSession' pkg/ops/topic_workon.go   # >= 1
grep -c 'var PersistGoalSessionID = persistGoalSessionID' pkg/ops/export_test.go     # 1
grep -c 'var PersistTopicSessionID = persistTopicSessionID' pkg/ops/export_test.go   # 1
grep -qF 'the stamp and the goal or topic persist records the binding session in `metrics_sessions`' docs/work-on-session-lifecycle.md
grep -n 'metrics_sessions' docs/work-on-session-lifecycle.md | grep -q 'goal'
awk '/^## /{sec=$0} /goal work-on. \/ .topic work-on. no longer overwrite/{print sec}' CHANGELOG.md | grep -qx '## Unreleased'
```

Finally run `ROOTDIR=/workspace make precommit` — must exit 0.
</verification>
