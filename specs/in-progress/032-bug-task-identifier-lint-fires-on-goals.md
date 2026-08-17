---
status: prompted
approved: "2026-08-17T08:12:14Z"
generating: "2026-08-17T08:15:18Z"
prompted: "2026-08-17T08:24:39Z"
branch: dark-factory/bug-task-identifier-lint-fires-on-goals
---

## Summary

- `vault-cli goal lint` reports `MISSING_TASK_IDENTIFIER` on goals, but goals have no identifier concept anywhere in the product — no domain field, no storage auto-generation, no reader, no documented schema entry.
- 268 of 269 goals across all vaults (99.6%) are flagged. The issue is not fixable by `--fix`, and its message tells the operator to "run backfill", which is a command that does not exist.
- The linter is what is out of step, not the goals. Identifier checks are task-only checks that the lint walker applies to every page type it is pointed at.
- The fix scopes the two identifier checks to the task collection, and makes the remaining task-side message name a remedy the operator can actually run.
- Goals are addressed by wikilink and renamed through `notesmd-cli`, which rewrites backlinks — they do not need a rename-stable ID, so no goal identifier is introduced.

## Problem

Every goal file in every vault that lacks a `task_identifier` key is reported as an error by `vault-cli goal lint`. That is 268 of 269 goals. The check is a task-only invariant — tasks get a UUID so they stay addressable across renames — but the lint walker runs it against whatever collection it was pointed at, with no page-type awareness. The result is a linter whose goal output is 99.6% noise, which trains the operator to ignore goal lint entirely and hides the real goal issues (orphan goals, status/phase mismatches) underneath.

The error is also unactionable: it is constructed non-fixable, so `--fix` does nothing, and its text instructs the operator to "run backfill" — an operation that exists in the codebase but is wired to no CLI command, so there is nothing to run. That second half is not cosmetic: **248 real tasks across the five vaults currently lack `task_identifier`** (Brogrammers 229, OpenBrain 12, Family 3, Personal 3, Gaming 1 — verified via `vault-cli task lint --vault <V> | grep -c MISSING_TASK_IDENTIFIER`). Those 248 are correct errors with no available remedy. Scoping the check to tasks without also wiring the remedy would leave the operator with 248 true errors and still nothing to run, so both halves belong in this fix.

## Goal

`vault-cli goal lint` (and theme / objective / vision lint) never reports identifier issues, because identifier checks only apply to the task collection. `vault-cli task lint` and `vault-cli task validate` are unchanged: a task without `task_identifier` is still an error on both paths. When that error fires, its message names a command the operator can execute, and that command exists.

## Reproduction

Verified live on 2026-08-17 against `vault-cli version v0.111.2-2-gc597c4d-dirty` (`vault-cli --version`, verbatim).

```
vault-cli goal lint --vault Brogrammers
```

Observed — one such line per goal without `task_identifier` (18 of 19 goals in Brogrammers, 268 of 269 across all vaults):

```
ERROR 23 Goals/<file>.md: MISSING_TASK_IDENTIFIER task_identifier is missing; run backfill to assign one
```

The advertised remedy does not exist:

```
vault-cli task backfill-identifiers   # no such command
vault-cli goal lint --fix --vault Brogrammers   # issue persists — constructed with Fixable: false
```

Per-vault flagged counts, verified via `vault-cli goal lint --vault <V> | grep -c MISSING_TASK_IDENTIFIER`:

| Vault | Goals | flagged | with `task_identifier` | with `goal_identifier` |
|---|---|---|---|---|
| Brogrammers | 19 | 18 | 1 | 0 |
| Family | 10 | 10 | 0 | 0 |
| Gaming | 1 | 1 | 0 | 0 |
| OpenBrain | 16 | 16 | 0 | 0 |
| Personal | 223 | 223 | 0 | 6 |
| **Total** | **269** | **268** | **1** | **6** |

