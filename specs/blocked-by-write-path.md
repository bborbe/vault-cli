---
status: draft
kind: bug
tags:
    - dark-factory
    - spec
---

## Blocked-by write path: record a dependency, refuse a malformed shape

## Summary

- `blocked_by` is the vault's only machine-readable dependency signal, and today nothing can write it: `add` rejects the field as unknown, and `set` writes a scalar that every reader is designed to ignore.
- This spec makes `vault-cli task add` and `vault-cli goal add` append a `blocked_by` entry — the recording path that does not exist.
- It makes `set` refuse a non-list value for the field and point at `add`, instead of silently writing a shape the system discards.
- It makes `remove` able to drop an entry — the same list-field allowlist that blocks `add` blocks `remove`, so the fix is shared and costs nothing extra.
- It makes `vault-cli task validate` report a scalar-shaped `blocked_by` as an error naming the field and the expected list shape, so the divergence is visible instead of silent.
- Nothing changes on the read side: a scalar still reads as an empty list, and the derived `blocked` flag, JSON emission, and `next-task` filtering keep the semantics spec 046 fixed.

## Problem

`blocked_by` is the vault's only machine-readable dependency signal: `/open --flagged` and the batch opener read it as JSON to decide what may start. Both write paths are broken, in three linked places, and nothing catches it. `vault-cli task add <task> blocked_by '[[X]]'` fails with `unknown field: "blocked_by"` because the task list-field allowlist holds only `goals` and `tags`, so a dependency cannot be recorded at all. `vault-cli task set <task> blocked_by '[[X]]'` takes the unknown-field branch and writes a scalar YAML value, which the dedicated list reader discards by design — so `set` records a dependency in a shape the system is built to ignore, and reports success while doing it. `vault-cli task validate` passes on both shapes, so the divergence has no detector. The observed cost: an ordering inversion in the Sunday shutdown maintenance chain went undetected because the dependency data was silently absent from the files that were supposed to declare it. The same defect exists for goals, whose documentation sanctions the same `set` and `clear` commands for the same field. A third verb is affected by the same allowlist: `vault-cli task remove <task> blocked_by '[[X]]'` also fails with `unknown field: "blocked_by"`, so an entry cannot be dropped either — the repair sequence the docs describe is unusable end to end.

## Reproduction

Smallest config: the Personal vault as configured in `~/.config/vault-cli/config.yaml` (a `vaults:` mapping with `tasks_dir: "25 Tasks"` and `goals_dir: "24 Goals"`). A bare `tasks_dir:`/`goals_dir:` file is not a valid config — the CLI exits `Error: no vaults configured`.

A scratch task with only `status`, `page_type`, `priority`, and `task_identifier` in its frontmatter. Against `vault-cli version v0.139.1`:

```
$ vault-cli task add "ZZ Scratch Repro Blockedby" blocked_by "[[Blocker A]]"
Error: unknown field: "blocked_by"
Error: unknown field: "blocked_by"
exit=1

$ vault-cli task set "ZZ Scratch Repro Blockedby" blocked_by "[[Blocker B]]"
✅ Set blocked_by=[[Blocker B]] on: ZZ Scratch Repro Blockedby
exit=0
```

The file after `set` — the write reported success and produced a scalar:

```
---
blocked_by: '[[Blocker B]]'
page_type: task
priority: 1
status: in_progress
task_identifier: 22222222-2222-2222-2222-222222222222
---
```

The third verb fails identically — an entry cannot be dropped either:

```
$ vault-cli task remove "ZZ Scratch Repro Blockedby" blocked_by "[[Blocker B]]"
Error: unknown field: "blocked_by"
Error: unknown field: "blocked_by"
exit=1
```

Every reader then discards what `set` wrote. `task get` echoes the raw scalar, and `task list --output json` emits **neither** key — no `blocked_by`, no `blocked`:

```
$ vault-cli task get "ZZ Scratch Repro Blockedby" blocked_by --output json
{ "key": "blocked_by", "name": "ZZ Scratch Repro Blockedby", "value": "[[Blocker B]]" }

$ vault-cli task list --output json | <this task>
{ "name": "ZZ Scratch Repro Blockedby", "status": "in_progress", "priority": 1,
  "vault": "personal", "category": "task", "modified_date": "2026-09-20T09:49:48Z" }
```

