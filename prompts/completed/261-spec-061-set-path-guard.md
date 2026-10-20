---
status: completed
spec: [061-goal-topic-approve]
summary: Added goal/topic todo-to-planning set refusals with unit and binary-level coverage
execution_id: vault-cli-goal-topic-approve-exec-261-spec-061-set-path-guard
dark-factory-version: v0.198.7
created: "2026-10-20T10:03:18Z"
queued: "2026-10-20T10:16:58Z"
started: "2026-10-20T10:17:58Z"
completed: "2026-10-20T10:20:59Z"
---

# Refuse the unrecorded exit from the goal and topic approval inbox (spec 061, prompt 1 of 5)

<summary>
- A goal waiting in the approval inbox can no longer be moved to planning with the generic field-set command.
- The same refusal applies to a topic.
- The refusal names the row, the phase it is in, the phase it would need, and the command to run instead.
- A refused set leaves the file on disk byte-identical.
- Every other set keeps its current behaviour: another target phase, a non-phase key, a row already past the inbox, and a row with no phase at all.
- The task-side guard and the task approval flow are untouched.
- Unit tests pin the refusal and its narrowness for both entity types.
- Binary-level tests prove the same two things against a real vault and a real binary.
</summary>

<objective>
Close the generic-set bypass for goals and topics so `phase: todo → planning` can only happen through the approval verb that prompt 2 adds. Mirror the task guard exactly: refuse one (key, value, current-phase) triple, refuse nothing else, and write nothing on the refusal path.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully, in this order:
- `docs/task-writing.md` § Phase transitions — the approval contract being mirrored. Read it rather than re-deriving the rules from code.
- `pkg/ops/frontmatter.go` — `todoPlanningSetRefusal` and its doc comment, plus where `frontmatterSetOperation.Execute` calls it. This is the exemplar: same shape, same narrowness, same pre-mutation placement.
- `pkg/ops/frontmatter_entity.go` — `goalSetOperation.Execute` and `topicSetOperation.Execute`. These are the two call sites you change.
- `pkg/domain/goal_frontmatter.go` — `Phase() *GoalPhase` (returns nil when the key is absent) and `setPhaseField`.
- `pkg/domain/topic_frontmatter.go` — `Phase() *TopicPhase` and `setPhaseField`.
- `pkg/domain/goal_phase.go` and `pkg/domain/topic_phase.go` — `GoalPhasePlanning`, `GoalPhaseTodo`, `TopicPhasePlanning`, `TopicPhaseTodo`. Goal and topic phases have no aliases and no normalizer; the canonical strings are the constants.
- `pkg/ops/frontmatter_entity_test.go` — the existing `Describe("NewGoalSetOperation", …)` and `Describe("NewTopicSetOperation", …)` blocks. Add your unit cases inside them.
- `integration/cli_test.go` — `createTempVaultWithGoals(tasks, goals)`, `createTempVaultWithTopics(topicsDir)`, and one existing topic spec that creates `<vault>/Topics/` itself and writes a page into it. Note that the storage writer re-serializes the whole frontmatter in alphabetical key order, so author every fixture's keys alphabetically.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Errorf` / `errors.Wrap` from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, and `gexec` session assertions for the binary-level specs.
</context>

<requirements>
1. **Add the two guard functions** in `pkg/ops/frontmatter_entity.go`, beside the set operations they guard:

   ```go
   func goalTodoPlanningSetRefusal(
       ctx context.Context,
       goal *domain.Goal,
       goalName, key, value string,
   ) error

   func topicTodoPlanningSetRefusal(
       ctx context.Context,
       topic *domain.Topic,
       topicName, key, value string,
   ) error
   ```

   Both implement exactly this decision sequence, returning `nil` at the first non-match:
   - `key != "phase"` → `nil`.
   - The target value is not the canonical planning phase → `nil`. Compare with `domain.GoalPhase(value) != domain.GoalPhasePlanning` (topics: `domain.TopicPhase(value) != domain.TopicPhasePlanning`). **Do not call `Validate`** — an unknown phase value must keep its existing rejection from `setPhaseField`, which is a different error with different wording (spec Non-goal).
   - `goal.Phase() == nil` → `nil`. A row with no `phase` key is exempt from the gate by operator decision; do not treat an absent phase as `todo`.
   - `*goal.Phase() != domain.GoalPhaseTodo` → `nil`.
   - Otherwise return a refusal from `github.com/bborbe/errors` whose message names the row, the phase found, the phase required, and the command to run instead. Use this wording as the model (the goal arm; the topic arm is identical with `topic` substituted):

     ```go
     return errors.Errorf(ctx,
         "refusing to set phase %q on goal %q: the goal is at phase %q, the operator's approval inbox. Leaving it is the operator's approval and is performed by `vault-cli goal approve %q`, which records approved_by and approved_at in the same write",
         domain.GoalPhasePlanning, goalName, string(domain.GoalPhaseTodo), goalName,
     )
     ```

     The literal `vault-cli goal approve` / `vault-cli topic approve` must appear in the message — acceptance criterion 3 asserts it on stderr.

