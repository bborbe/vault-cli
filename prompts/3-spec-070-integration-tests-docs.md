---
spec: ["070-parent-closure-cascade-on-complete"]
status: draft
created: "2026-11-19T18:57:00Z"
---

# Prove the cascade end to end, and document the goal→topic relation

<summary>
- The whole task → goal → topic chain closes in a single `task complete` run, proven against the built binary.
- A refusing goal or topic is reported on stdout and left open while the child still exits 0.
- Two parent goals are attempted in `goals:` order.
- An already-completed goal still closes its topic on a later run.
- `--output json` carries a `parents` array whose completed entries omit `open_items`.
- A task with no `goals:` and a goal with no `topics:` print no `parent …` line and omit `parents` from JSON.
- Recurring tasks never cascade.
- `[[Name]]`, `[[Name|label]]` and `[[Name#Heading]]` all resolve to the same parent.
- `docs/goal-writing.md` documents the `topics:` field and the cascade, and the changelog records the feature.
</summary>

<objective>
Add end-to-end integration tests for the cascade against the compiled `vault-cli` binary, document the goal `topics:` relation and the cascade in `docs/goal-writing.md`, and record the change in the changelog. This is the third and final prompt; it depends on prompts 1 and 2 being implemented.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read these files fully before changing anything:

- `specs/in-progress/070-parent-closure-cascade-on-complete.md` — Acceptance Criteria 1-12, Desired Behavior, and Failure Modes. Every Post-Deploy AC maps to an integration case below.
- `integration/cli_test.go` — `createTempVaultWithGoals(tasks, goals)`, `createTempVaultWithTopicPages(topicsDir, topics, goals)`, and the local `runCommand`/`readFile` closure style used throughout. `createTempVaultWithTopicPages` creates a `Tasks` directory but writes no tasks; write task files directly into `filepath.Join(vaultPath, "Tasks", name+".md")` after calling it.
- `integration/integration_suite_test.go` — `binPath` is built once by `gexec.Build`.
- `pkg/ops/closure_gate.go` — the closure rules that decide whether a scratch goal or topic closes: goal needs `# Success Criteria` + `# Definition of Done` all `[x]`; topic needs `# Success Criteria` + `# Completion Gate` all `[x]` with no non-checkbox list items, and every member goal under its `Goals` heading terminal.
- `pkg/ops/goal_complete.go` — the goal closure gate also treats every task whose `goals:` names the goal as a member; a task at `next`/`todo`/`in_progress` blocks the goal.
- `pkg/ops/topic_complete.go` and `parseGoalSectionMembers` — a topic's member goals come from its `Goals` heading (`# Goals` or `## Goals`, level-relative bound), not from the goal's `topics:` field.
- `docs/goal-writing.md` — § Frontmatter (the YAML example and the `blocked_by` subsection are the anchors).
- `CHANGELOG.md` — the `## Unreleased` section at the top.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, `gexec`, `Eventually`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry style.
</context>

<requirements>
## 1. Fixture shapes

Use these exact page bodies so the closure gates actually close.

Complete goal (closes):
```
---
status: in_progress
created: "2026-11-01"
topics:
  - "[[Scratch Topic]]"
---
# Scratch Goal
# Success Criteria
- [x] done
# Definition of Done
- [x] shipped
```
Drop the `topics:` block for cases that must not cascade.

Failing goal (refuses): same shape with `- [ ] an open criterion` under `# Success Criteria`.

Complete topic (closes). The `## Goals` heading must sit **before** `# Completion Gate`, because `# Completion Gate`'s body runs to the next level-1 heading and a non-checkbox list item inside it would make the gate unmeasurable:
```
---
status: in_progress
---
# Scratch Topic
# Success Criteria
- [x] done
## Goals
- [[Scratch Goal]]
# Completion Gate
- [x] gated
```
Failing topic: `- [ ] an open gate box` under `# Success Criteria`.

