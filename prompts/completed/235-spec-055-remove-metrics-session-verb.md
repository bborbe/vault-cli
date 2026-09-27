---
status: completed
spec: [055-remove-metrics-session]
summary: Added the task remove-metrics-session verb end to end (domain method, ops operation, generated mock, CLI command and registration, plus domain/ops/integration specs) while leaving metricsSessionsWriteRefusal, knownTaskListFields and the AC4 refusal spec byte-identical
execution_id: vault-cli-exec-235-spec-055-remove-metrics-session-verb
dark-factory-version: v0.196.0
created: "2026-09-27T09:26:40Z"
queued: "2026-09-27T10:04:31Z"
started: "2026-09-27T10:04:33Z"
completed: "2026-09-27T10:12:32Z"
branch: dark-factory/remove-metrics-session
---

# Add the `task remove-metrics-session` verb (spec 055, prompt 1 of 3)

<summary>
- Adds one new vault-cli verb, `task remove-metrics-session <task-name> <session-id>`, that removes every `metrics_sessions` entry carrying the supplied session id from one task and leaves every other entry — and every other frontmatter key, known or unknown — exactly as it was.
- This is the missing half of the session-connect pair: entries could already be appended but never removed, so the shared-session rule's clearing step had no programmatic path and required hand-editing a field whose own contract forbids it.
- When the removal takes the last entry, the `metrics_sessions` key is deleted outright rather than left as an empty list, so a task that has had its last session removed is indistinguishable from one that never had an entry.
- A call whose id matches nothing fails loudly and writes nothing — the file stays byte-identical — because a silent success here would reproduce the exact defect class this verb exists to close.
- A session id that is empty, not a well-formed UUID, or carries a path separator is refused before the task is even read, so a caller-supplied string can never reach the frontmatter block.
- Adds the domain method, the operation, the counterfeiter mock, the CLI command and its registration, plus domain, ops and integration specs.
- Leaves the existing refusal completely alone: `task set`, `task add` and `task remove` still reject `metrics_sessions` before any mutation, with no `--force` bypass, and the integration suite's existing AC4 spec is not modified.
- Does not touch the append path, does not write `claude_session_id`, and takes no clock — this verb stamps nothing.
- Covers spec 055 ACs 1-7.
</summary>

<objective>
Ship the `task remove-metrics-session` verb end to end — domain, ops, CLI, mock and tests — so an operator can remove exactly one session's `metrics_sessions` entries from a task without touching frontmatter by hand, while `metricsSessionsWriteRefusal` and `knownTaskListFields` remain byte-identical. This prompt covers spec 055 ACs 1-7. It is the precondition for prompt 3, which documents the verb, and is independent of prompt 2.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

**The verb is a mirror of `task append-metrics-session`.** That command, its operation and its mock shipped in v0.147.0 and are the shape to copy. Read them before writing anything; the new code should be recognisably the same code with the append replaced by a filtered removal and the clock dropped.

Read fully (in this order):
- `pkg/ops/metrics_session_append.go` — the whole file. The operation interface, the `//counterfeiter:generate` annotation, the constructor, and `Execute`'s validate-then-read-then-write order are all copied from here. Note that it stamps `started_at` from an injected `libtime.CurrentDateTime`; the new verb takes **no** clock, so its constructor takes only `TaskStorage`.
- `pkg/ops/metrics_session_write.go` — the whole file. `metricsSessionsWriteRefusal`. This file must NOT change.
- `pkg/domain/task_frontmatter_metrics.go` — the whole file. `MetricsSessions()`, `AppendMetricsSession`, `ClearMetricsSessions` and `coerceMetricsSession` live here; the new `RemoveMetricsSession` joins them.
- `pkg/domain/metrics.go` — the `MetricsSession` struct.
- `pkg/ops/metrics_session_append_test.go` — the whole file. The ops unit-spec idiom: `mocks.TaskStorage`, `seedTask`, `libtimetest.ParseDateTime`. The new verb needs no pinned clock, so drop the `libtime`/`libtimetest` bits.
- `pkg/domain/task_frontmatter_metrics_test.go` — the whole file. Note `Describe("Clear metrics", ...)` at the end; the new domain specs go beside it.
- `mocks/append-metrics-session-operation.go` — the whole file. This is what `make generate` will emit for the new interface; do not hand-write the mock.
- `pkg/cli/cli.go` — `createTaskAppendMetricsSessionCommand` (definition at line 2410) and its registration at line 1344. Also read `createTaskSetCommand` immediately below it for the vault-dispatch shape.
- `integration/cli_test.go` — the `Describe("task append-metrics-session", ...)` block starting at line 2588, in full. Its fixture helpers (`runEntityCommand`, `sha256OfFile`, `readFile`, `countMetricsEntries`, `frontmatterOf`, `sortedKeys`), its `baseFrontmatter` / `twoEntryFrontmatter` constants, and its `It("AC4: task set, add and remove each refuse metrics_sessions and leave the file byte-identical", ...)` spec at line 2809 are the template to copy from — ⚠️ they are declared **inside that Describe's own closure**, so they are *not* visible to a sibling Describe. Copy them into the new one; do not assume you can call them. The new Describe goes after this block (which ends before `Describe("vault-cli defer", ...)` at line 2873) and must NOT modify it.
- `pkg/ops/frontmatter_entity.go` — lines 735-775 only. `metricsSessionsWriteRefusal` is called at line 743, the `knownTaskListFields` check at line 750, and the dispatch switch reading only `Goals()` / `Tags()` / `BlockedBy()` at lines 760-768. Read this to understand why the allowlist route cannot work — but do not change a byte of it.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface → constructor → private struct, counterfeiter annotation, `errors.Wrap` idiom.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`, never `context.Background()` inside `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, counterfeiter mocks.

