---
status: draft
tags:
    - dark-factory
    - spec
---

## Blocked-by write path: record a dependency, refuse a malformed shape

## Summary

- `blocked_by` is the vault's only machine-readable dependency signal, and today nothing can write it: `add` rejects the field as unknown, and `set` writes a scalar that every reader is designed to ignore.
- This spec makes `vault-cli task add` and `vault-cli goal add` append a `blocked_by` entry — the recording path that does not exist.
- It makes `set` refuse a non-list value for the field and point at `add`, instead of silently writing a shape the system discards.
- It makes `vault-cli task validate` report a scalar-shaped `blocked_by` as an error naming the field and the expected list shape, so the divergence is visible instead of silent.
- Nothing changes on the read side: a scalar still reads as an empty list, and the derived `blocked` flag, JSON emission, and `next-task` filtering keep the semantics spec 046 fixed.

## Problem

`blocked_by` is the vault's only machine-readable dependency signal: `/open --flagged` and the batch opener read it as JSON to decide what may start. Both write paths are broken, in three linked places, and nothing catches it. `vault-cli task add <task> blocked_by '[[X]]'` fails with `unknown field: "blocked_by"` because the task list-field allowlist holds only `goals` and `tags`, so a dependency cannot be recorded at all. `vault-cli task set <task> blocked_by '[[X]]'` takes the unknown-field branch and writes a scalar YAML value, which the dedicated list reader discards by design — so `set` records a dependency in a shape the system is built to ignore, and reports success while doing it. `vault-cli task validate` passes on both shapes, so the divergence has no detector. The observed cost: an ordering inversion in the Sunday shutdown maintenance chain went undetected because the dependency data was silently absent from the files that were supposed to declare it. The same defect exists for goals, whose documentation sanctions the same `set` and `clear` commands for the same field.

## Goal

A task or goal file can record "I cannot start until these are done" with one command, the recorded value is a YAML list that the existing readers actually see, and any file whose `blocked_by` is shaped so that no reader can see it is reported by `vault-cli task validate` with the field named and the expected shape stated. No command silently writes a shape the read side ignores.

## Non-goals

- Rewriting the existing `blocked_by`-bearing task and goal files — this fixes the write path going forward and surfaces the malformed files; converting them is an operator action.
- Auditing or fixing every list-typed field across goals, themes, objectives, and visions beyond the enumeration in Desired Behavior 1.
- A general schema-validation framework — the validate rule is a targeted list-versus-scalar check for this field.
- Changing the read side: the scalar rejection in the `blocked_by` list reader, the computed `blocked` flag, its JSON emission, and `next-task` filtering stay exactly as spec 046 specified them.
- Adding `blocked_by` to themes, objectives, or visions — they have no `blocked_by` accessor and no dependency model; adding one is a separate spec.
- Do NOT add a `--force` / `--coerce` / `--allow-scalar` flag to `add` or `set` that bypasses the refusal — the refusal is the fix; a bypass re-opens the silent-divergence hole this spec closes. Invariant; if a future consumer demands variation, that is a separate spec.
- Do NOT add a config key or environment variable that downgrades the new validate rule to a warning. Invariant; same reason.
- Do NOT introduce a second issue type for the same condition — one rule, one issue type, reported by both `task validate` and `task lint` through their shared detector.
- No new scenario (see the scenario-coverage note under Acceptance Criteria).

## Acceptance Criteria

Fixture vault for ACs 1, 3, 4, 5, 6, 7, 8: a temp vault with `Tasks/Alpha.md` and `Goals/Beta.md`, plus a config naming `tasks_dir: Tasks` and `goals_dir: Goals` (the shape the existing `integration/` harness builds). `<bin>` is the binary built from HEAD — `gexec.Build` inside `integration/`, or `/tmp/new-vault-cli` on the host. `Alpha`'s frontmatter holds exactly `status: in_progress`, `page_type: task`, `priority: 1`, `task_identifier: <a well-formed UUID>` and no other key, so the only issue any `validate` run can report is the one under test.

