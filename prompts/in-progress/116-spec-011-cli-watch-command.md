---
status: approved
spec: [011-promote-task-watch-to-vault-watch]
created: "2026-05-10T00:00:00Z"
queued: "2026-05-10T22:07:30Z"
branch: dark-factory/promote-task-watch-to-vault-watch
---

<summary>
- A new top-level `vault-cli watch` command streams newline-delimited JSON events for all four entity kinds (task, goal, theme, objective) across all configured vaults (or a single vault when `--vault` is set)
- A `--types` flag on `vault-cli watch` accepts a comma-separated list; when set, only events matching the listed kinds are emitted; when omitted, all four kinds are emitted
- Unknown values in `--types` are rejected at startup (before the watcher starts) with a clear error listing the four valid values; an explicitly-empty `--types` value is also rejected
- `vault-cli watch --help` documents the command, the four entity kinds, and the `--types` filter with valid values
- `vault-cli task watch` gains a one-time stderr deprecation warning naming `vault-cli watch` as the replacement; stdout JSON output is unaffected by the warning
- Both commands share the same `WatchOperation.Execute` call — no duplicated watch loops
- Tests cover `--types` validation rejection paths and the deprecation warning emission
</summary>

<objective>
Add the canonical `vault-cli watch` top-level command with `--types` filtering and emit a deprecation warning from `vault-cli task watch`. This is Prompt 2 of 2 for spec 011. Prompt 1 must be completed first: it adds `WatchDir{Dir, Kind}` to `WatchTarget.WatchDirs` and `Type` to `WatchEvent`.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read `go-patterns.md` in `~/.claude/plugins/marketplaces/coding/docs/` for interface/struct patterns.
Read `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for Ginkgo/Gomega test patterns.
Read `go-enum-type-pattern.md` in `~/.claude/plugins/marketplaces/coding/docs/` for enum validation patterns.
Read `go-logging-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for slog usage.
Read `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` for which test types to write.

Key files to read before making changes:
- `pkg/ops/watch.go` — `WatchTarget`, `WatchDir`, `WatchEvent`, `WatchOperation` interface (after Prompt 1 changes)
- `pkg/cli/cli.go` lines 1–102 — `Run` function and how root-level commands are registered; lines 2015–2055 — `createTaskWatchCommand` (the model for the new command)
- `pkg/cli/cli_suite_test.go` — Ginkgo suite bootstrap for `cli_test` package
- `pkg/ops/watch_test.go` — example of how `captureWatchEvents` helper works (for reference, not to copy)
</context>

<requirements>
### 1. Add `createWatchCommand` to `pkg/cli/cli.go`

Add a new function `createWatchCommand` after `createTaskWatchCommand` (around line 2052). The function signature:

```go
func createWatchCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) *cobra.Command {
```

The command body:

```go
var typesStr string
cmd := &cobra.Command{
	Use:   "watch",
	Short: "Watch vault directories for changes (streaming JSON output)",
	Long: `Watch tasks, goals, themes, and objectives directories for file changes.
Emits one newline-delimited JSON event per debounced change.

Each event includes:
  event  - change type: created, modified, deleted, renamed
  name   - filename without .md extension
  vault  - vault name
  path   - vault-relative file path
  type   - entity kind: task, goal, theme, objective

Use --types to filter to a subset of entity kinds.
Valid type values: task, goal, theme, objective`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate --types before any I/O.
		var typeFilter []string
		if cmd.Flags().Lookup("types").Changed {
			if typesStr == "" {
				return errors.Errorf(ctx, "--types requires at least one value; valid values: task, goal, theme, objective")
			}
			parts := strings.Split(typesStr, ",")
			validKinds := []string{"task", "goal", "theme", "objective"}
			for _, part := range parts {
				valid := false
				for _, v := range validKinds {
					if part == v {
						valid = true
						break
					}
				}
				if !valid {
					return errors.Errorf(ctx, "unknown type %q in --types; valid values: task, goal, theme, objective", part)
				}
			}
			typeFilter = parts
		}

		vaults, err := getVaults(ctx, configLoader, vaultName)
		if err != nil {
			return errors.Wrap(ctx, err, "get vaults")
		}

		targets := make([]ops.WatchTarget, 0, len(vaults))
		for _, vault := range vaults {
			targets = append(targets, ops.WatchTarget{
				VaultPath: vault.Path,
				VaultName: vault.Name,
				WatchDirs: []ops.WatchDir{
					{Dir: vault.GetTasksDir(), Kind: "task"},
					{Dir: vault.GetGoalsDir(), Kind: "goal"},
					{Dir: vault.GetThemesDir(), Kind: "theme"},
					{Dir: vault.GetObjectivesDir(), Kind: "objective"},
				},
			})
		}

		watchOp := ops.NewWatchOperation()
		return watchOp.Execute(ctx, targets, func(event ops.WatchEvent) error {
			if len(typeFilter) > 0 {
				matched := false
				for _, t := range typeFilter {
					if event.Type == t {
						matched = true
						break
					}
				}
				if !matched {
					return nil
				}
			}
			enc := json.NewEncoder(os.Stdout)
			return enc.Encode(event)
		})
	},
}
cmd.Flags().StringVar(&typesStr, "types", "", "Comma-separated entity types to emit (task,goal,theme,objective). Omit for all types.")
return cmd
```