And nothing reports the divergence:

```
$ vault-cli task validate "ZZ Scratch Repro Blockedby"
✅ ZZ Scratch Repro Blockedby: no lint issues found
exit=0
```

## Expected vs Actual

Expected, per `docs/task-writing.md` § Dependencies: "a scalar value (`blocked_by: Blocker Task`) is malformed and reads as an empty list, so it never blocks", and the docs sanction `set` only for the empty-string clear (`set "…" blocked_by ""` / `clear "…" blocked_by`). So a scalar must be reported rather than silently accepted, and a dependency must be recordable as a list.

The `add` recording path this spec introduces does **not** exist in the docs today — § Dependencies documents only `clear` and `set ""`, and contains no `task add` invocation. Introducing and documenting that path is this spec's job (AC 11 writes it), not an existing sanctioned behavior being restored.

Actual: `add` rejects the field as unknown (exit 1), `set` writes the scalar and reports success (exit 0), the scalar is discarded by every reader, and `validate` passes on both shapes. The recorded dependency is invisible and the command that wrote it claimed success.

## Goal

A task or goal file can record "I cannot start until these are done" with one command, the recorded value is a YAML list that the existing readers actually see, and any file whose `blocked_by` is shaped so that no reader can see it is reported — tasks by `vault-cli task validate` and `vault-cli task lint`, goals by `vault-cli goal lint` — with the field named and the expected shape stated. No command silently writes a shape the read side ignores.

## Non-goals

- Rewriting the existing `blocked_by`-bearing task and goal files — this fixes the write path going forward and surfaces the malformed files (tasks via `task validate` / `task lint`, goals via `goal lint`); converting them is an operator action.
- Auditing or fixing every list-typed field across goals, themes, objectives, and visions beyond the enumeration in Desired Behavior 1.
- A general schema-validation framework — the validate rule is a targeted list-versus-scalar check for this field.
- Changing the read side: the scalar rejection in the `blocked_by` list reader, the computed `blocked` flag, its JSON emission, and `next-task` filtering stay exactly as spec 046 specified them.
- Adding `blocked_by` to themes, objectives, or visions — they have no `blocked_by` accessor and no dependency model; adding one is a separate spec.
- Do NOT add a `--force` / `--coerce` / `--allow-scalar` flag to `add` or `set` that bypasses the refusal — the refusal is the fix; a bypass re-opens the silent-divergence hole this spec closes. Invariant; if a future consumer demands variation, that is a separate spec.
- Do NOT add a config key or environment variable that downgrades the new validate rule to a warning. Invariant; same reason.
- Do NOT introduce a second issue type for the same condition — one rule, one issue type, reported by both `task validate` and `task lint` through their shared detector.
- No new scenario (see the scenario-coverage note under Acceptance Criteria).

## Alternatives Considered

| Alternative | Why rejected |
|---|---|
| Relax `blockedByList` to accept a scalar as a one-element list | Makes malformed data silently valid, and it is the exact failure mode this spec closes. Spec 046 specified the scalar rejection deliberately so a bad write fails safe; the write path is the thing that is wrong, not the reader. |
| Make `set` coerce a non-empty `blocked_by` value into a list | `set` would silently reinterpret a scalar — the same class of bug being fixed, just moved. It also diverges from `docs/task-writing.md` § Dependencies, which sanctions `set` only for the empty-string clear. |
| Add `blocked_by` to the `tags`/`goals` comma-split coercion | Comma-splitting a wikilink name would corrupt names containing commas and hides the "one entry per invocation" contract the `add` verb already expresses. |
| Leave the write path broken and only add the validate rule | Detects the divergence but does not close it — operators would still have no command that records a dependency. Detection without a recording path makes the field report-only. |
| Do nothing | See the Do-Nothing Option below. |

## Acceptance Criteria

Fixture vault for ACs 1, 2, 3, 4, 5, 6, 7, 8, 9: a temp vault with `Tasks/Alpha.md` and `Goals/Beta.md`, plus a config naming `tasks_dir: Tasks` and `goals_dir: Goals` (the shape the existing `integration/` harness builds). `<bin>` is the binary built from HEAD — `gexec.Build` inside `integration/`, or `/tmp/new-vault-cli` on the host. `Alpha`'s frontmatter holds the base keys `status: in_progress`, `page_type: task`, `priority: 1`, `task_identifier: <a well-formed UUID>`, and — the only non-base key — `blocked_by` in whichever shape the AC under test names (a one-entry list for AC 1, a scalar for AC 3, absent for AC 5). No other key is present, so the only issue any `validate` run can report is the one under test. The same base-key rule applies to `Goals/Beta.md`.