- [ ] `Alpha` whose frontmatter holds `blocked_by:` with one list entry `[[Blocker A]]`; `<bin> --config <cfg> task add Alpha blocked_by "[[Blocker B]]"` exits 0, and `<bin> --config <cfg> task list --output json` emits for `Alpha` a `blocked_by` array containing both `[[Blocker A]]` and `[[Blocker B]]`, in that order — evidence: exit code 0 plus stdout JSON match (the pre-existing entry surviving is the not-clobbering proof; the file content of `Tasks/Alpha.md` also contains both names).
- [ ] `Beta` whose frontmatter holds a `blocked_by` list with one entry; `<bin> --config <cfg> goal add Beta blocked_by "[[Blocker C]]"` exits 0 and `goal list --output json` emits both entries, while `<bin> --config <cfg> goal set Beta blocked_by "[[Blocker D]]"` exits non-zero with stderr naming `blocked_by` — evidence: exit codes plus stdout JSON match plus stderr match.
- [ ] `Alpha` whose frontmatter holds the scalar `blocked_by: Blocker A`; `<bin> --config <cfg> task add Alpha blocked_by "[[Blocker B]]"` exits non-zero, stderr names `blocked_by` and the expected list shape, and the sha256 of `Tasks/Alpha.md` is unchanged; a second invocation whose value contains a newline (`"Blocker B\nstatus: completed"`) also exits non-zero with the sha256 unchanged — evidence: exit code non-zero, stderr match, negative evidence (file hash identical before and after).
- [ ] `<bin> --config <cfg> task set Alpha blocked_by "[[Blocker A]]"` exits non-zero, stderr names `blocked_by` and contains the token `add`, and the sha256 of `Tasks/Alpha.md` is unchanged; in the same run `<bin> --config <cfg> task set Alpha tags "a,b"` and `task set Alpha goals "g1,g2"` both exit 0, and `task list --output json` for `Alpha` shows `"tags":["a","b"]` and `"goals":["g1","g2"]` — evidence: exit codes, stderr match, negative evidence (file hash identical), stdout JSON match (the `tags`/`goals` assertions are the regression lock against an over-broad refusal).
- [ ] `<bin> --config <cfg> task set Alpha blocked_by ""` exits 0 and `task list --output json` emits neither a `blocked_by` key nor a `blocked` key for `Alpha`; on a task whose frontmatter holds a `blocked_by` list, `<bin> --config <cfg> task clear Alpha blocked_by` exits 0 and `grep -c '^blocked_by:' Tasks/Alpha.md` returns 0 — evidence: exit codes plus negative evidence (JSON keys absent, grep count 0).
- [ ] `<bin> --config <cfg> task validate Alpha` on the scalar fixture (`blocked_by: Blocker A`) exits non-zero, and its stdout contains exactly one issue line, which names `blocked_by` and the expected list shape — evidence: exit code non-zero plus stdout line count and content (the "exactly one line" assertion is what rules out an unrelated issue causing the non-zero exit).
- [ ] `<bin> --config <cfg> task validate Alpha` on the list fixture — byte-identical to the scalar fixture except that the `blocked_by` line is a YAML list with one entry — exits 0 and its stdout contains no `blocked_by` line; the same command on a fixture whose only `blocked_by` line is the empty scalar `blocked_by: ""` exits 0; and on the scalar fixture `<bin> --config <cfg> task validate Alpha --output json` exits 0 with a non-empty `issues` array whose description names `blocked_by` — evidence: exit codes plus stdout content plus negative evidence (the scalar fixture still reads as unblocked: `task list --output json` emits no `blocked_by` and no `blocked` key for it).
- [ ] `<bin> --config <cfg> task lint --fix` over the fixture vault (which contains the scalar fixture) exits non-zero, and the sha256 of the scalar fixture file is unchanged — evidence: exit code non-zero plus negative evidence (file hash identical — the rule reports and never mutates).
- [ ] `go test ./pkg/ops/...` exits 0 and the suite contains `DescribeTable` blocks with `Entry` rows covering the append case (including the append onto a non-empty list that must keep its existing entries) and the `set` refusal case — evidence: exit code 0 plus `grep -c 'Entry(' pkg/ops/frontmatter_entity_test.go` returning ≥ 2 and `grep -c 'DescribeTable' pkg/ops/frontmatter_entity_test.go` returning ≥ 1.
- [ ] `docs/task-writing.md` and `docs/goal-writing.md` each name `vault-cli task add` / `vault-cli goal add` as the way to record a dependency, state that a scalar value is reported by `vault-cli task validate`, and give the repair sequence for a scalar-shaped file (`set … blocked_by ""` or `clear … blocked_by`, then `add`); `CHANGELOG.md` carries an `## Unreleased` bullet prefixed `fix:` describing the write-path repair — evidence: file content — `grep -c 'blocked_by' docs/task-writing.md docs/goal-writing.md` returns ≥ 1 for each file, `grep -n 'task add' docs/task-writing.md` returns a line, and `grep -A20 '^## Unreleased' CHANGELOG.md` returns a line starting with `- fix:`.
- [ ] **Post-Deploy (Rung-2):** the released binary is installed and the reproduction replay runs against it — on a scratch task in a temp vault, `vault-cli task add "<scratch>" blocked_by "[[Blocker]]"` exits 0 and `vault-cli task list --output json` then emits `"blocked_by":["[[Blocker]]"]` for that task; and the `vault-cli task lint --vault personal` sweep count for the new rule is recorded in the source task's `# Results` section — evidence: exit code plus stdout JSON match plus file content.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`

