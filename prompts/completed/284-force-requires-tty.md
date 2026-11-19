---
status: completed
summary: Added a terminal-required guard refusing --force on task/goal complete when stdin is not a TTY, with unit and integration coverage, and a CHANGELOG entry
execution_id: vault-cli-force-tty-exec-284-force-requires-tty
dark-factory-version: v0.198.17
created: "2026-11-18T19:35:00Z"
queued: "2026-11-18T19:44:19Z"
started: "2026-11-18T19:46:55Z"
completed: "2026-11-18T19:52:37Z"
---

# Refuse `--force` on `task complete` and `goal complete` when stdin is not a terminal

<summary>
- `--force` on `vault-cli task complete` and `vault-cli goal complete` is refused when stdin is not a terminal: exit 1, a message naming "stdin is not a terminal", and the page left unchanged.
- An operator at a terminal keeps the override: when stdin is a terminal the same command still completes the page.
- The refusal happens before any vault is read or written, so a refused invocation cannot partially mutate a page.
- `vault-cli topic complete` deliberately gains no `--force` flag — it has no close-out gate for the flag to override, and the guard must not ship a placeholder flag.
- A non-forced `complete` is unchanged: it still refuses an open-box page and still lists the open items.
- There is exactly one TTY-detection path (the same one `--mode auto` uses) and no global switch a test could flip.
- The existing integration spec that asserted a forced complete succeeds is replaced by one that asserts the refusal and that the page is left untouched.
- New unit tests cover the refusal, the terminal-accepted path, the unchanged non-forced path, and the absence of `--force` on `topic complete`.
- Ships with a CHANGELOG entry; the full precommit gate stays green.
</summary>

<objective>
`vault-cli task complete <name> --force` and `vault-cli goal complete <name> --force` fail with exit code 1 and a message naming "stdin is not a terminal" when stdin is not a TTY, and behave exactly as today when it is a TTY. No other command gains or loses a flag.
</objective>

<context>
Read `CLAUDE.md` for project conventions. Read `docs/development-patterns.md` § Testability and § Naming — dependencies are injected as parameters, never reached through package-level mutable state.

Read before implementing:
- `pkg/cli/cli.go`:
  - `resolveSessionMode(ctx, mode)` (~line 519): the existing stdin-TTY probe. Its `auto` branch does `fd := int(os.Stdin.Fd()) //#nosec G115 ...` then `term.IsTerminal(fd)`. **Reuse this function — do not write a second probe.** The `//#nosec G115` annotation is load-bearing: `gosec` is enabled and only `G104` is excluded, so an unannotated `int(os.Stdin.Fd())` cast fails `make precommit`.
  - `createCompleteCommand` (~line 240): declares `var force bool`, registers the flag at the bottom with `cmd.Flags().BoolVar(&force, "force", false, "Complete even if the task has incomplete checkboxes")`, and its `RunE` runs `taskName := args[0]` then `vaults, err := getVaults(...)`.
  - `createGoalCompleteCommand` (~line 1472): same shape, flag text "Complete even if open tasks are linked to this goal".
  - `createTopicCompleteCommand` (~line 1799): carries a doc comment explaining why it has no `--force`, `--reason` or `--gate-successor`. Leave that command and its comment exactly as they are.
  - `Run(ctx, args)` (~line 214) and `Execute()` (~line 3301): `Execute` prints `Error: %v` to stderr and exits 1 when `Run` returns an error. Returning an error from `RunE` is therefore the whole exit-code and stderr contract.
