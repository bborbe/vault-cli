---
status: draft
tags:
    - dark-factory
    - spec
---

## Task approve: the recorded `todo → planning` transition

## Summary

- Agents file new work into an approval inbox; only work the operator has approved may be opened by a manager.
- Today nothing can perform that approval. The only way out of the inbox writes no record, so a deliberate approval and an accidental move look identical in the file.
- This spec adds a command that performs the approval and records who approved it and when, together.
- It refuses any row that is not waiting for approval, so it cannot be used to move work that is already underway.
- It changes no existing command and no reader — it adds the missing writer.

## Problem

The vault's task lifecycle now has an approval gate: an agent files at `phase: todo` and stops, and `todo` is the operator's inbox. The gate is only half-built. The filing side exists (`/vault-cli:create-task` leaves the row at `todo` and no longer chains `plan-task`), but the approving side has no command at all, so the only way to leave `todo` is `vault-cli task set <name> phase planning`.

That is a problem because a phase flip is not an approval. `task set` writes one field and nothing else, so after it runs the row is at `planning` with no evidence that the operator approved it, when they approved it, or that they approved it at all. Any caller that can write frontmatter — a script, a mis-scoped sweep, a future agent path — produces a state indistinguishable from a deliberate human decision. The gate's whole purpose is that unapproved work cannot be opened, and the current transition cannot express the difference.

The cost is already visible in this repository's own history. The task that defines this gate was itself approved by hand: its frontmatter carries `approved_by: operator` and `approved_at: 2026-09-28T00:00:00Z`, written by an editor because the command did not exist. `approved_at` is midnight — a placeholder, not a measurement. A lifecycle whose first real transition was performed by hand-editing frontmatter has no mechanism to appeal to when the next one is disputed.

Two smaller consequences follow from the same absence. The `status` field is not moved, so an approved row can sit at `status: next` with `phase: planning` — a combination the existing planning entry contract deliberately avoids by promoting `next` to `in_progress` at the same moment. And there is no refusal surface: `task set` will happily move a row from `execution` back to `planning`, so the transition can be applied to rows that are already past the gate it guards.

## Goal

An operator can approve a `todo` row with one command, and afterwards the file records three things that were written together: the row is at `phase: planning`, `approved_by` names who approved it, and `approved_at` carries the wall-clock time of the transition. The command refuses any row not at `phase: todo`, so it cannot be used to move a row that has already left the inbox. A row that has been approved is distinguishable, from the file alone, from one that was moved by any other means.

## Non-goals

- Changing the filing path — `/vault-cli:create-task` and `task-creator` already leave new rows at `phase: todo`; this spec does not touch them.
- Changing any reader: the four sweep commands, `/supervisor:open --flagged`, and the `flag: true` carve-out keep their current behavior. This spec adds the writer, not the readers.
- Migrating rows that are already at `planning` without an approval record — the split-out task `[[Return Agent-Planned Tasks to the Operator's Approval Inbox]]` owns that.
- A UI or inbox view for pending approvals — `[[Give the Operator One Approval Inbox for Agent-Filed Tasks]]` owns that.
- An `unapprove` / `reject` verb, or any way to clear an approval record once written. Approval is one-way; sending a row back is `task set --force phase <earlier>`, which already exists and already requires the flag.
- Any change to `task set`'s existing behavior, including its `--force` phase-regression flag.
- Authentication or authorization of the approver — `approved_by` records a claimed identity, exactly as `assignee` does. Enforcing who may approve is out of scope for a local CLI. `--by` is kept rather than replaced by a hardcoded `operator`: the field is required by the record either way, and a value that can be stated truthfully is better than one that is false whenever a manager session runs the command on the operator's behalf. A richer approver model — an identity source, multiple approvers, an audit trail — is a separate spec.
- Recurring-system tasks (dated schedule instances) — they keep their own lifecycle and do not pass through the approval inbox.
- No new scenario (see the scenario-coverage note under Acceptance Criteria).

