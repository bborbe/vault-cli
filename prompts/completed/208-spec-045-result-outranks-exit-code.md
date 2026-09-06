---
status: completed
spec: [045-bug-exit-code-outranks-validated-turn]
summary: Inverted runDetachedTurn precedence in pkg/ops/claude_session.go so a validated turn result decides success and the child's exit code only decides when no usable result exists; added errClaudeOutputUnparseable sentinel, rejectTurn helper, doc-comment updates, and AC1-AC4 Ginkgo specs
execution_id: vault-cli-exit-code-exec-208-spec-045-result-outranks-exit-code
dark-factory-version: dev
created: "2026-09-06T13:20:00Z"
queued: "2026-09-06T13:42:38Z"
started: "2026-09-06T13:57:06Z"
completed: "2026-09-06T14:00:37Z"
---

# Result outranks exit code in the detached turn (spec 045, prompt 1 of 3)

<summary>
- A headless session whose child process produced a complete, valid turn result is no longer thrown away just because that process exited non-zero.
- The captured turn result becomes the authority on success; the exit code is consulted only when there is no usable result to judge.
- When the child's result is present but rejected, the reported error now leads with the child's own explanation instead of an opaque process status, so an operator reads the real reason first.
- When the result is missing, unreadable, or not valid turn JSON, the reported error still names the process exit status — that is the only case where the exit code decides.
- A turn that runs past its wait bound, or whose wait is cancelled, keeps failing exactly as before, and deliberately never inspects the partially-written result.
- The existing rule that a genuinely failed turn produces an error (so the caller can undo its bookkeeping) is unchanged — only the definition of "failed" moves.
- Unit tests cover all four result/exit combinations plus the timeout-with-a-complete-result case, which locks out the tempting refactor that would let a still-running child's partial output count as success.
- One pre-existing lock-lifecycle test that accidentally depended on the old precedence is corrected so it still tests what it claims to test.
</summary>

<objective>
Invert the precedence inside `runDetachedTurn` so a validated turn result decides success and the child's exit code only decides when no usable result exists. Covers spec 045 Desired Behaviors 1-5 and Acceptance Criteria 1-4.
</objective>

<context>
Read `CLAUDE.md` and `docs/dod.md` for project conventions first.

Read in full before changing anything:

- `pkg/ops/claude_session.go` — the whole file. The change is confined to `runDetachedTurn` and `validateSessionTurn`. Note the existing shape: `runDetachedTurn` creates the temp file, defers `os.Remove` + `Close`, calls `c.detachRun(args, cwd, outFile)`, then `select`s on the child-exit channel `done` against `waitCh` (fed by `c.waiter.Wait`). Today the `case exitErr := <-done:` branch returns immediately when `exitErr != nil`, before the `os.ReadFile(outFile.Name())` + `validateSessionTurn(ctx, output)` pair that sits below the `select`.
- `pkg/ops/errors.go` — the package's sentinel-error pattern (`stderrors "errors"` import alias, `stderrors.New(...)`, doc comment stating what the sentinel means). The new sentinel this prompt adds follows this exact pattern.
- `pkg/ops/claude_session_test.go` — the whole file. Ginkgo v2 + Gomega, `ops_test` package. The `Context("non-interactive branch")` block is where the new specs go; note its `validTurnJSON` constant, its blocking-waiter discipline (`bw := blockWaiter` captured spec-locally, never read from the outer variable inside a waiter closure), and how each spec that needs a bespoke child rebuilds `starter` via `ops.NewClaudeSessionStarterWithRunner`.
- `pkg/ops/workon_session_writeback_test.go` and `pkg/ops/goal_workon_test.go` — read only the specs that assert `ContainSubstring("exit status 1")` (search for that string). They must keep passing byte-identical; their `detachRun` fakes take `_ *os.File` and write nothing, so they exercise the zero-length-output path.

