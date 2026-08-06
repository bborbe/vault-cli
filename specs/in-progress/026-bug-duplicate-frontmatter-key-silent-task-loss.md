---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-08-06T16:49:17Z"
generating: "2026-08-06T16:51:54Z"
prompted: "2026-08-06T17:03:41Z"
verifying: "2026-08-06T17:51:15Z"
branch: dark-factory/bug-duplicate-frontmatter-key-silent-task-loss
---

## Summary

- When git-rest and vault-cli both stamp `task_identifier` on the same task file, the two writes land on different frontmatter lines and git merges them cleanly into a file with the key defined twice — invalid YAML.
- Such a file disappears from `vault-cli task list` with **exit code 0 and no output at all**: the enumeration path downgrades the parse failure to a DEBUG log line that nobody sees. A task can stay corrupted and invisible indefinitely.
- `vault-cli task lint` already flags the file as `DUPLICATE_KEY`, and `--fix` already repairs it — but `--fix` **keeps the wrong value**: it retains the duplicate git-rest prepended at line 1 and discards the one vault-cli itself wrote at its sorted position.
- After the fix, a duplicated top-level frontmatter key resolves to the last occurrence, and no task file can vanish from a listing without the operator being told.
- Ships as a released, locally installed vault-cli version, then a lint sweep across the Personal, Trading, and OpenBrain vaults.

## Problem

Two independent writers stamp identifiers into the same vault task files. The server-side git-rest service prepends `task_identifier` at frontmatter line 1; vault-cli writes it at its alphabetically sorted position. When both stamp a task that had no identifier yet, the writes touch different lines, so git merges them without a conflict and produces frontmatter with `task_identifier` defined twice. That is invalid YAML. Every vault-cli command that parses the file fails, and the task-enumeration path swallows that failure at DEBUG level — so the task silently drops out of listings while the command reports success. A June 2026 fix closed the *loud* variant of this race (same-line collisions, which git-rest quarantined into `_conflicts/`); this is the *quiet* variant, and it inverts the safety property that work relied on. Quarantine was noisy but recoverable; this is unbounded. The one instance found on 2026-08-05 was caught only because an unrelated command happened to address that task by name.

## Goal

A vault task file whose frontmatter repeats a top-level key is never silently absent from a vault-cli listing, and `vault-cli task lint --fix` repairs it by keeping the value vault-cli itself wrote — leaving the file parseable and the task queryable again.

## Non-goals / Out of Scope

- Do NOT change git-rest to write `task_identifier` at its sorted position so future collisions conflict loudly — different repository, filed as its own task.
- Do NOT remove the `WriteTask` identifier-stamping fallback — retained as the defensive net per the June 2026 decision.
- Do NOT address the pre-existing `STATUS_CHECKBOX_MISMATCH`, `ORPHAN_GOAL`, or `STATUS_PHASE_MISMATCH` findings in the Personal vault.
- Do NOT change how any skill creates task files — direct `Write` without a `task_identifier` is the sanctioned path; the repair belongs in lint, not in every caller.
- Do NOT introduce a new `DUPLICATE_FRONTMATTER_KEY` issue type. `DUPLICATE_KEY` already exists and already fires on the real corrupted file (verified below); a second type would be a duplicate rule with a different name.
- Do NOT add a flag, config key, or environment variable to opt out of the unreadable-file warning or to select keep-first vs keep-last — invariant; if a future consumer demands variation, that is a separate spec.
- Do NOT change the exit code of `task list` when a file is unreadable — see Constraints.

## Acceptance Criteria