## Acceptance Criteria

Fixture vault for ACs 1–9: a temp vault with `Tasks/Alpha.md` and a config naming `tasks_dir: Tasks` (the shape the existing `integration/` harness builds). `<bin>` is the binary built from HEAD — `gexec.Build` inside `integration/`, or `/tmp/new-vault-cli` on the host. `Alpha`'s frontmatter holds the base keys `status`, `page_type: task`, `priority`, `task_identifier` (a well-formed UUID), and `phase` in whichever value the AC under test names. No approval keys are present unless the AC says so.

- [ ] `Alpha` at `status: next`, `phase: todo`; `<bin> --config <cfg> task approve Alpha` exits 0, and `grep -nE '^(status|phase|approved_by|approved_at):' Tasks/Alpha.md` returns four lines reading `status: in_progress`, `phase: planning`, `approved_by: operator`, and an `approved_at` value that parses as RFC3339 — evidence: exit code 0 plus file content (all four keys, read from the file rather than from `task get`). The four-keys-in-one-read is the assertion that the transition is recorded, not merely performed.
- [ ] `Alpha` at `status: next`, `phase: todo`; `<bin> --config <cfg> task approve Alpha --by "Manager Layer"` exits 0 and `grep -nE '^approved_by:' Tasks/Alpha.md` returns exactly one line reading `approved_by: Manager Layer`, while `<bin> --config <cfg> task approve Alpha --by ""` on a fresh `todo` fixture exits non-zero with a stderr naming `approved_by` and leaves that file's sha256 unchanged — evidence: exit codes plus file content plus negative evidence (empty approver refused, hash identical).
- [ ] `Alpha` at `status: next`, `phase: todo`, carrying `assignee: someone`, `priority: 1`, `page_type: task`, `task_identifier: 22222222-2222-2222-2222-222222222222`, and an unknown key `custom_key: keep`; after `<bin> --config <cfg> task approve Alpha` exits 0, `grep -nE '^(assignee|priority|page_type|task_identifier|custom_key):' Tasks/Alpha.md` returns five lines whose values are identical to the pre-state, and `diff <(grep -vE '^(status|phase|approved_by|approved_at):' <pre-file>) <(grep -vE '^(status|phase|approved_by|approved_at):' <post-file>)` returns empty — evidence: file content plus negative evidence (the stripped diff is empty, so no unrelated line was added, deleted, reordered or re-indented). This AC is the one that fails if the implementation rebuilds the frontmatter from a literal instead of composing it onto the existing map; the fixture's `task_identifier` is a value other commands consume, so losing it is silent otherwise. The strip-and-diff shape is used rather than a bare `grep -c '^<'` on the full diff, because that proves only that nothing was deleted — not that nothing stray was added.
- [ ] `Alpha` at `phase: planning` (already past the gate), with no approval keys; `<bin> --config <cfg> task approve Alpha` exits non-zero, its stderr names `planning` and `todo`, and the sha256 of `Tasks/Alpha.md` taken immediately before and immediately after that one invocation is unchanged — evidence: exit code non-zero, stderr match on both phase names, negative evidence (file hash identical). The refusal is what makes the command an approval rather than a second `task set`.
- [ ] `Alpha` at `phase: execution`; `<bin> --config <cfg> task approve Alpha` exits non-zero and the file hash is unchanged — evidence: exit code non-zero plus negative evidence. Together with AC 3 this pins the refusal to "not `todo`" rather than to one specific wrong phase.
- [ ] Three `todo` fixtures, each otherwise identical: one carrying only `approved_by: someone`, one carrying only `approved_at: <an RFC3339 value>`, and one carrying both. For each, `<bin> --config <cfg> task approve Alpha` exits non-zero and that fixture's sha256 is unchanged — evidence: exit code non-zero plus negative evidence, asserted **per fixture** (three runs, three hashes). The one-key-each fixtures are load-bearing: a guard that checks only `approved_by` passes on the both-keys fixture, so a single combined fixture cannot distinguish "refuses either key" from "refuses one key".
- [ ] `go test ./pkg/ops/...` exits 0 and the suite contains a case that runs the approve operation against a counterfeiter `Storage` mock and asserts the mock recorded **exactly one** write call, plus a case asserting that a refusal records **zero** write calls — evidence: exit code 0 plus each case asserting on the mock's recorded call count, not on the presence of test syntax. The one-write assertion is the AC that makes "recorded together" checkable; a three-write implementation satisfies AC 1 but fails this one.
- [ ] `go test ./pkg/ops/...` exits 0 and the suite contains a case that injects a fixed `libtime.CurrentDateTime` and asserts the written `approved_at` equals that injected instant — evidence: exit code 0 plus the assertion on the resulting frontmatter value. This is the test that fails if the implementation reaches for `time.Now()`.
- [ ] `<bin> --config <cfg> task approve Alpha` on a `todo` fixture (plain output, the default) exits 0 and its stdout names both the task and the new phase — evidence: exit code plus stdout match on the task name and the string `planning`. `<bin> --config <cfg> task approve Alpha --output json` on a fresh `todo` fixture exits 0 and its stdout parses as JSON carrying the task name and the new `phase` — evidence: exit code plus parsed stdout. The refusal path with `--output json` exits non-zero — evidence: exit code. Both output paths are asserted because the plain path is the default a human sees, and no other AC covers it.
- [ ] `docs/task-writing.md` § Phase transitions names `vault-cli task approve` as the command that performs `todo → planning` and states that a phase set without an approval record is not an approval; the `todo` row of its phase table names the same command; `CHANGELOG.md` carries an `## Unreleased` bullet prefixed `feat:` describing the new verb — evidence: file content — `grep -cE 'task approve' docs/task-writing.md` returns ≥ 2 (the `todo` table row and the prose both name the verb, so a single mention is not sufficient — the count is on the two-word verb, never on the bare substring `approve`, which already occurs twice at HEAD inside unrelated words), `awk -F'|' '/^\| `todo`/ {print $0}' docs/task-writing.md | grep -c 'task approve'` returns ≥ 1 (the table row itself names it, asserted separately because the prose count alone cannot prove the row was touched), `grep -cE 'approved_(by|at)' docs/task-writing.md` returns ≥ 1 (the doc names the recorded fields, not merely the verb — this string occurs zero times at HEAD), and the `## Unreleased` section in `CHANGELOG.md` carries a line starting with `- feat:` that names the new verb. The CHANGELOG assertion is written as `awk '/^## /{s=$0} /^- feat:/{print s" | "$0}' CHANGELOG.md | head -1 | grep -c 'task approve'`, and it passes when the printed section is `## Unreleased` **or** the newest `## vX.Y.Z` — the repo's releaser renames the section when it cuts a release, and `.dark-factory.yaml` sets `autoRelease: false`, so the release-cut branch is the normal case here rather than an exception. The trailing `grep -c 'task approve'` is load-bearing, not decoration: the bare `awk | head -1` prints the pre-existing `## v0.152.0 | - feat:` bullet at HEAD and would pass with no change at all, which is the same vacuity this AC was rewritten to remove.
- [ ] **Post-Deploy (Rung-2):** the released binary performs the transition on a real row — a scratch task in the Personal vault at `status: next`, `phase: todo` is approved with the installed `vault-cli`, and `grep -nE '^(phase|approved_by|approved_at):' "<scratch file>"` then shows `phase: planning`, an `approved_by`, and an RFC3339 `approved_at` — evidence: exit code plus file content, with the transcript recorded in the source task's `# Results` section. The scratch row is deleted afterwards.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0)`

