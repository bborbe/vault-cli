---
status: approved
approved: "2026-10-20T09:50:13Z"
branch: dark-factory/goal-topic-approve
---

## Summary

- Add `vault-cli goal approve <goal>` and `vault-cli topic approve <topic>`.
- Each moves a row out of the approval inbox (`phase: todo`) into `phase: planning`, recording `approved_by` and `approved_at` in the same write.
- Close the generic-`set` bypass: `goal set … phase planning` and `topic set … phase planning` on a `todo` row are refused, exactly as `task set` already refuses them.
- Update the two command docs that currently teach the bypass (`commands/plan-goal.md`, `commands/execute-topic.md`).
- A row with no `phase` is exempt from the gate and cannot be approved; the refusal names the absent phase.

## Problem

A task enters planning only through `vault-cli task approve`, which records who approved it and when, and `task set … phase planning` is refused so the record cannot be skipped. Goals and topics have a `phase` field but neither the verb nor the guard. The manager-start gate in `bborbe/claude-supervisor` (`scripts/propose-managers.py`, the follow-on spec in `bborbe/claude-supervisor`) needs a recorded approval to read; without one it can only infer readiness from counts of approved tasks underneath. This spec gives that gate a single authoritative writer and closes the route that would write the phase without it.

## Goal

A goal or topic at `phase: todo` is approved by one command, which atomically writes `phase: planning`, `approved_by`, and `approved_at`. The generic `set` path cannot perform the same transition. A row that is not in the inbox, or that already carries an approval record, is refused with nothing written. Every other frontmatter key survives.

## Non-goals

- Backfilling `phase` on existing goals and topics.
- Changing the task approval flow or the task-side guards.
- Rejecting unknown phase values on the generic `set` path — `setPhaseField` already validates enum membership.
- Assigning an owner at approval time — goals and topics are not required to have an assignee, and this verb does not add one.
- A `docs/topic-writing.md` — topics have no writing guide (spec 051 declined one).
- Any change to `propose-managers.py` or `/supervisor:open` (those live in `bborbe/claude-supervisor`).
- **An accepted hole, mirrored from the task contract:** a row with no `phase` key can still reach `planning` via `goal set … phase planning`, because the guard fires only on `todo → planning`. `goal clear <goal> phase` followed by that set is therefore a two-step bypass. Closing it would require treating an absent phase as `todo`, which contradicts the operator's decision that absent `phase` means exempt.
- **No override exists.** `goal set` / `topic set` carry no `--force` flag, and this spec does not add one — adding it would change the shared entity-set command used by themes, objectives and visions.

## Assumptions

- Goals and topics already carry `phase`, and `todo` is the inbox phase. Verified in `pkg/domain/goal_phase.go` and `topic_phase.go`.
- `WriteGoal` / `WriteTopic` preserve unknown frontmatter keys. Verified: both serialize `RawMap()` (`pkg/storage/goal.go`, `pkg/storage/topic.go`).
- No goal or topic carries `approved_by` / `approved_at` today. Verified: only `pkg/ops/task_approve.go` writes those keys.
- `Goal` and `Topic` already embed their frontmatter and expose `Phase()`, `SetPhase()`, `Get()`, `Set()`. Verified in `pkg/domain/goal.go` and `topic.go`.
- `--by` is kept rather than replaced by a hardcoded `operator`: the record needs the field either way, and the flag already exists on `task approve`.

## Acceptance Criteria

