---
spec: ["045-bug-exit-code-outranks-validated-turn"]
status: draft
created: "2026-09-06T13:20:00Z"
---

# Task and goal write-back under the new precedence (spec 045, prompt 2 of 3)

<summary>
- Proves on real files that a task whose headless turn produced a valid result keeps its session id, even when the underlying process exited non-zero — the case the Vault UI needs in order to offer Resume.
- Proves the same for a goal, which persists its id on a different code path that must not be left behind.
- Proves the undo still fires for a genuinely failed turn: a child that reports its own failure leaves no session id on the task file.
- Proves the same undo on the goal path, where nothing is persisted for a failed turn in the first place.
- Confirms that the error an operator sees for a genuinely failed turn leads with the child's own explanation rather than a process exit status.
- Tests only — no behavior change in this prompt.
</summary>

<objective>
Extend the two caller-side write-back test files so the clear-vs-retain consequence of the new precedence is proven end-to-end through `Execute` on real vault files, for both the task path and the goal path. Covers spec 045 Desired Behavior 6 and Acceptance Criteria 5 and 6.
</objective>

<context>
This prompt depends on prompt 1 of spec 045 being already applied — `runDetachedTurn` now treats a validated turn result as authoritative over a non-zero child exit, and `validateSessionTurn`'s rejection messages lead with the child's own `result` text followed by the failed predicate in parentheses (`claude reported is_error: true`, `claude returned num_turns: 0`, `claude returned empty session_id`).

Read `CLAUDE.md` and `docs/dod.md` first. Then read in full:

- `pkg/ops/workon_session_writeback_test.go` — the whole file. Note: the shared `newStarter(detachRun)` helper, the spec-local `bw := blockWaiter` capture inside the waiter closure (a waiter that returns immediately races the select and makes the outcome nondeterministic), the `pinnedSessionID` constant, and the existing three Contexts: the happy-path `Context("task work-on")` (child writes frontmatter + a valid blob, `done <- nil`) and `Context("when the child exits non-zero inside the liveness window")` (child writes frontmatter, writes **nothing** to stdout, `done <- errors.New("exit status 1")`, asserts the id was cleared).
- `pkg/ops/goal_workon_test.go` — the whole file. Note `Context("goal work-on early exit rollback")`, which builds a real goal store on a temp vault, seeds `23 Goals/Rollback Goal.md`, and drives `Execute`; and note the file imports stdlib `errors` as `stderrors` while `github.com/bborbe/errors` is imported as `errors`.
- `pkg/ops/workon.go` — `handleClaudeSession`. The task path pre-persists the id plus a `metrics_sessions` entry BEFORE spawning, and runs a re-read-based compensating clear when `StartSession` returns an error.
- `pkg/ops/goal_workon.go` — `handleClaudeSession` (a separate method from the task one, on `goalWorkOnOperation`). The non-interactive goal path persists the id only AFTER a successful turn via `persistGoalSessionID`, and has no compensating clear because nothing was written.
- `pkg/ops/claude_session.go` — `runDetachedTurn` and `validateSessionTurn` as they now stand, so the seeded blobs and asserted messages match the real implementation.