Note: `strings.Split`, `json`, `os` are already imported in `cli.go`. Confirm before adding imports.

### 2. Register `createWatchCommand` at root level in `Run`

In the `Run` function (around line 80–100), add the new command after `createDecisionCommands`:

```go
rootCmd.AddCommand(createWatchCommand(ctx, &configLoader, &vaultName))
```

Place it alongside the other root-level `AddCommand` calls (search, task, goal, theme, objective, vision, decision). Do NOT nest it under any subcommand group.

### 3. Add deprecation warning to `createTaskWatchCommand`

In `createTaskWatchCommand` (around line 2015), inside the `RunE` function body, add a stderr deprecation warning as the very first statement, before `getVaults` is called. **Use cobra's `cmd.ErrOrStderr()` writer**, NOT `os.Stderr` directly — this lets tests capture stderr cleanly via `cmd.SetErr(w)` without mutating the global `os.Stderr` (which is racy + flaky):

```go
RunE: func(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(cmd.ErrOrStderr(), "DEPRECATED: 'vault-cli task watch' is deprecated; use 'vault-cli watch' instead. See spec 011.")

	vaults, err := getVaults(ctx, configLoader, vaultName)
	// ... rest unchanged
```

The warning is emitted on stderr, before any events are emitted. The stdout JSON stream is unaffected. (Per-process-uniqueness is implicitly satisfied by `cli.Run` being one invocation; explicit `sync.Once` is unnecessary.)

### 4. Write tests in `pkg/cli/watch_test.go`

Create `pkg/cli/watch_test.go` in the `cli_test` package (same package as `cli_suite_test.go`).

