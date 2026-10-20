---
spec: ["061-goal-topic-approve"]
status: draft
created: "2026-10-20T10:03:18Z"
---

# Add the topic approve operation that records phase, approver and instant (spec 061, prompt 3 of 5)

<summary>
- A topic waiting in the approval inbox can be approved in one operation.
- Approval writes the planning phase, who approved it, and when, in a single write.
- A topic that is not in the inbox is refused, and nothing is written.
- A topic that already carries an approval record is refused, and nothing is written.
- An empty approver is refused before the topic is even read.
- A zero clock reading is refused rather than recorded as a placeholder.
- Every unrelated frontmatter key on the topic survives the write.
- The goal and task approval flows are untouched; this is a sibling operation that reuses their shared record guard.
- Unit tests pin every recorded fact and every refusal.
</summary>

<objective>
Give topics the same recorded approval transition goals get in prompt 2: one operation that moves a `phase: todo` topic to `phase: planning` and records `approved_by` and `approved_at` in the same write, refusing every state that is not the approval inbox.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully, in this order:
- `docs/task-writing.md` § Phase transitions — the approval contract being mirrored.
- `pkg/ops/goal_approve.go` and `pkg/ops/approval_record.go` — the sibling shipped by prompt 2. This prompt is the topic twin: same sequence, same messages with `topic` substituted, and it **reuses** `refuseExistingEntityApprovalRecord` rather than defining a second guard. If `refuseExistingEntityApprovalRecord` is missing, stop and report that prompt 2 has not landed.
- `pkg/ops/task_approve.go` — the original exemplar for the sequence (empty approver → find → clock → inbox guard → existing-record guard → one composed write).
- `pkg/ops/topic_complete.go` — the topic-operation shape in this package (`TopicStorage`, `MutationResult`, `errors.Wrap(ctx, err, "find topic")` / `"write topic"`).
- `pkg/domain/topic.go` — `NewTopic(data map[string]any, meta FileMetadata, content Content) *Topic`.
- `pkg/domain/topic_frontmatter.go` — `Phase() *TopicPhase` (nil when absent), `SetPhase(*TopicPhase)`, `Get(key string) any`, `Set(key string, value any)`.
- `pkg/domain/topic_phase.go` — `TopicPhaseTodo`, `TopicPhasePlanning`, and `Ptr()`.
- `pkg/storage/storage.go` — `TopicStorage` with `WriteTopic(ctx, *domain.Topic) error` and `FindTopicByName(ctx, vaultPath, name string) (*domain.Topic, error)`.
- `pkg/ops/complete.go` — `MutationResult` (`Success`, `Name`, `Vault`, `Error`).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface → constructor → private struct.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap` / `errors.Errorf` from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega and Counterfeiter mocks.
</context>

<requirements>
1. **Add the operation** in a new file `pkg/ops/topic_approve.go`, following the interface → constructor → private struct shape:

   ```go
   //counterfeiter:generate -o ../../mocks/topic-approve-operation.go --fake-name TopicApproveOperation . TopicApproveOperation
   type TopicApproveOperation interface {
       Execute(
           ctx context.Context,
           vaultPath string,
           topicName string,
           vaultName string,
           approvedBy string,
       ) (MutationResult, error)
   }

   func NewTopicApproveOperation(
       topicStorage storage.TopicStorage,
       currentDateTime libtime.CurrentDateTime,
   ) TopicApproveOperation
   ```

   `libtime` is `github.com/bborbe/time`. Write a doc comment on the interface stating the contract: only a topic at `phase: todo` can be approved; the approval writes `phase: planning`, `approved_by` and `approved_at` in one write; a topic that is not in the inbox, or that already carries an approval record, is refused with nothing written; topics are not required to have an owner and this verb does not assign one.

2. **`Execute` runs exactly this sequence**, returning `MutationResult{Success: false, Error: err.Error()}, err` at each refusal:
   1. `approvedBy == ""` → refuse **before** reading the topic: `approved_by must not be empty: refusing to record a blank approver on topic %q`.
   2. `topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, topicName)`; on error return `errors.Wrap(ctx, err, "find topic")`.
   3. `now := o.currentDateTime.Now()`; if `now.Time().IsZero()` → refuse before any guard: `approved_at is not a valid instant: refusing to record a zero timestamp on topic %q`.
   4. Inbox guard: `phase := topic.Phase()`; if `phase == nil || *phase != domain.TopicPhaseTodo` → refuse. Render an absent phase as `(none)`, mirroring `task_approve.go`, and name both the phase found and the phase required: `refusing to approve %q: topic is at phase %q, not %q; only a topic in the approval inbox (phase %q) can be approved`.
   5. `refuseExistingEntityApprovalRecord(ctx, "topic", topicName, topic.Get)` — the helper from `pkg/ops/approval_record.go` (prompt 2). Do not duplicate it.
   6. Compose onto the existing map, then write once:
      ```go
      topic.SetPhase(domain.TopicPhasePlanning.Ptr())
      topic.Set("approved_by", approvedBy)
      topic.Set("approved_at", now.Time())
      ```
      `now.Time()` is a bare `time.Time` so yaml.v3 renders it unquoted, exactly as `task approve` writes it. Do not format it as a string — a string is quoted by the encoder and would diverge from the task record.
   7. `o.topicStorage.WriteTopic(ctx, topic)`; on error return `errors.Wrap(ctx, err, "write topic")`. Exactly one write per successful approval.
   8. Return `MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil`.

   Every guard runs before step 6, so a refusal leaves the file byte-identical.

3. **Unit tests** in a new file `pkg/ops/topic_approve_test.go`, package `ops_test`, using `mocks.Storage` and a pinned clock exactly as `task_approve_test.go` does. Cover:
   - one write; the written topic's `Phase()` is `domain.TopicPhasePlanning.Ptr()`, `GetString("approved_by")` is `operator`, `Get("approved_at")` is non-nil; `result.Name` and `result.Vault` are the call's values.
   - the recorded instant is the injected one and is a bare `time.Time` (type assertion).
   - YAML boundary: `yaml.Marshal(written.RawMap())` contains `approved_at: <the pinned instant in RFC3339>` unquoted (with the exemplar's pin, `approved_at: 2026-03-03T12:00:00Z`), and unmarshalling it back yields a `time.Time` equal to the injected instant. This is the serialization boundary the manager-start gate will read.
   - the approver named on the call is recorded verbatim (`Manager Layer`).
   - every other key survives: seed `status`, `page_type`, `tags`, `defer_date`, `custom_key` and assert each after the write.
   - the inbox guard, table-driven over `planning`, `execution`, `done`, and an absent `phase`: each refused, the error names the found and required phase, and `WriteTopicCallCount() == 0`.
   - the existing-record guard, three fixtures at `phase: todo`: only `approved_by`, only `approved_at`, and both. Each refused, the error names the present key(s), `WriteTopicCallCount() == 0`.
   - an empty `approvedBy` is refused with `FindTopicByNameCallCount() == 0` — the refusal happens before the read.
   - a zero clock (`zero := libtime.NewCurrentDateTime(); zero.SetNow(libtime.DateTime(time.Time{}))`) is refused with zero writes.
   - a `FindTopicByName` error is wrapped with `find topic`; a `WriteTopic` error is wrapped with `write topic`.

4. **Self-check before finishing.** Re-run the `<verification>` block and confirm every row; walk acceptance criteria 1, 2 and 4 of spec 061 against the change (the CLI-level halves are prompt 4's job).
</requirements>

<constraints>
- The approval contract being mirrored is `docs/task-writing.md` § Phase transitions — read it rather than re-deriving the rules.
- Reuse `storage.TopicStorage` and `FindTopicByName`; do not add a new storage method.
- Reuse `refuseExistingEntityApprovalRecord` from `pkg/ops/approval_record.go`. Do not define a second copy and do not modify the goal operation shipped by prompt 2.
- Do not change `pkg/ops/task_approve.go`, the task-side guards, or the frontmatter serializers.
- A refusal writes nothing: every guard runs before the first mutation, so the file on disk is byte-identical.
- No owner/assignee handling: topics are not required to have an assignee, and this verb does not add one (spec Non-goal).
- No `--force` and no override: re-approving an already-approved topic is refused.
- No backfilling of `phase` on existing topics.
- `pkg/cli/` is out of scope — prompt 4 wires the CLI. `docs/` and `CHANGELOG.md` are prompt 5's.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
EVIDENCE GREPS — run each and confirm the count. Absence assertions use `! grep -q` because `grep -c` exits 1 when the count is 0.

```
grep -c 'type TopicApproveOperation interface' pkg/ops/topic_approve.go       # 1
grep -c 'func NewTopicApproveOperation' pkg/ops/topic_approve.go              # 1
grep -c 'topicStorage.WriteTopic' pkg/ops/topic_approve.go                    # 1 — exactly one write path
grep -c 'refuseExistingEntityApprovalRecord(ctx' pkg/ops/topic_approve.go     # 1 — the reused helper's call site
! grep -q 'func refuseExistingEntityApprovalRecord' pkg/ops/topic_approve.go   # the helper is not duplicated here
grep -c 'counterfeiter:generate' pkg/ops/topic_approve.go                     # 1
grep -c 'Describe("TopicApproveOperation"' pkg/ops/topic_approve_test.go      # 1
grep -c 'custom_key' pkg/ops/topic_approve_test.go                            # >= 1 — the key-preservation fixture
grep -c 'yaml.Marshal' pkg/ops/topic_approve_test.go                          # >= 1 — the serialization boundary
```

FROZEN-NEIGHBOUR GUARD — the container has no usable `.git` (the daemon runs with `hideGit=true`), so this cannot be checked with git. The `<constraints>` forbid touching these files, and `make precommit` catches a broken neighbour. If a hard guard is wanted, record `shasum -a 256` of the four files at authoring time and compare:

```
sha256sum pkg/ops/task_approve.go pkg/ops/frontmatter.go pkg/ops/goal_approve.go pkg/ops/approval_record.go > /tmp/frozen.sha256   # BEFORE editing
sha256sum -c /tmp/frozen.sha256                                                                                                  # after — every row must print OK
```

TESTS:

```
go build ./...
go test ./pkg/ops/... -ginkgo.focus="TopicApproveOperation" -ginkgo.v
make precommit
```

⚠️ `-ginkgo.focus` with zero matching specs exits 0 while running nothing. After the focus run, confirm the new spec names appear in the output; a log without them is a failure, not a pass.

SYNTAX:

```
gofmt -e -l pkg/ops/topic_approve.go pkg/ops/topic_approve_test.go
```

`make precommit` runs `generate` (which regenerates `mocks/` from the counterfeiter directives) and then `addlicense` (which restores the copyright header on every `.go` file, including `mocks/mocks.go`). Confirm `mocks/topic-approve-operation.go` was generated. Do not use git to restore anything — `.git` is masked in the container.
</verification>
