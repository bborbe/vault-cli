---
status: completed
spec: [062-goal-topic-phase-set]
summary: Widened goal/topic phase enums to the six canonical task phases and derived the valid-value list in validator errors from the enum collections
execution_id: vault-cli-goal-topic-phase-set-exec-261-spec-062-phase-enum
dark-factory-version: v0.198.7
created: "2026-10-20T10:50:00Z"
queued: "2026-10-20T10:51:07Z"
started: "2026-10-20T10:52:53Z"
completed: "2026-10-20T10:57:59Z"
---

# Widen the goal and topic phase enums and derive the valid-value list in the error (spec 062, prompt 1 of 2)

<summary>
- A goal or topic accepts exactly the six canonical task phases (`todo`, `planning`, `execution`, `ai_review`, `human_review`, `done`).
- An invalid phase produces an error that names the value and lists the accepted set.
- The accepted set is rendered from the enum collection itself, so it cannot drift when a phase is added.
- `vault-cli goal set … phase bogus` now prints the validator's message instead of a locally rebuilt one.
- `in_progress` stays rejected by goal and topic validation.
- Unit tests lock the enum equality, the valid/invalid values, and the derived message.
</summary>

<objective>
A goal or topic accepts the same six phases a task does, and an invalid phase is refused with a message that lists every accepted value, derived from the enum itself so the list cannot drift.
</objective>

<context>
Read `CLAUDE.md` for project conventions and `docs/development-patterns.md` for the entity layout.
Read these files before editing:
- `pkg/domain/task_phase.go` — the canonical six-value set (`AvailableTaskPhases`); `in_progress` is an alias, not a member.
- `pkg/domain/goal_phase.go` and `pkg/domain/topic_phase.go` — the two four-value enums to widen.
- `pkg/domain/goal_frontmatter.go` (`setPhaseField`, ~line 371) and `pkg/domain/topic_frontmatter.go` (`setPhaseField`, ~line 82).
- `specs/completed/052-topic-phase-field.md` — the constraint that the canonical set is defined once and the error is the validator's own wording.
</context>

<requirements>
1. In `pkg/domain/goal_phase.go`, add the two constants after `GoalPhaseExecution` and before `GoalPhaseDone`, keeping the existing doc-comment style:
   ```go
   // GoalPhaseAIReview means automated checks are running.
   GoalPhaseAIReview GoalPhase = "ai_review"
   // GoalPhaseHumanReview means manual review is required.
   GoalPhaseHumanReview GoalPhase = "human_review"
   ```
   Add both to `AvailableGoalPhases` in declaration order: `todo, planning, execution, ai_review, human_review, done`.

2. In `pkg/domain/topic_phase.go`, add the same two constants (`TopicPhaseAIReview`, `TopicPhaseHumanReview`) and add both to `AvailableTopicPhases` in the same order.

3. Add a `String() string` method on the **plural** collection types `GoalPhases` and `TopicPhases` that renders the values comma-separated with a space (`todo, planning, execution, ai_review, human_review, done`), in declaration order. Use `strings.Join` over a preallocated slice — `parts := make([]string, 0, len(g))`, then `parts = append(parts, string(p))` in a `range` loop (the `prealloc` linter flags an unpreallocated append slice); add the `strings` import where missing. Do not rename or change the existing singular `String()` methods.

4. Change each `Validate` error to include the accepted set, formatted from the collection:
   ```go
   return errors.Wrapf(ctx, validation.Error,
       "unknown goal phase '%s' (valid: %s)", g, AvailableGoalPhases)
   ```
   and the topic equivalent. Keep the membership check `if !AvailableXs.Contains(v)` — no switch.

5. In `pkg/domain/goal_frontmatter.go` `setPhaseField`, replace
   ```go
   return errors.Wrapf(ctx, validation.Error, "unknown goal phase '%s'", value)
   ```
   with
   ```go
   return err
   ```
   so the validator's message (including the valid set) reaches the CLI. Keep the empty-value branch that clears the key. The `errors` and `validation` imports stay — both are used elsewhere in the file.

6. Leave `pkg/domain/topic_frontmatter.go` `setPhaseField` unchanged: it already returns `errors.Wrap(ctx, err, "invalid topic phase")`, which now carries the valid set because the validator does.

7. Update the unit tests:
   - `pkg/domain/goal_phase_test.go`: add `ai_review` and `human_review` entries to the valid `DescribeTable`; add `ai_review` and `human_review` `Contains` assertions to the existing `AvailableGoalPhases.Contains` block; add a test asserting `AvailableGoalPhases` equals `AvailableTaskPhases` element-for-element (length 6, same order) by comparing `string(AvailableGoalPhases[i])` to `string(AvailableTaskPhases[i])` — a direct Gomega `Equal` compares distinct types (`GoalPhases` vs `TaskPhases`) and fails even when the values match; add a test asserting `AvailableGoalPhases.String()` equals `todo, planning, execution, ai_review, human_review, done`; add a test asserting `GoalPhase("bogus").Validate(ctx)`'s error contains the string built by iterating `AvailableGoalPhases` and joining with `", "`. Keep the `in_progress` rejection test.
   - `pkg/domain/topic_phase_test.go`: the same additions (`DescribeTable` entries, `Contains` assertions, element-wise equality against `AvailableTaskPhases`, the `String()` contract, the derived-message test). **Invert** the existing `It("returns an error for the task-only phase ai_review")` — `ai_review` is now valid, so move it into the valid table and delete that rejection case. Keep the `in_progress` rejection test.
   - `pkg/domain/goal_frontmatter_test.go`: add `ai_review` and `human_review` entries to the `SetField phase` / "accepts canonical values" table.
   - `pkg/domain/topic_frontmatter_test.go`: the same table additions.
   - `pkg/ops/frontmatter_entity_test.go`: add `ai_review` and `human_review` entries to the topic "setting each canonical phase value" `DescribeTableSubtree`.

8. Before finishing, re-run the `<verification>` block and walk each requirement above against the diff.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT change task phase validation, statuses, lint rules, or the `show` / `get` / `list` read paths.
- Do NOT accept `in_progress` in either goal or topic validation — it stays an alias-only value.
- Do NOT duplicate the six-value list as a literal anywhere in `pkg/domain/`; the message must format the collection.
- Existing tests must still pass after the `ai_review` rejection case is inverted.
- Follow the enum pattern already implemented in `pkg/domain/task_phase.go` (constants → `AvailableXs` collection → `Contains` → `Validate`); the generic guide is `~/.claude-yolo/plugins/marketplaces/coding/docs/go-enum-type-pattern.md`.
</constraints>

<verification>
Run `make precommit` — must pass.

Then:
- `go test ./pkg/domain/... ./pkg/ops/...` — must pass.
- `! grep -rq 'todo, planning' pkg/domain/` — must succeed (no inline literal list in the domain package).
- `! grep -q 'unknown goal phase' pkg/domain/goal_frontmatter.go` — must succeed (the local rebuild is gone).
- `grep -n 'ai_review' pkg/domain/goal_phase.go pkg/domain/topic_phase.go` — must return matches in both files, including the constant declaration and the `Available` collection entry.
</verification>
