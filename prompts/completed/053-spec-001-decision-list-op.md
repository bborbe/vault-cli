---
status: completed
spec: ["001"]
summary: Implemented DecisionListOperation in pkg/ops/decision_list.go with filter modes (unreviewed/reviewed/all), plain and JSON output, alphabetical sorting, and generated counterfeiter mock; tests cover all filter modes, output formats, empty results, and sorting at 85.9% coverage
container: vault-cli-053-spec-001-decision-list-op
dark-factory-version: v0.54.0
created: "2026-03-16T00:00:00Z"
queued: "2026-03-16T10:36:41Z"
started: "2026-03-16T10:45:29Z"
completed: "2026-03-16T10:49:47Z"
branch: dark-factory/decision-list-ack
---

<summary>
- A new DecisionListOperation produces filtered, sorted output of vault decisions
- Default mode shows only unreviewed decisions; --reviewed flag shows only reviewed; --all shows all
- Plain output emits one line per decision: "[reviewed/unreviewed] relative/path/from/vault/root"
- JSON output emits a structured array with all relevant decision fields
- The operation is injectable via interface and has a counterfeiter annotation for mock generation
- Tests cover all three filter modes, both output formats, empty results, and sorting
</summary>

<objective>
Implement `DecisionListOperation` in `pkg/ops/decision_list.go` that filters and formats decisions from a vault, enabling the CLI to render `vault-cli decision list` output.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/ops/list.go` — follow the interface/constructor/struct pattern; note how `TaskListItem` is defined and how JSON vs plain output is handled.
Read `pkg/domain/decision.go` — the Decision struct (fields: NeedsReview, Reviewed, ReviewedDate, Status, Type, PageType; metadata: Name, Content, FilePath).
Read `pkg/storage/markdown.go` — the `Storage` interface with the `ListDecisions` method added in prompt 2.
Read `docs/development-patterns.md` — "Adding a New Command" and "Output Format" sections.
</context>

<requirements>
1. Create `pkg/ops/decision_list.go` with a `DecisionListOperation` interface and implementation:

```go
//counterfeiter:generate -o ../../mocks/decision-list-operation.go --fake-name DecisionListOperation . DecisionListOperation
type DecisionListOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        vaultName string,
        showReviewed bool,
        showAll bool,
        outputFormat string,
    ) error
}
```

2. Constructor:

```go
func NewDecisionListOperation(storage storage.Storage) DecisionListOperation {
    return &decisionListOperation{storage: storage}
}
```

3. Implement `Execute` on `decisionListOperation`:
   - Call `storage.ListDecisions(ctx, vaultPath)` to get all decisions — if error, return `errors.Wrap(ctx, err, "list decisions")`
   - Filter based on flags:
     - `showAll == true`: include all decisions (no filter)
     - `showReviewed == true` (and `showAll == false`): include only decisions where `Reviewed == true`
     - Default (both false): include only decisions where `Reviewed == false`
   - Sort decisions alphabetically by `Name` (case-insensitive)
   - For plain output (`outputFormat != "json"`):
     - Print one line per decision: `fmt.Printf("[%s] %s\n", reviewStatus, decision.Name)`
     - `reviewStatus` is `"reviewed"` if `decision.Reviewed == true`, otherwise `"unreviewed"`
   - For JSON output (`outputFormat == "json"`):
     - Build a `[]DecisionListItem` and encode with `json.NewEncoder(os.Stdout)` with `SetIndent("", "  ")`
     - Return the encoder's error

4. Define `DecisionListItem` struct for JSON output:

```go
type DecisionListItem struct {
    Name         string `json:"name"`
    Reviewed     bool   `json:"reviewed"`
    ReviewedDate string `json:"reviewed_date,omitempty"`
    Status       string `json:"status,omitempty"`
    Type         string `json:"type,omitempty"`
    PageType     string `json:"page_type,omitempty"`
    Vault        string `json:"vault"`
}
```

Populate from `domain.Decision` fields + `vaultName` for `Vault`.

5. Create `pkg/ops/decision_list_test.go` in the external test package `ops_test`:
   - Uses `mocks.Storage` (counterfeiter mock) — call `go generate ./...` after adding the interface if mock doesn't exist yet
   - Test: default filter returns only unreviewed decisions
   - Test: `showReviewed=true` returns only reviewed decisions
   - Test: `showAll=true` returns all decisions regardless of reviewed state
   - Test: empty vault returns no output (no error)
   - Test: plain output format produces `[unreviewed] Some Page\n` lines
   - Test: JSON output format produces valid JSON array
   - Test: results are sorted alphabetically by name
</requirements>

<constraints>
- Decision is a separate domain type — do NOT reuse ListOperation or reference TaskListItem
- The `showReviewed` and `showAll` flags are booleans passed by the CLI — the operation does not parse cobra flags
- Plain output status string is `"reviewed"` or `"unreviewed"` (not `"true"`/`"false"`)
- Empty result must produce empty JSON array `[]` not `null`; use `make([]DecisionListItem, 0)` before the loop
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- License header required (copy from list.go)
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
