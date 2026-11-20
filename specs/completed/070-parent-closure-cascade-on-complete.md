---
status: completed
approved: "2026-11-19T14:50:14Z"
generating: "2026-11-19T19:40:53Z"
prompted: "2026-11-19T19:50:16Z"
verifying: "2026-11-19T21:55:25Z"
completed: "2026-11-20T01:15:52Z"
branch: dark-factory/parent-closure-cascade-on-complete
---

## Summary

- Completing a task also attempts to complete each goal that task declares as its parent.
- Completing a goal also attempts to complete each topic that goal declares as its parent.
- A parent that refuses does not fail the child: the child stays completed and exits 0.
- Every parent attempt is reported, in text and in `--output json`.
- The closure rule stays in one place; the cascade calls the existing complete operations.

## Problem

Today the only thing that closes a goal or a topic is a live manager session running
`vault-cli goal complete` / `vault-cli topic complete`. More than fifty goals sit
`in_progress` in the Personal vault (measured 2026-11-19) while roughly ten sessions exist
to close them, so finished work stays open until a manager happens to look. The relation already exists on disk — a task names
its goals in `goals:`, a goal names its topics in `topics:` — but nothing acts on it when
the last child closes.

## Goal

A single `vault-cli task complete` closes the whole chain it can: the task, then every
goal whose closure check now passes, then every topic whose closure check now passes.
Anything that does not pass is reported and left open, and the command still succeeds.

## Non-goals

- Changing the closure rule itself.
- Cascading above topic level (topic to theme or objective).
- A `--force` TTY guard.
- Retroactively closing the goals already open; the cascade fires only on a new complete.
- Closing the child differently: a task still closes exactly as it does today.

## Assumptions

- The closure gate shipped in `pkg/ops/closure_gate.go` and is reachable from the working
  tree; spec 069 is at `verifying`, so its code is merged but not yet formally closed.
  This spec depends on that code, not on 069's lifecycle state.
- A goal page's `topics:` frontmatter and a task page's `goals:` frontmatter are the
  authoritative parent declarations. Both hold wikilinks (`[[Name]]`), so parent names
  are normalised through the existing `normaliseGoalLink` helper before lookup.
- `topics:` is currently read by no Go code. The cascade is its first consumer, and it
  reads the field through the frontmatter map (`GetStringSlice("topics")`); a `Topics()`
  accessor mirrors the existing `Goals()` accessor on `Page`.
- A parent attempt is cheap and in-process; no queue, no background job.

## Acceptance Criteria