Read these coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` wrapping with a real `ctx`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo/Gomega conventions in this codebase.

Library facts already verified against the module — do not re-derive them:
- `github.com/bborbe/errors` exposes `Wrap(ctx, err, msg)`, `Wrapf(ctx, err, format, args...)`, `Errorf(ctx, format, args...)`, `New(ctx, msg)` and `Is(err, target)`. `Wrap`/`Wrapf` build on `github.com/pkg/errors`, so `Error()` reads `"<message>: <cause>"` and the chain supports `errors.Is`.
- `Errorf(ctx, ...)`'s `Error()` is exactly the formatted string — no prefix is added. That is what makes a result-text-first message possible.
- The file already imports `"encoding/json"`, `"log/slog"`, `"os"` and `"github.com/bborbe/errors"` (aliased as `errors`). Adding a sentinel needs a `stderrors "errors"` import, matching `pkg/ops/errors.go`.
</context>

<requirements>

1. **Add an unparseable-output sentinel** to `pkg/ops/claude_session.go`, declared immediately above `validateSessionTurn`, following the `pkg/ops/errors.go` pattern (add the `stderrors "errors"` import to this file):

   ```go
   // errClaudeOutputUnparseable marks a turn result that could not be parsed at all,
   // as opposed to one that parsed and then failed a predicate. The distinction is
   // load-bearing: a parsed blob carries the child's own `result` text and can explain
   // itself, while an unparseable one cannot — so only the unparseable case falls back
   // to the child's exit status as the reason.
   var errClaudeOutputUnparseable = stderrors.New("claude output is not valid turn JSON")
   ```

2. **Rewrite `validateSessionTurn`'s three rejection messages so the child's own `result` text leads.** Keep the function as the single shared validator for both branches, keep the struct, keep the predicate order (`session_id` → `num_turns` → `is_error`), and keep the `ctx`-carrying `github.com/bborbe/errors` constructors.

   - Parse failure: return `errors.Wrapf(ctx, errClaudeOutputUnparseable, "parse claude output: %v", jsonErr)`. The message must still contain the literal substring `parse claude output` (the existing interactive-branch spec asserts it) and `errors.Is(err, errClaudeOutputUnparseable)` must be true.
   - Add a small unexported helper beside `validateSessionTurn` that composes a rejection so the three predicate branches do not duplicate the empty-result fallback:

     ```go
     // rejectTurn builds the error for a turn result that parsed but failed a predicate.
     // The child's own `result` text leads, because it is the only part of the message an
     // operator can act on; the predicate name follows in parentheses. When the child
     // reported no result text at all there is nothing to lead with, so the predicate
     // stands alone.
     func rejectTurn(ctx context.Context, resultText string, reason string) error {
         if resultText == "" {
             return errors.New(ctx, reason)
         }
         return errors.Errorf(ctx, "%s (%s)", resultText, reason)
     }
     ```

   - Empty `session_id` → `rejectTurn(ctx, result.Result, "claude returned empty session_id")`
   - `num_turns == 0` → `rejectTurn(ctx, result.Result, "claude returned num_turns: 0")`
   - `is_error == true` → `rejectTurn(ctx, result.Result, "claude reported is_error: true")`

   Each reason string deliberately contains the JSON field name (`session_id`, `num_turns`, `is_error`) — spec AC2 asserts on those names.

3. **Invert the precedence inside `runDetachedTurn`.** Move the read + validate pair from below the `select` into the `case exitErr := <-done:` branch. The waiter branch is untouched and must NOT read the file. The resulting branch, in order:

   1. `output, readErr := os.ReadFile(outFile.Name())` — on error `return errors.Wrap(ctx, readErr, "read claude output")`. This is the fd/permission failure mode; the read error is the reason, not the exit status.
   2. `validateErr := validateSessionTurn(ctx, output)`.
   3. `validateErr == nil` → the turn succeeded. If `exitErr != nil`, emit exactly one line, verbatim:

      ```go
      slog.Warn("validated turn result overrides non-zero child exit", "err", exitErr)
      ```

      This is the only trace of a signal that is now deliberately ignored, so the message string is pinned by a `<verification>` check rather than left to wording. Then `return nil`.
   4. `exitErr != nil && errors.Is(validateErr, errClaudeOutputUnparseable)` → `return errors.Errorf(ctx, "claude session exited with error: %v", exitErr)`. A zero-length file lands here naturally, because unmarshalling empty bytes is a parse failure. This is the sole surviving path where the exit code is the reason, and this string must appear exactly once in the file.
   5. Otherwise `return validateErr` **unwrapped**. Do not wrap it — `errors.Wrap` would prepend text and destroy the result-text-leads property AC2 asserts.

   Keep the existing `// The child has exited, so its fd is closed and the file is complete.` reasoning as a comment inside the branch, and add a comment stating explicitly why the read may not be hoisted above the `select`: on the timeout and cancellation paths the child is still running, so any bytes present are partial by definition and must never be validated as success.

4. **Update the stale doc comments in `pkg/ops/claude_session.go`** so the file's prose matches the new contract:
   - `ClaudeSessionStarter.StartSession`'s comment currently says "Any outcome other than a clean, validated turn returns an error". Restate it as: the turn is judged by its validated result; a non-zero child exit is not itself a failure when the result validates.
   - `runDetachedTurn`'s comment currently says "every exit path except a clean, validated turn returns an error". Restate the same way, and keep the existing rationale about not returning before the child exits.
   - Do not touch the `sessionTurnTimeout` comment, `defaultDetachedRunner`, `defaultCommandRunner`, the `maxTurns` field comment, or the interactive branch.

