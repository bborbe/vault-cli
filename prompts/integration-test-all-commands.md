---
status: created
created: "2026-03-17T12:00:00Z"
---

<summary>
- Every CLI command and subcommand is verified to exist via an integration test
- A missing command registration immediately fails CI instead of silently passing
- Table-driven test covers all entity types (task, goal, theme, objective, vision, decision) and their subcommands
- Root-level commands (search, config) are also covered
- No vault setup required — tests only invoke `--help` which needs no config
</summary>

<objective>
Add a comprehensive integration test that verifies every expected CLI command and subcommand is actually registered by running `vault-cli <cmd> <subcmd> --help` and asserting exit code 0. This catches the class of bug where prompts claim to wire commands but the registration is missing.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `integration/cli_test.go` — existing integration tests using Ginkgo/gexec pattern; new test block goes here
- `integration/integration_suite_test.go` — test suite setup; `binPath` variable holds the compiled binary path
- `pkg/cli/cli.go` — all command registrations; scan for every `cmd.AddCommand(...)` and `rootCmd.AddCommand(...)` call to verify the expected command list is complete
</context>

<requirements>

## 1. Add a new `Describe("command registration", ...)` block in `integration/cli_test.go`

Add a new top-level `Describe` block inside the existing `var _ = Describe("vault-cli integration tests", func() { ... })` block. Place it after the existing `Describe("vault-cli --help", ...)` block and before `Describe("vault-cli list", ...)`.

## 2. Use `DescribeTable` with `Entry` for table-driven tests

Import `DescribeTable` and `Entry` from `github.com/onsi/ginkgo/v2` (they are already dot-imported via `. "github.com/onsi/ginkgo/v2"`).

Pattern to follow:

```go
Describe("command registration", func() {
    DescribeTable("exits 0 for --help",
        func(args ...string) {
            helpArgs := append(args, "--help")
            cmd := exec.Command(binPath, helpArgs...)
            session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
            Expect(err).NotTo(HaveOccurred())
            Eventually(session).Should(gexec.Exit(0))
        },
        // entries go here
    )
})
```

## 3. Add entries for ALL expected command+subcommand combinations

Add one `Entry` per command. Use the format `Entry("<cmd> <subcmd>", "cmd", "subcmd")` for subcommands and `Entry("<cmd>", "cmd")` for root-level commands.

Complete list of entries (verify each exists in `pkg/cli/cli.go` before adding):

**Task subcommands:**
- `Entry("task list", "task", "list")`
- `Entry("task show", "task", "show")`
- `Entry("task complete", "task", "complete")`
- `Entry("task defer", "task", "defer")`
- `Entry("task update", "task", "update")`
- `Entry("task work-on", "task", "work-on")`
- `Entry("task get", "task", "get")`
- `Entry("task set", "task", "set")`
- `Entry("task clear", "task", "clear")`
- `Entry("task lint", "task", "lint")`
- `Entry("task validate", "task", "validate")`
- `Entry("task search", "task", "search")`
- `Entry("task watch", "task", "watch")`
- `Entry("task add", "task", "add")`
- `Entry("task remove", "task", "remove")`

**Goal subcommands:**
- `Entry("goal list", "goal", "list")`
- `Entry("goal lint", "goal", "lint")`
- `Entry("goal search", "goal", "search")`
- `Entry("goal show", "goal", "show")`
- `Entry("goal get", "goal", "get")`
- `Entry("goal set", "goal", "set")`
- `Entry("goal clear", "goal", "clear")`
- `Entry("goal complete", "goal", "complete")`
- `Entry("goal add", "goal", "add")`
- `Entry("goal remove", "goal", "remove")`

**Theme subcommands:**
- `Entry("theme list", "theme", "list")`
- `Entry("theme lint", "theme", "lint")`
- `Entry("theme search", "theme", "search")`
- `Entry("theme show", "theme", "show")`
- `Entry("theme get", "theme", "get")`
- `Entry("theme set", "theme", "set")`
- `Entry("theme clear", "theme", "clear")`
- `Entry("theme add", "theme", "add")`
- `Entry("theme remove", "theme", "remove")`

**Objective subcommands:**
- `Entry("objective list", "objective", "list")`
- `Entry("objective lint", "objective", "lint")`
- `Entry("objective search", "objective", "search")`
- `Entry("objective show", "objective", "show")`
- `Entry("objective get", "objective", "get")`
- `Entry("objective set", "objective", "set")`
- `Entry("objective clear", "objective", "clear")`
- `Entry("objective complete", "objective", "complete")`
- `Entry("objective add", "objective", "add")`
- `Entry("objective remove", "objective", "remove")`

**Vision subcommands:**
- `Entry("vision list", "vision", "list")`
- `Entry("vision lint", "vision", "lint")`
- `Entry("vision search", "vision", "search")`
- `Entry("vision show", "vision", "show")`
- `Entry("vision get", "vision", "get")`
- `Entry("vision set", "vision", "set")`
- `Entry("vision clear", "vision", "clear")`
- `Entry("vision add", "vision", "add")`
- `Entry("vision remove", "vision", "remove")`

**Decision subcommands:**
- `Entry("decision list", "decision", "list")`
- `Entry("decision ack", "decision", "ack")`

**Root-level commands:**
- `Entry("search", "search")`

**Config subcommands:**
- `Entry("config list", "config", "list")`
- `Entry("config current-user", "config", "current-user")`

## 4. Cross-check against `pkg/cli/cli.go`

Before writing the test, read the ENTIRE `pkg/cli/cli.go` file and verify that every command listed above is actually registered via `AddCommand`. If any command from the list above is NOT registered in `cli.go`, do NOT add an Entry for it — instead leave a `// TODO: missing registration` comment next to the skipped entry so the gap is visible.

Conversely, if `cli.go` registers commands NOT listed above, add Entry lines for those too.

## 5. Keep the `DescribeTable` function signature compatible with variadic args

The `DescribeTable` body function must accept `args ...string` (variadic) so that both single-arg entries like `Entry("search", "search")` and multi-arg entries like `Entry("task list", "task", "list")` work.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass unchanged — do not modify any existing `Describe`/`It` blocks
- Follow the existing Ginkgo/gexec pattern in `integration/cli_test.go`
- Use dot-imported `DescribeTable` and `Entry` from `github.com/onsi/ginkgo/v2` (already dot-imported)
- Use `github.com/bborbe/errors` for error wrapping if needed
- No vault/config setup needed — `--help` works without a config file
- Do NOT add any new imports beyond what is already in the file (exec, gexec, ginkgo/v2, gomega are all already imported)
- `make precommit` must pass
</constraints>

<verification>
Run `make precommit` — must pass.

Additionally verify that the new tests actually run:
```bash
go test ./integration/ -v -run "command registration" -count=1
```

Expected: one PASS line per Entry, all green. If any entry fails with a non-zero exit code, that command is not registered and the test is correctly catching the bug.
</verification>