- [ ] **Post-Deploy (Rung-2):** a scratch goal whose Success Criteria and Definition of Done are all `[x]` reads `status: in_progress` immediately before `vault-cli task complete "<last open scratch task>"`; the command exits 0, stdout contains the line `parent <goal-name>: completed`, and the goal reads `status: completed` immediately after. Control: the failing-parent run of the third AC prints no `: completed` line. — evidence: state transition + stdout line match.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** a scratch topic whose Success Criteria and Completion Gate are all `[x]` reads `status: in_progress` immediately before `vault-cli goal complete "<last open scratch goal>"`; the command exits 0, stdout contains the line `parent <topic-name>: completed`, and the topic reads `status: completed` immediately after. Control: the failing-topic run of the fourth AC prints no `: completed` line. — evidence: state transition + stdout line match.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** when the parent's check fails (scratch goal with one open SC box), `vault-cli task complete` exits 0, the task reads `completed`, the goal still reads `in_progress`, and stdout contains the line `parent <goal-name>: refused — open: <open item>` where `<open item>` is the gate's item text. — evidence: state transition + stdout line match.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** when a topic's check fails (scratch topic with one open SC box), `vault-cli goal complete` exits 0, the goal reads `completed`, the topic still reads `in_progress`, and stdout contains the line `parent <topic-name>: refused — open: <open item>`. — evidence: state transition + stdout line match.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** a task whose `goals:` lists two goals attempts both in `goals:` order: stdout carries two `parent ` lines, one `completed` and one `refused`. — evidence: positive count (2) + stdout line match.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** completing a task whose goal already reads `status: completed` exits 0, the goal stays `completed`, stdout contains `parent <goal-name>: completed`, and the JSON entry reads `result: "completed"` and carries no `open_items` key. The same run still attempts that goal's topics: a scratch topic that is fully checked and still `in_progress` reads `completed` afterwards, and stdout carries `parent <topic-name>: completed`. — evidence: stdout line match + JSON field + state transition.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** a task with no `goals:` field prints no line matching `^parent `, and a goal with no `topics:` field prints no line matching `^parent `; `--output json` on each omits the `parents` key entirely (`jq 'has("parents")'` prints `false`). — evidence: negative evidence (`! vault-cli … | grep -q '^parent '` + `jq 'has("parents")'`).
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** end-to-end, a scratch topic holding a goal holding its last open task, all three fully checked, closes in one `vault-cli task complete "<task>"` run; task, goal and topic all read `status: completed` afterwards. — evidence: three state transitions after one command.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** `--output json` carries a `parents` array of entries `{name, result, open_items}` where `result` is `completed` or `refused`; a completed entry omits the `open_items` key (the field is `omitempty`), and a refused entry carries at least one item with the exact open item text. Holds for both `vault-cli task complete` and `vault-cli goal complete`. — evidence: JSON shape.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** a task whose `goals:` entry is written `"[[Scratch Goal|label]]"` still prints `parent Scratch Goal: completed` and the goal reads `completed`; the same holds for `"[[Scratch Goal#Heading]]"`. — evidence: stdout line match + state transition.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** `vault-cli task complete "<recurring task>"` on a task with `recurring: 1d` and a fully-checked parent goal exits 0, the task keeps its prior status, the goal stays `in_progress`, and stdout contains no line matching `^parent `. — evidence: negative evidence (`! vault-cli … | grep -q '^parent '`).
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] `make precommit` exits 0, and `pkg/ops/` holds one close sequence per kind: `grep -c 'WriteGoal' pkg/ops/complete.go` prints `0` and `grep -c 'WriteTopic' pkg/ops/goal_complete.go` prints `0`. Unit tests assert the same behaviourally: the task cascade's `GoalStorage.WriteGoal` fake is never called, and the goal cascade's `TopicStorage.WriteTopic` fake is never called (mock-call count 0). — evidence: exit code + absence assertions + mock-call count.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `ROOTDIR=/workspace make precommit` — lint, format, generate, tests, checks
- `ROOTDIR=/workspace make test` — unit + integration suite
- `grep -c 'func checkClosureGate' pkg/ops/closure_gate.go` — must print `1`
- `grep -n 'checkClosureGate' pkg/ops/goal_complete.go pkg/ops/topic_complete.go` — both files must appear

### Operator-executable (host, after PR merge / release)

- `make install` then `vault-cli --version` — shows the new tag
- The scratch-page probes above, run against the installed binary

## Desired Behavior

1. `task complete` closes the task exactly as today, then reads the task's `goals:` list and attempts each goal in document order.
2. `goal complete` closes the goal exactly as today, then reads the goal's `topics:` list and attempts each topic in document order.
3. An attempt calls the parent's existing complete operation with `force=false` and no reason or successor. The cascade never re-implements the parent's close sequence.
4. An already-terminal parent (`status: completed`, or `aborted`, which reports the same `completed` result) reports `completed` with no write, and still cascades one level up — so a topic can close on a later run even when its goal closed earlier.
5. `Success == true` records `completed`; `Success == false` records `refused` with the operation's `OpenItems` (or, when it carries none, the returned error text).
6. A refusal on any parent never changes the child's outcome and never changes the exit code: the child stays completed and the command exits 0. The cascade swallows every parent error.
7. Each attempt produces one report entry: the parent's name, `completed` or `refused`, and the open items on a refusal.
8. The text output prints one line per attempt: `parent <name>: completed` or `parent <name>: refused — open: <item>; <item>`.
9. `--output json` adds a top-level `parents` array carrying the same entries; the field is omitted when no attempt ran, and a completed entry omits `open_items`.
10. A recurring task never cascades — it never changes status, so there is nothing for a parent to close on.
11. Parent names are wikilink-normalised: `[[Name]]`, `[[Name|label]]` and `[[Name#Heading]]` resolve to the same parent.

## Constraints