NOTE: git IS available in this container (`.dark-factory.yaml` is `workflow: direct`, no `hideGit`), so `git diff --exit-code` guards run here.
</context>

<requirements>
1. **Add the domain method.** In `pkg/domain/task_frontmatter_metrics.go`, beside `AppendMetricsSession`:
   ```go
   // RemoveMetricsSession removes every "metrics_sessions" entry whose SessionID
   // equals sessionID, preserving every other entry in order. When the last entry
   // goes, the key is deleted rather than left as an empty list. Returns the number
   // of entries removed; 0 means nothing matched and nothing was written.
   func (f *TaskFrontmatter) RemoveMetricsSession(sessionID string) int
   ```
   Semantics, all four of which are asserted:
   - Filter `f.MetricsSessions()` on `entry.SessionID == sessionID`, keeping the survivors in their original order.
   - `removed == 0` → return 0 and leave the field untouched. Do NOT call `Set` — a no-op write would still change the in-memory map.
   - `removed > 0 && len(kept) == 0` → `f.Delete("metrics_sessions")`, return `removed`.
   - `removed > 0 && len(kept) > 0` → `f.Set("metrics_sessions", kept)`, return `removed`.
   A duplicate id is removed in full, not once — the append accumulator deliberately never suppresses a repeat, so N > 1 is reachable and must be handled.

2. **Add the operation.** New file `pkg/ops/metrics_session_remove.go`, mirroring `metrics_session_append.go`:
   ```go
   //counterfeiter:generate -o ../../mocks/remove-metrics-session-operation.go --fake-name RemoveMetricsSessionOperation . RemoveMetricsSessionOperation
   type RemoveMetricsSessionOperation interface {
       Execute(ctx context.Context, vaultPath, taskName, sessionID string) error
   }

   func NewRemoveMetricsSessionOperation(taskStorage storage.TaskStorage) RemoveMetricsSessionOperation
   ```
   `Execute` does exactly this, in this order:
   - `if err := uuid.Validate(sessionID); err != nil` → `errors.Wrapf(ctx, validation.Error, "invalid session id %q: expected a well-formed UUID", sessionID)`. Copy the append's guard verbatim. This runs **before** the task is read, which is what makes AC5's "nothing written" true by construction.
   - `o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)`; on error `errors.Wrap(ctx, err, "find task")`.
   - `removed := task.RemoveMetricsSession(sessionID)`.
   - `removed == 0` → return `errors.Errorf(ctx, "no metrics_sessions entry with session id %q on task %q: nothing removed", sessionID, taskName)`. The message must name the id and say nothing was removed.
   - Only now `o.taskStorage.WriteTask(ctx, task)`; on error `errors.Wrap(ctx, err, "write task")`. When nothing matched, `WriteTask` is **never called** — the byte-identical guarantee is structural, not a rewrite that happens to be stable.
   - Return nil.
   The operation never touches `claude_session_id`, takes no clock, and adds no `--force` / `--all` / `--started-at` flag. Removal is by session id only.