- [ ] Running `task lint --fix` on a task file whose frontmatter contains `task_identifier` twice — once at line 1 and once at its alphabetically sorted position — leaves exactly one `task_identifier` in the file, and its value is the one from the sorted position. Evidence: file content — after the fix, `grep -c '^task_identifier:' <file>` returns `1`, and `grep '^task_identifier:' <file>` contains the sorted-position UUID and does not contain the line-1 UUID.
- [ ] `task get "<name>" status` on that repaired file exits 0 and prints the status value. Evidence: exit code 0 plus stdout containing the status string. Before the fix the same command exits 1 with `unmarshal yaml frontmatter: yaml: unmarshal errors: ... mapping key "task_identifier" already defined at line 1`.
- [ ] `task lint` (no `--fix`) on that file prints a `DUPLICATE_KEY` finding naming `task_identifier`. Evidence: stdout line matching `DUPLICATE_KEY key "task_identifier" defined multiple times`.
- [ ] `task lint` on a task file with an arbitrary top-level key repeated (a key other than `task_identifier`, e.g. `assignee`) prints a `DUPLICATE_KEY` finding naming that key. Evidence: stdout line matching `DUPLICATE_KEY key "assignee"`.
- [ ] `task lint` on a task file where every top-level key appears once reports no `DUPLICATE_KEY` finding. Evidence: negative — `task lint` stdout piped through `grep -c DUPLICATE_KEY` returns `0` for that file.
- [ ] `task lint` on a task file whose frontmatter contains an indented nested mapping key and a YAML list whose entries reuse names that also appear as top-level keys reports no `DUPLICATE_KEY` finding for that file. Evidence: negative — `task lint` stdout contains zero lines naming that file with `DUPLICATE_KEY`.
- [ ] `task list` over a tasks directory containing one parseable task and one duplicate-key task names the unparseable file on **stderr** together with the parse error, while stdout still lists the parseable task. Evidence: stderr contains a line naming the unparseable file's path and the substring `already defined`; stdout contains the parseable task's name and does not contain the unparseable file's name.
- [ ] `task list` in that same situation still exits 0, and `task list --output json` still emits parseable JSON on stdout. Evidence: exit code 0, and stdout piped to a JSON parser exits 0.
- [ ] The unit test that previously asserted duplicate-key repair keeps the *first* occurrence now asserts it keeps the *last* occurrence, and no test or comment in the repository still describes keep-first behavior. Evidence: negative — `grep -rni "first occurrence" pkg/` returns 0 lines (this also catches the two stale doc-comments at `pkg/ops/lint.go:840` and `:853`, not just the test name at `lint_test.go:194`).
- [ ] `task lint --fix` on the reproduction file touches only the duplicated key's surplus line — every other frontmatter key, its value, and its formatting are byte-identical to the pre-fix file. Evidence: `git diff --numstat <file>` after the fix shows exactly `0` insertions and `1` deletion (the removed line-1 occurrence).
- [ ] `make precommit` exits 0 at the repository root. Evidence: exit code 0.
- [ ] `CHANGELOG.md` contains an `## Unreleased` bullet prefixed `fix:` describing the duplicate-key repair change, and no version string in `.claude-plugin/` was hand-edited. Evidence: file content — `grep -A20 '^## Unreleased' CHANGELOG.md` returns a line starting with `- fix:`; and `git diff origin/master -- .claude-plugin/` is empty.
- [ ] **Post-Deploy (Rung-2):** the released version is installed locally and reports the released tag. Evidence: `vault-cli --version` stdout ends with the released tag.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** a `task lint` sweep has been run against the Personal, Trading, and OpenBrain vaults with the newly installed binary (depends on the prior AC's install), and its findings are recorded in the source task's `# Results` section. Evidence: file content — the task note `24 Tasks/Duplicate task_identifier merges silently and hides the task from vault-cli.md` contains a `# Results` heading naming the released version, the merged PR URL, and the per-vault `DUPLICATE_KEY` count.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`

**Scenario coverage: no new scenario.** Both behaviors are reachable from unit tests — duplicate-key repair is a pure content transformation, and the enumeration warning is exercisable against a temporary tasks directory. The existing `scenarios/001`–`004` still run unchanged as part of the release gate. None of the four conditions in dark-factory `docs/rules/scenario-writing.md` holds here.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
grep -rni "first occurrence" pkg/                 # must return 0 lines
grep -A20 '^## Unreleased' CHANGELOG.md           # must contain a "- fix:" bullet
```

### Operator-executable (runs on the host after PR merge)

```
# Release gate — mandatory before make install, per docs/releasing-vault-cli.md
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
/tmp/new-vault-cli --version
ls scenarios/*.md        # walk each scenario's Action + Expected against /tmp/new-vault-cli

# Reproduction replay against the fresh binary (see Reproduction section)
# Version alignment before any plugin release
make release-check

# Install the binary (make install) — AC's deploy_check measures the vault-cli BINARY, not the plugin
make install
vault-cli --version

# Install the Claude Code plugin manifests (separate artifact, commands/agents only)
claude plugin update vault-cli@vault-cli   # then restart Claude Code

# Sweep
vault-cli task lint --vault personal
vault-cli task lint --vault trading
vault-cli task lint --vault openbrain
```

## Reproduction

Observed 2026-08-05 on `24 Tasks/Order CR2450 Batteries for Kitchen Switch.md` in the Personal vault. The corrupted content is preserved at commit `fb6640372` in that vault's git history.

**How the corruption arose:**

| Time (CEST) | Actor | Event |
|---|---|---|
| ~20:49 | Claude, via `/email-inbox` direct `Write` | file written, no `task_identifier` |
| 20:49:36 | obsidian-git | autocommit + push — un-stamped version reaches origin |
| 20:51:00 | git-rest (`vault-obsidian-personal@quant.benjamin-borbe.de`) | stamps `e1bc4321-7570-41f9-bfc6-a783d7aa4371`, **prepended at frontmatter line 1**, commit `47c172d3` |
| 20:52:55 | vault-cli `task complete` (local) | `WriteTask` fallback stamps `9fba815b-e1bb-442d-bc3e-87722f767a1f` at its **sorted position** |
| 21:00:15 | obsidian-git | `git merge origin/master` — different lines, clean merge, no conflict |

**Resulting frontmatter (verbatim, first 12 lines):**

```
---
task_identifier: e1bc4321-7570-41f9-bfc6-a783d7aa4371
assignee: bborbe
completed_date: "2026-08-05T20:52:55.279162+02:00"
page_type: task
phase: done
priority: 3
status: completed
task_identifier: 9fba815b-e1bb-442d-bc3e-87722f767a1f
themes:
    - '[[Administration]]'
---
```

**Minimal reproduction (replayed 2026-08-06 against installed `v0.101.3` and against a fresh build of `v0.102.3`; both behave identically):**

1. Create a scratch vault with a `24 Tasks/` directory and a `~/.vault-cli/config.yaml` pointing at it.
2. Copy the frontmatter above into `24 Tasks/Order CR2450 Batteries for Kitchen Switch.md`.
3. Add a second, healthy task file with a single `task_identifier`.
4. Run the four commands below.

**Observed evidence (verbatim):**

```
$ vault-cli task list --all --vault repro
[in_progress] Healthy
$ echo $?
0
```

The corrupted task is absent. Exit code 0. Nothing on stderr. The only trace is a `slog.Debug("skipping unreadable page", "file", fileName, "error", err)` call at `pkg/storage/page.go:57`, inside `pageStorage.ListPages` — the function `vault-cli task list` actually calls via `pkg/cli/cli.go` → `ops.NewListOperation(pageStore).Execute` → `pkg/ops/list.go:76` `l.pageStorage.ListPages`. (Note: `pkg/storage/task.go:100`'s `ListTasks`/`slog.Debug("skipping unreadable task", ...)` is a *different*, unrelated function reached only from `ensure_task_identifiers.go` and `goal_complete` — not the `task list` path. See Non-goals.) The line is below the default log level: `pkg/cli/cli.go:91-97` sets `slog.LevelWarn` on `os.Stderr` unless `--verbose` is passed.

```
$ vault-cli task get "Order CR2450 Batteries for Kitchen Switch" status --vault repro
Error: find task: parse frontmatter: unmarshal yaml frontmatter: yaml: unmarshal errors:
  line 8: mapping key "task_identifier" already defined at line 1
$ echo $?
1
```

```
$ vault-cli task lint --vault repro
WARN  24 Tasks/Order CR2450 Batteries for Kitchen Switch.md: DUPLICATE_KEY key "task_identifier" defined multiple times
Error: lint issues found
$ echo $?
1
```

```
$ vault-cli task lint --fix --vault repro
FIXED 24 Tasks/Order CR2450 Batteries for Kitchen Switch.md: DUPLICATE_KEY key "task_identifier" defined multiple times
$ head -3 "24 Tasks/Order CR2450 Batteries for Kitchen Switch.md"
---
task_identifier: e1bc4321-7570-41f9-bfc6-a783d7aa4371
assignee: bborbe
```

The repair kept `e1bc4321…` — the identifier git-rest prepended — and discarded `9fba815b…`, the identifier vault-cli itself wrote and the one every local reference uses.

## Expected vs Actual

| Behavior | Expected | Actual (verified 2026-08-06) |
|---|---|---|
| Duplicate top-level key is detected by `task lint` | Reported as `DUPLICATE_KEY` | **Already correct** — reported |
| Nested / list keys sharing a top-level key's name | Not reported | **Already correct** — not reported |
| `task lint --fix` picks which duplicate to keep | Keeps the value vault-cli wrote at its sorted position (the last occurrence) | Keeps the first occurrence — the value git-rest prepended at line 1 |
| `task list` encounters an unparseable task file | Names the file and its parse error on stderr so the operator can act | Omits it from stdout, exits 0, logs at DEBUG only — the task is invisible |

The lint behavior contract is `pkg/ops/lint.go`, whose current comment reads `fixDuplicateKeys removes duplicate YAML keys, keeping the first occurrence` — the code matches its own comment, so this is a design defect rather than an implementation slip. The silent-drop contract is violated against the vault-cli design principle in `CLAUDE.md`: `pkg/ops/` is a library layer that returns structured results; a parse failure is a result, not something to discard.

## Why this is a bug

Data loss without notification. A user of `vault-cli task list` cannot distinguish "this vault has no matching tasks" from "this vault has a corrupted task that I am hiding from you", because both produce empty stdout and exit 0. The predecessor spec (shipped as vault-cli v0.79.0) accepted noisy quarantine as the price of never losing a task; this path loses tasks quietly, which is strictly worse. Separately, `--fix` keeping the prepended value is an active correctness regression: it discards the identifier that vault-cli, the local task index, and every prior local reference use, and preserves the one written by an external service — so a "successful" repair silently re-points the task at a different identity.

## Desired Behavior

1. When `task lint --fix` repairs a top-level frontmatter key that appears more than once, the value retained is the one from the **last** occurrence in the frontmatter block; all earlier occurrences of that key are removed. This inverts today's keep-first rule.
2. The repaired frontmatter is written only when it parses as valid YAML; if it does not, the file is left byte-identical and the issue is reported as unfixed. (This guard exists today and is preserved.)
3. Detection continues to run on the raw frontmatter text before any YAML unmarshal — an unmarshal-based check cannot fire, because unmarshal is exactly what fails on this input.
4. Detection continues to consider only top-level frontmatter keys: indented nested mapping keys and YAML list items are never counted as duplicates of a top-level key.
5. When `pageStorage.ListPages` (`pkg/storage/page.go`, the enumeration function backing `task list` via `pkg/ops/list.go`'s `listOperation.Execute`) skips a file it cannot parse, the file's full path and the parse error reach the operator on stderr — change the existing `slog.Debug("skipping unreadable page", "file", fileName, "error", err)` call at `page.go:57` to `slog.Warn`, and change its `"file"` field from `fileName` (the basename with `.md` stripped) to the full `filePath` already in scope at that line. This is a log-level + field change only: `pkg/cli/cli.go`'s logger already writes `slog.LevelWarn` to `os.Stderr` by default (see Reproduction), so no new interface, no new return type, and no plumbing through `ListOperation`/`ListPages`'s signature — `pkg/ops/` and the CLI layer are unchanged. The set of tasks written to stdout and the process exit code are unchanged from today.

## Constraints

- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. The CLI layer owns all output formatting. See `docs/development-patterns.md`.
- The duplicate-key check runs on raw frontmatter lines, before `yaml.Unmarshal`. Any rewrite that routes detection through unmarshal is a regression, because unmarshal is the failing operation.
- The existing `DUPLICATE_KEY` issue type name, its `Fixable: true` flag, and its plain-output rendering (`WARN`/`FIXED` prefix, `key %q defined multiple times` wording) stay as they are — the lint output format is consumed by scripts and agents.
- The sibling silent-drop in `pkg/storage/task.go:100` (`taskStorage.ListTasks`, its own `slog.Debug("skipping unreadable task", ...)`) is a different function on a different call path (used only by `ensure_task_identifiers.go` and `goal_complete`, never by `task list`). It has the same defect shape but is explicitly out of scope for this spec — fixing it is a follow-up with its own Non-goals decision, not silently bundled here.
- `task list` exit code stays 0 when a file is unparseable but the walk otherwise succeeds. Downstream callers (task-orchestrator, skills, scripts) treat non-zero as fatal; making one corrupt file fail the whole listing trades a silent bug for a loud outage.
- `--output json` stdout stays machine-parseable. The new diagnostic goes to stderr, never stdout.
- All existing scenarios `scenarios/001`–`004` continue to pass against a freshly built binary — this is the mandatory release gate in `docs/releasing-vault-cli.md`, not optional.
- Version strings are not hand-edited. The repository is `autoRelease: true`; add an `## Unreleased` bullet prefixed `fix:` and let `github-releaser-agent` rename the section, bump all four version strings, tag, and push. See `docs/releasing-vault-cli.md` and `CLAUDE.md`.
- Unknown frontmatter fields survive read-write cycles (map-based frontmatter). Repair removes only the surplus duplicate lines; every other key and its formatting are untouched.

## Failure Modes

| Trigger | Detection | Expected behavior | Recovery | Reversibility |
|---|---|---|---|---|
| Duplicate key exists but removing the earlier occurrences still yields invalid YAML (e.g. a second, unrelated syntax error) | `task lint` reports the issue without a `FIXED` prefix | File left byte-identical; issue reported as unfixed | Operator edits the file by hand, re-runs `task lint --fix` | Reversible — no write occurred |
| The duplicated key's alphabetically sorted position *is* the first line (degenerate file whose only key is the duplicated one) | `task lint --fix` output plus a post-fix `grep` of the file | Keep-last still applies; the surviving value is the last occurrence. This is accepted: git-rest prepends only `task_identifier`, and a real task file always carries keys sorting before it (`assignee`, `category`, `page_type`, `phase`, `priority`, `status`) | Operator restores the intended value from vault git history | Reversible via `git` |
| A key appears three or more times | `task lint` reports one `DUPLICATE_KEY` finding for the key | All occurrences except the last are removed in a single pass | None needed | Reversible via `git` |
| Write of the repaired file fails (read-only file, full disk, permission denied) | `task lint --fix` reports the issue as unfixed | Issue reported as unfixed; the process does not abort the remaining files in the walk | Operator fixes permissions or frees disk, re-runs `task lint --fix` | Reversible — partial or no write |
| Crash or interrupt partway through a `--fix` sweep of many files | Re-running `task lint` lists the remaining unrepaired files | Each file is repaired independently; already-repaired files are already durable on disk | Re-run `task lint --fix`; the operation is idempotent | Partial — completed repairs persist |
| obsidian-git commits a repaired file while the sweep is still running | `git log` on the vault shows interleaved autocommits | Repair is a whole-file write; a concurrent commit captures either the pre- or post-repair content, never a torn file | Re-run `task lint` after the sweep to confirm zero findings | Reversible via `git` |
| Many unparseable files in one vault | stderr volume during `task list` | Every unparseable file gets one stderr line; stdout and exit code are unaffected | Operator runs `task lint --fix` to repair the reported files | N/A |
| A vault's tasks directory is missing entirely | `task list` output (empty list, exit 0) | Unchanged from today — `pageStorage.ListPages` (`page.go:33-40`) already catches `fs.ErrNotExist`, logs at DEBUG, and returns `(nil, nil)`; `task list` exits 0 with an empty list, not an error. This spec does not change that path. | Operator fixes the path in `~/.vault-cli/config.yaml` | Reversible |
| A vault's tasks directory exists but is otherwise unreadable (e.g. permission denied) | `task list` exits non-zero with a walk error | Unchanged from today — a non-`ErrNotExist` `os.ReadDir` failure is still fatal | Operator fixes permissions | Reversible |

## Security / Abuse Cases

- **Attacker-controlled input:** the frontmatter of any `.md` file under a configured vault's tasks directory. Vault content is synced from git-rest, so a compromised or buggy server-side writer can shape it.
- **Trust boundary:** file content crosses into the lint parser and the frontmatter serializer. The repair rewrites a file in place.
- **Path handling:** file paths come from a directory walk rooted at the configured vault path; no path component is taken from file content. Repair never writes outside the walked directory.
- **Resource exhaustion:** a frontmatter block with a very large number of repeated keys causes one line-scan pass and one write. No unbounded recursion, no retry loop, no network call — nothing that can hang.
- **Diagnostic output:** the new stderr line includes a file path and a YAML parse error. Both already appear in existing error output; no new class of information is disclosed. Frontmatter *values* are not echoed into the diagnostic.

## Suggested Decomposition

Prompts are generated in this order.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Invert duplicate-key repair to keep-last in the lint operation; update the keep-first unit test; add unit tests for duplicate `task_identifier`, duplicate arbitrary key, single-key-clean, and nested/list non-flagging | 1, 2, 3, 4 | 1, 3, 4, 5, 6, 9 | — |
| 2 | In `pkg/storage/page.go`'s `ListPages` (the enumeration function `task list` actually calls), change the existing `slog.Debug("skipping unreadable page", ...)` call to `slog.Warn` and its `"file"` field from basename to the full `filePath` already in scope — a log-level + field change, no new interface or plumbing through `pkg/ops`/CLI; unit test with one healthy and one duplicate-key file confirming stderr names the full path | 5 | 7, 8 | — |
| 3 | `## Unreleased` changelog entry with a `fix:` bullet, plus repo-wide precommit | — | 10, 11 | prompts 1, 2 |

Rationale: prompts 1 and 2 touch disjoint code paths (lint repair vs task enumeration) and can execute in either order; prompt 3 must run last so the changelog entry describes both landed changes and precommit runs against the final tree. ACs 2, 12, and 13 are operator-side: AC 2 replays the reproduction against the release-gate binary, AC 12 confirms the local install, and AC 13 records the multi-vault sweep — none are container-executable.

## Assumptions

- git-rest continues to prepend `task_identifier` at frontmatter line 1. If that changes, keep-last stops matching "keep the sorted-position value" and this rule needs revisiting — that dependency is why the git-rest change is filed separately rather than done first.
- vault-cli's `WriteTask` fallback continues to write frontmatter keys in alphabetical order, so the value it writes is never the first line of a real task file.
- One affected file existed vault-wide as of the 2026-08-05 scan, already repaired by hand. The sweep in AC 13 establishes the current count across all three vaults.
- The vault git history retains the pre-repair content of any file `--fix` touches, so an incorrect repair is recoverable.

## Do-Nothing Option

Not acceptable. Doing nothing leaves two live defects. First, any future git-rest/vault-cli identifier race produces a task that is invisible to `task list`, `next-task`, and every agent workflow built on them, with no error anywhere — the corruption is unbounded in duration because nothing surfaces it. Second, the repair path that does exist actively picks the wrong value, so an operator who notices the problem and runs `task lint --fix` gets a file that parses but carries the wrong identity, and gets no signal that anything was lost. The manual workaround — remembering to hand-edit each corrupted file and keep the correct UUID — depends on someone noticing a silent absence, which is exactly what failed for eleven days until an unrelated command addressed the task by name.

## Workaround (until the fix ships)

Run `vault-cli task lint` (without `--fix`) across each vault and hand-edit any file reporting `DUPLICATE_KEY key "task_identifier"`: delete the occurrence at frontmatter line 1, keep the one at the alphabetically sorted position. Do not use `--fix` on `task_identifier` duplicates before this spec lands — it keeps the wrong value.
