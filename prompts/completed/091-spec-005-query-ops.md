---
status: completed
spec: [005-ops-no-stdout]
summary: Refactored five query operations (list, show, search, decision-list, entity-show) to return structured results instead of writing to stdout; CLI layer now owns all output formatting
container: vault-cli-091-spec-005-query-ops
dark-factory-version: v0.59.5-dirty
created: "2026-03-20T00:00:00Z"
queued: "2026-03-20T19:42:24Z"
started: "2026-03-20T19:42:26Z"
completed: "2026-03-20T19:52:49Z"
---

<summary>
- Five read-only operations stop writing to stdout and return structured results instead
- Each operation returns its result data rather than printing it directly
- The output format parameter is removed from all five operation interfaces
- The CLI layer receives results and formats them for plain or JSON output
- All mocks are regenerated to match the new interfaces
- All existing tests pass with assertions updated from stdout capture to direct result checks
</summary>

<objective>
Refactor the five read-only/query operations in `pkg/ops/` so they return structured results and never write to stdout. The CLI layer calls each operation, receives the result, and formats output. This is the first of three prompts for spec 005.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Key files to read before making changes:
- `pkg/ops/list.go` — ListOperation interface + TaskListItem type + formatDateOrDateTime helper
- `pkg/ops/show.go` — ShowOperation interface + TaskDetail type
- `pkg/ops/search.go` — SearchOperation interface
- `pkg/ops/decision_list.go` — DecisionListOperation interface + DecisionListItem type
- `pkg/ops/frontmatter_entity.go` — EntityShowOperation interface (line 546–610)
- `pkg/cli/cli.go` — call sites: createTaskListCommand (line 307), createGenericListCommand (line 474), createEntityShowCommand (line 698), createDecisionListCommand (line ~1280), createSearchCommand (line ~1348), createGenericSearchCommand (line ~1400), createTaskShowCommand (line ~1605)
- `pkg/cli/output.go` — PrintJSON helper
- `mocks/` — counterfeiter-generated mocks to regenerate

Current state: all five operations accept `outputFormat string` as a parameter and call `fmt.Printf`/`json.NewEncoder(os.Stdout)` directly.
</context>

<requirements>
### 1. `pkg/ops/list.go` — ListOperation

Change the interface:
```go
type ListOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        vaultName string,
        pagesDir string,
        statusFilters []string,
        showAll bool,
        assigneeFilter string,
        goalFilter string,
    ) ([]TaskListItem, error)
}
```

In `listOperation.Execute`:
- Remove `outputFormat string` parameter
- Replace the output block (lines 102–138) with `return items, nil` for JSON-formatted items (always build the `[]TaskListItem` slice) and return it
- Remove all `fmt.Printf`, `json.NewEncoder(os.Stdout)` calls
- Remove unused imports (`encoding/json`, `fmt`, `os`) if no longer needed

### 2. `pkg/ops/show.go` — ShowOperation

Change the interface:
```go
type ShowOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        vaultName string,
        taskName string,
    ) (TaskDetail, error)
}
```

In `showOperation.Execute`:
- Remove `outputFormat string` parameter
- Return `(TaskDetail, error)` — build `detail` as before, then `return detail, nil`
- Remove all `fmt.Printf`, `fmt.Println`, `json.Marshal`/`json.Encoder` output calls
- Remove unused imports

### 3. `pkg/ops/search.go` — SearchOperation

Change the interface:
```go
type SearchOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        scopeDir string,
        query string,
        topK int,
    ) ([]string, error)
}
```

In `searchOperation.Execute`:
- Remove `outputFormat string` parameter
- After capturing `output` from `cmd.CombinedOutput()`, return the lines as `[]string`:
  - If result is empty: `return []string{}, nil`
  - Otherwise: `return strings.Split(result, "\n"), nil`
- Remove all `fmt.Println`, `json.NewEncoder(os.Stdout)` output calls
- Remove unused imports (`encoding/json`, `fmt`, `os`)

### 4. `pkg/ops/decision_list.go` — DecisionListOperation

Change the interface:
```go
type DecisionListOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        vaultName string,
        showReviewed bool,
        showAll bool,
    ) ([]DecisionListItem, error)
}
```

In `decisionListOperation.Execute`:
- Remove `outputFormat string` parameter
- Build the `[]DecisionListItem` slice (always, regardless of format) and `return items, nil`
- Remove all `fmt.Printf`, `json.NewEncoder(os.Stdout)` output calls
- Remove unused imports (`encoding/json`, `fmt`, `os`)

### 5. `pkg/ops/frontmatter_entity.go` — EntityShowOperation

Define a new result type in `pkg/ops/frontmatter_entity.go`:
```go
// EntityShowResult is the structured result from EntityShowOperation.
type EntityShowResult struct {
    Name     string            `json:"name"`
    FilePath string            `json:"file_path"`
    Vault    string            `json:"vault"`
    Fields   map[string]string `json:"fields"`
    Content  string            `json:"content"`
}
```

Change the interface (line ~548):
```go
type EntityShowOperation interface {
    Execute(ctx context.Context, vaultPath, vaultName, entityName string) (EntityShowResult, error)
}
```

In `entityShowOperation.Execute`:
- Remove `outputFormat string` parameter
- Build and return `EntityShowResult` with the name, filePath, vault, fields, and content
- Remove all `json.Marshal`, `fmt.Printf`, `fmt.Println` output calls
- Remove unused imports if needed