2. **Call each guard before any mutation.**
   - In `goalSetOperation.Execute`: immediately after `FindGoalByName` succeeds and after the existing `blockedBySetRefusal` call, **before** `writeGoalCloseOutFieldsIfCloseOut` and before `goal.SetField`. Return the error unchanged (no extra wrapping — the message must stay readable and must still contain the approve verb).
   - In `topicSetOperation.Execute`: immediately after `FindTopicByName` succeeds, before `topic.SetField`.
   - Nothing below the guard runs on the refusal path, so the file on disk is byte-identical. Do not add a `--force` bypass and do not consult `reason`, `gateSuccessor`, or status.

3. **Unit tests** in `pkg/ops/frontmatter_entity_test.go`, inside the existing set-operation Describes. Each case asserts both the error and `WriteGoalCallCount()` / `WriteTopicCallCount()`.

   Goal cases:
   - goal at `phase: todo`, `set phase planning` → error matching `ContainSubstring("goal approve")`, `WriteGoalCallCount() == 0`, and the in-memory goal's `Phase()` is still `todo`.
   - goal at `phase: todo`, `set phase execution` → no error, one write, written goal's `Phase()` is `execution`. This is the narrowness half of acceptance criterion 3.
   - goal at `phase: todo`, `set priority 2` → no error, one write, written goal's `Priority()` is `2`.
   - goal with **no** `phase` key, `set phase planning` → no error. This pins the deliberate exemption (spec Non-goals: the accepted hole) so a later reader does not "fix" it into a refusal.
   - goal at `phase: planning`, `set phase planning` → no error (only `todo → planning` is guarded).

   Topic cases (same five, with `topic` and `ContainSubstring("topic approve")`). A topic has no `Priority()` accessor — for the non-phase case assert the written topic's `GetField("priority")` is `"2"`.

4. **Binary-level specs** in `integration/cli_test.go`. Add one new `Describe("goal and topic set phase guard", …)` inside the suite's outer `Describe`, beside the other mutation-verb blocks. Declare its own `runEntityCommand` / `sha256OfFile` / `readFile` helpers locally — the sibling blocks' helpers are closure-scoped and not visible.

   Fixtures:
   - goal: `createTempVaultWithGoals(nil, map[string]string{"Alpha": "<alphabetical frontmatter>"})`, with `phase: todo` and `status: next`.
   - topic: `createTempVaultWithTopics("Topics")`, then `os.MkdirAll(filepath.Join(vaultPath, "Topics"), 0755)` and write `Alpha.md` with the same shape.
   - Author every fixture's keys alphabetically (`page_type`, `phase`, `priority`, `status`).

   Cover:
   - goal at `phase: todo`: `goal set Alpha phase planning` exits non-zero, stderr contains `vault-cli goal approve`, and the file's sha256 taken immediately before and after that one invocation is unchanged.
   - topic at `phase: todo`: `topic set Alpha phase planning` exits non-zero, stderr contains `vault-cli topic approve`, sha256 unchanged.
   - goal at `phase: todo`: `goal set Alpha phase execution` exits 0 and `goal get Alpha phase` prints `execution`.
   - goal at `phase: todo`: `goal set Alpha priority 2` exits 0 and `goal get Alpha priority` prints `2`.

   Use `Eventually(session).Should(gexec.Exit(1))` for the refusals (the neighbouring mutation specs pin exit 1) and `gexec.Exit(0)` for the allowed cases.