The Ginkgo test suite bootstrap is at `pkg/cli/cli_suite_test.go` in `package cli_test`. Do NOT create or modify it.

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test
```

Write the following test cases using Ginkgo/Gomega:

#### Test 4a: `--types unknown` returns an error

Call `cli.Run` with `watch --types unknown`. Since `--types` validation happens before `getVaults`, no config file is needed. Assert the returned error is non-nil and the error message contains "unknown" (case-insensitive) or "valid values":

```go
var _ = Describe("vault-cli watch --types", func() {
	It("returns an error for an unknown type value", func() {
		ctx := context.Background()
		err := cli.Run(ctx, []string{"watch", "--types", "unknown"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown"))
	})

	It("returns an error for multiple values where one is unknown", func() {
		ctx := context.Background()
		err := cli.Run(ctx, []string{"watch", "--types", "task,foo"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("foo"))
	})

	It("returns an error when --types flag is explicitly set to empty string", func() {
		ctx := context.Background()
		err := cli.Run(ctx, []string{"watch", "--types", ""})
		Expect(err).To(HaveOccurred())
	})
})
```

Note: `cli.Run` is exported from `pkg/cli/cli.go` as `func Run(ctx context.Context, args []string) error`. Import it as `"github.com/bborbe/vault-cli/pkg/cli"`.

#### Test 4b: deprecation warning on `vault-cli task watch`

Use cobra's native `cmd.SetErr(buf)` / `cmd.SetOut(buf)` to capture stderr+stdout — **do NOT mutate `os.Stderr`** (racy under `-race`, flaky timing). Build the cobra command directly via `cli.NewRootCommand(ctx)` (or whichever constructor `cli.Run` uses internally — discover by reading `pkg/cli/cli.go` lines 47–100; if no exported constructor exists, add one as part of this prompt: a single-line refactor extracting the cobra-tree construction so it can be invoked from tests with explicit Stderr/Stdout).

```go
var _ = Describe("vault-cli task watch deprecation", func() {
	It("writes a deprecation warning to stderr before streaming and stdout stays JSON-clean", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create a minimal vault dir and config.
		vaultDir, err := os.MkdirTemp("", "vault-deprecation-test-*")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.RemoveAll(vaultDir) }()

		tasksDir := filepath.Join(vaultDir, "Tasks")
		Expect(os.MkdirAll(tasksDir, 0750)).To(Succeed())

		configContent := fmt.Sprintf(`vaults:
  - name: test
    path: %s
`, vaultDir)
		configFile, err := os.CreateTemp("", "vault-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = os.Remove(configFile.Name()) }()
		_, err = configFile.WriteString(configContent)
		Expect(err).NotTo(HaveOccurred())
		Expect(configFile.Close()).To(Succeed())

		// Build root cobra command and wire test buffers.
		rootCmd := cli.NewRootCommand(ctx)
		var stderrBuf, stdoutBuf bytes.Buffer
		rootCmd.SetErr(&stderrBuf)
		rootCmd.SetOut(&stdoutBuf)
		rootCmd.SetArgs([]string{"--config", configFile.Name(), "task", "watch"})

		runDone := make(chan struct{})
		go func() {
			defer close(runDone)
			_ = rootCmd.ExecuteContext(ctx)
		}()

		// Poll for the deprecation line with a deadline (no fixed Sleep — flakier).
		Eventually(func() string { return stderrBuf.String() }, 2*time.Second, 20*time.Millisecond).
			Should(ContainSubstring("deprecated"))

		cancel()
		<-runDone

		// AC line: stdout JSON stream is unaffected by the deprecation warning.
		Expect(stdoutBuf.String()).NotTo(ContainSubstring("deprecated"), "deprecation warning leaked to stdout")
	})
})
```

If `pkg/cli/cli.go` does not yet expose a `NewRootCommand(ctx) *cobra.Command` constructor (because `cli.Run` builds the tree inline), refactor `Run` to call into a new exported `NewRootCommand` function and have `Run` simply call `NewRootCommand(ctx).ExecuteContext(ctx)`. This is a one-place mechanical extraction with zero behavior change for production callers. The test then uses the constructor.

Required imports for the test file: `"bytes"`, `"context"`, `"fmt"`, `"os"`, `"path/filepath"`, `"time"`, `. "github.com/onsi/ginkgo/v2"`, `. "github.com/onsi/gomega"`, `"github.com/bborbe/vault-cli/pkg/cli"`.
</requirements>

<constraints>
- `vault-cli watch` and `vault-cli task watch` MUST share the same `WatchOperation.Execute` implementation — no duplicated watch loops
- `--types` validation MUST happen before any `getVaults` call, so that the error test (Test 4a) does not require a config file
- The deprecation warning goes to `os.Stderr` only — stdout JSON events are unaffected
- The deprecation warning is emitted exactly once per process invocation, synchronously, before `watchOp.Execute` is called
- Valid `--types` values are the closed set `{"task", "goal", "theme", "objective"}` — no other values are accepted
- `createWatchCommand` and `createTaskWatchCommand` both build `[]ops.WatchTarget` with `[]ops.WatchDir` using the same four entity types and their kinds (per Prompt 1's format)
- Per project rule (`CLAUDE.md` "Scenario-skip rule"), no new scenario file is added
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Prompt 1 (ops type field) must be completed before this prompt
</constraints>

<verification>
```
make precommit
```

```
# Confirm vault-cli watch is registered at root level
grep -n 'createWatchCommand' pkg/cli/cli.go
# expected: definition + registration call (two lines minimum)

# Confirm deprecation warning is in task watch
grep -nF "vault-cli watch" pkg/cli/cli.go
# expected: one or two lines in createTaskWatchCommand

# Confirm --types flag exists on watch command
grep -n '"types"' pkg/cli/cli.go
# expected: one line (flag registration in createWatchCommand)

# Confirm new test file exists
ls pkg/cli/watch_test.go

# Run just the cli package tests
go test -v ./pkg/cli/...
```

```
# Manual smoke test (against a real vault):
# vault-cli watch --types unknown
# Expected: exit non-zero with error message naming valid values

# vault-cli watch --types task,goal &
# touch a task file → expect "type":"task" event
# touch a theme file → expect no event emitted (filtered out)
# kill %1

# vault-cli task watch
# Expected: first line on stderr contains "deprecated", then events stream normally
```
</verification>