3. **Generate the mock.** Add the counterfeiter annotation from requirement 2 and run `make generate`. ⚠️ `make generate` rewrites `mocks/mocks.go` as a bare `package mocks` line and **strips its four-line copyright header** — a pre-existing generator defect, not yours to fix. Restore `mocks/mocks.go` to its committed content (`git checkout -- mocks/mocks.go`) and do not carry the deletion. The new `mocks/remove-metrics-session-operation.go` is the only mock file that should appear as a change.

4. **Add the CLI command.** In `pkg/cli/cli.go`, immediately after `createTaskAppendMetricsSessionCommand`, add `createTaskRemoveMetricsSessionCommand(ctx context.Context, configLoader *config.Loader, vaultName *string, outputFormat *string) *cobra.Command` and register it next to the append at line 1344:
   ```go
   cmd.AddCommand(createTaskRemoveMetricsSessionCommand(ctx, configLoader, vaultName, outputFormat))
   ```
   The command mirrors the append exactly, minus the clock:
   - `Use: "remove-metrics-session <task-name> <session-id>"`, `Args: cobra.ExactArgs(2)`, and a `Short:` describing per-session entry removal.
   - `getVaults(ctx, configLoader, vaultName)`, then `ops.NewVaultDispatcher().FirstSuccess(ctx, vaults, func(vault *config.Vault) error { ... })` building `ops.NewRemoveMetricsSessionOperation(storage.NewTaskStorage(storage.NewConfigFromVault(vault)))` and calling `Execute(ctx, vault.Path, taskName, sessionID)`. This is what makes the verb behave identically under `--vault` and under all-vaults, exactly as `task set` does.
   - Error path: `if OutputFormat(*outputFormat).IsJSON() { return PrintJSON(map[string]any{"success": false, "error": err.Error()}) }` then `return err`. Mirror the append precisely — the JSON branch returns `PrintJSON`'s result (normally nil, so exit 0 with a `success:false` object) and only the plain branch propagates the error and exits non-zero. This asymmetry is the established contract and AC7 depends on it; do NOT "fix" it to return the error in both branches.
   - Success path: JSON → `PrintJSON(map[string]any{"success": true, "name": taskName, "session_id": sessionID})`; plain → a confirmation line naming the id and the task, in the same style as the append's `✅ Appended metrics session %s to: %s`.
   - **No `encoding/json` import in `pkg/cli/`.** Output goes through `PrintJSON`.
   - Add the same `//nolint:dupl,...` directive the neighbouring mutation commands carry if the linter demands it.

5. **Add the domain specs.** In `pkg/domain/task_frontmatter_metrics_test.go`, beside `Describe("Clear metrics", ...)`:
   - removes the named entry and preserves the survivor's own id **and** its own `started_at` (a filter that keeps the wrong entry, or rewrites the survivor's timestamp, fails here);
   - removes all entries carrying a duplicated id, leaving exactly one;
   - deletes the key when the last entry goes — assert the key is absent, not merely empty;
   - returns 0 and leaves the field untouched when nothing matches;
   - returns 0 on an absent `metrics_sessions` key and on a non-list value.

6. **Add the ops specs.** New file `pkg/ops/metrics_session_remove_test.go`, mirroring `metrics_session_append_test.go`'s structure (`mocks.TaskStorage`, a local `seedTask`, external `ops_test` package). Cover:
   - a matching id calls `WriteTask` exactly once and leaves the task's `metrics_sessions` with only the survivors;
   - a non-matching id returns an error whose message contains the id, and `WriteTask` is **never called** (`Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))`) — this is the assertion that pins "writes only when something changed";
   - an empty id, `"not-a-uuid"` and `"../escape"` each return an error and never call `FindTaskByName` (`Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(0))`) — the validation runs before the read;
   - a `FindTaskByName` error is wrapped and propagated;
   - a `WriteTask` error is wrapped and propagated.

