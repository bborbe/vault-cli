---
status: approved
approved: "2026-09-28T21:58:15Z"
branch: dark-factory/task-approve-assignee
---

## Summary

- `vault-cli task approve` moves a task from `todo` to `planning` and records `approved_by` / `approved_at`, but never touches `assignee`.
- A task can therefore enter planning and execution with nobody owning it: 1,322 of the 8,236 task files in the operator's `25 Tasks/` carry no non-empty `assignee` (measured 2026-09-28 — 1,003 with no key, 319 blank).
- Approval becomes the moment an owner is fixed: fill `assignee` from `vault-cli config current-user` when it is empty, keep it when it is already set, refuse when neither an existing value nor a resolvable current user exists.
- A new `--assignee <name>` flag sets the owner explicitly, overriding both the empty and the already-set case.
- The planning → execution gate warns — never blocks — when `assignee` is empty, and clearing an assignee after approval keeps working, because an empty `assignee` is the escalation channel an agent uses to hand work back to the operator.

## Problem

Nothing forces the question "who has to do this?" at the one moment the operator is already deciding a task is real. `task approve` records that a human approved the work but not who holds the baton, so the operator sets `assignee: bborbe` by hand afterwards — and when they forget, the task is invisible to every "who owns this" query while still being live in planning or execution.

The obvious fix — make the assignee mandatory — is wrong on its own, because `docs/task-writing.md` gives an empty `assignee` two deliberate meanings: an unclaimed inbox at creation, and a *park* when an executor or agent clears it to escalate back to the operator. A rule that blocks an empty assignee would close the escalation channel. The rule therefore belongs at approval (fill it, once, when the operator is looking) and the execution gate only warns. `--assignee` exists for the two cases where the configured owner is the wrong one: an operator approving work that belongs to a teammate, and an operator whose config names no current user at all.

## Goal

After this change, every task that passes the approval gate has an owner recorded at the moment of approval, and the approval command is the single place that decides it. An operator who wants a different owner can say so on the command line; an operator whose config names no current user gets a refusal rather than a silently unowned task; and an agent that clears its own assignee to ask for help still can.

## Non-goals

- Backfilling `assignee` on tasks already past approval.
- Blocking (rather than warning) at the planning → execution gate.
- Validating the assignee against a user list or a known-agent registry.
- Changing the `assignee` semantics documented in `docs/task-writing.md` — unclaimed-inbox at creation, park at clear both stay as they are.
- Changing `task work-on`'s existing three-case assignee matrix.

## Assumptions

- `vault-cli config current-user` is configured in the operator's environment. When it is not, the refusal path (AC4) is the expected outcome, not a defect.
- `--assignee ""` is treated as "flag not given", so config resolution applies. An empty flag value is not a request for an unowned task.
- The keys approve writes (`status`, `phase`, `approved_by`, `approved_at`) plus `assignee` compose onto one in-memory map and reach storage in a single `WriteTask`, so no partial state is observable.
- `MutationResult.Warnings` is the established channel for a non-fatal notice travelling from an operation up to the CLI layer.

## Acceptance Criteria

Each criterion names the observable the verifier inspects.

- [ ] **AC1 — approve fills an empty assignee.** `vault-cli task approve "<scratch>"` on a task whose `assignee` is empty sets it to the value of `vault-cli config current-user`. Evidence: `vault-cli task get "<scratch>" assignee` prints the current user (stdout match), and `grep -c '^assignee: <current-user>$'` over the task file returns `1`.
- [ ] **AC2 — approve keeps an existing assignee.** With `assignee: someone-else` pre-set, approve leaves it untouched. Evidence: `vault-cli task get "<scratch>" assignee` prints `someone-else` after approval.
- [ ] **AC3 — `--assignee` overrides both cases.** `vault-cli task approve "<scratch>" --assignee X` on an empty assignee prints `X`; the same flag on a task pre-set to `someone-else` also prints `X`. Evidence: two invocations, each followed by `vault-cli task get "<scratch>" assignee`.
- [ ] **AC4 — unresolvable owner refuses.** With an empty assignee, no `--assignee`, and no resolvable current user, approve exits non-zero, its stderr names the missing owner, and the phase is unchanged. Evidence: exit code non-zero; stderr contains `assignee`; `vault-cli task get "<scratch>" phase` still prints `todo`.
- [ ] **AC5 — the execution gate warns on an empty assignee and proceeds.** With `assignee` empty, `/vault-cli:execute-task` emits a line containing `assignee is empty` and still flips the phase. Evidence: `grep -n 'assignee is empty' commands/execute-task.md` returns ≥1 line; secondary confirmation — the gate's printed output contains the literal string and `vault-cli task get "<scratch>" phase` prints `execution`.
- [ ] **AC6 — the warning is conditional, not unconditional.** Negative criterion. With `assignee` set the gate emits no such line. Evidence: `grep -B2 -A2 'assignee is empty' commands/execute-task.md` returns context lines naming the empty-assignee condition, so the line sits inside a branch rather than being an unconditional print; secondary confirmation — with `assignee` set the gate's output contains `0` occurrences, while the same gate run on an empty assignee contains `≥1` (the paired positive control).
- [ ] **AC7 — clearing after approval still works.** `vault-cli task set "<scratch>" assignee ""` on a `planning`/`execution` task succeeds and leaves the field empty. Evidence: exit code `0`; `vault-cli task get "<scratch>" assignee` prints the empty string.
- [ ] **AC8 — the contract is documented and the tree is green.** `docs/task-writing.md` states the approve-time owner rule in its `assignee` semantics section, and the repo's gate passes. Evidence: `grep -n 'approve' docs/task-writing.md` returns ≥1 line inside that section; `make precommit` exits `0`.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint, format, generate, tests and `check-versions` all clean
- `make test` — unit + integration suites pass
- `grep -n 'assignee' pkg/ops/task_approve.go` — the fill/keep/refuse branches are present
- `grep -n 'assignee' commands/execute-task.md` — the warning line is present

