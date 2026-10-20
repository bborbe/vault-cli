---
spec: ["061-goal-topic-approve"]
status: draft
created: "2026-10-20T10:03:18Z"
---

# Add the goal approve operation that records phase, approver and instant (spec 061, prompt 2 of 5)

<summary>
- A goal waiting in the approval inbox can be approved in one operation.
- Approval writes the planning phase, who approved it, and when, in a single write.
- A goal that is not in the inbox is refused, and nothing is written.
- A goal that already carries an approval record is refused, and nothing is written.
- An empty approver is refused before the goal is even read.
- A zero clock reading is refused rather than recorded as a placeholder.
- Every unrelated frontmatter key on the goal survives the write.
- The task approval flow is untouched; this is a new sibling operation.
- Unit tests pin every recorded fact and every refusal.
</summary>

<objective>
Give goals the same recorded approval transition tasks already have: one operation that moves a `phase: todo` goal to `phase: planning` and records `approved_by` and `approved_at` in the same write, refusing every state that is not the approval inbox. This is the writer the manager-start gate reads.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully, in this order:
- `docs/task-writing.md` § Phase transitions — the approval contract being mirrored. Read it rather than re-deriving the rules from `pkg/ops/task_approve.go`.
- `pkg/ops/task_approve.go` — the exemplar: `TaskApproveOperation`, its constructor, and its `Execute` sequence (empty approver → find → clock → inbox guard → existing-record guard → one composed write). Note what this prompt deliberately omits: the task variant's assignee / current-user owner resolution.
- `pkg/ops/task_approve_test.go` — the test shape: `mocks.Storage`, a pinned `libtime.CurrentDateTime` via `libtimetest.ParseDateTime`, and the YAML round-trip spec for `approved_at`.
- `pkg/domain/goal.go` — `NewGoal(data map[string]any, meta FileMetadata, content Content) *Goal`.
- `pkg/domain/goal_frontmatter.go` — `Phase() *GoalPhase` (nil when absent), `SetPhase(*GoalPhase)`, `Get(key string) any`, `Set(key string, value any)`.
- `pkg/domain/goal_phase.go` — `GoalPhaseTodo`, `GoalPhasePlanning`, and `Ptr()`.
- `pkg/domain/frontmatter_map.go` — `Get` returns the raw value and `nil` for an absent key; that is what the existing-record guard checks.
- `pkg/storage/storage.go` — `GoalStorage` with `WriteGoal(ctx, *domain.Goal) error` and `FindGoalByName(ctx, vaultPath, name string) (*domain.Goal, error)`.
- `pkg/ops/complete.go` — `MutationResult` (`Success`, `Name`, `Vault`, `Error`).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface → constructor → private struct.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap` / `errors.Errorf` from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega and Counterfeiter mocks.
</context>

<requirements>
1. **Add the shared approval-record guard** in a new file `pkg/ops/approval_record.go`:

   ```go
   // refuseExistingEntityApprovalRecord returns the refusal error for a goal or
   // topic row that already carries an approved_by or an approved_at key, and
   // nil otherwise.
   func refuseExistingEntityApprovalRecord(
       ctx context.Context,
       kind string,
       name string,
       get func(string) any,
   ) error
   ```

   Behaviour: check `get("approved_by")` and `get("approved_at")` independently; collect the names of the keys that are present (`[]string`, in that order); return `nil` when none is present; otherwise return `errors.Errorf` naming the row, the kind, and every present key, e.g. `refusing to approve %q: goal already carries an approval record (approved_by); re-approving is refused`. Both keys are checked independently so a row carrying either one is refused. Do **not** call `task_approve.go`'s `refuseExistingApprovalRecord` and do **not** modify it — it is typed on `*domain.Task` and frozen.

2. **Add the operation** in a new file `pkg/ops/goal_approve.go`, following the interface → constructor → private struct shape:

   ```go
   //counterfeiter:generate -o ../../mocks/goal-approve-operation.go --fake-name GoalApproveOperation . GoalApproveOperation
   type GoalApproveOperation interface {
       Execute(
           ctx context.Context,
           vaultPath string,
           goalName string,
           vaultName string,
           approvedBy string,
       ) (MutationResult, error)
   }

   func NewGoalApproveOperation(
       goalStorage storage.GoalStorage,
       currentDateTime libtime.CurrentDateTime,
   ) GoalApproveOperation
   ```

   `libtime` is `github.com/bborbe/time`. Write a doc comment on the interface stating the contract: only a goal at `phase: todo` can be approved; the approval writes `phase: planning`, `approved_by` and `approved_at` in one write; a goal that is not in the inbox, or that already carries an approval record, is refused with nothing written; goals are not required to have an owner and this verb does not assign one.

3. **`Execute` runs exactly this sequence**, returning `MutationResult{Success: false, Error: err.Error()}, err` at each refusal:
   1. `approvedBy == ""` → refuse **before** reading the goal: `approved_by must not be empty: refusing to record a blank approver on goal %q`.
   2. `goal, err := o.goalStorage.FindGoalByName(ctx, vaultPath, goalName)`; on error return `errors.Wrap(ctx, err, "find goal")`.
   3. `now := o.currentDateTime.Now()`; if `now.Time().IsZero()` → refuse before any guard: `approved_at is not a valid instant: refusing to record a zero timestamp on goal %q`.
   4. Inbox guard: `phase := goal.Phase()`; if `phase == nil || *phase != domain.GoalPhaseTodo` → refuse. Render an absent phase as `(none)`, mirroring `task_approve.go`, and name both the phase found and the phase required: `refusing to approve %q: goal is at phase %q, not %q; only a goal in the approval inbox (phase %q) can be approved`.
   5. `refuseExistingEntityApprovalRecord(ctx, "goal", goalName, goal.Get)`.
   6. Compose onto the existing map, then write once:
      ```go
      goal.SetPhase(domain.GoalPhasePlanning.Ptr())
      goal.Set("approved_by", approvedBy)
      goal.Set("approved_at", now.Time())
      ```
      `now.Time()` is a bare `time.Time` so yaml.v3 renders it unquoted, exactly as `task approve` writes it. Do not format it as a string — a string is quoted by the encoder and would diverge from the task record.
   7. `o.goalStorage.WriteGoal(ctx, goal)`; on error return `errors.Wrap(ctx, err, "write goal")`. Exactly one write per successful approval.
   8. Return `MutationResult{Success: true, Name: goal.Name, Vault: vaultName}, nil`.

   Every guard runs before step 6, so a refusal leaves the file byte-identical.

4. **Unit tests** in a new file `pkg/ops/goal_approve_test.go`, package `ops_test`, using `mocks.Storage` and a pinned clock exactly as `task_approve_test.go` does. Cover:
   - one write; the written goal's `Phase()` is `domain.GoalPhasePlanning.Ptr()`, `GetString("approved_by")` is `operator`, `Get("approved_at")` is non-nil; `result.Name` and `result.Vault` are the call's values.
   - the recorded instant is the injected one and is a bare `time.Time` (type assertion).
   - YAML boundary: `yaml.Marshal(written.RawMap())` contains `approved_at: <the pinned instant in RFC3339>` unquoted (with the exemplar's pin, `approved_at: 2026-03-03T12:00:00Z`), and unmarshalling it back yields a `time.Time` equal to the injected instant. This is the serialization boundary the manager-start gate will read.
   - the approver named on the call is recorded verbatim (`Manager Layer`).
   - every other key survives: seed `priority`, `page_type`, `theme`, `assignee`, `custom_key` and assert each after the write.
   - the inbox guard, table-driven over `planning`, `execution`, `done`, and an absent `phase`: each refused, the error names the found and required phase, and `WriteGoalCallCount() == 0`.
   - the existing-record guard, three fixtures at `phase: todo`: only `approved_by`, only `approved_at`, and both. Each refused, the error names the present key(s), `WriteGoalCallCount() == 0`.
   - an empty `approvedBy` is refused with `FindGoalByNameCallCount() == 0` — the refusal happens before the read.
   - a zero clock (`zero := libtime.NewCurrentDateTime(); zero.SetNow(libtime.DateTime(time.Time{}))`) is refused with zero writes.
   - a `FindGoalByName` error is wrapped with `find goal`; a `WriteGoal` error is wrapped with `write goal`.

5. **Self-check before finishing.** Re-run the `<verification>` block and confirm every row; walk acceptance criteria 1, 2 and 4 of spec 061 against the change (the CLI-level halves are prompt 4's job).
</requirements>

<constraints>
- The approval contract being mirrored is `docs/task-writing.md` § Phase transitions — read it rather than re-deriving the rules.
- Reuse `storage.GoalStorage` and `FindGoalByName`; do not add a new storage method.
- Do not change `pkg/ops/task_approve.go`, the task-side guards, or the frontmatter serializers.
- A refusal writes nothing: every guard runs before the first mutation, so the file on disk is byte-identical.
- No owner/assignee handling: goals are not required to have an assignee, and this verb does not add one (spec Non-goal).
- No `--force` and no override: re-approving an already-approved goal is refused.
- No backfilling of `phase` on existing goals.
- `pkg/cli/` is out of scope — prompt 4 wires the CLI. `docs/` and `CHANGELOG.md` are prompt 5's.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
EVIDENCE GREPS — run each and confirm the count. Absence assertions use `! grep -q` because `grep -c` exits 1 when the count is 0.

```
grep -c 'type GoalApproveOperation interface' pkg/ops/goal_approve.go        # 1
grep -c 'func NewGoalApproveOperation' pkg/ops/goal_approve.go               # 1
grep -c 'goalStorage.WriteGoal' pkg/ops/goal_approve.go                      # 1 — exactly one write path
grep -c 'func refuseExistingEntityApprovalRecord' pkg/ops/approval_record.go  # 1
grep -c 'refuseExistingEntityApprovalRecord(ctx' pkg/ops/goal_approve.go     # 1 — the call site
grep -c 'counterfeiter:generate' pkg/ops/goal_approve.go                     # 1
grep -c 'Describe("GoalApproveOperation"' pkg/ops/goal_approve_test.go       # 1
grep -c 'custom_key' pkg/ops/goal_approve_test.go                            # >= 1 — the key-preservation fixture
grep -c 'yaml.Marshal' pkg/ops/goal_approve_test.go                          # >= 1 — the serialization boundary
```

FROZEN-NEIGHBOUR GUARD — the container has no usable `.git` (the daemon runs with `hideGit=true`), so this cannot be checked with git. The `<constraints>` forbid touching these files, and `make precommit` catches a broken neighbour. If a hard guard is wanted, record `shasum -a 256` of the three files at authoring time and compare:

```
sha256sum pkg/ops/task_approve.go pkg/ops/frontmatter.go pkg/ops/frontmatter_entity.go > /tmp/frozen.sha256   # BEFORE editing
sha256sum -c /tmp/frozen.sha256                                                                              # after — every row must print OK
```

TESTS:

```
go build ./...
go test ./pkg/ops/... -ginkgo.focus="GoalApproveOperation" -ginkgo.v
make precommit
```

⚠️ `-ginkgo.focus` with zero matching specs exits 0 while running nothing. After the focus run, confirm the new spec names appear in the output; a log without them is a failure, not a pass.

SYNTAX:

```
gofmt -e -l pkg/ops/goal_approve.go pkg/ops/approval_record.go pkg/ops/goal_approve_test.go
```

`make precommit` runs `generate` (which regenerates `mocks/` from the counterfeiter directives) and then `addlicense` (which restores the copyright header on every `.go` file, including `mocks/mocks.go`). Confirm `mocks/goal-approve-operation.go` was generated. Do not use git to restore anything — `.git` is masked in the container.
</verification>