5. **Update the existing assertions that pin the old predicate wording** in `pkg/ops/claude_session_test.go`. These are the only pre-existing assertions this prompt may change, and each changes for a stated reason:
   - Interactive spec `"returns error containing 0 turns and result"`: `ContainSubstring("0 turns")` → `ContainSubstring("num_turns")`. Keep the `ContainSubstring("Unknown command: /x")` assertion.
   - Non-interactive spec `"validates the turn and rejects a zero-turn result"`: `ContainSubstring("0 turns")` → `ContainSubstring("num_turns")`.
   - Non-interactive spec `"validates the turn and rejects an is_error result"`: `ContainSubstring("claude reported error")` → `ContainSubstring("claude reported is_error")`.
   - The `empty session_id` / `missing session_id field` / `rejects an unparseable turn result` specs need no change — verify that and leave them alone.

6. **Fix the one pre-existing spec that silently depended on the old precedence.** In the `Context("session lock lifecycle")` block, the spec `"releases the lock when the child exits with an error"` sends `lockDoneCh <- ErrTest` while that Context's shared `JustBeforeEach` `detachRun` writes `validTurnJSON` to the stdout file. Under the new precedence that combination is a **success**, so the spec's `Expect(err).To(HaveOccurred())` would fail and the lock-release property it exists to prove would go untested.

   Rebuild `starter` inside that spec (the same way sibling specs in that Context already do) with a `detachRun` that takes `_ *os.File`, writes nothing, increments `spawnCount`, and returns a channel carrying `ErrTest`. Keep both assertions (`HaveOccurred`, `ContainSubstring("exited with error")`) and the re-acquire/release assertions unchanged. Add a comment explaining that the child must leave the output file empty for the exit status to be authoritative under the result-over-exit-code precedence. Do not change the sibling lock specs.

7. **Add new specs to `Context("non-interactive branch")` in `pkg/ops/claude_session_test.go`.** Each rebuilds `starter` locally with `ops.NewClaudeSessionStarterWithRunner`, captures the blocking waiter spec-locally (`bw := blockWaiter`) exactly as neighbouring specs do, and drives `starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)`:

   - **AC1 — valid blob, non-zero exit → success.** `detachRun` writes `{"session_id":"session-abc","num_turns":3,"is_error":false,"result":"done"}` to `stdout` and returns a channel carrying `errors.New("exit status 1")` (this test file imports stdlib `errors` unaliased — the same call the neighbouring `"treats a child exit error as an error"` spec already makes). Assert `Expect(err).To(BeNil())`.
   - **AC2 — parsed-but-rejected blob leads with the child's reason.** Three specs, each with a non-zero exit on the channel and a seeded distinctive `result` string (e.g. `"seeded failure text"`):
     - `{"session_id":"session-abc","num_turns":2,"is_error":true,"result":"seeded failure text"}` → `Expect(err.Error()).To(HavePrefix("seeded failure text"))` and `ContainSubstring("is_error")`.
     - `{"session_id":"session-abc","num_turns":0,"is_error":false,"result":"seeded failure text"}` → `HavePrefix("seeded failure text")` and `ContainSubstring("num_turns")`.
     - `{"session_id":"","num_turns":2,"is_error":false,"result":"seeded failure text"}` → `HavePrefix("seeded failure text")` and `ContainSubstring("session_id")`.
     Each of the three must additionally assert `Expect(err.Error()).NotTo(HavePrefix("exit status"))` — an exit-status mention may trail, but must never precede the child's own reason.
   - **AC3 — non-empty but unparseable output with a non-zero exit names the exit status.** `detachRun` writes `not valid json at all` to `stdout` and returns a channel carrying `errors.New("exit status 1")`. Assert the message contains `exit status`. (The zero-length half of AC3 is already covered by the untouched spec `"treats a child exit error as an error"` — do not modify it.)
   - **AC4 — timeout with a valid blob already on disk still fails.** `detachRun` writes the valid blob to `stdout` and returns a channel that never fires (`make(chan error)`); the waiter returns `nil` immediately so the timeout branch wins the select. Assert `err != nil` and `ContainSubstring("did not complete within")`. Add a comment naming this as the regression lock against hoisting the read above the `select`: the child is still running, so the bytes present are partial by definition.
   - **Read error is surfaced as a read error, not an exit status** is not separately testable without an injected filesystem — do not fake it, and do not add a filesystem seam for it. The wrap in requirement 3.1 is the implementation; leave it untested here.