- [ ] `Alpha` whose frontmatter holds `blocked_by:` with one list entry `[[Blocker A]]`; `<bin> --config <cfg> task add Alpha blocked_by "[[Blocker B]]"` exits 0, and `<bin> --config <cfg> task list --output json` emits for `Alpha` a `blocked_by` array containing both `[[Blocker A]]` and `[[Blocker B]]`, in that order — evidence: exit code 0 plus stdout JSON match (the pre-existing entry surviving is the not-clobbering proof; the file content of `Tasks/Alpha.md` also contains both names).
- [ ] `Alpha` whose frontmatter holds a `blocked_by` list with two entries `[[Blocker A]]` and `[[Blocker B]]`; `<bin> --config <cfg> task remove Alpha blocked_by "[[Blocker B]]"` exits 0 and `task list --output json` emits a `blocked_by` array containing `[[Blocker A]]` and not `[[Blocker B]]` — evidence: exit code 0 plus stdout JSON match. This AC exists because `remove` gates on the same list-field allowlist as `add` (`knownTaskListFields`), so `task remove <t> blocked_by '[[X]]'` fails today with `unknown field: "blocked_by"` exactly as `add` does; the repair sequence this spec documents is unusable without it.
- [ ] `Beta` whose frontmatter holds a `blocked_by` list with one entry `[[Blocker E]]` (named so it cannot collide with the `[[Blocker C]]` this AC appends — `add` refuses a duplicate with a non-zero exit); `<bin> --config <cfg> goal add Beta blocked_by "[[Blocker C]]"` exits 0 and `goal list --output json` emits both entries, while `<bin> --config <cfg> goal set Beta blocked_by "[[Blocker D]]"` exits non-zero with stderr naming `blocked_by` — evidence: exit codes plus stdout JSON match plus stderr match.
- [ ] `Alpha` whose frontmatter holds the scalar `blocked_by: Blocker A`; `<bin> --config <cfg> task add Alpha blocked_by "[[Blocker B]]"` exits non-zero, stderr names `blocked_by`, states the expected list shape, and contains the token `clear`, and the sha256 of `Tasks/Alpha.md` is unchanged — evidence: exit code non-zero, stderr match on all three tokens, negative evidence (file hash identical before and after).
- [ ] `<bin> --config <cfg> task set Alpha blocked_by "[[Blocker A]]"` exits non-zero, stderr names `blocked_by` and contains the token `add`, and the sha256 of `Tasks/Alpha.md` is unchanged; in the same run `<bin> --config <cfg> task set Alpha tags "a,b"` and `task set Alpha goals "g1,g2"` both exit 0, `task list --output json` for `Alpha` shows a `goals` array containing `g1` and `g2`, `<bin> --config <cfg> task get Alpha tags` echoes `a,b`, and `grep -nE '^[[:space:]]*- a$' Tasks/Alpha.md` returns ≥ 1 line — evidence: exit codes, stderr match, negative evidence (file hash identical), stdout match on `goals` and on `task get tags`, plus file content for `tags` (the `tags`/`goals` assertions are the regression lock against an over-broad refusal). Two shape notes the evidence must respect: `task list --output json` structurally emits no `tags` key, so `tags` is asserted via `task get` and the file rather than the list JSON; and the YAML encoder writes list items at four-space indent, so the file grep is written indent-independently rather than pinning a leading-space count.
- [ ] `<bin> --config <cfg> task set Alpha blocked_by ""` exits 0 and `task list --output json` emits neither a `blocked_by` key nor a `blocked` key for `Alpha`; on a task whose frontmatter holds a `blocked_by` list, `<bin> --config <cfg> task clear Alpha blocked_by` exits 0 and `grep -c '^blocked_by:' Tasks/Alpha.md` returns 0 — evidence: exit codes plus negative evidence (JSON keys absent, grep count 0).
- [ ] `<bin> --config <cfg> task validate Alpha` on the scalar fixture (`blocked_by: Blocker A`) exits non-zero, and its stdout contains exactly one issue line, which names `blocked_by` and the expected list shape — evidence: exit code non-zero plus stdout line count and content (the "exactly one line" assertion is what rules out an unrelated issue causing the non-zero exit).
- [ ] `<bin> --config <cfg> task validate Alpha` on the list fixture — byte-identical to the scalar fixture except that the `blocked_by` line is a YAML list with one entry — exits 0 and its stdout contains no `blocked_by` line; the same command on a fixture whose only `blocked_by` line is the empty scalar `blocked_by: ""` exits 0; and on the scalar fixture `<bin> --config <cfg> task validate Alpha --output json` exits 0 with a non-empty `issues` array whose description names `blocked_by` — evidence: exit codes plus stdout content plus negative evidence (the scalar fixture still reads as unblocked: `task list --output json` emits no `blocked_by` and no `blocked` key for it).
- [ ] `<bin> --config <cfg> task lint --fix` over the fixture vault (which contains the scalar fixture) exits non-zero, and the sha256 of the scalar fixture file is unchanged — evidence: exit code non-zero plus negative evidence (file hash identical — the rule reports and never mutates).
- [ ] `Goals/Beta.md` whose frontmatter holds the scalar `blocked_by: Blocker C`; `<bin> --config <cfg> goal lint` over the fixture vault exits non-zero and its output contains a line naming `blocked_by`, while the same command on the list-shaped goal fixture exits 0 with no `blocked_by` line — evidence: exit codes plus stdout match. This AC exists because `task lint` walks only the tasks directory and `task validate` resolves only task names, so `goal lint` — a separate registration whose walk root is the goals directory — is the only surface that can report a scalar-shaped goal.
- [ ] `go test ./pkg/ops/...` exits 0, and the suite contains behavioral cases — table-driven per repository convention — asserting: (a) `add` on a task whose `blocked_by` is absent yields a one-entry list; (b) `add` on a task whose `blocked_by` holds one entry yields a two-entry list in order with the pre-existing entry first; (c) `add` on a task whose `blocked_by` is a non-empty scalar returns an error naming `blocked_by`; (d) `set` with a non-empty `blocked_by` value returns an error naming the field and `add` — evidence: exit code 0 plus each case asserting on the resulting file content or the returned error, not on the presence of test syntax. The cases may live in any `pkg/ops` test file; the assertion is what the cases prove, not which file holds them.
- [ ] `docs/task-writing.md` and `docs/goal-writing.md` each name `vault-cli task add` / `vault-cli goal add` as the way to record a dependency, state that a scalar value is reported by `vault-cli task validate`, and give the repair sequence for a scalar-shaped file (`set … blocked_by ""` or `clear … blocked_by`, then `add`); `CHANGELOG.md` carries an `## Unreleased` bullet prefixed `fix:` describing the write-path repair — evidence: file content — `grep -nE 'task add[^|]*blocked_by' docs/task-writing.md` returns ≥ 1 line and `grep -nE 'goal add[^|]*blocked_by' docs/goal-writing.md` returns ≥ 1 line (the recording path is stated, not merely the field mentioned), `grep -cE 'blocked_by.*validate|validate.*blocked_by' docs/task-writing.md` and the same for `docs/goal-writing.md` each return ≥ 1 (the validate rule is stated, run once per file so the count is per-file), `grep -nE 'clear .*blocked_by' -A2 docs/task-writing.md | grep -c 'add'` returns ≥ 1 (the repair sequence is stated **including its `add` follow-up** — the `clear`/`set ""` half alone already appears at `docs/task-writing.md:130` before this change, so it is not discriminating on its own), and the `## Unreleased` section in `CHANGELOG.md` carries a line starting with `- fix:`. Two shapes are explicitly NOT sufficient evidence: a bare `grep -c 'blocked_by'` (both files already contain seven occurrences before this change), and a `clear`-only grep (satisfied pre-change at line 130). The CHANGELOG assertion is branch-dependent by design: the repo's releaser renames `## Unreleased` to `## vX.Y.Z` when it cuts a release, so on a branch where the release has already been cut the bullet is asserted against the newest `## vX.Y.Z` section instead — same bullet, same `- fix:` prefix.
- [ ] **Post-Deploy (Rung-2):** the released binary is installed and the reproduction replay runs against it — on a scratch task in a temp vault, `vault-cli task add "<scratch>" blocked_by "[[Blocker]]"` exits 0 and `vault-cli task list --output json` then emits a `blocked_by` array containing `[[Blocker]]` for that task (the CLI pretty-prints with two-space indent, so the assertion is on the parsed array, not on a literal compact-JSON string); and the pre-existing reproduction from the Problem section no longer reproduces — evidence: exit code plus parsed stdout JSON plus the recorded replay transcript in the source task's `# Results` section.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0)`
  - The vault-wide sweep that accompanies this AC is recorded, not asserted, and lives in Verification — the Personal vault holds no scalar-shaped `blocked_by` entity files (every `blocked_by:` list found is list-shaped; the only non-entity occurrence is a documentation example), so a zero count is true by construction and proves nothing on its own.

