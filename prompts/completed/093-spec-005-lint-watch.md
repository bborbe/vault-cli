---
status: completed
spec: [005-ops-no-stdout]
summary: Refactored LintOperation and WatchOperation to return structured results instead of writing to stdout; CLI layer now owns all output formatting and exit behavior; subprocess-based exit tests replaced with direct result assertions; mocks regenerated.
container: vault-cli-093-spec-005-lint-watch
dark-factory-version: v0.59.5-dirty
created: "2026-03-20T00:00:00Z"
queued: "2026-03-20T19:42:30Z"
started: "2026-03-20T20:13:49Z"
completed: "2026-03-20T20:31:40Z"
---

<summary>
- LintOperation stops writing to stdout and returns []LintIssue instead
- outputFormat parameter removed from LintOperation.Execute and LintOperation.ExecuteFile
- os.Exit(1) calls removed from lint internals — CLI returns an error to produce non-zero exit
- WatchOperation accepts a handler callback instead of writing events to os.Stdout directly
- A structured result type supports CLI formatting for single-file validation
- CLI formats lint output (plain and JSON) and handles exit behavior
- CLI passes a handler callback to the watch operation for event streaming
- Subprocess-based exit tests replaced with direct result assertion tests
- All mocks regenerated and all existing tests pass with updated assertions
</summary>

<objective>
Refactor `LintOperation` (complex: two methods, fix mode, exit code behavior, JSON/plain) and `WatchOperation` (streaming: callback pattern) so neither writes to stdout. The CLI layer receives lint issues and formats them; the watch CLI passes a handler callback. This is the third and final prompt for spec 005. Prompts 1 and 2 must be completed first.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Key files to read before making changes:
- `pkg/ops/lint.go` — LintOperation interface + all output helpers (full file, ~925 lines)
- `pkg/ops/lint_validate_exit_test.go` — subprocess-based exit tests to replace
- `pkg/ops/watch.go` — WatchOperation interface + handleEvent helper
- `pkg/cli/cli.go` — call sites:
  - `createGenericLintCommand` (~line 431): iterates vaults, calls `lintOp.Execute`
  - `createValidateCommand` (~line 378): calls `lintOp.ExecuteFile`
  - `createTaskWatchCommand` (~line 1625): calls `watchOp.Execute(ctx, targets)`
- `pkg/cli/output.go` — PrintJSON helper
- `mocks/lint-operation.go` — to regenerate
- `mocks/watch-operation.go` — to regenerate
</context>

<requirements>
### 1. `pkg/ops/lint.go` — LintOperation

#### New interface

```go
type LintOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        tasksDir string,
        fix bool,
    ) ([]LintIssue, error)
    ExecuteFile(
        ctx context.Context,
        filePath string,
        taskName string,
        vaultName string,
    ) ([]LintIssue, error)
}
```

Both methods return `([]LintIssue, error)`:
- The error is non-nil only for infrastructure errors (file I/O, walk errors) — NOT for lint issues found
- Lint issues found are conveyed through the `[]LintIssue` return value
- The CLI is responsible for returning an error (causing non-zero exit) when unfixed issues exist

#### `lintOperation.Execute` changes

Old body ends with `return l.outputIssues(vaultPath, issues, fix, outputFormat)`.

New body: after the walk loop, return the issues slice directly:
```go
return issues, nil
```
(Return any walk error wrapped as before.)

When `fix` is true, the `lintFile` helper already mutates issues in place (setting `issue.Fixed = true`). This remains unchanged.

#### `lintOperation.ExecuteFile` changes

Old body: calls `outputValidateJSON` or `outputValidatePlain` which call `os.Exit(1)`.

New body: return the issues slice directly:
```go
issues, err := l.lintFile("", filePath, false)
if err != nil {
    return nil, errors.Wrap(ctx, err, fmt.Sprintf("lint file %s", filePath))
}
return issues, nil
```

#### Remove internal output helpers