Read this coding-plugin doc (in-container path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo/Gomega conventions in this codebase.
</context>

<requirements>

1. **Add a retain spec to `pkg/ops/workon_session_writeback_test.go`** — new `Context`, modelled on the existing `Context("when the child exits non-zero inside the liveness window")` but with the opposite outcome. Name it so the intent is legible, e.g. `Context("when the child exits non-zero after writing a valid turn result")`.

   Seed the fixture with `phase: execution` (same as the sibling Context's `rollbackFixture`). The `detachRun` fake must, in this order: re-read the task through `taskStore.FindTaskByName`, set `domain.TaskPhasePlanning` — deliberately DIFFERENT from the seeded value, or the "child's write survived" assertion is vacuous — write it back (proving the child's frontmatter write survives), write the valid blob
   `{"session_id":"<pinnedSessionID>","num_turns":3,"is_error":false,"result":"done"}`
   to the `stdout *os.File`, then send `errors.New("exit status 1")` on a buffered `done` channel. The waiter must block, exactly as the neighbouring Contexts do.

   Drive it through `workOnOp.Execute(...)` (never a direct `StartSession` call — the persist/clear is caller-side and only `Execute` observes it) and assert:
   - `Expect(err).To(BeNil())` and `Expect(result.Success).To(BeTrue())` and `Expect(result.SessionID).To(Equal(pinnedSessionID))`
   - On the raw file bytes (`os.ReadFile` of `24 Tasks/Repro Task.md`), `strings.Count(string(raw), "claude_session_id:")` equals `1` — this is the in-test form of the spec's `grep -c '^claude_session_id:' <task file>` evidence.
   - `strings.Contains(string(raw), pinnedSessionID)` is true, and `strings.Contains(string(raw), "phase: planning")` is true — the fixture seeded `execution`, so this only passes if the child's write actually landed.

   Add a comment stating what this spec is for: a non-zero exit no longer discards a turn whose result validated, so the pre-persisted id survives and the Vault UI can offer Resume.

2. **Add an `is_error` clear spec to `pkg/ops/workon_session_writeback_test.go`** — new `Context`, e.g. `Context("when the child reports its own failure")`. The `detachRun` fake writes the child's frontmatter change (same re-read/set-phase/write-back shape as the sibling Contexts, so the "preserve the child's write" property is asserted here too), then writes
   `{"session_id":"<pinnedSessionID>","num_turns":0,"is_error":true,"result":"seeded failure text"}`
   to `stdout`, then sends `errors.New("exit status 1")` on the `done` channel.

   Assert through `Execute`:
   - `Expect(err).To(HaveOccurred())` and `Expect(result.Success).To(BeFalse())`
   - `err.Error()` contains `seeded failure text`, and the child's reason precedes any exit-status mention. `Execute` wraps with `start work-on session`, so `HavePrefix` is unavailable at this level; assert ordering explicitly:

     ```go
     msg := err.Error()
     Expect(msg).To(ContainSubstring("seeded failure text"))
     if idx := strings.Index(msg, "exit status"); idx >= 0 {
         Expect(strings.Index(msg, "seeded failure text")).To(BeNumerically("<", idx))
     }
     ```

     Spec AC2 permits an exit-status mention as a trailing clause; it must never precede the child's own reason. Do NOT assert full-string absence of `exit status` — that would narrow prompt 1's contract below what the spec allows.
   - On the raw file bytes, `strings.Count(string(raw), "claude_session_id:")` equals `0` and `strings.Contains(string(raw), pinnedSessionID)` is false — the compensating clear removed both the id and this run's metrics entry.
   - The child's frontmatter write survived the clear: `strings.Contains(string(raw), "phase: planning")` is true, given the fixture seeded `phase: execution`. Do not seed and write the same phase — that assertion would pass even if the child wrote nothing.

3. **Add the mirrored pair to `pkg/ops/goal_workon_test.go`**, alongside `Context("goal work-on early exit rollback")` and using the same real-goal-store-on-a-temp-vault setup that Context already establishes (temp vault dir, `23 Goals` + `24 Tasks` dirs, a seeded goal fixture, `ops.NewSessionLockerWithDir` on a temp lock dir, a blocking waiter with a `DeferCleanup` close). Critical detail: this file drives `Execute` from a top-level `JustBeforeEach` (~line 75) over the outer `vaultPath`, `goalName`, and `goalWorkOnOp` vars — each new Context's `BeforeEach` MUST reassign all three exactly as the rollback Context does (~lines 524-531), or the spec silently drives the mock store and passes meaninglessly. Do not add a Context-local `Execute` call. The file does not currently import `strings` — add it:

   - **Retain**: `detachRun` writes `{"session_id":"<pinnedSessionID>","num_turns":3,"is_error":false,"result":"done"}` to `stdout` and sends `stderrors.New("exit status 1")`. Assert `err` is nil, `result.Success` is true, and the goal re-read through the real store has `ClaudeSessionID() == pinnedSessionID`. Also assert on the raw goal file bytes that `claude_session_id:` appears exactly once. Add a comment: `pkg/ops/goal_workon.go` has its own `handleClaudeSession` that persists only after a successful turn, so the goal path must be proven separately from the task path — it is the one most easily left behind.
   - **Clear**: `detachRun` writes `{"session_id":"<pinnedSessionID>","num_turns":0,"is_error":true,"result":"seeded failure text"}` to `stdout` and sends `stderrors.New("exit status 1")`. Assert `err` occurred, `result.Success` is false, `err.Error()` contains `seeded failure text` with any `exit status` mention appearing strictly after it (same ordering assertion as requirement 2 — never a full-string absence assertion), and the raw goal file (`23 Goals/Rollback Goal.md`) contains zero occurrences of `claude_session_id:`. Note in a comment that the goal path needs no compensating clear because nothing was persisted for a failed turn — the assertion proves the invariant, not a clear.

4. **Do not modify the two protected specs in the files this prompt may touch** (a third, in `pkg/ops/claude_session_test.go`, belongs to prompt 1 and is out of scope here). The `"goal work-on early exit rollback preserves the child's frontmatter write"` spec and the `"clears the pre-persisted session id and preserves the child's frontmatter write when the child exited non-zero inside the window"` spec must stay byte-identical, along with their `detachRun` fakes that ignore the `*os.File` parameter. They encode the zero-length-output contract: exit status is authoritative only when there is no usable blob.

5. **Do not change production code in this prompt.** No edits under `pkg/ops/*.go` except the two `_test.go` files named above. If a new spec fails, the fix belongs in the test's seeded blob or fake, not in `runDetachedTurn` — unless the failure reveals prompt 1 shipped the wrong routing, in which case fix `pkg/ops/claude_session.go` and say so explicitly in the completion summary.

6. **Do not touch** `docs/work-on-session-lifecycle.md`, `CHANGELOG.md`, or anything under `scenarios/` — prompt 3 owns those.

7. **Self-check before finishing.** Re-run every command in `<verification>` and confirm it passes. Then walk spec 045 Acceptance Criteria 5 and 6 against the four new specs and state which spec satisfies which half.

</requirements>

<constraints>
- Tests use Ginkgo v2 + Gomega with Counterfeiter mocks — no stdlib `t.Run` table tests.
- The waiter in every new spec must BLOCK (closed by `DeferCleanup`). A waiter that returns immediately makes both select branches ready and flips the outcome nondeterministically between success and a spurious turn-timeout error.
- Capture the block channel into a spec-local variable before the waiter closure reads it — `StartSession` can return via the child-exit branch while the waiter goroutine is still parked, so that goroutine outlives the spec and must not read a variable the next spec reassigns.
- Every new success-path fake must write a valid JSON blob to the `stdout *os.File`, or validation fails with `parse claude output`.
- The compensating clear is not weakened by this spec — a genuinely failed turn must still clear the id. These tests are the proof, not a relaxation.
- Errors wrap via `github.com/bborbe/errors` with a real `ctx` — no `fmt.Errorf`, no bare `return err` — if any helper code is added.
- Note for the agent, not a task: an older spec (041) recorded evidence greps pinning the count of `ClaudeSessionID()` / `MetricsSessions()` accessor calls in `workon_session_writeback_test.go` at 2. Those counts are historical evidence for a completed spec, are not enforced by any script or by `make precommit`, and adding specs here legitimately changes them. Prefer raw-file `strings.Count` assertions (as the existing rollback spec does) so the new specs assert the spec-045 evidence shape directly.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from the repo root:

```
make precommit
```

Must exit 0.

Then:

```
go test ./pkg/ops/... 2>&1 | tail -5
```
must report no failures.

```
grep -c 'seeded failure text' pkg/ops/workon_session_writeback_test.go
```
must print `1` or more.

```
grep -c 'seeded failure text' pkg/ops/goal_workon_test.go
```
must print `1` or more.

```
grep -c 'claude_session_id:' pkg/ops/workon_session_writeback_test.go
```
must print `3` or more (the pre-existing rollback assertion plus the two new ones).

```
grep -c 'claude_session_id:' pkg/ops/goal_workon_test.go
```
must print `2` or more (the two new raw-file assertions).

```
grep -c 'exit status 1' pkg/ops/workon_session_writeback_test.go
```
must print `4` or more. The baseline is 2 — the protected spec's `done <- errors.New(...)` line and its `ContainSubstring` assertion — plus one line from each of the two new specs. A threshold of 3 is satisfied by adding only one of the two.

```
test "$(grep -c 'is_error\":false' pkg/ops/workon_session_writeback_test.go)" -ge 3
test "$(grep -c 'is_error\":false' pkg/ops/goal_workon_test.go)" -ge 3
```
Both must exit 0. Baseline in each file is 2 (the existing happy-path blobs), so this
gates the AC6 *retain* half — the actual bug being fixed. Without it, every other check
here is satisfiable by adding only the two *clear* specs.
</verification>