The 6 Personal-vault goals carrying `goal_identifier` are flagged too — that key is an abandoned experiment and no Go code reads it, so it does nothing to satisfy the check. The single goal carrying `task_identifier` (`Brogrammers/23 Goals/BRO-19718 Migrate S3 Services to Garage.md`) was set by hand on 2026-08-17 while investigating this bug — it is not evidence of a convention, it is why the count is 268 and not 269.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| `vault-cli goal lint` on a goal without any identifier | no identifier issue — goals have no identifier concept | `ERROR ... MISSING_TASK_IDENTIFIER` |
| `--fix` on that issue | either fixes it or the issue is never raised | issue persists (`Fixable: false`) |
| Error message remedy | names a runnable command | names "backfill", which is wired to no CLI command |
| `vault-cli task lint` on a task without `task_identifier` | `ERROR ... MISSING_TASK_IDENTIFIER` | same — correct today, must stay |
| `vault-cli task validate` on a task without `task_identifier` | reports the issue | same — correct today, must stay |

## Why this is a bug

The check asserts an invariant that the documented schema defines for tasks only.

**Documented source:** `docs/task-writing.md:90` lists `task_identifier: <uuid>  # generated by tooling` in the *task* frontmatter schema. `docs/goal-writing.md` has no identifier field at all — `grep -n identifier docs/goal-writing.md` returns nothing. The linter demands of goals a field that only the task schema documents.

The code agrees with the docs, not with the linter:

- `pkg/domain/goal.go` has no identifier field or accessor.
- `pkg/storage/goal.go`'s `WriteGoal` has no identifier auto-generation, unlike `pkg/storage/task.go:44-47` where `WriteTask` generates a UUID when `TaskIdentifier()` is empty.
- `TaskIdentifier` is referenced only by task-side code: `pkg/storage/task.go`, `pkg/ops/ensure_task_identifiers.go`, `pkg/ops/lint.go`, `pkg/domain/task_frontmatter.go`.

The linter therefore demands a key that no code writes for goals, no code reads for goals, and no documented goal workflow produces. Additionally, the message promises a remedy that cannot be invoked (`EnsureAllTaskIdentifiersOperation` exists in `pkg/ops/` but has no CLI wiring), which is independently a wrong user-visible error message.

## Context — do not retry the creator-side fix