**Scenario coverage: no new scenario.** The append and refusal rules are reachable by unit tests over the list operations, and the validate exit-code contract is reachable by the existing `integration/` harness, which builds the real binary and asserts exit codes against a temp vault — no Docker, no cluster, no `gh`, no external service. No essential user journey beyond that CLI contract depends on this, and the repository's existing `scenarios/001`–`005` still run unchanged as part of the release gate. None of the four conditions in dark-factory `docs/rules/scenario-writing.md` holds, so the default applies: no new scenario.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
grep -n 'blocked_by' pkg/ops/frontmatter_entity.go          # ≥1 line (smoke; the ACs carry the functional proof)
grep -nE 'task add[^|]*blocked_by' docs/task-writing.md     # ≥1 line (recording path stated)
grep -nE 'goal add[^|]*blocked_by' docs/goal-writing.md     # ≥1 line
grep -nE 'clear .*blocked_by' -A2 docs/task-writing.md | grep -c 'add'   # ≥1 (repair sequence incl. its add follow-up; the clear half alone already exists pre-change)
grep -A20 '^## Unreleased' CHANGELOG.md                     # must contain a "- fix:" bullet (on a release-cut branch, assert against the newest "## vX.Y.Z" section instead)
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

# Sweep — recorded, not asserted (the vault holds zero scalar-shaped blocked_by files today)
vault-cli task lint --vault personal
vault-cli goal lint --vault personal
```

## Desired Behavior

1. `vault-cli task add <name> blocked_by <value>` and `vault-cli goal add <name> blocked_by <value>` append exactly one entry to the field's existing list, preserve every entry already present, and exit 0 reporting the field and value they added; `vault-cli task remove <name> blocked_by <value>` and the goal equivalent drop exactly that entry and exit 0. Every list-typed field reachable through `add` or `remove` is enumerated here with a per-field verdict — no list-typed field is left with an unstated status:

   | Field | Entity | List-typed | `add` verdict | `set` verdict | `remove` verdict |
   |---|---|---|---|---|---|
   | `blocked_by` | task | yes | **appended (new)** | **refused with a pointer to `add`** | **removed (new)** |
   | `blocked_by` | goal | yes | **appended (new)** | **refused with a pointer to `add`** | **removed (new)** |
   | `goals` | task | yes | appended (already) | comma-split coercion (unchanged) | removed (already) |
   | `tags` | task | yes | appended (already) | comma-split coercion (unchanged) | removed (already) |
   | `tags` | goal, theme, objective, vision | yes | appended (already) | comma-split coercion (unchanged) | removed (already) |
   | `blocked_by` | theme, objective, vision | no accessor | excluded — no `blocked_by` in their model | excluded — same | excluded — same |
   | `theme`, `status`, `assignee`, `priority`, dates | all | no — scalar | excluded — `not a list field` | scalar (unchanged) | excluded — same |

   The `add` and `remove` verdicts share one cause and one fix: both gate on `knownTaskListFields`, so widening the allowlist for `blocked_by` enables both verbs. `remove` is in scope because the repair sequence this spec documents (drop a malformed entry, then re-add it as a list) is unusable without it.

2. `add` refuses when the field's current value is a non-empty scalar: it exits non-zero, its message names the field, states the expected list shape, and names the `clear` remedy, and it writes nothing. `set` refuses any non-empty value for `blocked_by`: it exits non-zero and its message names the field and points at `add`. Neither refusal touches the file — no partial frontmatter, no key reordering. `set <name> blocked_by ""` and `clear <name> blocked_by` remain the legal clears and keep their current effect: the first empties the effective list, the second removes the key.
3. `tags` and `goals` keep their existing `set` behavior (a non-empty value comma-splits into a list) and their existing `add` behavior — the refusal rule applies only to list-typed fields with no list form in `set`, which today is `blocked_by` alone. Duplicate detection keeps the existing exact-string semantics of the shared list-mutation helper: `X` and `[[X]]` are two distinct entries that resolve to the same blocker, and blocked state is unaffected.
4. `vault-cli task validate <name>`, `vault-cli task lint`, and `vault-cli goal lint` report a `blocked_by` whose value is a non-empty scalar as one issue that names the field and the expected list shape. Tasks are reached by `task validate` and `task lint`; goals are reached by `goal lint` alone, since `task lint` walks only the tasks directory and `task validate` resolves only task names. The check does not fire for a YAML list value, for the empty scalar value, or for an absent key. The check is not fixable: `task lint --fix` reports it and leaves the file byte-identical.
5. `vault-cli task validate <name>` in plain output exits non-zero when that issue fires and exits 0 when it does not. `vault-cli task validate <name> --output json` keeps exit code 0 when the issue fires and lists the issue in its `issues` array — the existing JSON exit contract is unchanged.
6. The new behaviors carry Ginkgo v2 `DescribeTable`/`Entry` coverage — an append row set that includes the append-onto-a-non-empty-list case, a remove row set, and a refusal row set for `set`. The assertions are behavioral: they exercise `add`, `remove`, and `set` and assert the resulting file content and returned error, not the presence of test syntax. Separately, `docs/task-writing.md` and `docs/goal-writing.md` name the `add` recording path, state the validate rule, and give the repair sequence for a scalar-shaped file; `CHANGELOG.md` carries an `## Unreleased` `fix:` bullet.