5. **Self-check before finishing.** Re-run the `<verification>` block and confirm every row; walk acceptance criterion 3 of spec 061 (both halves — the refusal and the narrowness) against the change.
</requirements>

<constraints>
- The approval contract being mirrored is `docs/task-writing.md` § Phase transitions — read it rather than re-deriving the rules.
- The guard mirrors `todoPlanningSetRefusal` in `pkg/ops/frontmatter.go`. Do **not** change that function, `checkPhaseRegression`, `frontmatterSetOperation`, or `pkg/ops/task_approve.go`.
- Do not change the frontmatter serializers (`serializeMapAsFrontmatter` and friends) or the `setPhaseField` validators.
- The guard is deliberately one (field, value, current-phase) triple: `phase`, `planning`, current `todo`. A missing phase, an unknown phase, every non-phase key, and every other target phase keep their existing behaviour.
- An accepted hole is carried forward from the task contract: a row with **no** `phase` key can still reach planning via `set`. Do not close it — closing it would treat an absent phase as `todo`, which contradicts the operator's decision that an absent phase means exempt.
- No override exists and none is added: `goal set` / `topic set` carry no `--force` flag, and this prompt does not add one.
- Reuse the existing storage interfaces and find-by-name methods; add no new storage method.
- No backfilling of `phase` on existing goals or topics.
- `docs/` and `CHANGELOG.md` are out of scope — prompt 5 owns them. Do not add the `## Unreleased` bullet here.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass. The integration suite's `topic defer writes defer_date for a relative and an absolute date` spec is a known pre-existing failure for the hours when the local date leads the UTC date; do not "fix" it and do not let it block this prompt.
</constraints>

<verification>
EVIDENCE GREPS — run each and confirm the count. Absence assertions use `! grep -q` because `grep -c` exits 1 when the count is 0.

```
grep -c 'func goalTodoPlanningSetRefusal' pkg/ops/frontmatter_entity.go     # 1
grep -c 'func topicTodoPlanningSetRefusal' pkg/ops/frontmatter_entity.go    # 1
grep -c 'vault-cli goal approve' pkg/ops/frontmatter_entity.go              # >= 1
grep -c 'vault-cli topic approve' pkg/ops/frontmatter_entity.go             # >= 1
grep -c 'goal and topic set phase guard' integration/cli_test.go            # 1
grep -c 'vault-cli goal approve' integration/cli_test.go                    # >= 1
grep -c 'vault-cli topic approve' integration/cli_test.go                   # >= 1
grep -c 'Execute(ctx context.Context, vaultPath, entityName, key, value, reason, gateSuccessor string) error' pkg/ops/frontmatter_entity.go   # >= 1 — the frozen set signature is untouched
```

FROZEN-NEIGHBOUR GUARD — the container has no usable `.git` (the daemon runs with `hideGit=true`), so this cannot be checked with git. The `<constraints>` forbid touching these files, and `make precommit` catches a broken neighbour. If a hard guard is wanted, record `shasum -a 256` of the four files at authoring time and compare:

```
sha256sum pkg/ops/frontmatter.go pkg/ops/task_approve.go pkg/domain/goal_frontmatter.go pkg/domain/topic_frontmatter.go > /tmp/frozen.sha256   # BEFORE editing
sha256sum -c /tmp/frozen.sha256                                                                                                              # after — every row must print OK
```

TESTS:

```
go build ./...
go test ./pkg/ops/... -ginkgo.focus="SetOperation" -ginkgo.v
go test ./integration/... -ginkgo.focus="set phase guard" -ginkgo.v
make precommit
```

⚠️ `-ginkgo.focus` with zero matching specs exits 0 while running nothing. After each focus run, confirm the new spec names appear in the output; a log without them is a failure, not a pass.

SYNTAX:

```
gofmt -e -l pkg/ops/frontmatter_entity.go pkg/ops/frontmatter_entity_test.go integration/cli_test.go
```

`make precommit` must exit 0 apart from the named pre-existing `topic defer` integration failure. `make precommit` runs `generate` (which re-creates `mocks/`) and then `addlicense` (which restores the copyright header on every `.go` file, including `mocks/mocks.go`). Do not use git to restore anything — `.git` is masked in the container.
</verification>