Found on 2026-08-17 while scaffolding a goal via `/jira-sprint-sync`. The first attempted fix was a one-line change to `agents/goal-creator.md` instructing it to emit `task_identifier` on new goals (bborbe/vault-cli PR #92). That PR was closed unmerged once the vault data showed 268 of 269 existing goals carry no identifier at all: making the creator emit one would fix zero existing files, invent a field nothing reads, and cement the linter's wrong assumption. The linter is the defect. Do not re-open the creator-side approach.

## Non-goals

- Introducing `goal_identifier` as a modelled field, or any goal identifier at all.
- Migrating or removing the 6 existing `goal_identifier:` keys, or the 1 stray `task_identifier:` on a goal — they become inert, not errors.
- Changing how tasks get their `task_identifier`, or altering `WriteTask`'s auto-generation.
- Any change to the `goal-creator` / `task-creator` agent definitions.
- Do NOT add a flag to re-enable identifier checks on non-task page types — invariant; if a future consumer demands a goal identifier, that is a separate spec.
- Do NOT add `--dry-run`, `--limit`, or any tuning flag to the new backfill command; it takes only the vault selection flags that every other multi-vault command already takes (`--vault`, `--config`).
- Do NOT make `MISSING_TASK_IDENTIFIER` fixable via `--fix` — that is a behavior change to task linting, out of scope here.
- Do NOT rename the new command away from `task backfill-identifiers` — the name is pinned (see Constraints); it is quoted inside a user-visible error message.
- Do NOT refactor the other 66 inline `ops.New*` constructions in `pkg/cli/cli.go` to take injected operations — only the new command factory gets that seam (see Constraints).

## Acceptance Criteria

Container-executable evidence (ACs 1-8) runs at prompt time; operator-executable evidence (ACs 9-13) runs on the host after merge.

- [ ] `make precommit` exits 0 — evidence: exit code.
- [ ] A goal-collection file with no `task_identifier` produces zero identifier issues — evidence: a unit test row in `pkg/ops/lint_test.go` named for the goal page type asserts the returned issue slice contains no `MISSING_TASK_IDENTIFIER` and no `INVALID_TASK_IDENTIFIER`; `go test ./pkg/ops/...` exits 0.
- [ ] A task missing the key still reports the issue on the collection-walker path — evidence: a regression unit test row over `Execute` asserts a task file with no `task_identifier` yields exactly one `MISSING_TASK_IDENTIFIER` issue; `go test ./pkg/ops/...` exits 0.
- [ ] A task missing the key still reports the issue on the **single-file** path — evidence: a named test row in the existing `ExecuteFile` coverage (`pkg/ops/lint_validate_exit_test.go`; `pkg/ops/lint_test.go` is equally acceptable) calls `ExecuteFile` on a task fixture with no `task_identifier` and asserts exactly one `MISSING_TASK_IDENTIFIER` issue; `go test ./pkg/ops/...` exits 0.
- [ ] A goal carrying a non-UUID `task_identifier` produces zero issues (inert, not an error) — evidence: unit test row over a goal fixture with `task_identifier: not-a-uuid` asserts zero `INVALID_TASK_IDENTIFIER` issues.
- [ ] The new command is really wired, not a stub — evidence: a `pkg/cli` test using the existing counterfeiter mock `mocks.EnsureAllTaskIdentifiersOperation` (`mocks/ensure-all-task-identifiers-operation.go:11`) asserts `ExecuteCallCount() == 1` per selected vault after invoking the command's `RunE`; `go test ./pkg/cli/...` exits 0. (This is the first mock-based test in `pkg/cli` — see the injection-seam Constraint.)
- [ ] Docs name the new command — evidence: `grep -n 'backfill-identifiers' README.md` returns a line inside the task command block (`README.md:88-93`), and `grep -n 'backfill-identifiers' docs/task-writing.md` returns ≥1 line (the `task_identifier` schema comment at ~line 90 names the command that assigns one).
- [ ] `CHANGELOG.md` has an entry under `## Unreleased` describing both halves — evidence: `grep -n 'task_identifier' CHANGELOG.md` **and** `grep -n 'backfill-identifiers' CHANGELOG.md` each return a line inside the `## Unreleased` section.
- [ ] Real-vault sweep: identifier issues are gone from goal lint for all 269 goals (268 of which are flagged today) — evidence: for each of `Brogrammers Family Gaming OpenBrain Personal`, `/tmp/new-vault-cli goal lint --vault <V> | grep -c 'TASK_IDENTIFIER'` returns `0` (negative evidence, five runs).
- [ ] The other non-task collections are equally clean — evidence: `/tmp/new-vault-cli theme lint --vault Personal`, `objective lint`, and `vision lint` each return `0` for `grep -c 'TASK_IDENTIFIER'`.
- [ ] Task lint output is unchanged apart from the remedy wording — evidence: the identifier-normalised diff below produces empty output and exits 0, over Personal's 3 flagged tasks (`vault-cli` = installed pre-fix binary). **The `MISSING_TASK_IDENTIFIER` description text is the only permitted delta in task lint output; any other differing line fails this AC.**
  ```
  diff <(vault-cli task lint --vault Personal | sed 's/MISSING_TASK_IDENTIFIER.*/MISSING_TASK_IDENTIFIER/') \
       <(/tmp/new-vault-cli task lint --vault Personal | sed 's/MISSING_TASK_IDENTIFIER.*/MISSING_TASK_IDENTIFIER/')
  ```
- [ ] The remedy named in the `MISSING_TASK_IDENTIFIER` message is invokable — evidence: `grep -n 'run backfill to assign one' pkg/ops/lint.go` returns 0 lines, the new message contains the literal `vault-cli task backfill-identifiers`, and `/tmp/new-vault-cli task backfill-identifiers --help` exits 0.
- [ ] The backfill command actually assigns identifiers and reports what it did — evidence: against a scratch vault copy (one task with no `task_identifier`, addressed through its own config file because `--vault` resolves via the config loader at `pkg/cli/cli.go:31-44`), `/tmp/new-vault-cli task backfill-identifiers --config /tmp/scratch-config.yaml --vault scratch` exits 0 and its stdout matches `modified: 1` and `skipped: 0`; then `grep -c '^task_identifier: ' <that task file>` returns `1` and `/tmp/new-vault-cli task lint --config /tmp/scratch-config.yaml --vault scratch | grep -c MISSING_TASK_IDENTIFIER` returns `0`.

**Scenario coverage: no new scenario.** Unit tests reach both lint entry paths directly, the CLI mock test proves the command is wired, and the operator-executable rung replays the exact reproduction against real vaults. No Docker, cluster, or external service is involved.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
go test ./pkg/ops/... ./pkg/cli/...
grep -n 'run backfill to assign one' pkg/ops/lint.go                        # must return no lines
grep -n 'backfill-identifiers' pkg/ops/lint.go README.md docs/task-writing.md CHANGELOG.md
```

### Operator-executable (runs on the host after PR merge)

```
go build -o /tmp/new-vault-cli .
for V in Brogrammers Family Gaming OpenBrain Personal; do
  echo -n "$V goal: "; /tmp/new-vault-cli goal lint --vault "$V" | grep -c 'TASK_IDENTIFIER'
done                                     # every line must print 0
/tmp/new-vault-cli theme lint --vault Personal     | grep -c 'TASK_IDENTIFIER'   # 0
/tmp/new-vault-cli vision lint --vault Personal    | grep -c 'TASK_IDENTIFIER'   # 0
/tmp/new-vault-cli objective lint --vault Personal | grep -c 'TASK_IDENTIFIER'   # 0
diff <(vault-cli task lint --vault Personal | sed 's/MISSING_TASK_IDENTIFIER.*/MISSING_TASK_IDENTIFIER/') \
     <(/tmp/new-vault-cli task lint --vault Personal | sed 's/MISSING_TASK_IDENTIFIER.*/MISSING_TASK_IDENTIFIER/')
# single-file path must still report on a task known to lack the key
/tmp/new-vault-cli task validate "<a Brogrammers task with no task_identifier>" | grep -c MISSING_TASK_IDENTIFIER   # 1
/tmp/new-vault-cli task backfill-identifiers --help
# scratch-vault check (AC 13) — copy one identifier-less task into a scratch vault first
/tmp/new-vault-cli task backfill-identifiers --config /tmp/scratch-config.yaml --vault scratch
```

## Desired Behavior

1. The lint walker knows which page type it is linting, and the two identifier checks (`MISSING_TASK_IDENTIFIER`, `INVALID_TASK_IDENTIFIER`) run only when that page type is `task`.
2. **Page type comes from the lint entry point, not from frontmatter.** The lint command factory `createGenericLintCommand` **already takes a `pageType string` parameter** (`pkg/cli/cli.go:585`), today used only to build the `Short` help text (`cli.go:594`). It is supplied at all **five** registration sites — `cli.go:512` (task), `1245` (goal), `1502` (theme), `1579` (objective), `1700` (vision) — so the value is already total and correct at every walker entry point. The only new plumbing is factory → `ops.LintOperation.Execute`; nothing new needs to be threaded into the factory itself. Frontmatter `page_type` is NOT used: only 178 of 269 goals carry the key (Brogrammers 18/19, Family 4/10, Gaming 1/1, OpenBrain 16/16, Personal 139/223), so frontmatter-driven detection would leave 91 goals still flagged, and a "missing page_type ⇒ skip" default would silently stop checking tasks that lack the key too. Frontmatter is data; the entry point is the contract.
3. The single-file lint path (`ExecuteFile`) is the sixth entry point and is not covered by the factory's `pageType`: it has exactly one non-test caller, `createValidateCommand` at `pkg/cli/cli.go:571`, which resolves a task by name. That path continues to apply identifier checks — `vault-cli task validate` on a task with no `task_identifier` still reports it — and its task-only nature is made explicit at the call site rather than left implicit or defaulted.
4. All other lint checks (duplicate keys, invalid priority, invalid status, status/phase, status/date, orphan goals, status/checkbox) continue to run for every page type exactly as today.
5. A goal or theme carrying a stray `task_identifier` or `goal_identifier` key is inert — no issue of any type is raised for it.
6. `EnsureAllTaskIdentifiersOperation` is wired to the CLI as `vault-cli task backfill-identifiers`, taking only the vault selection flags sibling task commands take. It walks tasks in the selected vault(s), writes an identifier into each task missing one, leaves tasks that already have one untouched, and prints how many files it modified and how many it skipped.
7. The `MISSING_TASK_IDENTIFIER` message names that command verbatim instead of the word "backfill", so an operator can copy the message text into a shell and have it work against the 248 tasks that legitimately need it.

## Constraints

- `vault-cli task lint` and `vault-cli task validate` behavior for task files must not change **except** for the `MISSING_TASK_IDENTIFIER` description string — same issue types, same set of flagged files, same exit codes. `printLintIssuesPlain` (`pkg/cli/cli.go:671-684`) prints `issue.Description` verbatim, so that one string is the entire user-visible delta; AC 11 normalises exactly that line and nothing else.
- **The command name `task backfill-identifiers` is pinned**, not an implementation choice: it is written into a persistent user-visible error message and into this spec's reproduction. Changing it later means changing the error text again.
- **Injection seam, authorised for the new command only.** Every CLI command today constructs its operation inline inside `RunE` (`ops.NewLintOperation()` at `cli.go:570` and `:608`; 67 `ops.New*` sites in `cli.go` total), and no test in `pkg/cli` uses a counterfeiter mock today. AC 6 therefore requires a first-of-its-kind seam: the new command's factory takes the `EnsureAllTaskIdentifiersOperation` as a parameter (with the production call site passing the real one) so a fake can be injected. Every other command keeps inline construction — do not refactor them.
- `LintOperation` is a counterfeiter-generated interface (`pkg/ops/lint.go:40`, mock at `mocks/lint-operation.go`). Changing `Execute`'s signature to carry the page type requires regenerating the mock — `make precommit` fails otherwise.
- No new frontmatter key is introduced or written to any file by the lint path.
- `WriteTask`'s existing UUID auto-generation stays as-is; the new command reaches it through the existing task storage write path rather than generating identifiers itself.
- `pkg/ops/` stays a library layer: the new operation call returns a structured result and the CLI layer owns all output formatting (per `CLAUDE.md` Key Design Decisions).
- Existing tests in `pkg/ops/lint_test.go` and `pkg/ops/lint_validate_exit_test.go` must still pass unmodified except where a test asserted identifier issues on a non-task fixture.
- The lint JSON output shape (`LintIssueJSON`, `ValidateResult`) is a public contract for `/vault-cli:*` commands and must not change fields.
- Version alignment rules in `CLAUDE.md` apply: any release commit bumps `CHANGELOG.md` plus the three `.claude-plugin/` version fields together.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---|---|---|---|---|---|
| Page type reaches the lint operation as an unrecognised string | Identifier checks do not run; all other checks run normally (fail-open for identifier only) | Operator sees zero identifier issues; add the page type at the registration site | Unit test row for an unknown page-type literal | n/a | n/a |
| A future caller adds a lint entry point and forgets the page type | Identifier checks do not run for that caller — silently permissive, never a false error on goals | Add the page type at the new call site | Unit test asserting the task path still reports; new call sites are compile-visible because the page type is a required argument | n/a | n/a |
| The single-file path (`ExecuteFile`) is routed through the gate with a non-`task` value | `vault-cli task validate` stops reporting missing identifiers — a silent regression | Pass the task page type at `cli.go:571` | AC 4's `ExecuteFile` test row fails; operator-rung `task validate` check returns 0 instead of 1 | Reversible — code-only | n/a |
| Backfill command run against a vault whose task dir does not exist | Command exits non-zero with a message naming the missing directory; no files written | Operator corrects the vault name | Exit code + stderr | n/a — nothing written | n/a |
| Backfill interrupted mid-run (Ctrl-C, context cancel) | Already-written tasks keep their identifiers; the command reports the partial modified-file count rather than discarding it | Re-run the command; tasks that already have identifiers are skipped (idempotent) | Reported modified/skipped counts on stdout | Partial — written identifiers persist and are correct | Re-running is safe; a second concurrent run may write the same file twice but the second write is a no-op skip once the first has landed |
| Backfill hits an unparseable task file | That file is skipped and counted in `SkippedFiles`; the run continues | Operator fixes the file's frontmatter (`vault-cli task lint --fix`) and re-runs | Skipped count > 0 on stdout | n/a — file untouched | n/a |
| Backfill writes a task file while Obsidian has unsaved edits to it | vault-cli's write wins on disk; Obsidian may overwrite it on its next save | Operator re-runs backfill after closing the file | `vault-cli task lint` reports the identifier missing again | Reversible — re-run restores | Single-writer assumption; documented, not enforced |
| Disk full / read-only vault during backfill | Command exits non-zero naming the file that failed; earlier writes stand | Free space and re-run (idempotent) | Exit code + error naming the path | Partial | n/a |
| Backfill run across all vaults at once (no `--vault`) touches all 248 identifier-less tasks | All 248 get identifiers in one pass; autocommit vaults produce one large commit | Identifiers are additive and idempotent; no rollback needed | Modified-file count on stdout; `git status` in the vault | Reversible via vault git history | Single-writer assumption |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Thread page type from the five factory registration sites into `LintOperation.Execute`, gate the two identifier checks on `task`, pass the task page type explicitly at the `ExecuteFile` call site (`cli.go:571`); regenerate the lint mock; unit tests for goal/theme/task/unknown page types on both `Execute` and `ExecuteFile` | 1, 2, 3, 4, 5 | 1, 2, 3, 4, 5, 9, 10, 11 | — |
| 2 | Wire `EnsureAllTaskIdentifiersOperation` to `vault-cli task backfill-identifiers` behind an injected-operation factory seam; reword the `MISSING_TASK_IDENTIFIER` message to name it; first `pkg/cli` mock test; update `README.md`, `docs/task-writing.md`, `CHANGELOG.md` | 6, 7 | 1, 6, 7, 8, 12, 13 | prompt 1 (the message text lives behind the gate added there) |

Rationale: prompt 1 is the bug fix proper and is self-contained in the lint layer plus its six call sites; prompt 2 is the message-truthfulness half and touches the CLI command tree, and is the only prompt authorised to introduce the injection seam. Splitting keeps the regression-critical change (prompt 1) reviewable on its own. ACs 9-13 are the operator-executable rung and are checked once, after both prompts land.

## Do-Nothing Option

Goal lint stays 99.6% noise, so the operator keeps skipping it and real goal issues (orphan goal links, status/phase mismatches, duplicate keys) go unread. Every new goal created by `/vault-cli:create-goal` immediately joins the flagged set, and the 248 tasks that genuinely need identifiers keep pointing at a remedy that does not exist. The false error also invites exactly the wrong repair — PR #92 already tried to make the goal creator emit a meaningless UUID, and the next person reading the lint output will try it again. Not acceptable.

## Open Questions

1. Whether the six inert `goal_identifier:` keys in the Personal vault are eventually removed from those files is deliberately out of scope here; it needs no code and can be a one-off vault edit later.