**Scenario coverage: no new scenario.** The transition, the refusal, the one-write property, and the clock injection are all reachable by unit tests over the operation (mocked storage, injected clock) and by the existing `integration/` harness, which builds the real binary and asserts exit codes and file content against a temp vault — no Docker, no cluster, no `gh`, no external service. No essential user journey depends on behavior beyond that CLI contract, and the repository's existing `scenarios/` still run unchanged as part of the release gate. None of the four conditions in dark-factory `docs/rules/scenario-writing.md` holds, so the default applies: no new scenario.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
grep -rn 'TaskApprove\|task approve' pkg/ops/ pkg/cli/    # ≥1 line (smoke over the two conventional locations; the spec does not pin which file holds it)
grep -cE 'task approve' docs/task-writing.md              # ≥2 (the todo table row and the prose both name the verb, not the substring "approve")
awk -F'|' '/^\| `todo`/ {print $0}' docs/task-writing.md | grep -c 'task approve'   # ≥1 (the table row itself names it)
grep -cE 'approved_(by|at)' docs/task-writing.md          # ≥1 (the recorded fields are named; this string occurs 0 times at HEAD)
awk '/^## /{s=$0} /^- feat:/{print s" | "$0}' CHANGELOG.md | head -1 | grep -c 'task approve'   # ≥1 (the bullet names the new verb; returns 0 at HEAD)
```

### Operator-executable (runs on the host after PR merge)

```
# Release gate — mandatory before make install, per docs/releasing-vault-cli.md
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
/tmp/new-vault-cli --version
ls scenarios/*.md        # walk each scenario's Action + Expected against /tmp/new-vault-cli

# Transition replay against the fresh binary, on a scratch row
/tmp/new-vault-cli --config <fixture-config> task approve Alpha
grep -nE '^(status|phase|approved_by|approved_at):' <fixture>/Tasks/Alpha.md   # four lines
/tmp/new-vault-cli --config <fixture-config> task approve Alpha                # second run → exit 1

# Version alignment, then install
make release-check
make install
vault-cli --version

# Install the Claude Code plugin manifests (separate artifact)
claude plugin update vault-cli@vault-cli   # then restart Claude Code
```

## Desired Behavior

1. `vault-cli task approve <name>` resolves the named task through the existing multi-vault lookup and, when the row is at `phase: todo`, writes in a single storage call: `status: in_progress`, `phase: planning`, `approved_by`, `approved_at`. Every other frontmatter key is preserved byte-for-byte, including key order for keys the operation does not touch. It exits 0 and reports the task name and the new phase.
2. The approver is `operator` unless `--by <approver>` is given, in which case that literal string is written. `--by ""` is refused with a non-zero exit rather than writing an empty approver, so a record can never be present-but-blank.
3. `approved_at` is an RFC3339 timestamp read from the injected `libtime.CurrentDateTime`. The operation never calls `time.Now()` directly, so a test can pin the instant. The value is written unquoted-as-a-string in the same shape the vault's other vault-cli-owned date keys use, so it round-trips through `parseToFrontmatterMap` without the `- - X` corruption that path exists to prevent.
4. The command refuses, with a non-zero exit and a message naming both the current phase and `todo`, any row that is not at `phase: todo` — including rows at `planning`, `execution`, `ai_review`, `human_review`, `done`, and rows with no `phase` key at all. A refusal writes nothing: no partial frontmatter, no key reordering, no `approved_at`.
5. The command refuses a `todo` row that already carries an `approved_by` or an `approved_at` key, with the same no-write guarantee. A row carrying a record is not awaiting approval, and re-approving must not overwrite who approved it first.
6. `--output json` emits the task name and the resulting `phase` on success, and on refusal emits the error and exits non-zero. The existing plain/JSON output conventions of the other `task` verbs are unchanged.
7. The operation is a first-class `pkg/ops` operation built the way every other `task` verb in this repository is built: it holds an injected `Storage` and an injected `libtime.CurrentDateTime`, does not perform file I/O itself, and its factory function is pure composition. The layer-by-layer shape is the repository's documented convention rather than something this spec re-specifies — see `docs/development-patterns.md` § Adding a New Command.
8. `docs/task-writing.md` § Phase transitions names the command and states the rule that a phase set without an approval record is not an approval, and the `todo` row of the phase table names it; `CHANGELOG.md` carries an `## Unreleased` `feat:` bullet.

## Assumptions

- `phase: todo` is written by the filing path on every new task, so a `todo` row is the normal entry state rather than an edge case. (Shipped: `/vault-cli:create-task` leaves the row at `todo` and `task-creator` writes the field.)
- The approver's identity is a claim, not an authenticated fact — the same trust level as `assignee` and `claude_session_id`, both of which are already plain frontmatter strings written by whichever process runs the command.
- A single storage write is achievable for the four keys because the storage layer already writes the whole frontmatter map in one file operation; the operation composes the new map and hands it over once.
- The vault is writable at command time and the entity file is not a symlink; both preconditions already hold for every write command in this CLI.

## Constraints

- `task set` behavior is frozen, including its `--force` phase-regression flag and its `unknown field` pass-through. This spec adds a verb; it does not change an existing one.
- The read side is frozen: the four sweep commands, `/supervisor:open --flagged`, and the `flag: true` carve-out are untouched by this spec, and no existing test asserting their behavior may change.
- The `approved_by` and `approved_at` keys are plain frontmatter strings; no new field type, no nested map, no schema object.
- No new field is added to the `task list --output json` payload by this spec — the approval keys are read from the file or through `task get`, exactly as other arbitrary frontmatter keys are.
- `approved_at` is written through the same frontmatter path as every other vault-cli-owned date key (see `docs/development-patterns.md`), so the bare-wikilink quoting invariant that path carries is not bypassed by a new read path.
- The operation never calls `time.Now()`; the clock arrives through `libtime.CurrentDateTime` injection, per repository convention.
- Tests follow repository convention: Ginkgo v2 / Gomega with `DescribeTable` and `Entry` for the table-driven coverage, and counterfeiter mocks for `Storage`. The cases may live in any `pkg/ops` test file — the spec does not pin a file.
- `docs/task-writing.md` § Phase transitions already owns the phase lifecycle and is the doc this spec updates; no new `docs/` page is introduced.
- The refusal has no bypass flag. There is no `--force` on `approve`; re-opening a closed transition is `task set --force phase <earlier>`, which already exists and already requires the flag.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---|---|---|---|---|---|
| `approve` on a row not at `todo` | Refused; message names the current phase and `todo`; file byte-identical | Use `task set --force phase <target>` if the intent is a deliberate reset, not an approval | Exit code non-zero plus the stderr message | Reversible — nothing written | — |
| `approve` on a `todo` row that already carries an approval record | Refused; file byte-identical | Inspect the existing record with `task get <name> approved_by` | Exit code non-zero plus the stderr message | Reversible — nothing written | — |
| Storage write refused at open (permission denied, symlink refusal, vault not found) | Non-zero exit carrying the storage layer's own error; the file is byte-identical | Fix the write precondition, then re-run `approve`; it exits 0 and AC 1's four-key grep returns four lines | Exit code non-zero plus `diff -u` between the pre- and post-attempt file showing no change | Reversible — nothing written | — |
| Storage write fails mid-write (disk full) | Non-zero exit carrying the storage layer's own error. The file may be left **truncated** — the storage layer writes with `O_TRUNC` and is not atomic (no temp-file + rename), so this row does not claim byte-identity. The logical guarantee still holds: a row is never left at `planning` without a record, because a truncated file carries no `phase` at all | Restore the file from the vault's autocommit history, then re-run `approve` | `task get <name> phase` fails, or the frontmatter parse error is reported | Partial — the write path is not atomic today; making it atomic is out of scope | — |
| Crash after composing the new frontmatter, before the write completes | The process dies with the file either untouched or truncated — the same non-atomic write as the row above. The guarantee is logical, not byte-level: a row is never left at `planning` without a record, because all four keys travel in one map | Restore from the vault's autocommit history if truncated, then re-run `approve`; it exits 0 and AC 1's four-key grep returns four lines | `grep -cE '^phase:' <file>` still reads `todo`, or the file fails to parse | Partial — see the disk-full row | Two concurrent `approve` calls on the same row: last write wins, and both write the same resulting phase; the `approved_at` of the loser is discarded. Same last-write-wins class the existing `add`/`set` verbs already have; no lock is added |
| Vault not found, or the named task resolves in no vault | Refused by the existing lookup path before any write | Fix the name or the `--vault` argument | Exit code non-zero, existing error message | Reversible — nothing written | — |
| Clock source returns a zero or malformed instant | Refused with a non-zero exit and a message naming `approved_at`; the file is byte-identical. A malformed value is not written — DB 2's "a record can never be present-but-blank" applies to a corrupt value as much as to an empty approver | Re-run once the clock source returns a well-formed instant | Exit code non-zero plus the stderr message; no `approved_at` key appears in the file | Reversible — nothing written | — |

## Security / Abuse Cases

- **Attacker-controllable input:** the `--by <approver>` string and the task name used for file resolution. The approver value reaches YAML frontmatter as a scalar, so a value containing a newline is serialized by the YAML encoder as a block scalar and cannot introduce a sibling key. No newline-specific guard is added; the scalar encoding is the containment.
- **Trust boundaries:** the CLI writes only inside the configured vault directories, resolved through the existing entity-name lookup — no shell interpolation, no path traversal beyond what the existing commands already do. The symlink refusal in the write path stays in force.
- **What the command is not:** it is not an authorization mechanism. `approved_by` records a claimed identity with the same trust level as `assignee`; any process that can run `vault-cli` can claim any approver. The gate's guarantee is *that* an approval was recorded and *when*, not *who* — enforcing the who would require an identity source this CLI does not have, and that is explicitly out of scope.
- **What must be validated:** the current `phase` value against `todo` (the refusal), the absence of a pre-existing approval record, and the non-emptiness of `--by` when supplied.
- **What can hang or retry forever:** nothing — the command is a single find, mutate, write sequence with no retry loop and no network I/O.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `TaskApproveOperation` in `pkg/ops` — the four-key single write, key preservation, the `todo`-only and no-existing-record refusals, `--by` handling, injected clock; behavioral Ginkgo rows plus the counterfeiter one-write and zero-write assertions | 1, 2, 3, 4, 5, 7 | 1–8 | — |
| 2 | Cobra leaf `vault-cli task approve` wired under `task`, plain and `--output json` output, `--by` flag | 6 | 9 | prompt 1 (calls the operation) |
| 3 | Docs (`docs/task-writing.md` phase table + prose) and the `CHANGELOG.md` `## Unreleased` `feat:` bullet | 8 | 10 | prompts 1–2 |

Rationale: prompt 1 is the behavior and carries every assertion that makes the transition checkable — it is the only prompt whose failure leaves the gate unenforced. Prompt 2 is the thin CLI surface over it and cannot land first. Prompt 3 documents the surface prompts 1–2 created. AC 11 is operator-executed after merge and is not a prompt. The spec touches four layers (domain, storage, ops, cli) plus docs, which is why the decomposition is explicit; each prompt stays within one or two of them.

## Do-Nothing Option

`todo` stays an inbox with no way to leave it that records a decision. The only transition remains `task set <name> phase planning`, which writes no approver and no timestamp, so a row that an operator deliberately approved and a row that some other process moved are the same file. The gate's guarantee — that unapproved work cannot be opened — degrades to a convention that nothing enforces and nothing can audit, and the first disputed transition has no artifact to appeal to. The cost is unbounded and is already being paid: the task defining this gate approved itself by hand-editing frontmatter, with a midnight placeholder where a measurement should be.
