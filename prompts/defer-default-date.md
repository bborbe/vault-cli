---
status: created
created: "2026-03-07T20:30:00Z"
---

<summary>
- Makes the date argument optional for `vault-cli task defer`
- When no date is provided, defaults to `+1d` (tomorrow)
- `vault-cli task defer "My Task"` now works (defers to tomorrow)
- `vault-cli task defer "My Task" +3d` still works as before
- Adds integration test covering both the 1-arg default and 2-arg explicit cases
</summary>

<objective>
Make the date argument optional in `vault-cli task defer`, defaulting to `+1d` when omitted. This removes friction for the most common defer use case — pushing a task to tomorrow.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/cli/cli.go` — the `createDeferCommand` function (line ~150) defines the cobra command with `cobra.ExactArgs(2)`.
Read `pkg/ops/defer.go` for the `Execute` method signature.
Read `integration/cli_test.go` — the `vault-cli defer` describe block (line ~386) for the existing test pattern.
Read `~/Documents/workspaces/coding-guidelines/go-testing-guide.md` for test patterns.
</context>

<requirements>
1. In `pkg/cli/cli.go`, function `createDeferCommand` (line ~150):
   - Change `Args: cobra.ExactArgs(2)` (line 165) to `Args: cobra.RangeArgs(1, 2)`
   - After extracting `taskName := args[0]` (line 167), set `dateStr` conditionally:
     ```go
     dateStr := "+1d"
     if len(args) > 1 {
         dateStr = args[1]
     }
     ```
   - Update `Use:` (line 157) from `"defer <task-name> <date>"` to `"defer <task-name> [date]"`
   - Update the `Long:` string (lines 159-164) to:
     ```go
     Long: `Defer a task to a specific date.

If no date is provided, defaults to +1d (tomorrow).

Date formats:
  +Nd         - Relative days (e.g., +7d for 7 days from now)
  monday      - Next occurrence of weekday
  2024-12-31  - ISO date format (YYYY-MM-DD)`,
     ```

2. In `integration/cli_test.go`, inside the `Describe("vault-cli defer", ...)` block (line ~386), add a new test case in the `Context("when task exists", ...)` block (after the existing `It("exits 0 and adds defer_date", ...)` at line ~407):
   ```go
   It("defaults to +1d when no date argument provided", func() {
       cmd := exec.Command(
           binPath,
           "--config",
           configPath,
           "--vault",
           "test",
           "task",
           "defer",
           "my-task",
       )
       session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
       Expect(err).NotTo(HaveOccurred())
       Eventually(session).Should(gexec.Exit(0))

       // Verify file was updated with defer_date
       taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
       content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
       Expect(err).NotTo(HaveOccurred())
       Expect(string(content)).To(ContainSubstring("defer_date:"))
   })
   ```
</requirements>

<constraints>
- Do NOT change `pkg/ops/defer.go` — the operation layer already handles `+1d` as a date string
- Do NOT change any interfaces or other commands
- Existing tests must still pass
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