**Scenario coverage: no new scenario.** The append and refusal rules are reachable by unit tests over the list operations, and the validate exit-code contract is reachable by the existing `integration/` harness, which builds the real binary and asserts exit codes against a temp vault — no Docker, no cluster, no `gh`, no external service. No essential user journey beyond that CLI contract depends on this, and the repository's existing `scenarios/001`–`005` still run unchanged as part of the release gate. None of the four conditions in dark-factory `docs/rules/scenario-writing.md` holds, so the default applies: no new scenario.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
grep -n 'blocked_by' pkg/ops/frontmatter_entity.go          # ≥1 line (smoke; the ACs carry the functional proof)
grep -c 'Entry(' pkg/ops/frontmatter_entity_test.go         # ≥2
grep -n 'blocked_by' docs/task-writing.md docs/goal-writing.md   # ≥1 line each
grep -A20 '^## Unreleased' CHANGELOG.md                     # must contain a "- fix:" bullet
```

### Operator-executable (runs on the host after PR merge)

```
# Release gate — mandatory before make install, per docs/releasing-vault-cli.md
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
/tmp/new-vault-cli --version
ls scenarios/*.md        # walk each scenario's Action + Expected against /tmp/new-vault-cli

# Reproduction replay against the fresh binary (see the Acceptance Criteria fixtures)
/tmp/new-vault-cli --config <fixture-config> task add Alpha blocked_by "[[Blocker B]]"
/tmp/new-vault-cli --config <fixture-config> task list --output json | grep blocked_by
/tmp/new-vault-cli --config <fixture-config> task validate Alpha     # scalar fixture → exit 1

# Version alignment, then install
make release-check
make install
vault-cli --version

# Install the Claude Code plugin manifests (separate artifact)
claude plugin update vault-cli@vault-cli   # then restart Claude Code

# Sweep — record the count for the source task's Results
vault-cli task lint --vault personal
```

## Desired Behavior

1. `vault-cli task add <name> blocked_by <value>` and `vault-cli goal add <name> blocked_by <value>` append exactly one entry to the field's existing list, preserve every entry already present, and exit 0 reporting the field and value they added. Every list-typed field reachable through `add` is enumerated here with a per-field verdict — no list-typed field is left with an unstated status:

   | Field | Entity | List-typed | `add` verdict | `set` verdict |
   |---|---|---|---|---|
   | `blocked_by` | task | yes | **appended (new)** | **refused with a pointer to `add`** |
   | `blocked_by` | goal | yes | **appended (new)** | **refused with a pointer to `add`** |
   | `goals` | task | yes | appended (already) | comma-split coercion (unchanged) |
   | `tags` | task | yes | appended (already) | comma-split coercion (unchanged) |
   | `tags` | goal, theme, objective, vision | yes | appended (already) | comma-split coercion (unchanged) |
   | `blocked_by` | theme, objective, vision | no accessor | excluded — no `blocked_by` in their model | excluded — same |
   | `theme`, `status`, `assignee`, `priority`, dates | all | no — scalar | excluded — `not a list field` | scalar (unchanged) |

2. `add` refuses when the field's current value is a non-empty scalar: it exits non-zero, its message names the field, states the expected list shape, and names the `clear` remedy, and it writes nothing. `set` refuses any non-empty value for `blocked_by`: it exits non-zero and its message names the field and points at `add`. Neither refusal touches the file — no partial frontmatter, no key reordering. `add` also refuses a value containing a newline or a carriage return, with no write. `set <name> blocked_by ""` and `clear <name> blocked_by` remain the legal clears and keep their current effect: the first empties the effective list, the second removes the key.
3. `tags` and `goals` keep their existing `set` behavior (a non-empty value comma-splits into a list) and their existing `add` behavior — the refusal rule applies only to list-typed fields with no list form in `set`, which today is `blocked_by` alone. Duplicate detection keeps the existing exact-string semantics of the shared list-mutation helper: `X` and `[[X]]` are two distinct entries that resolve to the same blocker, and blocked state is unaffected.
4. `vault-cli task validate <name>` and `vault-cli task lint` report a `blocked_by` whose value is a non-empty scalar as one issue that names the field and the expected list shape. The check does not fire for a YAML list value, for the empty scalar value, or for an absent key. The check is not fixable: `task lint --fix` reports it and leaves the file byte-identical.
5. `vault-cli task validate <name>` in plain output exits non-zero when that issue fires and exits 0 when it does not. `vault-cli task validate <name> --output json` keeps exit code 0 when the issue fires and lists the issue in its `issues` array — the existing JSON exit contract is unchanged.
6. The two new behaviors carry Ginkgo v2 `DescribeTable`/`Entry` coverage: an append row set that includes the append-onto-a-non-empty-list case, and a refusal row set for `set`. `docs/task-writing.md` and `docs/goal-writing.md` name the `add` recording path, state the validate rule, and give the repair sequence for a scalar-shaped file; `CHANGELOG.md` carries an `## Unreleased` `fix:` bullet.

## Assumptions

- The `add` and `set` commands keep their current argument shapes (`add <name> <field> <value>`, `set <name> <key> <value>`); no new subcommand or flag is introduced.
- The list-mutation helper's append and duplicate behavior is the correct contract for this field too — a dependency is recorded once per exact string.
- An empty scalar `blocked_by: ""` is a legitimate state (the documented clear), not malformed data — it reads as an empty list, so the validate rule leaves it alone.
- The vault is writable at command time and the entity file is not a symlink; both preconditions already hold for every write command in this CLI.

## Constraints

- The read side is frozen: the `blocked_by` list reader continues to reject a scalar value by returning an empty list, and the existing test asserting that a scalar reads as no dependency list must still pass. Spec 046 owns that behavior and this spec does not relax it.
- The `blocked_by` JSON field, the computed `blocked` boolean, and `next-task` filtering are unchanged — no new key, no re-typed field, no change to when either key is emitted.
- `task set` / `goal set` behavior for every field other than `blocked_by` is unchanged, including the comma-split coercion for `tags` and `goals` and the `unknown field` pass-through for keys outside the known set.
- `task lint --fix` must not become able to repair a malformed `blocked_by`; the new issue is reported with the not-fixable flag so the fix path skips it.
- `vault-cli task validate --output json` keeps exit code 0 when issues exist; only the plain path exits non-zero, as it does today.
- Entity-name resolution, file permissions (0600), and the symlink refusal in the write path are unchanged.
- Tests follow repository convention: Ginkgo v2 / Gomega with `DescribeTable` and `Entry` for the new table-driven coverage; existing stdlib `TestXxx` tests elsewhere are untouched.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---|---|---|---|---|---|
| `add` on a file whose `blocked_by` is a non-empty scalar (legacy shape) | Refused; message names the field, the expected list shape, and the `clear` remedy; file byte-identical | `set <name> blocked_by ""` (or `clear <name> blocked_by`), then `add` | Exit code non-zero plus the stderr message; `task validate` also names the file | Reversible — nothing written | Two concurrent `add` calls on the same task: last write wins, one entry is lost (same last-write-wins class the existing `goals`/`tags` add already has; no lock is added) |
| Operator passes a comma-joined value to `set … blocked_by "A,B"` | Refused; no coercion | Run `add` once per entry | Exit code non-zero plus stderr naming `add` | Reversible | — |
| Operator adds the same blocker twice in different spellings (`X` and `[[X]]`) | Two entries that resolve to the same blocker; blocked state unaffected | `task remove <name> blocked_by "[[X]]"` | `task list --output json` shows both entries | Reversible | — |
| `add` value contains a newline or carriage return | Refused; no write | Re-invoke with a single-line value | Exit code non-zero | Reversible | — |
| Vault-wide `task lint` now exits non-zero because a legacy scalar file exists | Intended surfacing: the issue is reported and the sweep fails until the files are repaired | Repair each flagged file (`set ""` then `add`), or accept the non-zero exit until then | `task lint` exit code plus the per-file issue line | Reversible | — |
| Crash mid-write (pre-existing property of the storage layer, unchanged by this spec) | Task file can be truncated; no new partial state is introduced by this spec | Restore the file from the vault's autocommit history, then re-run the command | `task get <name>` fails or the frontmatter parse error is reported | Partial — the write path is not atomic today; making it atomic is out of scope | — |

## Security / Abuse Cases

- **Attacker-controllable input:** the `<value>` argument of `add` and `set`, and the entity name used for file resolution. Values reach YAML frontmatter; a value carrying a newline or carriage return is refused outright so it cannot introduce a frontmatter key or a second list entry, and values are stored as YAML list items, so a crafted string cannot escape into a top-level key.
- **Trust boundaries:** the CLI writes only inside the configured vault directories, resolved through the existing entity-name lookup — no shell interpolation, no path traversal beyond what the existing commands already do. The symlink refusal in the write path stays in force.
- **What can hang or retry forever:** nothing — both commands are single find, mutate, write sequences with no retry loop and no network I/O.
- **What must be validated:** the field name against the list-field allowlist (an unknown or scalar field still fails), the value against the newline/CR rule, and the existing frontmatter value's shape before appending (a non-empty scalar is refused rather than overwritten).

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `add blocked_by` append for tasks and goals; refusal when the current value is a non-empty scalar; value sanitization; `DescribeTable` append rows | 1, 2 | 1, 2, 3, 9 | — |
| 2 | `set` refusal for `blocked_by` (task + goal) with a pointer to `add`; clears unchanged; `tags`/`goals` coercion regression-locked; `DescribeTable` refusal rows | 2, 3 | 4, 5, 9 | prompt 1 (shares the list-typed-field classification the refusal keys on) |
| 3 | `task validate` / `task lint` scalar-list detector: names the field and the expected shape, non-fixable, plain exit non-zero, JSON exit unchanged | 4, 5 | 6, 7, 8 | — |
| 4 | Docs (`task-writing.md`, `goal-writing.md`) + `CHANGELOG.md` `## Unreleased` bullet | 6 | 10 | prompts 1–3 |

Rationale: prompt 1 establishes the recording path and the shape rule it enforces; prompt 2 reuses that same list-typed-field classification for the `set` refusal, so it must land after; prompt 3 is the detector and is independent of both, but its ACs assert the read side still rejects scalars, which is what makes prompt 1's refusal necessary rather than cosmetic; prompt 4 documents the surface prompts 1–3 created. AC 11 is operator-executed after merge and is not a prompt.

## Do-Nothing Option

`blocked_by` stays unwritable: dependencies cannot be recorded with a command, `set` keeps reporting success while writing a value every reader discards, and `validate` keeps passing on both shapes. The dependency graph stays invisible in the CLI, and the next ordering inversion caused by a silently absent dependency is again undetectable until a human notices the schedule ran in the wrong order. The cost is unbounded and already paid once.