### Operator-executable (runs on the host, after the branch is built)

- `make install`, then walk AC1–AC7 as live probes against the installed binary on a scratch task, quoting each command's output
- `which vault-cli` — confirms the installed binary is the built one and not a shadowing Homebrew cask
- `vault-cli --version` — reports the released version

## Desired Behavior

1. `task approve` resolves an owner before it writes: the `--assignee` flag when given, otherwise the task's existing `assignee`, otherwise `config current-user`.
2. When the resolved owner is non-empty and differs from the task's current value, the new value is written in the same single write that sets `status: in_progress`, `phase: planning`, `approved_by` and `approved_at` — a task never lands in planning without its owner beside it.
3. When the task already carries the same assignee the flag or config would produce, the file is not dirtied by an assignee write.
4. When no owner can be resolved, approve writes nothing at all: no status change, no phase change, no approval record, and the command exits non-zero with an error naming the missing owner.
5. The planning → execution gate inspects `assignee` and, when it is empty, emits one warning line containing `assignee is empty` before proceeding; the phase transition still happens.
6. The same gate emits no such line when `assignee` is non-empty.
7. `vault-cli task set "<name>" assignee ""` continues to clear the field on a task in any phase, including after approval.
8. `docs/task-writing.md` records that approval fixes the owner, placed with the existing unclaimed-inbox and park paragraphs so the three states read as one contract.

## Constraints

- `pkg/ops/` is a library layer: operations return structured results and never write to stdout. Any non-fatal notice goes on `MutationResult.Warnings`; the CLI layer owns all output formatting.
- The refusal path must follow the file's existing convention — the current `approved_by`-empty and phase guards both return `MutationResult{Success: false, Error: ...}` with a wrapped error, and the new refusal does the same.
- A new CLI flag requires an entry in `integration/cli_test.go`'s command registration table (currently `Entry("task approve", "task", "approve")`).
- Tests are Ginkgo v2 / Gomega with Counterfeiter mocks; the operation's interface signature changes, so its mock must be regenerated.
- The existing refusals (empty `approved_by`, zero clock, wrong phase, pre-existing approval record) must keep their current behavior and messages.
- `docs/task-writing.md`'s `assignee` semantics and `task work-on`'s three-case matrix must not contradict the new rule.
- This repo is `autoRelease: true` via `.maintainer.yaml`: add a `## Unreleased` CHANGELOG bullet and let the releaser own the version bump and tag. Do not hand-bump the plugin manifests or `git tag`.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| `current_user` absent from the config file | Refuse before any write; exit non-zero naming the missing owner; phase stays `todo` | Operator sets `current_user` in the config, or passes `--assignee` |
| Task already has a different assignee | Preserve it; no warning, no error | Operator re-runs with `--assignee` to take it over |
| Assignee cleared after approval | Clear succeeds; the escalation notification path is unchanged | None needed — this is the intended escalation channel |
| Two `task approve` invocations race, or the process is killed between a guard and `WriteTask` | The single-write composition means a reader sees either the pre-approval row or the fully-approved row, never a half-written one | None — re-running approve on a row still at `todo` is safe; a row that already carries an approval record is refused |
| Mock not regenerated after the interface change | `make precommit` fails at the generate step | Run `make generate` (or `go generate ./...`) and commit the mock |
| A caller passes `--assignee ""` explicitly | Treated as "no flag given", so config resolution applies | None — an empty flag value is not a way to request an unowned task; use `task set assignee ""` after approval |

## Security / Abuse Cases

The change reads one value from the operator's local config (`current_user`) and writes it into a task file's frontmatter. It introduces no network call, no credential handling, and no new input path: the `--assignee` value is written into a YAML scalar exactly as the existing `task set <name> assignee <value>` already does. No user input reaches a shell.

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `task approve` owner resolution — the `--assignee` flag, the fill/keep/refuse branches, the regenerated mock, the `integration/cli_test.go` table entry, the `docs/task-writing.md` amendment, unit tests for all four cases | 1, 2, 3, 4, 7, 8 | 1, 2, 3, 4, 7, 8 | — |
| 2 | The `assignee is empty` warning in `commands/execute-task.md`, plus the `## Unreleased` CHANGELOG bullets for both prompts. The releaser owns the version bump and the tag — do not hand-bump the plugin manifests | 5, 6 | 5, 6 | prompt 1 (the warning describes the owner rule prompt 1 establishes) |

Rationale: prompt 1 is the Go change and carries every test the repo's gate needs; prompt 2 is markdown in the plugin and can only be written once the rule it warns about exists. The two touch disjoint files, so prompt 2 depends on prompt 1 only for wording, not for code.

## Do-Nothing Option

The operator keeps setting `assignee` by hand after each approval, and unowned tasks keep entering planning and execution — 1,322 of them today. The cost is not the keystroke; it is that "who has to do this?" has no reliable answer for a live task, and nothing prompts the question at the moment the operator is already deciding the task is real. The command that performs the approval is the natural place to ask, and it is the only place that can ask exactly once.