### 6. Regenerate mocks

Run:
```
go generate ./pkg/ops/...
```
This regenerates:
- `mocks/list-operation.go`
- `mocks/show-operation.go`
- `mocks/search-operation.go`
- `mocks/decision-list-operation.go`
- `mocks/entity-show-operation.go`

### 7. Update `pkg/cli/cli.go` — CLI call sites

**`createTaskListCommand` (line ~307) and `createGenericListCommand` (line ~474):**

Change `listOp.Execute(...)` call to receive `(items, err)`. Then format output:
```go
items, err := listOp.Execute(ctx, vault.Path, vault.Name, ...)
if err != nil {
    return err
}
if *outputFormat == cli.OutputFormatJSON {
    // collect across vaults: append items to a slice, encode at end
} else {
    for _, item := range items {
        fmt.Printf("[%s] %s\n", item.Status, item.Name)
    }
}
```

For `createTaskListCommand` with multi-vault JSON, accumulate all items across vaults into one slice and encode once at the end.

For `createGenericListCommand` (used by goal list, theme list, etc.), same pattern — accumulate across vaults.

**`createTaskShowCommand` (line ~1605):**
```go
detail, err := showOp.Execute(ctx, vault.Path, vault.Name, taskName)
if err != nil {
    return errors.Wrap(ctx, err, "show task")
}
if *outputFormat == cli.OutputFormatJSON {
    return cli.PrintJSON(detail)
}
fmt.Printf("Task: %s\n", detail.Name)
fmt.Printf("Status: %s\n", detail.Status)
if detail.Assignee != "" {
    fmt.Printf("Assignee: %s\n", detail.Assignee)
}
if detail.Priority != 0 {
    fmt.Printf("Priority: %d\n", detail.Priority)
}
if detail.Phase != "" {
    fmt.Printf("Phase: %s\n", detail.Phase)
}
return nil
```

**`createEntityShowCommand` (line ~698):**
```go
result, err := showOp.Execute(ctx, vault.Path, vault.Name, entityName)
if err != nil {
    return err
}
if *outputFormat == cli.OutputFormatJSON {
    return cli.PrintJSON(result)
}
fmt.Printf("%s: %s\n", entityType, result.Name)
for _, name := range fieldOrder {  // preserve original field ordering
    fmt.Printf("%s: %s\n", name, result.Fields[name])
}
return nil
```

Note: the original entity show plain-text output iterates fields in order. You may preserve order by changing `EntityShowResult.Fields` to use `[]EntityShowField` (name+value pairs) or by keeping field order in a separate slice. Choose the simpler approach: add `FieldOrder []string` to `EntityShowResult` so CLI can print fields in order. Populate `FieldOrder` in `entityShowOperation.Execute` from `fieldOrder` slice already computed there.

**`createDecisionListCommand` (line ~1280):**
```go
items, err := listOp.Execute(ctx, vault.Path, vault.Name, showReviewed, showAll)
if err != nil {
    slog.Warn("vault error", "vault", vault.Name, "error", err)
    continue
}
if *outputFormat == cli.OutputFormatJSON {
    allItems = append(allItems, items...)
} else {
    for _, item := range items {
        reviewStatus := "unreviewed"
        if item.Reviewed {
            reviewStatus = "reviewed"
        }
        fmt.Printf("[%s] %s\n", reviewStatus, item.Name)
    }
}
```
Encode allItems as JSON after iterating all vaults.

**`createSearchCommand` and `createGenericSearchCommand` (line ~1348, ~1400):**
```go
results, err := searchOp.Execute(ctx, vault.Path, scopeDir, query, topK)
if err != nil {
    return err
}
if *outputFormat == cli.OutputFormatJSON {
    return cli.PrintJSON(results)
}
fmt.Println(strings.Join(results, "\n"))
return nil
```

### 8. Update tests

In test files for each changed operation:
- Remove stdout capture setup (`os.Stdout` redirect, buffer reading)
- Assert directly on the returned value: `Expect(result).To(...)` or `Expect(err).To(...)`
- Keep all test cases; only change the assertion style

Files to update:
- `pkg/ops/list_test.go`
- `pkg/ops/show_test.go`
- `pkg/ops/search_test.go` (if exists)
- `pkg/ops/decision_list_test.go`
- `pkg/ops/frontmatter_entity_test.go`
</requirements>

<constraints>
- CLI output format must not change — same text, same JSON structure, same field names
- Operation naming convention is preserved (no renames)
- Mock generation comments (`//counterfeiter:generate`) are preserved in the interface files; mocks are regenerated after interface changes
- Factory function pattern (pure composition, no I/O) is preserved
- Do NOT commit — dark-factory handles git
- Existing tests must still pass after assertion updates
- No operation in pkg/ops/ may write to os.Stdout after this prompt (for the five operations changed here)
</constraints>

<verification>
```
make precommit
```

```
grep -r 'os\.Stdout' pkg/ops/list.go pkg/ops/show.go pkg/ops/search.go pkg/ops/decision_list.go pkg/ops/frontmatter_entity.go
# expected: no output
```

```
grep -r 'fmt\.Print' pkg/ops/list.go pkg/ops/show.go pkg/ops/search.go pkg/ops/decision_list.go pkg/ops/frontmatter_entity.go
# expected: no output
```
</verification>
