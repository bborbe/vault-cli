---
status: completed
summary: Restored both-branch warning printing in printTaskCompleteResult with an injectable writer, wrapped the session-mode error, made the non-forced TTY test assert unconditionally, and extended the existing Unreleased changelog bullet
execution_id: vault-cli-force-tty-exec-285-force-tty-review-fixes
dark-factory-version: v0.198.17
created: "2026-11-18T21:05:00Z"
queued: "2026-11-18T21:11:51Z"
started: "2026-11-18T21:15:03Z"
completed: "2026-11-18T21:19:26Z"
---

# Restore warnings on the refusal path and harden the force-guard tests

<summary>
- The task-complete helper again prints warnings in both branches, as the inline code it replaced did — the refusal branch is unreachable today, so this is behaviour preservation, not a user-visible fix.
- The new guard no longer contains a bare `return err`; the error from the session-mode probe is wrapped like every other error in the package.
- The non-forced `complete` test asserts unconditionally instead of only when an error happens to be returned, so a silently broken command path can no longer pass it.
- No behaviour change to the TTY guard itself: a forced complete still refuses off a terminal and still proceeds at one.
- `make precommit` passes.
</summary>

<objective>
`printTaskCompleteResult` prints `result.Warnings` in both the refusal and the success branch exactly as the inline code did before the extraction, and the two test/error-handling gaps the review found are closed, with no other behavioural change.
</objective>

<context>
Read `CLAUDE.md` for project conventions. Read `docs/development-patterns.md`.

Error wrapping: `go-error-wrapping-guide.md` (coding plugin docs) — `errors.Wrap(ctx, err, msg)`, never a bare `return err` in new code.

Read before implementing:
- `pkg/cli/cli.go`:
  - `createCompleteCommand` (~line 240): its `RunE` calls `printTaskCompleteResult(result)` in the non-JSON branch.
  - `printTaskCompleteResult` (~line 299): the helper added by the previous commit. It currently `return`s after printing the refusal reason, so `result.Warnings` is skipped whenever `result.Reason != ""`.
  - `requireForceFromTerminal` (~line 546): calls `resolveSessionMode(ctx, "auto")` and currently does a bare `return err` on the impossible error branch.
  - `resolveSessionMode` (~line 519): its `auto` branch returns `(term.IsTerminal(fd), nil)` and can never error.
- `pkg/cli/force_tty_test.go`: the specs added by the previous commit; the non-forced case is written as `if err != nil { Expect(...) }`.
- `pkg/ops/complete.go` (~line 234): `checkSubtaskCompletion` is the only producer of a non-empty `Result.Reason`. It sets no `Warnings` today, so the dropped-warnings path is latent — fix it anyway: the extraction claimed to be behaviour-preserving and is not.

Before this commit the inline code printed the warnings in BOTH branches:
```go
if result.Reason != "" {
    fmt.Printf("⚠️  Cannot complete: %s\n", result.Reason)
} else {
    fmt.Printf("✅ Task completed: %s\n", result.Name)
}
for _, w := range result.Warnings {
    fmt.Printf("⚠️  %s\n", w)
}
```
</context>

<requirements>
1. In `pkg/cli/cli.go`, restore both-branch warning printing in `printTaskCompleteResult`, and take the destination writer as a parameter so the output is testable without capturing `os.Stdout`:
   ```go
   // printTaskCompleteResult writes the human-readable outcome of a task
   // completion. runMutation handles the JSON case, so this is plain mode only.
   // Warnings print in both the refusal and the success branch: the refusal
   // branch used to return early and drop them.
   func printTaskCompleteResult(w io.Writer, result ops.MutationResult) {
       if result.Reason != "" {
           fmt.Fprintf(w, "⚠️  Cannot complete: %s\n", result.Reason)
       } else {
           fmt.Fprintf(w, "✅ Task completed: %s\n", result.Name)
       }
       for _, warning := range result.Warnings {
           fmt.Fprintf(w, "⚠️  %s\n", warning)
       }
   }
   ```
   Update its one call site in `createCompleteCommand`'s `RunE` to `printTaskCompleteResult(os.Stdout, result)`, and add `io` to the imports.

   Then add a `PrintTaskCompleteResultForTest(w io.Writer, result ops.MutationResult)` hook in `pkg/cli/export_test.go` (matching the existing `FormatRollupWeeklyPlainForTest` pattern) and a spec in `pkg/cli/force_tty_test.go` that writes into a `bytes.Buffer` with both `Reason` and `Warnings` set and asserts the buffer contains both — so the changed behaviour is covered even though the production refusal branch is unreachable. Never swap `os.Stdout` in the test.

2. In `pkg/cli/cli.go`, replace the bare `return err` in `requireForceFromTerminal` with a wrapped error, matching `go-error-wrapping-guide.md` ("never bare `return err`"):
   ```go
   interactive, err := resolveSessionMode(ctx, "auto")
   if err != nil {
       return errors.Wrap(ctx, err, "resolve session mode")
   }
   ```

3. In `pkg/cli/force_tty_test.go`, make the non-forced spec assert unconditionally. With no config file the loader falls back to a default vault at `$HOME/Documents/vault` (`getDefaultConfig`), so `cli.Run` fails on the task lookup rather than on config; the assertion that matters is that the error is not the TTY refusal:
   ```go
   It("does not gate a non-forced task complete", func() {
       err := cli.Run(context.Background(), []string{"task", "complete", "Some Task"})
       Expect(err).To(HaveOccurred(), "no task store is reachable, so this must fail before any TTY gate")
       Expect(err.Error()).NotTo(ContainSubstring("stdin is not a terminal"))
   })
   ```
   Keep every other spec in that file unchanged.

4. Extend the existing `## Unreleased` bullet in `CHANGELOG.md` rather than adding a new one — this branch has not been released, so a fresh `- fix:` line would record a fix to code that never shipped. Append to the end of the existing bullet:
   ```
   ; the extracted task-complete helper now prints warnings in both branches and wraps the session-mode error
   ```
   Do not create a second `## Unreleased` heading; `grep -c '^## Unreleased$' CHANGELOG.md` must print `1` afterwards.

5. Before finishing, re-run `<verification>` and confirm every line passes.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do not change the TTY guard's behaviour, its message text, its placement, or which commands carry `--force`.
- Do not touch `commands/complete-task.md` or `commands/complete-goal.md` — out of scope for this branch.
- Do not revert or edit `specs/` or `prompts/` record files.
- Error handling: `github.com/bborbe/errors` with `ctx`; never `fmt.Errorf`, never a bare `return err`.
- Tests: Ginkgo v2 / Gomega only.
</constraints>

<verification>
- `make precommit` → exit 0.
- `grep -c '^## Unreleased$' CHANGELOG.md` → exactly 1.
- `grep -A9 'func printTaskCompleteResult' pkg/cli/cli.go | grep -cE '^[[:space:]]*return$'` → 0 and `grep -A9 'func printTaskCompleteResult' pkg/cli/cli.go | grep -c 'range result.Warnings'` → 1 (the early return is gone AND the warnings loop is still present, so neither the unfixed code nor a fix that deletes the loop passes).
- `grep -A8 'func requireForceFromTerminal' pkg/cli/cli.go | grep -c 'return err$'` → 0 (the window covers the whole function so an inserted line cannot shift the bare return out of range; the file's other bare `return err` sites are pre-existing and out of scope).
- `go test ./pkg/cli/...` → green.
</verification>
