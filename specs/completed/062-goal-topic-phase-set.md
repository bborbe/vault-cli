---
status: completed
approved: "2026-10-20T10:46:01Z"
generating: "2026-10-20T10:51:08Z"
prompted: "2026-10-20T10:51:08Z"
verifying: "2026-10-20T11:00:34Z"
completed: "2026-10-20T12:00:07Z"
branch: dark-factory/goal-topic-phase-set
---

## Summary

- Goal and topic phase validation currently accepts four values (`todo`, `planning`, `execution`, `done`) and rejects the two review phases a task accepts (`ai_review`, `human_review`).
- The rejection message names only the invalid value; it never lists the accepted set, so a caller cannot learn the valid values from the error.
- This spec makes both enums equal the six canonical task phases and derives the "valid values" list in the error from the enum itself, so the message can never drift from the set.
- It reverses the four-value decision recorded in `specs/completed/024-goal-phase-field.md` and `specs/completed/052-topic-phase-field.md`, both of which said extending the enum is a separate spec — this is that spec.

## Problem

`vault-cli goal set <goal> phase ai_review` and the topic equivalent fail, while `vault-cli task set <task> phase ai_review` succeeds. The parent goal `Topics and Goals Run on the Same Phase Lifecycle and Manager Tree as Tasks` requires goals and topics to accept exactly the task phase set, so the manager lifecycle can advance a goal or topic through the same review stages a task uses. Today the mismatch blocks that lifecycle and the error does not tell the caller what values are accepted: `goal set <g> phase bogus` prints `unknown goal phase 'bogus': validation error`, exit 1.

## Goal

A goal or topic accepts exactly the six canonical task phases (`todo`, `planning`, `execution`, `ai_review`, `human_review`, `done`), and an invalid phase exits non-zero with a message that lists all six values, rendered from the same enum the validator checks.

## Assumptions

- The six-value set is `AvailableTaskPhases` in declaration order.
- `in_progress` stays a task-only read alias (`NormalizeTaskPhase` → `execution`), not a canonical value; goal/topic validation keeps rejecting it.
- The "extending the enum is a separate spec" deferral in `specs/completed/024-goal-phase-field.md` and `052-topic-phase-field.md` is discharged by this spec.
- Topic `setPhaseField` already wraps the validator's error; goal `setPhaseField` does not and must change.
- `detectStatusPhaseMismatch` already treats `ai_review` and `human_review` as active phases, so no lint change is needed.
- `pkg/domain/topic_phase_test.go` currently asserts "returns an error for the task-only phase `ai_review`"; that case is inverted by this change, and the valid-value `DescribeTable` lists in `goal_phase_test.go` and `topic_phase_test.go` gain `ai_review` and `human_review`.

## Non-goals

- Accepting `in_progress` as a goal/topic alias.
- The `goal approve` / `topic approve` verbs and the `todo → planning` set-path guard (owned by the vault task `Goals and Topics Carry a Phase Field Gating Manager Start`).
- Manager-start gating on the phase field.
- Backfilling `phase` on existing goal or topic pages.
- Reconciling the vault's `Goal Lifecycle Guide`, which restates the four-value decision — handled as a follow-up after release, since the spec cannot edit the vault.

## Acceptance Criteria

- [ ] `AvailableGoalPhases` and `AvailableTopicPhases` each equal `AvailableTaskPhases` value-for-value, in declaration order. Evidence: a table assertion in `pkg/domain/goal_phase_test.go` and `topic_phase_test.go` compares each collection to `AvailableTaskPhases` element-by-element and asserts length 6; `make test` exits 0.
- [ ] `vault-cli goal set <goal> phase ai_review` and `phase human_review` exit 0 and write the value to disk. Evidence: integration test in `integration/cli_test.go` against a temp vault — exit 0, the file contains `phase: ai_review` (resp. `phase: human_review`), and `goal show --output json` returns the same value at `.fields.phase`.
- [ ] `vault-cli topic set <topic> phase ai_review` and `phase human_review` exit 0 and write the value to disk. Evidence: the same integration shape against a temp topic — exit 0, `phase: ai_review` on disk, `topic show --output json` returns it.
- [ ] `vault-cli goal set <goal> phase bogus` exits 1 and stderr contains `unknown goal phase 'bogus'` followed by all six values `todo, planning, execution, ai_review, human_review, done`. Evidence: integration test asserts exit 1 and the stderr substring; the page is byte-identical (sha256) before and after.
- [ ] `vault-cli topic set <topic> phase bogus` exits 1 and stderr contains `unknown topic phase 'bogus'` followed by the same six values. Evidence: integration test asserts exit 1 and the stderr substring; the page is byte-identical before and after.
- [ ] `in_progress` stays rejected by goal and topic validation. Evidence: unit tests assert `GoalPhase("in_progress").Validate` and `TopicPhase("in_progress").Validate` return an error, and integration `goal set … phase in_progress` / `topic set … phase in_progress` exit 1.
- [ ] The valid-value list is derived from the enum, not a second literal list. Evidence: `! grep -rq 'todo, planning' pkg/domain/` succeeds (no inline literal anywhere in the domain package); `! grep -q 'unknown goal phase' pkg/domain/goal_frontmatter.go` succeeds (the local rebuild is gone, not merely supplemented); and a unit test asserts `GoalPhase.Validate(ctx)`'s error contains a list built by iterating `AvailableGoalPhases` (topic equivalent likewise), so the test fails if the enum grows and the message does not.
- [ ] `CHANGELOG.md` carries one `## Unreleased` bullet describing the widened enum and the listed-valid-values error. Evidence: `sed -n '/^## Unreleased/,/^## v/p' CHANGELOG.md | grep -q ai_review` succeeds.

