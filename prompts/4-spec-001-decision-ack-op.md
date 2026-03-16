---
status: created
spec: ["001"]
created: "2026-03-16T00:00:00Z"
branch: dark-factory/decision-list-ack
---

<summary>
- A new DecisionAckOperation finds a decision by name and marks it as reviewed
- Sets reviewed: true and reviewed_date to today's date (YYYY-MM-DD) on the decision
- Optionally overrides the decision's status field when --status flag is provided
- The markdown body content is preserved unchanged — only frontmatter is updated
- Ambiguous partial matches and not-found names return descriptive errors
- The operation injects libtime.CurrentDateTime for testable date generation
- Tests cover successful ack, optional status override, not-found, ambiguous match, and write failure
</summary>

<objective>
Implement `DecisionAckOperation` in `pkg/ops/decision_ack.go` that finds a decision by name, sets `reviewed: true` and `reviewed_date: <today>`, optionally updates the `status` field, and writes the file back — enabling `vault-cli decision ack <name>` to work.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/ops/complete.go` — follow the interface/constructor/struct pattern; note how `libtime.CurrentDateTime` is injected and how `errors.Wrap` is used.
Read `pkg/domain/decision.go` — the Decision struct (fields: NeedsReview, Reviewed, ReviewedDate, Status; metadata: Name, Content, FilePath).
Read `pkg/storage/markdown.go` — the `Storage` interface with `FindDecisionByName` and `WriteDecision` added in prompt 2.
Read `docs/development-patterns.md` — "Multi-Vault Pattern" and "Testability" sections.
</context>

<requirements>
1. Create `pkg/ops/decision_ack.go` with a `DecisionAckOperation` interface and implementation:

```go
//counterfeiter:generate -o ../../mocks/decision-ack-operation.go --fake-name DecisionAckOperation . DecisionAckOperation
type DecisionAckOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        vaultName string,
        decisionName string,
        statusOverride string,
        outputFormat string,
    ) error
}
```

2. Constructor:

```go
func NewDecisionAckOperation(
    storage storage.Storage,
    currentDateTime libtime.CurrentDateTime,
) DecisionAckOperation {
    return &decisionAckOperation{
        storage:         storage,
        currentDateTime: currentDateTime,
    }
}
```

3. Implement `Execute` on `decisionAckOperation`:
   a. Call `storage.FindDecisionByName(ctx, vaultPath, decisionName)`
      - If error: return `errors.Wrap(ctx, err, "find decision")`
   b. Set `decision.Reviewed = true`
   c. Set `decision.ReviewedDate = currentDateTime.Now().Format("2006-01-02")`
      - `libtime.DateTime` has a `.Format()` method — call it directly (same pattern as `complete.go` line 134)
      - Do NOT call `.Time()` first — `.Format()` works directly on `DateTime`
   d. If `statusOverride != ""`: set `decision.Status = statusOverride`
   e. Call `storage.WriteDecision(ctx, decision)`
      - If error: return `errors.Wrap(ctx, err, "write decision")`
   f. Output:
      - Plain: `fmt.Printf("Acknowledged: %s\n", decision.Name)`
      - JSON: encode a `MutationResult{Success: true, Name: decision.Name, Vault: vaultName}` using `json.NewEncoder(os.Stdout)` with `SetIndent("", "  ")`
      - Reuse `MutationResult` from `pkg/ops/complete.go` — it is already defined there

4. Create `pkg/ops/decision_ack_test.go` in the external test package `ops_test`:
   - Uses `mocks.Storage` and `mocks.CurrentDateTime` (counterfeiter mocks)
   - Test: successful ack sets Reviewed=true and ReviewedDate=today, writes decision, prints plain output
   - Test: successful ack with statusOverride sets decision.Status
   - Test: successful ack outputs valid JSON MutationResult when outputFormat="json"
   - Test: FindDecisionByName error is propagated
   - Test: WriteDecision error is propagated
   - Set `mockCurrentDateTime.NowReturns(...)` in tests to return a fixed date for deterministic assertions
</requirements>

<constraints>
- `MutationResult` is already defined in `pkg/ops/complete.go` — import and reuse it, do NOT redefine it
- `libtime.CurrentDateTime` is an interface; inject it — never call `time.Now()` directly
- `currentDateTime.Now()` returns `libtime.DateTime` which has `.Format()` directly — use `currentDateTime.Now().Format("2006-01-02")` (same as `complete.go` line 134)
- `statusOverride` is a plain string — no validation against known values (decisions have open-ended status)
- The ack operation does NOT check whether the decision is already reviewed — it is idempotent
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- License header required (copy from complete.go)
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