- `CompleteOperation` gains a `GoalCompleteOperation` dependency; `GoalCompleteOperation` gains a `TopicCompleteOperation`. Neither regains `GoalStorage` or `TopicStorage` for writing — the deliberate removal of the goal `# Tasks` write documented in `NewCompleteOperation` stands untouched.
- The closure rule must not be duplicated: `checkClosureGate` in `pkg/ops/closure_gate.go` remains the single implementation, and the parent operations remain its only callers.
- `pkg/ops/` stays a library layer: it returns structured results and never writes to stdout. Printing lives in `pkg/cli`.
- `MutationResult` gains the `parents` field; the field is additive and omitted when empty, so existing JSON consumers keep working.
- The exact call shapes are frozen: `GoalCompleteOperation.Execute(ctx, vaultPath, goalName, vaultName, force, reason, gateSuccessor)` with `force=false`, `""`, `""`; `TopicCompleteOperation.Execute(ctx, vaultPath, topicName, vaultName)`.
- Every constructor call site is updated: `pkg/cli/cli.go` (both `NewCompleteOperation` and `NewGoalCompleteOperation`) and the two unit-test call sites in `pkg/ops/complete_test.go` and `pkg/ops/goal_complete_test.go`.
- `docs/goal-writing.md` gains a `topics:` section — the frontmatter field, its wikilink form, and the goal→topic cascade — in the same change, so the relation outlives this spec.
- Existing behaviour for tasks with no `goals:`, goals with no `topics:`, recurring tasks, and `--force` on the child is unchanged.
- `--output json` and the `MutationResult` envelope follow `docs/development-patterns.md` (§ Output Format); the closure sections and the `goals:` relation follow `docs/task-writing.md` and `docs/goal-writing.md`.
- `MutationResult.Parents` is additive and `omitempty`; a `completed` entry omits `open_items`, a `refused` entry always carries at least one item.
- Every new exported type and method carries GoDoc.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Parent goal file missing or unreadable | The attempt records `refused` with the read error as its open item; the child stays completed, exit 0 | Operator runs `vault-cli goal complete "<name>"`, which exits 0 once the page is fixed |
| Parent's gate returns UNMEASURABLE (unterminated fence, absent section) | Recorded as `refused` with the gate's message; child unaffected | Operator repairs the parent page, then `vault-cli goal complete "<name>"` exits 0 |
| Parent already completed | Reported `completed`, no write; its own parents are still attempted | None needed |
| Topic refused by its gate | Recorded `refused` with the open items; the goal stays `completed`, exit 0 | Operator runs `vault-cli topic complete "<name>"`; it exits 0 once the topic's boxes and member goals are terminal |
| Malformed `goals:`/`topics:` value (a scalar, not a list) | `GetStringSlice` comma-splits it into parent names; each is attempted and typically reports `refused`; the child still completes | Operator corrects the frontmatter and re-runs the parent close |
| Crash after the child write, before a parent write | Child is completed, parent stays open; nothing retries it (the cascade fires only on a new complete) | Operator runs `vault-cli goal complete "<name>"`; it exits 0 when the gate passes |
| Two runs race on the same parent | Both read the pre-write state and both may write; the writes set the same `status`/`completed`/`phase` fields, so the final state is stable | None needed |
| Cascade read of `goals:`/`topics:` fails | Warning recorded, child still completes and exits 0 | Operator runs the parent close by hand |

## Security / Abuse

The cascade only mutates vault markdown pages in the same vault, under the same
permissions the single-verb close already uses. It adds no network access, no
credential, and no new writable target. The blast radius is bounded by the existing
closure gate: a page closes only when its own boxes and members are terminal.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Parent outcome type + `MutationResult.parents` + cascade in `complete.go` (task → goals) | 1, 3, 4, 5, 6, 7, 9, 10, 11 | 1, 3, 5, 6, 7, 10, 11 | — |
| 2 | Cascade in `goal_complete.go` (goal → topics) + CLI printing for both hops | 2, 3, 4, 5, 6, 7, 8, 9, 11 | 2, 4, 5, 6, 7, 8 | prompt 1 (shares the outcome type) |
| 3 | Unit + integration tests, JSON schema for both verbs, the single-write-path invariant | — | 9, 12 | prompts 1, 2 |

## Do-Nothing Option

The cascade stays absent. Goals and topics keep closing only when a manager session
happens to run the verb, so finished work stays open and the parent goal's SC3
("finished work closes itself", [[Finished Work Closes Itself]]) stays unmet. The cost
is a permanent backlog of completed-but-open goals and a manager fleet that spends its
rounds closing instead of steering.