## Assumptions

- The `add` and `set` commands keep their current argument shapes (`add <name> <field> <value>`, `set <name> <key> <value>`); no new subcommand or flag is introduced.
- The list-mutation helper's append and duplicate behavior is the correct contract for this field too — a dependency is recorded once per exact string.
- An empty scalar `blocked_by: ""` is a legitimate state (the documented clear), not malformed data — it reads as an empty list, so the validate rule leaves it alone.
- The vault is writable at command time and the entity file is not a symlink; both preconditions already hold for every write command in this CLI.

## Constraints

- The read side is frozen: the `blocked_by` list reader continues to reject a scalar value by returning an empty list, and the existing test asserting that a scalar reads as no dependency list must still pass. Spec 046 owns that behavior and this spec does not relax it.
- The `blocked_by` JSON field, the computed `blocked` boolean, and `next-task` filtering are unchanged — no new key, no re-typed field, no change to when either key is emitted.
- `task set` / `goal set` behavior for every field other than `blocked_by` is unchanged, including the comma-split coercion for `tags` and `goals` and the `unknown field` pass-through for keys outside the known set.
- `add` and `remove` keep their existing append/remove semantics for every field already in the allowlist (`goals`, `tags`); widening the allowlist adds `blocked_by` and changes nothing about how the existing fields mutate.
- The YAML encoder's four-space list indent is not changed — the spec's file-content greps are written to be indent-independent rather than pinning a format the encoder owns.
- `task lint --fix` must not become able to repair a malformed `blocked_by`; the new issue is reported with the not-fixable flag so the fix path skips it.
- `vault-cli task validate --output json` keeps exit code 0 when issues exist; only the plain path exits non-zero, as it does today.
- Entity-name resolution, file permissions (0600), and the symlink refusal in the write path are unchanged.
- Tests follow repository convention: Ginkgo v2 / Gomega with `DescribeTable` and `Entry` for the new table-driven coverage; existing stdlib `TestXxx` tests elsewhere are untouched. The test cases may live in any `pkg/ops` test file — the spec does not pin a file.
- `docs/task-writing.md` and `docs/goal-writing.md` § Dependencies already own this field's semantics and are the docs this spec updates; no new `docs/` page is introduced.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---|---|---|---|---|---|
| `add` on a file whose `blocked_by` is a non-empty scalar (legacy shape) | Refused; message names the field, the expected list shape, and the `clear` remedy; file byte-identical | `set <name> blocked_by ""` (or `clear <name> blocked_by`), then `add` | Exit code non-zero plus the stderr message; `task validate` also names the file | Reversible — nothing written | Two concurrent `add` calls on the same task: last write wins, one entry is lost (same last-write-wins class the existing `goals`/`tags` add already has; no lock is added) |
| Operator passes a comma-joined value to `set … blocked_by "A,B"` | Refused; no coercion | Run `add` once per entry | Exit code non-zero plus stderr naming `add` | Reversible | — |
| Operator adds the same blocker twice in different spellings (`X` and `[[X]]`) | Two entries that resolve to the same blocker; blocked state unaffected | `task remove <name> blocked_by "[[X]]"` (in scope — see DB1; it gates on the same allowlist as `add`) | `task list --output json` shows both entries | Reversible | — |
| A legacy scalar-shaped `blocked_by` file exists in a vault | Intended surfacing: the issue is reported on the next `task validate` / `task lint` (tasks) or `goal lint` (goals) run over that file | Repair the flagged file (`set ""` then `add`) | The per-file issue line naming `blocked_by` and the expected shape | Reversible | — |
| Crash mid-write (pre-existing property of the storage layer, unchanged by this spec) | Task file can be truncated; no new partial state is introduced by this spec | Restore the file from the vault's autocommit history, then re-run the command | `task get <name>` fails or the frontmatter parse error is reported | Partial — the write path is not atomic today; making it atomic is out of scope | — |