- `pkg/cli/export_test.go`: test-only exports for `package cli_test`, one wrapper per symbol, no state mutation.
- `pkg/cli/task_approve_help_test.go`: the pattern for walking the built command tree (`cli.NewRootCommand(context.Background()).Commands()` then each subcommand's `Commands()`).
- `go.mod`: `golang.org/x/term v0.48.0` is already a direct dependency.

Why `term.IsTerminal` and not `os.Stdin.Stat()`: `/dev/null` is a character device, so `(stat.Mode() & os.ModeCharDevice) != 0` reports a TTY for `< /dev/null` and the guard would silently do nothing. `term.IsTerminal` issues an `isatty` ioctl and correctly reports false for `/dev/null` and true for a pty.
</context>

<requirements>
1. In `pkg/cli/cli.go`, directly after `resolveSessionMode`, add two small functions. Do NOT add a package-level variable — no `var stdinIsTTY`, no setter:
   ```go
   // requireForceFromTerminal refuses --force when stdin is not a terminal.
   // A forced complete overrides the closing check, so it must be confirmable
   // by a human at a terminal; an agent piping input, or a process running with
   // stdin redirected, must not be able to take the override.
   func requireForceFromTerminal(ctx context.Context, force bool) error {
       if !force {
           return nil
       }
       interactive, err := resolveSessionMode(ctx, "auto")
       if err != nil {
           return err
       }
       return requireTTYForForce(ctx, force, interactive)
   }

   // requireTTYForForce returns an error when force is set and isTTY is false.
   // It is separate from requireForceFromTerminal so both branches are
   // unit-testable without allocating a pty.
   func requireTTYForForce(ctx context.Context, force bool, isTTY bool) error {
       if !force || isTTY {
           return nil
       }
       return errors.Errorf(
           ctx,
           "--force requires an interactive terminal: stdin is not a terminal, so the override cannot be confirmed by a human. Drop --force, or re-run from a terminal.",
       )
   }
   ```
   The message MUST contain the literal substring `stdin is not a terminal` — the unit spec in requirement 5 asserts on it.

2. In `pkg/cli/cli.go`, call the guard in both `RunE` bodies immediately before `getVaults` (it may follow `taskName := args[0]` / `goalName := args[0]`):
   ```go
   if err := requireForceFromTerminal(ctx, force); err != nil {
       return err
   }
   ```
   - `createCompleteCommand` (`task complete`)
   - `createGoalCompleteCommand` (`goal complete`)
   Nothing else in either command changes: same flags, same output, same operation call.

3. Do NOT add `--force` to `createTopicCompleteCommand`. Its existing doc comment explains why (no close-out gate to override); adding a flag only so it can be refused would ship a placeholder.

4. In `pkg/cli/export_test.go`, add one wrapper (no state, no setter):
   ```go
   // RequireTTYForForceForTest exposes requireTTYForForce with an explicit TTY
   // result, so both branches are tested without a pty.
   func RequireTTYForForceForTest(ctx context.Context, force bool, isTTY bool) error {
       return requireTTYForForce(ctx, force, isTTY)
   }
   ```

5. Add `pkg/cli/force_tty_test.go` (package `cli_test`, Ginkgo v2 / Gomega, same license header as the sibling test files). Register no second `RunSpecs` — `cli_suite_test.go` already runs the suite. Cover:
   - **the guard itself** — `cli.RequireTTYForForceForTest(context.Background(), true, false)` returns an error whose message contains `stdin is not a terminal`; `(…, true, true)` returns nil; `(…, false, false)` returns nil (a non-forced complete is never gated).
   - **`task complete --force` end to end through `cli.Run`** — `err := cli.Run(context.Background(), []string{"task", "complete", "Some Task", "--force"})`. `go test` connects the test binary's stdin to the null device, so the real `resolveSessionMode` probe returns false and the real wiring is exercised. Assert `err` is not nil and `err.Error()` contains `stdin is not a terminal`.
   - **`goal complete --force` end to end** — same, for `[]string{"goal", "complete", "Some Goal", "--force"}`.
   - **`task complete` without `--force`** — `cli.Run(context.Background(), []string{"task", "complete", "Some Task"})` must NOT return an error containing `stdin is not a terminal`, so the guard never fires on a non-forced complete.
   - **`topic complete` exposes no `--force` flag** — walk `cli.NewRootCommand(context.Background())` to `topic` → `complete` and assert `sub.Flags().Lookup("force")` is nil.
   - **`task complete` and `goal complete` do expose `--force`** — assert `Flags().Lookup("force")` is not nil on both, so the guard cannot pass by the flag having silently disappeared.

5b. Update the existing integration spec that this change breaks. `integration/cli_test.go` has `It("completes with --force despite incomplete subtasks and without close-out fields")` (~line 1990) which runs `task complete my-task --force` under `gexec`. `gexec` leaves the child's stdin at the null device, so the new guard makes it exit 1 and `make test` — and therefore `make precommit` — fails. Replace that `It` with a spec asserting the new contract:
   ```go
   It("refuses --force when stdin is not a terminal and leaves the task unchanged", func() {
       cmd := exec.Command(
           binPath,
           "--config", configPath,
           "--vault", "test",
           "task", "complete", "my-task", "--force",
       )
       session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
       Expect(err).NotTo(HaveOccurred())
       Eventually(session).Should(gexec.Exit(1))
       Expect(session.Err).To(gbytes.Say("stdin is not a terminal"))

       taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
       content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
       Expect(err).NotTo(HaveOccurred())
       Expect(string(content)).To(Equal("---\nstatus: in_progress\n---\n# My Task\n\n- [ ] not done\n"))
   })
   ```
   `gbytes` and `gexec` are already imported in that file. Do not leave any spec asserting exit 0 for a forced complete with a null-device stdin, and do not add a pty dependency.

6. Add a `## Unreleased` bullet to `CHANGELOG.md`. Resolve the newest versioned heading at run time and insert `## Unreleased` directly above it; if an `## Unreleased` already exists, append the bullet inside it (never create a second — `grep -c '^## Unreleased$' CHANGELOG.md` must print `1` afterwards). Bullet:
   ```
   - fix: refuse `--force` on `task complete` and `goal complete` when stdin is not a terminal, so an agent running without a terminal cannot bypass the closing check with it
   ```
   Do NOT bump any version string and do NOT create a tag.

7. Before finishing, re-run `<verification>` and confirm every line passes.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do not add `--force` to any command other than the two named, and do not remove or rename an existing flag.
- Do not introduce a package-level mutable variable or a test-only setter — the TTY result is a parameter.
- Do not write a second stdin-TTY probe; reuse `resolveSessionMode`.
- Do not change `pkg/ops` — the guard is a CLI-layer concern about the process's stdin, and `pkg/ops` must keep returning structured results without touching stdout/stderr.
- Do not change what a non-forced `complete` does, and do not change any refusal message it already prints.
- Error handling: `github.com/bborbe/errors` with `ctx`; never `fmt.Errorf`.
- Tests: Ginkgo v2 / Gomega only; never allocate a pty in a unit test (the pty path is verified on the installed binary, not here).
</constraints>

<verification>
- `make precommit` → exit 0.
- `grep -c 'stdin is not a terminal' pkg/cli/cli.go` → at least 1.
- `grep -c 'requireForceFromTerminal(ctx, force)' pkg/cli/cli.go` → exactly 2.
- `grep -c 'BoolVar(&force, "force"' pkg/cli/cli.go` → exactly 3: the two `complete` commands plus the pre-existing `task set --force` phase-regression flag, which is out of scope and must be left untouched. Any other count means a flag was added or removed.
- `grep -rn 'var stdinIsTTY' pkg/cli/ | wc -l` → 0.
- `go build ./...` → exit 0.
- `go test ./pkg/cli/... ./integration/...` → green, including the new specs and the rewritten integration spec.
</verification>