7. **Add the integration specs.** In `integration/cli_test.go`, add a new `Describe("task remove-metrics-session", ...)` after the append block. The append block's fixture helpers (`runEntityCommand`, `sha256OfFile`, `readFile`, `countMetricsEntries`, `frontmatterOf`, `sortedKeys`) and its `baseFrontmatter` / `twoEntryFrontmatter` constants are declared **inside that Describe's closure** (starting line 2588) and are therefore not visible to a sibling Describe — declare your own copies inside the new Describe, as the file already does (`runEntityCommand := func(...)` is declared locally in three separate Describes). Do NOT modify the append Describe or the AC4 refusal spec at line 2809. Note that in `twoEntryFrontmatter` the fixture's `task_identifier` **is** `11111111-1111-4111-8111-111111111111`, which is also the first entry's session id: every absence assertion must match the entry-line prefix (`- session_id: <id>`), never a bare id substring, or it will match the surviving `task_identifier` line and read a false positive. Cover ACs 1-7:
   - **AC1** — from the two-entry fixture, removing the first id exits 0 and leaves exactly one entry whose `session_id` and `started_at` are the survivor's own; `countMetricsEntries` returns 1 and the entry-line prefix for the removed id returns 0.
   - **AC2** — removing both ids in turn leaves no `metrics_sessions` occurrence at all, while `page_type`, `priority`, `status`, `task_identifier` and the body line are unchanged.
   - **AC3** — a three-entry fixture with the first id duplicated leaves exactly one entry, the one carrying the third id.
   - **AC4** — removing an absent id exits non-zero, stderr contains the id and says nothing was removed, and the file's `sha256` is unchanged before and after.
   - **AC5** — `""`, `"not-a-uuid"` and `"../escape"` each exit non-zero with a message naming the required shape, and the file hash is unchanged after all three.
   - **AC6** — `task set`, `task add` and `task remove` on `metrics_sessions` still each exit non-zero, and the file hash is unchanged. The pre-existing AC4 spec must remain **unmodified** and still pass.
   - **AC7** — `go test ./integration/... -ginkgo.focus="remove-metrics-session"` exits 0; `--output json` emits `{"success": true, ...}` for a matching id and `{"success": false, ...}` for a non-matching one.
   Also add the registration entry the project's DoD requires (`docs/dod.md`: "New CLI commands/subcommands have an entry in the integration test command registration table"): in the existing `Describe("command registration", ...)` `DescribeTable` (`integration/cli_test.go`, ~line 447, beside `Entry("task append-metrics-session", "task", "append-metrics-session")`) add `Entry("task remove-metrics-session", "task", "remove-metrics-session")`. That DescribeTable is a different Describe from the append block, so this does not violate the "do not modify the append Describe" constraint. Additionally, inside the new Describe, assert `--help task remove-metrics-session` exits 0 and its output names the verb.

8. **Do not touch the refusal.** `pkg/ops/metrics_session_write.go` and the `knownTaskListFields` map in `pkg/ops/frontmatter_entity.go` must be byte-identical to their committed state, and the integration AC4 spec must be unmodified. Verify with `git diff --exit-code` on those two files at the end.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff` / `git diff --exit-code` only read; never stage or commit.
- **The refusal is a frozen invariant.** `metricsSessionsWriteRefusal` and `knownTaskListFields` are not modified, and the integration AC4 spec at `integration/cli_test.go:2809` is not modified. Any diff touching them fails review.
- **Do NOT add `metrics_sessions` to `knownTaskListFields`.** The entry is inert — the refusal fires first at line 743, and the dispatch switch at lines 760-768 reads only `[]string` fields, so a map-valued field falls through setting nothing — and it would additionally open `task add metrics_sessions …`.
- **Do NOT exempt `remove` from the refusal.** The dedicated verb is the only shape that leaves the refusal intact.
- **The append path is untouched.** `task append-metrics-session`, `AppendMetricsSessionOperation` and `TaskFrontmatter.AppendMetricsSession` keep their behaviour and their accumulate-never-replace semantics.
- **The write goes through the domain and `TaskStorage.WriteTask`.** A string-level rewrite of the `metrics_sessions:` block that bypasses the domain would satisfy every AC while violating this constraint — the domain unit specs in requirement 5 are what rule it out.
- **`ClearMetricsSessions` is not the tool.** It runs on task completion and clears the whole field; this verb removes per entry and adds no flag that clears the field.
- **No new flags.** No `--force`, no `--all`, no `--started-at`, and no removal by timestamp — removal is by session id, the key the shared-session rule reads.
- **No `encoding/json` import in `pkg/cli/`** — output goes through `PrintJSON`.
- **The verb writes only when something changed**, never writes `claude_session_id`, and takes no clock. Its constructor takes only `TaskStorage`.
- **`mocks/mocks.go` must be restored** after `make generate` (see requirement 3). Do not carry the header deletion.
- ⚠️ **Known pre-existing failure, not this change's to fix.** `integration/cli_test.go`'s `topic defer writes defer_date for a relative and an absolute date` is red for the ~2 h each day when the local date leads the UTC date: it computes its expectation from `time.Now().UTC().AddDate(0,0,7)` while the CLI writes the local date. CI runs UTC and never sees it. Do NOT "fix" it opportunistically, and do not let it block the prompt — `make test` passing apart from exactly that named failure is a pass.
- Existing tests must still pass; the `InteractionCounter` specs and spec 038's dedup specs are unmodified.
</constraints>

<verification>
PRIMARY GATE — evidence greps. Run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c 'createTaskRemoveMetricsSessionCommand' pkg/cli/cli.go        # >= 2 (the AddCommand registration and the factory definition)
grep -c 'RemoveMetricsSession' pkg/domain/task_frontmatter_metrics.go # >= 1
grep -c 'RemoveMetricsSessionOperation' pkg/ops/metrics_session_remove.go  # >= 2 (interface + constructor)
grep -c 'invalid session id' pkg/ops/metrics_session_remove.go        # >= 1 (AC5 guard)
grep -n 'metrics_sessions' pkg/cli/cli.go                             # only Short:/Use: string literals expected (the append's Short at line 2418 plus the new command's own); any other hit is a raw field-name write — see below
grep -n 'remove-metrics-session' pkg/cli/cli.go                       # >= 1 (the Use: field)
grep -c 'WriteTaskCallCount' pkg/ops/metrics_session_remove_test.go   # >= 1 (the "writes only when changed" pin)
```

