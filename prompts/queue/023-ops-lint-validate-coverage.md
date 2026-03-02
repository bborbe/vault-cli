---
status: queued
---

<objective>
Increase test coverage for `pkg/ops/lint.go` validate functions from 0% to ≥80%.
Targets: `ExecuteFile`, `outputValidateJSON`, `outputValidatePlain`, `outputIssuesJSON` — all currently 0%.
</objective>

<context>
Go CLI project at ~/Documents/workspaces/vault-cli.
Read CLAUDE.md for project conventions.
Read ~/.claude/docs/go-testing.md for testing patterns.

`pkg/ops/lint.go` has existing tests in `lint_test.go` using real temp files.
Follow that exact pattern. The `ExecuteFile` method was added for `task validate` (prompt 022).

Current coverage: `pkg/ops` = 71%. Target: ≥80%.
</context>

<requirements>
Add tests in `./pkg/ops/lint_test.go` for:

1. `ExecuteFile`:
   - Valid file, no issues → plain output "no lint issues found", no error
   - Valid file, no issues → json output `{"issues": []}`, no error
   - File with invalid status → plain output shows ERROR, returns error
   - File with invalid status → json output shows issue in array, returns error
   - Non-existent file path → error propagated

2. `outputValidateJSON`:
   - Empty issues slice → `{"name": "...", "vault": "...", "issues": []}`
   - Issues present (fixable + non-fixable) → correct type field ("WARN"/"ERROR"), no error
   - Verify JSON can be decoded correctly

3. `outputValidatePlain`:
   - No issues → prints "✅ My Task: no lint issues found"
   - Issues present → prints "WARN ..." or "ERROR ..." prefix per issue
   - Verify non-zero exit implied (method returns nil but caller checks issues)

4. `outputIssuesJSON`:
   - Empty issues → returns nil, outputs valid JSON `[]`
   - Issues present → each issue has correct fields
</requirements>

<implementation>
Follow existing lint_test.go pattern — use real temp files:

```go
Context("ExecuteFile", func() {
    var (
        tmpFile   string
        taskName  string
        vaultName string
        outputFmt string
    )

    BeforeEach(func() {
        taskName = "My Task"
        vaultName = "personal"
        outputFmt = "plain"

        f, err := os.CreateTemp("", "task-*.md")
        Expect(err).To(BeNil())
        tmpFile = f.Name()
        _, _ = f.WriteString("---\nstatus: in_progress\npriority: 1\n---\n\nContent\n")
        _ = f.Close()
    })

    AfterEach(func() {
        _ = os.Remove(tmpFile)
    })

    JustBeforeEach(func() {
        err = lintOp.ExecuteFile(ctx, tmpFile, taskName, vaultName, outputFmt)
    })

    It("returns no error for valid file", func() {
        Expect(err).To(BeNil())
    })

    Context("invalid status", func() {
        BeforeEach(func() {
            _ = os.WriteFile(tmpFile, []byte("---\nstatus: wip\n---\n"), 0600)
        })
        It("returns error", func() {
            Expect(err).NotTo(BeNil())
        })
    })

    Context("json output", func() {
        BeforeEach(func() { outputFmt = "json" })
        It("returns no error for valid file", func() {
            Expect(err).To(BeNil())
        })
    })
})
```

For `outputValidateJSON` and `outputValidatePlain`, call them directly:
```go
lintOp := ops.NewLintOperation(...)
// call via ExecuteFile with controlled temp files
```
Or if not accessible directly, test via `ExecuteFile` with different file states.
</implementation>

<constraints>
- Do NOT modify lint.go — tests only
- Check if suite file exists before creating: `pkg/ops/ops_suite_test.go`
- Use real temp files for file-based tests, clean up with AfterEach
- Do NOT run make precommit iteratively — use make test; run make precommit once at the end
</constraints>

<verification>
Run: `make test`
Run: `go test -mod=mod -cover ./pkg/ops/...`

Target: `pkg/ops` coverage ≥80%.
</verification>

<success_criteria>
- make test passes
- pkg/ops coverage ≥80%
- ExecuteFile tested for plain + json output, valid + invalid file
- outputValidateJSON tested for empty and non-empty issues
- outputValidatePlain tested for no-issues and issues-present cases
</success_criteria>