## Verification

Scenario coverage: **NO new scenario** — domain unit tests plus integration tests against a temp vault reach the behavior; no Docker, cluster, or external system is involved.

### Container-executable (runs inside the YOLO container at prompt time)

- `ROOTDIR=/workspace make precommit` — format, generate, test, lint, vet, vulncheck, license, changelog structure all pass
- `go test ./pkg/domain/... ./integration/...` — the enum, frontmatter and CLI integration specs pass
- `! grep -rq 'todo, planning' pkg/domain/` — no inline valid-value literal in the domain package
- `! grep -q 'unknown goal phase' pkg/domain/goal_frontmatter.go` — the goal setter no longer rebuilds the message
- `sed -n '/^## Unreleased/,/^## v/p' CHANGELOG.md | grep -q ai_review` — the new bullet sits under `## Unreleased`

### Operator-executable (runs on the host after PR merge)

- `gh pr view <n> --repo bborbe/vault-cli --json state,mergeCommit` — merged
- `git tag --contains <merge-sha>` — lists the release tag `vX.Y.Z`
- `make install` after the release gate in `docs/releasing-vault-cli.md`, then `$(command -v vault-cli) --version` — prints that same `vX.Y.Z`
- `vault-cli goal set <scratch-goal> phase ai_review` and `phase bogus` — exit 0 and exit 1 respectively, the latter naming all six values

## Desired Behavior

1. Goal and topic phase validation accepts exactly the six canonical task phases (`todo`, `planning`, `execution`, `ai_review`, `human_review`, `done`); `in_progress` stays rejected.
2. An invalid phase produces an error naming the value and the accepted set, and the set is rendered by iterating `AvailableGoalPhases` / `AvailableTopicPhases` — no second literal list of phase strings exists in the domain package.
3. `vault-cli goal set` and `vault-cli topic set` surface that validator error, so the CLI prints the accepted set for both verbs (the goal setter stops discarding the validator's error; the topic setter keeps wrapping it).
4. A goal or topic carrying `phase: ai_review` / `phase: human_review` is written to disk and read back unchanged by `show` / `get`.

## Constraints

- The canonical set is defined once per phase type, as `AvailableGoalPhases` / `AvailableTopicPhases`; the error text must derive from it. A duplicated literal list is the defect this spec exists to avoid. The two collections stay distinct typed collections for type safety; their equality to `AvailableTaskPhases` is enforced by test rather than by aliasing.
- `Validate` stays a membership check against the collection (`if !AvailableXs.Contains(v)`), per the enum pattern in `coding/docs/go-enum-type-pattern.md`.
- The error keeps the `unknown goal phase '<value>'` / `unknown topic phase '<value>'` substring the existing tests and the measured probe rely on (a substring, not a prefix: the goal path now wraps the validator error).
- `in_progress` stays an alias-only value: `AvailableTaskPhases` excludes it and goal/topic validation must not add it.
- No behavior change for tasks, statuses, lint rules, or the `show` / `get` / `list` read paths. In particular, a goal or topic at `phase: ai_review` / `phase: human_review` with `status: in_progress` stays free of `STATUS_PHASE_MISMATCH` — `detectStatusPhaseMismatch` already treats both as active phases, so this is a regression guard, not new work.
- `CHANGELOG.md` gains `## Unreleased` above the newest `## vX.Y.Z`, below the preamble.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| A caller passes `in_progress` to `goal set` / `topic set` | Exit 1 with `unknown goal phase 'in_progress' (valid: …)` | Caller uses `execution`, the canonical value |
| A legacy page hand-typed to `phase: in_progress` is read | `show` / `get` return the raw value unchanged — the read path does no validation | A write re-validates and rejects until the page is corrected to `execution` |
| Two concurrent `goal set` writes on the same page | Last writer wins; each write is whole-file | Re-read and re-apply the intended value |
| The process is killed during a write | The page can be left truncated or partial (a whole-file rewrite is not atomic) | Restore the page from git, then re-apply the intended value |
| A future edit adds a phase to the task enum but not to the goal/topic enums | The enum-equality test fails in `make test` | Add the value to both enums; the error text updates automatically because it derives from the collection |

## Security / Abuse Cases

N/A. The only new input is a phase string on the existing `goal set` / `topic set` verbs; it is validated against a closed enum before any write, and the target page is resolved through the existing vault path, so no path traversal is introduced. No network, no secrets, no new files.

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Widen both phase enums; derive the valid-value list in `Validate`; make both `setPhaseField` surface the validator's error; unit tests for enum equality, valid/invalid values, the `in_progress` rejection, and the derived message | 1, 2, 3 | 1, 6, 7 | — |
| 2 | Integration specs for `goal set` / `topic set` with `ai_review` / `human_review` and `bogus` (exit code, stderr list, byte-identical page) plus the `## Unreleased` CHANGELOG bullet | 3, 4 | 2, 3, 4, 5, 8 | prompt 1 |

Rationale: prompt 1 establishes the enum and the message wording the integration specs assert on; prompt 2 can only pass once prompt 1 has landed.

## Do-Nothing Option

Goals and topics keep rejecting `ai_review` / `human_review`, so a goal or topic cannot enter the review stages the manager lifecycle uses, and callers who pass an invalid phase get no hint of the accepted values. The parent goal's SC1 stays unmet and every future caller re-derives the four-value set by trial and error.