## Security / Abuse Cases

- **Attacker-controllable input:** the `<value>` argument of `add` and `set`, and the entity name used for file resolution. Values reach YAML frontmatter as YAML list items, so a crafted string cannot escape into a top-level key — a value containing a newline is serialized by the YAML encoder as a block scalar (`- |-`), which nests under the list item and cannot introduce a sibling key. No newline-specific guard is added; the list-item encoding is the containment.
- **Trust boundaries:** the CLI writes only inside the configured vault directories, resolved through the existing entity-name lookup — no shell interpolation, no path traversal beyond what the existing commands already do. The symlink refusal in the write path stays in force.
- **What can hang or retry forever:** nothing — both commands are single find, mutate, write sequences with no retry loop and no network I/O.
- **What must be validated:** the field name against the list-field allowlist (an unknown or scalar field still fails), and the existing frontmatter value's shape before appending (a non-empty scalar is refused rather than overwritten).

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | List-field allowlist widened for `blocked_by`, enabling `add` append **and** `remove` for tasks and goals; refusal when the current value is a non-empty scalar; behavioral append/remove/refusal test rows | 1, 2 | 1, 2 (add half), 3, 4, 11 | — |
| 2 | `set` refusal for `blocked_by` (task + goal) with a pointer to `add`; clears unchanged; `tags`/`goals` coercion regression-locked; behavioral refusal test rows | 2, 3 | 2 (set half), 5, 6, 11 | prompt 1 (shares the list-typed-field classification the refusal keys on) |
| 3 | `task validate` / `task lint` / `goal lint` scalar-list detector: names the field and the expected shape, non-fixable, plain exit non-zero, JSON exit unchanged, goals reachable via `goal lint` | 4, 5 | 7, 8, 9, 10 | — |
| 4 | Docs (`task-writing.md`, `goal-writing.md`) + `CHANGELOG.md` `## Unreleased` bullet | 6 | 12 | prompts 1–3 |

Rationale: prompt 1 establishes the recording path and the shape rule it enforces; prompt 2 reuses that same list-typed-field classification for the `set` refusal, so it must land after; prompt 3 is the detector and is independent of both, but its ACs assert the read side still rejects scalars, which is what makes prompt 1's refusal necessary rather than cosmetic; prompt 4 documents the surface prompts 1–3 created. AC 11 (test coverage) is split across prompts 1 and 2 because each prompt owns the behavior it tests; AC 12 (docs + CHANGELOG) is prompt 4. AC 13 is operator-executed after merge and is not a prompt. `add` and `remove` share prompt 1 because they share one cause — both gate on `knownTaskListFields`.

## Do-Nothing Option

`blocked_by` stays unwritable: dependencies cannot be recorded with a command, `set` keeps reporting success while writing a value every reader discards, and `validate` keeps passing on both shapes. The dependency graph stays invisible in the CLI, and the next ordering inversion caused by a silently absent dependency is again undetectable until a human notices the schedule ran in the wrong order. The cost is unbounded and already paid once.