⚠️ **AC7's grep trap — do NOT assert `>= 2` on the hyphenated verb literal.** The spec's AC7 asks for `grep -n 'createTaskRemoveMetricsSessionCommand' pkg/cli/cli.go` returning ≥ 2 lines, which is what the row above checks. The hyphenated literal `remove-metrics-session` appears in that file **exactly once** (the `Use:` field) because the constructor is camelCase — the twin `append-metrics-session` behaves identically. A `>= 2` threshold on the hyphenated form is unachievable by a correct implementation; do not edit the source to force it. The row `grep -n 'metrics_sessions' pkg/cli/cli.go` guards the other direction: every hit must be inside a `Use:`/`Short:` string literal (the append's at line 2418 and the new command's own). A hit anywhere else means the new command is naming the raw field outside its help text — the write must go through the domain and `TaskStorage.WriteTask`.

FROZEN-NEIGHBOUR GUARD — these must exit 0 with empty output:
```
git diff --exit-code -- pkg/ops/metrics_session_write.go
git diff -U0 -- integration/cli_test.go | grep -c 'AC4: task set, add and remove'   # must print 0 — the AC4 spec body is unmodified (the file itself gains the new Describe by design, so a whole-file --exit-code check would fail spuriously)
```

TESTS:
```
go test ./pkg/domain/...                                          # domain specs pass
go test ./pkg/ops/...                                             # ops specs pass
go test ./integration/... -ginkgo.focus="remove-metrics-session"  # exits 0
make test                                                         # exits 0 apart from the named `topic defer` failure
```

⚠️ **Guard the focus run against a false pass.** `-ginkgo.focus` with zero matching specs can exit 0 while running nothing. After the focus run, assert the new spec names actually appear in the output (run with `-v -ginkgo.v` and grep for a distinctive spec name), and treat a run whose log does not contain them as a failure.

SYNTAX:
```
gofmt -e -l pkg/domain/task_frontmatter_metrics.go pkg/ops/metrics_session_remove.go pkg/cli/cli.go integration/cli_test.go   # must list NO files
```

FULL GATE — `make precommit` at the repo root must exit 0. If it fails on something this prompt introduced, fix it and re-run only the failing target (`make lint`, `make vet`, `make vulncheck`, `make check-changelog`, `make generate`, ...), then `make precommit` once more. ⚠️ `make precommit` does **not** run `check-versions` — that check is release-time only (`make release-check`, see `docs/releasing-vault-cli.md` § Version alignment). The four version strings must still be untouched: this prompt adds no release, and a `## Unreleased` CHANGELOG section is prompt 3's job, not this one's.

⚠️ **`make precommit` ends in `generate`, which re-runs the generator and re-strips the `mocks/mocks.go` copyright header.** So requirement 3's restoration must be the **last** action, after the final `make precommit` — restoring it earlier just gets stripped again:
```
git checkout -- mocks/mocks.go
git diff --exit-code -- mocks/mocks.go   # the generator's header strip is not carried
```
</verification>