- [ ] Happy path, goal and topic. `vault-cli goal approve <goal>` on a goal at `phase: todo` exits 0 and prints one confirmation line containing the goal name; afterwards `vault-cli goal get <goal> phase` reads `planning`, `vault-cli goal get <goal> approved_by` reads `operator`, and the raw `approved_at:` line in the goal file is RFC3339 and falls within 60 seconds of `date -u` read before the invocation (`vault-cli goal get <goal> approved_at` is non-empty — the getter renders a `time.Time` as Go's `%v` form, not RFC3339). `vault-cli topic approve <topic>` behaves identically on a topic. `--by alice` records `alice` on both.
- [ ] Table-driven refusals, goal and topic rows. Each exits non-zero, prints a message naming the row and the phase found vs required, and leaves `shasum -a 256` of the file unchanged, under **default text output**:
  - approve on a row at `phase: planning`, `execution`, `done`, or with no `phase` key
  - approve on a `todo` row that already carries `approved_by` or `approved_at`
  - approve with `--by ""`
  - approve on an unknown row name
- [ ] Set-path guard, goal and topic. `vault-cli goal set <goal> phase planning` on a goal at `phase: todo` exits non-zero, names `vault-cli goal approve` in the message, and leaves the file checksum unchanged; `vault-cli topic set <topic> phase planning` does the same. Narrowness: `vault-cli goal set <goal> phase execution` on a `todo` goal exits 0 and `vault-cli goal get <goal> phase` reads `execution`; `vault-cli goal set <goal> priority 2` on a `todo` goal exits 0 and `vault-cli goal get <goal> priority` reads `2`.
- [ ] Other keys preserved. With a fixture goal carrying an unknown key (`custom_key: keep`), the frontmatter after a successful approve differs from before only in `phase`, `approved_by`, `approved_at`. Evidence: `diff <(grep -vE '^(phase|approved_by|approved_at):' pre.md) <(grep -vE '^(phase|approved_by|approved_at):' post.md)` is empty, and `grep -c '^custom_key: keep' post.md` is 1.
- [ ] Docs updated. The `todo` arms of `commands/plan-goal.md` and `commands/execute-topic.md` name `goal approve` / `topic approve`; their `done`-reopen arms still name `set … phase planning`; `grep -c 'goal approve' docs/goal-writing.md` returns at least 1.
- [ ] `make precommit` exits 0.

## Verification

**Container-executable** (runs inside the YOLO container at `/workspace`):

```bash
make precommit
make test
```

**Operator-executable** (host, after the binary builds — never against a live vault):

```bash
make build
cp -r example /tmp/vault-approve-e2e
sed -i '' "s|__VAULT_PATH__|/tmp/vault-approve-e2e/vault|" /tmp/vault-approve-e2e/config.yaml
# create a scratch goal at phase: todo in /tmp/vault-approve-e2e/vault/23 Goals/
./bin/vault-cli --config /tmp/vault-approve-e2e/config.yaml goal approve "<scratch goal>"; echo "exit=$?"
./bin/vault-cli --config /tmp/vault-approve-e2e/config.yaml goal get "<scratch goal>" phase
./bin/vault-cli --config /tmp/vault-approve-e2e/config.yaml goal get "<scratch goal>" approved_by
./bin/vault-cli --config /tmp/vault-approve-e2e/config.yaml goal get "<scratch goal>" approved_at
```

Repeat with `topic` and a scratch topic. Run the refusal cases against the same copy, comparing `shasum -a 256` before and after each.

## Desired Behavior

1. `vault-cli goal approve <goal>` reads the goal by name, refuses unless its phase is exactly `todo`, refuses if it already carries `approved_by` or `approved_at`, then writes `phase: planning`, `approved_by`, and `approved_at` in one write. `vault-cli topic approve <topic>` does the same for a topic.
2. `--by` sets the recorded approver and defaults to `operator`. An empty `--by` is refused before the row is read.
3. The recorded `approved_at` is the operation's clock reading, formatted the same way `task approve` writes it. A zero clock reading is refused rather than recorded — a unit-test-only path with no operator-observable trigger, so it carries a unit test rather than an acceptance criterion.
4. A refusal prints a message naming the row, the phase found, and the phase required, and exits non-zero. Nothing is written.
5. Success prints a one-line confirmation naming the row, and exits 0.
6. Unknown or extra frontmatter keys on the row survive the write untouched.
7. `goal set <goal> phase planning` and `topic set <topic> phase planning` on a row at `phase: todo` are refused before any mutation, with a message pointing at the approve verb. Only that (key, value, current-phase) triple is guarded; a missing phase, an unknown phase, every non-phase key and every other target phase keep their existing behaviour.

## Constraints

- The approval contract being mirrored is documented in `docs/task-writing.md` § Phase transitions — read it rather than re-deriving the rules from `pkg/ops/task_approve.go`.
- The set-path guard mirrors `todoPlanningSetRefusal` in `pkg/ops/frontmatter.go`.
- Reuse the existing storage interfaces `WriteGoal` / `WriteTopic` and the existing find-by-name methods; do not add a new storage method.
- Do not change `task approve`, the task-side guards, or the frontmatter serializers.
- A refusal writes nothing: the guard and every precondition run before the first mutation, so the file on disk is byte-identical.
- A `commands/` change requires a plugin version bump — the release tail is `autoRelease: true`, so the releaser handles it; do not hand-bump the manifests.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Row not found | Non-zero exit; stderr names the row | `vault-cli goal list \| grep <name>` to find the real name |
| Phase is not `todo` (including absent) | Non-zero exit; stderr names found vs required | `vault-cli goal set <goal> phase todo`, then approve |
| Row already carries `approved_by` / `approved_at` | Non-zero exit; stderr names the present keys | Nothing to do — already approved; read the record |
| `--by` empty | Non-zero exit before any read | Pass `--by <name>` |
| Clock reading is zero | Non-zero exit, nothing written | Fix the injected clock; re-run |
| Write fails | Non-zero exit; error wrapped with context | Re-run; the file is unchanged |
| Two concurrent approves of the same row | Single-writer assumption: last writer wins, and both may record an approval. Accepted — the operator is the only writer, and the existing-record guard makes the second call refuse once the first write is visible | Re-read the row; if the record is wrong, `vault-cli goal clear <goal> approved_by` and `vault-cli goal clear <goal> approved_at`, then `vault-cli goal set <goal> phase todo`, then `vault-cli goal approve <goal>` (the row is at `planning` after clearing, so it must be returned to the inbox first) |

## Security / Abuse Cases

The verbs write to a markdown file inside the configured vault. They take a row name from the operator and perform no shell interpolation. They refuse rather than guessing when the row is not in the expected state. No network access, no credential handling, and no path traversal beyond the existing name lookup the other goal/topic verbs already use.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Set-path guard: refuse `todo → planning` on the goal and topic `set` paths, mirroring `todoPlanningSetRefusal`; tests | 7 | 3 | — |
| 2 | `pkg/ops/goal_approve.go` + Ginkgo tests, modelled on `task_approve.go` | 1, 2, 3, 4, 5, 6 | 1, 2, 4 | — |
| 3 | `pkg/ops/topic_approve.go` + Ginkgo tests | 1, 2, 3, 4, 5, 6 | 1, 2, 4 | 2 |
| 4 | CLI wiring: `goal approve` and `topic approve` subcommands with `--by`, modelled on `createTaskApproveCommand` | 1, 2, 5 | 1, 2 | 2, 3 |
| 5 | Command docs (`commands/plan-goal.md`, `commands/execute-topic.md`), `docs/goal-writing.md` approval paragraph, CHANGELOG `## Unreleased` bullet | — | 5 | 4 |

## Do-Nothing Option

Without this verb, no goal or topic can move from `todo` to `planning` with a recorded approval, and the generic `set` path can move it without one. The manager-start gate would keep inferring readiness from task counts, which is exactly the ambiguity this work exists to remove. The cost is one small command per repository; the alternative is that the `phase` field stays half-wired.