Remove these methods entirely from `lintOperation`:
- `outputIssues`
- `outputIssuesJSON`
- `outputIssuesPlain`
- `outputValidateJSON`
- `outputValidatePlain`
- `countUnfixedError` (if it's only used by the removed methods; check first)
- `issueTypeName` (keep if used elsewhere; if only in output helpers, remove it too)

Remove unused imports: `encoding/json`, `fmt`, `os` (if no longer used in non-test code).

#### Define ValidateIssueResult for CLI use

Add a new exported type in `pkg/ops/lint.go` to help the CLI format `ExecuteFile` results:

```go
// ValidateIssueJSON is the per-issue structure used in validate JSON output.
type ValidateIssueJSON struct {
    Type        string `json:"type"`
    IssueType   string `json:"issue_type"`
    Description string `json:"description"`
}

// ValidateResult is the structured result from ExecuteFile used in CLI JSON output.
type ValidateResult struct {
    Name   string              `json:"name"`
    Vault  string              `json:"vault"`
    Issues []ValidateIssueJSON `json:"issues"`
}
```

### 2. `pkg/ops/watch.go` — WatchOperation

#### New interface

```go
type WatchOperation interface {
    Execute(ctx context.Context, vaults []WatchTarget, handler func(WatchEvent) error) error
}
```

The `handler` callback is invoked for each debounced event. The caller (CLI) provides the handler that writes to stdout.

#### `watchOperation.Execute` changes

Remove the `enc := json.NewEncoder(os.Stdout)` line.

Pass the `handler` through to `handleEvent`:
```go
handleEvent(e, dirToVault, handler, debouncer)
```

#### `handleEvent` changes

Change signature from:
```go
func handleEvent(e fsnotify.Event, dirToVault map[string]vaultInfo, enc *json.Encoder, debouncer *debouncer)
```
to:
```go
func handleEvent(e fsnotify.Event, dirToVault map[string]vaultInfo, handler func(WatchEvent) error, debouncer *debouncer)
```

In the debounced callback, replace `_ = enc.Encode(ev)` with `_ = handler(ev)`.

Remove `encoding/json` and `os` imports from `watch.go` (only if no longer needed; keep `fmt` if used elsewhere).

### 3. Regenerate mocks

Run:
```
go generate ./pkg/ops/...
```
This regenerates:
- `mocks/lint-operation.go`
- `mocks/watch-operation.go`

### 4. Update `pkg/cli/cli.go` — lint call sites

#### `createGenericLintCommand` (~line 431)

```go
issues, err := lintOp.Execute(ctx, vault.Path, getDirFunc(storageConfig), fix)
if err != nil {
    return err
}

if *outputFormat == cli.OutputFormatJSON {
    jsonIssues := make([]ops.LintIssueJSON, len(issues))
    for i, issue := range issues {
        relPath, _ := filepath.Rel(vault.Path, issue.FilePath)
        typeName := lintIssueTypeName(issue, fix)
        jsonIssues[i] = ops.LintIssueJSON{
            File:        relPath,
            Type:        typeName,
            Description: issue.Description,
            Fixed:       issue.Fixed,
        }
    }
    if encErr := cli.PrintJSON(jsonIssues); encErr != nil {
        return encErr
    }
} else {
    for _, issue := range issues {
        relPath, _ := filepath.Rel(vault.Path, issue.FilePath)
        typeName := lintIssueTypeName(issue, fix)
        fmt.Printf("%-5s %s: %s %s\n", typeName, relPath, issue.IssueType, issue.Description)
    }
    if len(issues) == 0 {
        fmt.Println("No lint issues found")
    }
}

// Return error if any unfixed issues remain (preserves non-zero exit code)
for _, issue := range issues {
    if !issue.Fixed {
        return errors.Errorf(ctx, "lint issues found")
    }
}
```

Add a package-level helper in `cli.go` (or inline):
```go
func lintIssueTypeName(issue ops.LintIssue, fix bool) string {
    if issue.Fixed {
        return "FIXED"
    }
    if issue.Fixable && !fix {
        return "WARN"
    }
    return "ERROR"
}
```

#### `createValidateCommand` (~line 378)

```go
issues, err := lintOp.ExecuteFile(ctx, taskFilePath, taskName, foundInVault.Name)
if err != nil {
    return err
}

if *outputFormat == cli.OutputFormatJSON {
    result := ops.ValidateResult{
        Name:  taskName,
        Vault: foundInVault.Name,
    }
    for _, issue := range issues {
        issueTypeStr := "WARN"
        if !issue.Fixable {
            issueTypeStr = "ERROR"
        }
        result.Issues = append(result.Issues, ops.ValidateIssueJSON{
            Type:        issueTypeStr,
            IssueType:   string(issue.IssueType),
            Description: issue.Description,
        })
    }
    if encErr := cli.PrintJSON(result); encErr != nil {
        return encErr
    }
} else {
    if len(issues) == 0 {
        fmt.Printf("✅ %s: no lint issues found\n", taskName)
    } else {
        for _, issue := range issues {
            issueTypeStr := "WARN"
            if !issue.Fixable {
                issueTypeStr = "ERROR"
            }
            fmt.Printf("%-5s %s: %s %s\n", issueTypeStr, taskName+".md", string(issue.IssueType), issue.Description)
        }
    }
}

// Return error if any issues found (preserves non-zero exit code behavior)
if len(issues) > 0 {
    return errors.Errorf(ctx, "lint issues found in %s", taskName)
}
return nil
```

### 5. Update `pkg/cli/cli.go` — watch call site

#### `createTaskWatchCommand` (~line 1625)

```go
watchOp := ops.NewWatchOperation()
return watchOp.Execute(ctx, targets, func(event ops.WatchEvent) error {
    enc := json.NewEncoder(os.Stdout)
    return enc.Encode(event)
})
```

Add `"encoding/json"` and `"os"` to imports in `cli.go` if not already present (check; they may already be imported for other uses).

Alternatively, use `cli.PrintJSON` but note that watch events are newline-delimited (not pretty-printed). Keep the raw `json.NewEncoder` for watch to preserve streaming behavior.

### 6. Update `pkg/ops/lint_validate_exit_test.go`

Replace all subprocess-based exit tests with direct result assertion tests. The new interface returns `([]LintIssue, error)` — no `os.Exit` needed.

**Replace the file with tests like:**

```go
package ops_test

import (
    "context"
    "os"
    "testing"

    . "github.com/onsi/gomega"
    "github.com/bborbe/vault-cli/pkg/ops"
)

func TestValidateExecuteFileWithInvalidStatus(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()
    lintOp := ops.NewLintOperation()

    f, err := os.CreateTemp("", "task-*.md")
    g.Expect(err).NotTo(HaveOccurred())
    defer os.Remove(f.Name())

    content := "---\nstatus: invalid_status\npriority: 1\n---\n# Task\n"
    _, err = f.WriteString(content)
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(f.Close()).To(Succeed())

    issues, err := lintOp.ExecuteFile(ctx, f.Name(), "Test Task", "test")
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(issues).NotTo(BeEmpty())
    g.Expect(issues[0].IssueType).To(Equal(ops.IssueTypeInvalidStatus))
}

func TestValidateExecuteFileWithNoIssues(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()
    lintOp := ops.NewLintOperation()

    f, err := os.CreateTemp("", "task-*.md")
    g.Expect(err).NotTo(HaveOccurred())
    defer os.Remove(f.Name())

    content := "---\nstatus: todo\npriority: 1\n---\n# Task\n"
    _, err = f.WriteString(content)
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(f.Close()).To(Succeed())

    issues, err := lintOp.ExecuteFile(ctx, f.Name(), "Test Task", "test")
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(issues).To(BeEmpty())
}

func TestValidateExecuteFileWithMissingFrontmatter(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()
    lintOp := ops.NewLintOperation()

    f, err := os.CreateTemp("", "task-*.md")
    g.Expect(err).NotTo(HaveOccurred())
    defer os.Remove(f.Name())

    content := "# Task Without Frontmatter\n\nThis task is missing frontmatter.\n"
    _, err = f.WriteString(content)
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(f.Close()).To(Succeed())

    issues, err := lintOp.ExecuteFile(ctx, f.Name(), "Missing Frontmatter", "test")
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(issues).NotTo(BeEmpty())
    g.Expect(issues[0].IssueType).To(Equal(ops.IssueTypeMissingFrontmatter))
}

func TestValidateExecuteFileWithFixableIssues(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()
    lintOp := ops.NewLintOperation()

    f, err := os.CreateTemp("", "task-*.md")
    g.Expect(err).NotTo(HaveOccurred())
    defer os.Remove(f.Name())

    content := "---\nstatus: next\npriority: high\nassignee: alice\nassignee: bob\n---\n# Task\n"
    _, err = f.WriteString(content)
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(f.Close()).To(Succeed())

    issues, err := lintOp.ExecuteFile(ctx, f.Name(), "Test Task", "test")
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(issues).NotTo(BeEmpty())
}
```

### 7. Update `pkg/ops/lint_test.go` (if it captures stdout)

If any existing lint tests use stdout capture, replace with direct `(issues, err)` assertions. Read the file first and only update tests that use stdout capture patterns.
</requirements>

<constraints>
- CLI output format must not change — same text, same JSON structure, same field names
- Operation naming convention is preserved (no renames)
- Mock generation comments (`//counterfeiter:generate`) are preserved; mocks are regenerated after interface changes
- Factory function pattern (pure composition, no I/O) is preserved
- Do NOT commit — dark-factory handles git
- Existing tests must still pass after assertion updates
- No operation in pkg/ops/ may write to os.Stdout after this prompt
- The watch CLI handler uses raw `json.NewEncoder(os.Stdout)` (not `cli.PrintJSON`) to preserve streaming newline-delimited JSON behavior
- Exit code behavior is preserved: lint issues → non-zero exit (via returning error from CLI, not os.Exit)
</constraints>

<verification>
```
make precommit
```

```
grep -r 'os\.Stdout' pkg/ops/lint.go pkg/ops/watch.go
# expected: no output
```

```
grep -r 'fmt\.Print' pkg/ops/lint.go
# expected: no output
```

```
grep -r 'os\.Stdout' pkg/ops/ | grep -v _test.go
# expected: no output (confirms full spec compliance)
```

```
grep -r 'fmt\.Print' pkg/ops/ | grep -v _test.go
# expected: no output (confirms full spec compliance)
```
</verification>