8. **Do NOT touch** `pkg/ops/workon.go`, `pkg/ops/goal_workon.go`, `pkg/ops/workon_session_writeback_test.go`, `pkg/ops/goal_workon_test.go`, `pkg/ops/workon_test.go`, `docs/work-on-session-lifecycle.md`, `CHANGELOG.md`, or anything under `scenarios/`. Prompts 2 and 3 of this spec own those.

9. **Self-check before finishing.** Re-run every command in `<verification>` and confirm each passes. Then walk spec 045 Acceptance Criteria 1-4 one by one against the actual diff and state which spec or code line satisfies each.

</requirements>

<constraints>
- `validateSessionTurn` stays the single shared validator for both branches — do not fork its logic into the detached path.
- **The read must stay inside the child-exited branch.** Hoisting `os.ReadFile` above the `select` is the specific refactor this spec forbids: a timed-out child is still writing, so its partial blob could validate as success. The `awk` check in `<verification>` fails if the read moves.
- The temp file must still be unlinked on every return path, including cancel and timeout where the child holds the fd — do not touch the existing `defer` that does `os.Remove` + `Close`.
- Stderr still goes to `os.DevNull` — `defaultDetachedRunner` is unchanged.
- The interactive branch's `cmd.Output()` + `validateSessionTurn` call sequence must not change.
- The compensating clear is not weakened: `runDetachedTurn` must still return an error for a genuinely failed turn. Only the definition of "failed" moves.
- Tests use Ginkgo v2 + Gomega with Counterfeiter mocks — no stdlib `t.Run` table tests.
- Errors wrap via `github.com/bborbe/errors` with a real `ctx` — no `fmt.Errorf`, no bare `return err`.
- The three existing `exit status 1` assertions must keep passing **unchanged**: the `"treats a child exit error as an error"` spec in `pkg/ops/claude_session_test.go`, the `"goal work-on early exit rollback..."` spec in `pkg/ops/goal_workon_test.go`, and the `"clears the pre-persisted session id..."` spec in `pkg/ops/workon_session_writeback_test.go`. Their `detachRun` stubs ignore the `*os.File` parameter, so they exercise the zero-length-output path and encode the AC3 contract. If a change makes one of them fail, the empty-file case has been misrouted to the predicate branch — fix the routing, never the assertion.
- Do NOT add config fields, flags, thresholds, or metrics. The spec asks for a precedence inversion and one log line; nothing else.
- Do NOT root-cause why a clean-`end_turn` child exits 1 — explicit spec Non-goal.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from the repo root:

```
make precommit
```

Must exit 0.

Then every one of these must exit 0. They are written as self-failing assertions on
purpose: a bare `grep -c` exits 0 for any non-zero count, and a piped `go test` takes
the exit status of the last stage, so both forms report success on unchanged code.

```
test "$(grep -c 'claude session exited with error' pkg/ops/claude_session.go)" = "1"
test "$(grep -c 'errClaudeOutputUnparseable' pkg/ops/claude_session.go)" -ge 3
test "$(grep -c 'did not complete within' pkg/ops/claude_session_test.go)" -ge 3
test "$(grep -c 'HavePrefix' pkg/ops/claude_session_test.go)" -ge 3
test "$(grep -c 'exited with error' pkg/ops/claude_session_test.go)" -ge 2
test "$(grep -c 'validated turn result overrides non-zero child exit' pkg/ops/claude_session.go)" = "1"
```

The read must sit inside the child-exited case branch, not merely below the `select`
keyword — the unchanged file already satisfies "read line > select line", so that
weaker form proves nothing:

```
awk '/case exitErr := <-done:/{a=NR} /case err := <-waitCh:/{b=NR} /os\.ReadFile\(outFile\.Name\(\)\)/{r=NR} END{exit !(a>0 && b>a && r>a && r<b)}' pkg/ops/claude_session.go
```

The two stale contract sentences in the `runDetachedTurn` doc comment must be gone
(requirement 4 is otherwise unchecked and a lazy implementation skips it silently):

```
! grep -q 'Any outcome other than a clean' pkg/ops/claude_session.go
! grep -q 'except a clean, validated turn returns an error' pkg/ops/claude_session.go
```

The new AC1 and AC3 specs must exist by name, or an agent that ships the code change
and skips them passes everything above:

```
grep -q 'returns nil when the child writes a valid blob and exits non-zero' pkg/ops/claude_session_test.go
grep -q 'names the exit status when the output is non-empty but unparseable' pkg/ops/claude_session_test.go
grep -q 'read claude output' pkg/ops/claude_session.go
```

No stdlib error formatting, and the suite passes unpiped:

```
! grep -q 'fmt.Errorf' pkg/ops/claude_session.go
go test ./pkg/ops/...
```
</verification>