Complete task (closes):
```
---
status: next
page_type: task
task_identifier: 11111111-1111-4111-a111-111111111111
goals:
  - "[[Scratch Goal]]"
---
# Scratch Task
- [x] done
```
Drop the `goals:` block for the no-cascade case. Add `recurring: 1d` for the recurring case.

## 2. Integration tests in `integration/cli_test.go`

Add a new `Describe("vault-cli parent closure cascade", ...)` block near the existing goal/topic completion describes. Each `It` gets its own temp vault, its own local `runCommand(args ...string) *gexec.Session` closure (built with `exec.Command(binPath, append([]string{"--config", configPath}, args...)...)` and `gexec.Start`), and its own local `readFile` closure. Do not add package-level helpers.

Required cases, each naming the spec AC it proves:

1. **Task → goal** — `createTempVaultWithGoals` with the complete task and complete goal (no topics). Run `task complete "Scratch Task"`. Assert exit 0, stdout contains `parent Scratch Goal: completed`, and the goal file reads `status: completed`.
2. **Goal → topic** — `createTempVaultWithTopicPages("Topics", {"Scratch Topic": complete topic}, {"Scratch Goal": complete goal with `topics:`})`. Run `goal complete "Scratch Goal"`. Assert exit 0, stdout contains `parent Scratch Topic: completed`, and the topic reads `status: completed`.
3. **Failing goal** — complete task, failing goal. Run `task complete "Scratch Task"`. Assert exit 0, the task reads `status: completed`, the goal still reads `status: in_progress`, and stdout contains `parent Scratch Goal: refused — open: an open criterion`.
4. **Failing topic** — complete goal with `topics:` pointing at a failing topic (and the topic's `## Goals` listing the goal). Run `goal complete "Scratch Goal"`. Assert exit 0, the goal reads `status: completed`, the topic still reads `status: in_progress`, and stdout contains `parent Scratch Topic: refused — open: an open gate box`.
5. **Two goals in order** — task `goals:` listing a complete goal then a failing goal. Run `task complete`. Assert exit 0 and exactly two lines match `^parent `; the first is `parent Goal A: completed` and the second is `parent Goal B: refused — open: …`. Count lines with a regexp over `strings.Split`.
6. **Already-completed goal still cascades** — goal file with `status: completed` and `topics:` pointing at a complete topic; task `goals:` names that goal. Run `task complete`. Assert exit 0, stdout contains both `parent Scratch Goal: completed` and `parent Scratch Topic: completed`, the goal still reads `status: completed`, and the topic reads `status: completed`.
7. **No parents, no lines** — task with no `goals:` and goal with no `topics:`. Run `task complete` and `goal complete`. Assert exit 0, stdout has no line matching `^parent `, and `--output json` output has no `parents` key (unmarshal into `map[string]any`, assert `_, ok := m["parents"]; Expect(ok).To(BeFalse())`).
8. **End to end** — complete task → complete goal → complete topic, wired on all three sides (task `goals:`, goal `topics:`, topic `## Goals`). One `task complete "Scratch Task"`. Assert exit 0 and the task, goal and topic all read `status: completed`.
9. **JSON shape** — run `task complete --output json` against the two-goals fixture (one completed, one refused). Unmarshal into `map[string]any`; assert `parents` is a `[]any` of length 2, each entry has `name` and `result`, the completed entry has no `open_items` key, and the refused entry's `open_items` is a non-empty `[]any`. Repeat once for `goal complete --output json`.
10. **Wikilink forms** — two `It`s, one with `goals: ["[[Scratch Goal|label]]"]` and one with `goals: ["[[Scratch Goal#Heading]]"]`, each against a complete goal. Assert exit 0, stdout contains `parent Scratch Goal: completed`, and the goal reads `status: completed`.
11. **Recurring task never cascades** — task with `recurring: 1d`, `status: in_progress`, and `goals:` naming a complete goal. Run `task complete`. Assert exit 0, stdout has no line matching `^parent `, the task keeps `status: in_progress`, and the goal still reads `status: in_progress`.

For JSON assertions use `encoding/json` unmarshalling in the test file; do not depend on `jq` being installed.

## 3. Document `topics:` in `docs/goal-writing.md`

1. Add an optional `topics:` entry to the frontmatter YAML example in § Frontmatter, near `blocked_by`:
   ```yaml
   topics:                                          # optional — the topics this goal rolls up into
     - "[[Parent Topic]]"
   ```
2. Add a `### Topics (\`topics:\`)` subsection after `### Dependencies (\`blocked_by\`)` covering:
   - the field is a YAML list of topic names; each entry may be a plain name or a wikilink, and `[[Name]]`, `[[Name|label]]` and `[[Name#Heading]]` resolve to the same topic;
   - a scalar value (`topics: Parent Topic`) is read as a comma-split list and typically refuses at close time — write a list;
   - `vault-cli goal complete` attempts each topic in document order after the goal closes; a topic that refuses is reported on stdout as `parent <topic>: refused — open: <item>` and left open, and the goal still exits 0;
   - the cascade reaches one level only: `vault-cli task complete` closes a task, then its goals, then those goals' topics, and never continues above topic level;
   - a goal that is already `completed` still attempts its topics, so a topic can close on a later run.

Keep the prose consistent with `docs/task-writing.md`'s description of the task `goals:` relation.

## 4. Changelog

Add one bullet under the existing `## Unreleased` heading in `CHANGELOG.md` (do not create a second `## Unreleased`):

```
- feat: completing a task also attempts its parent goals and, through them, their topics; each attempt prints `parent <name>: completed` or `parent <name>: refused — open: …` and appears in `--output json` under `parents`, and a refusal never fails the child
```

Do not bump any version manifest and do not tag.
</requirements>

<constraints>
- **The cascade only mutates markdown pages in the same vault**, under the same permissions the single-verb close already uses. Do not add network access, credentials, or a new writable target.
- **A parent refusal never changes the child's outcome or exit code.** Every integration case asserts the child's exit 0 and terminal state.
- **The closure rule is not duplicated**: `checkClosureGate` in `pkg/ops/closure_gate.go` remains the single implementation.
- **`pkg/ops` stays a library layer**: it never writes to stdout; all printing lives in `pkg/cli`.
- **No cascade above topic level.** Do not add theme or objective attempts.
- **No retroactive closure.** The cascade fires only on a new complete; do not add a migration or backfill.
- **Do not hand-bump version manifests or tag.** The releaser owns the tag post-merge.
- **Do not relax or delete any existing integration assertion.** Add new cases alongside the existing ones.
- **Test names must appear in the `-ginkgo.v` output**; a focus that matches nothing exits 0 while running nothing.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make test` — must exit 0.

Focused integration run (confirm every new case name appears; `-ginkgo.v` is the reporter that prints names):

```
go test -mod=mod ./integration -run TestIntegration -ginkgo.focus='parent closure cascade' -ginkgo.v
```

The output must list all eleven case names from requirement 2. Then run the full integration suite:

```
go test -mod=mod ./integration -run TestIntegration -v
```

Single-write-path and closure-rule invariants (each count is exact):

```
grep -c 'WriteGoal' pkg/ops/complete.go                     # 0
grep -c 'WriteTopic' pkg/ops/goal_complete.go               # 0
grep -c 'func checkClosureGate' pkg/ops/closure_gate.go     # 1
grep -n 'checkClosureGate' pkg/ops/goal_complete.go pkg/ops/topic_complete.go  # both files appear
grep -c '^## Unreleased$' CHANGELOG.md                      # 1
grep -c 'topics:' docs/goal-writing.md                      # at least 1
```

Finally run `make precommit` once — must exit 0. If it exits non-zero, fix and re-run the failing target, then `make precommit` again. A non-zero `make precommit` is a failure, never rationalised as success.

<!--
Reviewer note — AC 12's "single close sequence per kind" is enforced structurally by
the two `grep -c` counts above and behaviourally by the unit tests added in prompts 1
and 2 (`CompleteOperation` holds no goal storage; `GoalCompleteOperation` holds no
topic storage). The integration cases here prove the user-visible chain.
-->
</verification>
