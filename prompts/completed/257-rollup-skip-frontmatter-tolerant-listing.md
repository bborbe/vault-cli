---
status: completed
spec: [060-bug-rollup-aborts-on-frontmatterless-task-file]
summary: Added TaskStorage.ListTasksTolerant returning readable tasks plus sorted, sanitised skipped-file names, with regenerated mocks and Ginkgo specs.
execution_id: vault-cli-rollup-skip-frontmatter-exec-257-rollup-skip-frontmatter-tolerant-listing
dark-factory-version: v0.198.7
created: "2026-10-19T17:55:00Z"
queued: "2026-10-19T18:10:28Z"
started: "2026-10-19T18:10:30Z"
completed: "2026-10-19T18:16:52Z"
branch: dark-factory/bug-rollup-aborts-on-frontmatterless-task-file
---

# List tasks tolerantly, naming the files skipped

<!--
Decomposition note (for the auditor): AC5's spec text asks for a `git diff` proving
`pkg/storage/task.go` gained additions only. This prompt substitutes a positive grep
of `ListTasksStrict`'s unchanged error line plus a dedicated spec asserting it still
fails loudly. The additions-only diff is not in the spec's Verification section —
it lives only in AC5's evidence — so add it there if the stronger check is wanted.
-->

<summary>
- A new task-listing method returns the tasks it could read and separately names the files it could not.
- One unreadable or frontmatter-less task file no longer aborts the whole listing.
- Skipped names are paths relative to the configured tasks directory, not bare file names.
- Two files with the same name in different subdirectories come back as two distinct entries.
- The skipped list is sorted, so the report order is stable across runs.
- A control character in a filename is replaced by a space, so a name can never forge an extra report line.
- A missing or unreadable tasks directory is still an error — a misconfigured vault is never reported as empty.
- The existing strict listing keeps its current behaviour and its current body; the tolerant path is an addition.
- The existing tolerant listing keeps its current behaviour.
- The project still builds and the full test suite passes with the new listing method in place.
</summary>

<objective>
Add a third task-listing path to the storage layer that reads every task file it can parse and returns the names of the files it skipped, so a caller can report a week's figures from the readable population instead of aborting on one malformed file. This prompt owns the storage boundary only — no rollup, result or renderer changes.
</objective>

<context>
Read `CLAUDE.md` for project conventions and `docs/dod.md` for the project's Definition of Done. This repo is a single Go module at the repo root; run `make test` and `make precommit` at the repo root.

Read these files before editing:

- `pkg/storage/task.go` — the existing `ListTasks`, `ListTasksStrict`, `walkTaskFiles` and `readTaskFromPath` methods. Note the callback contract inlined below; it is the boundary this prompt builds on.
- `pkg/storage/storage.go` — the `TaskStorage` interface, the composed `Storage` interface (which embeds `TaskStorage`), and `NewTaskStorage` / `NewStorage`.
- `pkg/storage/base.go` — `readEntityComponentsFromPath` is the per-file error source; `ParseFrontmatterMap` returns the error text `no frontmatter found` for a file with no `---` block.
- `pkg/storage/task_test.go` — the Ginkgo suite and its `BeforeEach` helpers (`store`, `vaultDir`, `tasksDir`, `taskContent`).
- `pkg/storage/storage_suite_test.go` — suite bootstrap and the counterfeiter `//go:generate` directive.

Coding guides (in-container paths):

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-security-linting.md`

Load-bearing contract already in the tree — do NOT change it:

```go
func (t *taskStorage) walkTaskFiles(
	ctx context.Context,
	vaultPath string,
	onFile func(task *domain.Task, name string, path string, err error) error,
) error
```

`walkTaskFiles` hands each per-file read/parse error to `onFile` as `err` and returns a walk-level error (missing or unreadable `tasks_dir`) wrapped with `walk tasks directory <dir>`. It passes the absolute `path` and the filename stem as `name`.
</context>

<requirements>
1. In `pkg/storage/task.go`, add a new method on `*taskStorage`. The signature is frozen by the spec:

   ```go
   // ListTasksTolerant returns every task in the vault's configured tasks_dir,
   // including subdirectories, and names the files it could not read. A per-file
   // read/parse error is skipped and its tasks_dir-relative path recorded; a walk
   // error (a missing or unreadable directory) is returned, so a misconfigured
   // vault is never reported as an empty one. The skipped names are sorted and
   // carry no control characters, so each renders on one line.
   func (t *taskStorage) ListTasksTolerant(
       ctx context.Context,
       vaultPath string,
   ) ([]*domain.Task, []string, error)
   ```

2. Implement `ListTasksTolerant` as follows. Keep the project's error-wrapping style (`github.com/bborbe/errors`, never `fmt.Errorf`).

   ```go
   func (t *taskStorage) ListTasksTolerant(
       ctx context.Context,
       vaultPath string,
   ) ([]*domain.Task, []string, error) {
       tasksDir := filepath.Join(vaultPath, t.config.TasksDir)
       var tasks []*domain.Task
       skipped := make([]string, 0)
       if err := t.walkTaskFiles(
           ctx,
           vaultPath,
           func(task *domain.Task, _, path string, err error) error {
               if err != nil {
                   skipped = append(skipped, skippedTaskName(tasksDir, path))
                   return nil
               }
               tasks = append(tasks, task)
               return nil
           },
       ); err != nil {
           return nil, nil, err
       }
       sort.Strings(skipped)
       return tasks, skipped, nil
   }
   ```

   The returned `skipped` slice is non-nil (an empty slice when nothing was skipped). The walk error is returned unchanged — do not wrap it again, because `walkTaskFiles` already names the directory.

3. Add the pure helper that turns an absolute walked path into the reported name. Place it beside `ListTasksTolerant` in `pkg/storage/task.go`:

   ```go
   // skippedTaskName reduces an absolute task file path to the path relative to
   // tasks_dir, with each control character replaced by a single space so a
   // filename can never forge an extra line in a report.
   func skippedTaskName(tasksDir, path string) string {
       rel, err := filepath.Rel(tasksDir, path)
       if err != nil {
           rel = filepath.Base(path)
       }
       return strings.Map(func(r rune) rune {
           if unicode.IsControl(r) {
               return ' '
           }
           return r
       }, filepath.ToSlash(rel))
   }
   ```

   Add the `sort` and `unicode` imports. Keep the existing imports (`fmt`, `log/slog`, `os`, `path/filepath`, `strings`, `github.com/bborbe/errors`, `github.com/google/uuid`, `pkg/domain`) used by the other methods.

4. Add the method to the `TaskStorage` interface in `pkg/storage/storage.go`, immediately after `ListTasksStrict`, with a doc comment that states the three-way difference:

   ```go
   // ListTasksTolerant returns every task in the vault's configured tasks_dir and
   // names the files it skipped because they could not be read or parsed. A walk
   // error (a missing or unreadable directory) is returned rather than reported as
   // an empty set. Unlike ListTasks the skipped names are returned to the caller,
   // not logged; unlike ListTasksStrict a single unreadable file does not abort the
   // listing.
   ListTasksTolerant(ctx context.Context, vaultPath string) ([]*domain.Task, []string, error)
   ```

   Do NOT add a second declaration to the composed `Storage` interface — it embeds `TaskStorage`, so the method is promoted. Do NOT change the `ListTasks` or `ListTasksStrict` interface entries or their doc comments.

5. Regenerate the counterfeiter fakes. `Storage` embeds `TaskStorage`, so both fakes gain the new method and would otherwise fail to compile. Run:

   ```bash
   make generate
   ```

   Confirm `mocks/task-storage.go` and `mocks/storage.go` each contain a generated `ListTasksTolerant` implementation. Do not hand-edit anything under `mocks/` — `make generate` deletes and recreates that directory.

6. Add Ginkgo specs to `pkg/storage/task_test.go` inside the existing `Describe("TaskStorage")` container. Reuse `store`, `vaultDir`, `tasksDir` and `taskContent()` from `BeforeEach`. Add a new `Describe("ListTasksTolerant")` block with these specs:

   a. **Skips a frontmatter-less file and names it.** Write `Tasks/Healthy.md` with `taskContent()`, then write `Tasks/Broken.md` with the raw body `Failed to authenticate: OAuth session expired and could not be refreshed\n` (no `---` block) using `os.WriteFile(..., 0600)`. Assert `err` is nil, `tasks` has length 1 with `tasks[0].Name == "Healthy"`, and `skipped` equals exactly `[]string{"Broken.md"}`.

   b. **Reports subdirectory paths, sorted and distinct.** Write `Tasks/Healthy.md` with `taskContent()`, then `os.MkdirAll` `Tasks/sub-a` and `Tasks/sub-b` and write `Tasks/sub-a/Broken.md` and `Tasks/sub-b/Broken.md` with `os.WriteFile(..., 0600)` (both frontmatter-less). Assert `err` is nil, `tasks` has length 1, and `skipped` equals exactly `[]string{"sub-a/Broken.md", "sub-b/Broken.md"}` in that order. The two names must not collapse to `Broken.md`.

   c. **Replaces a control character with a space.** Write a frontmatter-less file whose name contains a newline: `filepath.Join(tasksDir, "bad\nname.md")`. Assert `err` is nil and `skipped` equals exactly `[]string{"bad name.md"}` (one space, no newline). This is the report-integrity guard: a newline in a name must not produce a second physical line.

   d. **Returns a walk error for a missing tasks directory.** `Expect(os.RemoveAll(tasksDir)).To(Succeed())`, then call the method. Assert `err` is not nil, `tasks` is nil, and `skipped` is nil. This is the invariant the skip must not swallow: a missing directory is a misconfiguration, never "every task was skipped".

   e. **A clean vault reports an empty, non-nil skipped list.** With only `Tasks/Healthy.md`, assert `err` is nil and `skipped` equals `[]string{}` (use `Equal([]string{})` or `BeEmpty()` plus `NotTo(BeNil())`).

   f. **`skippedTaskName` maps and sanitises (table test).** Add a `DescribeTable` (or a table-driven `It`) for the helper, covering: `("/vault/Tasks", "/vault/Tasks/sub/Broken.md") → "sub/Broken.md"`; `("/vault/Tasks", "relative/path.md") → "path.md"` (the `filepath.Rel` error branch, where `path` is not under `tasksDir`); `("/vault/Tasks", "/vault/Tasks/bad\nname.md") → "bad name.md"`. Every branch of the helper must be exercised — the project DoD requires tests for all added code paths.

7. Add a `Describe("ListTasksStrict")` block (or a single spec) that locks the negative: with `Tasks/Broken.md` frontmatter-less, `store.ListTasksStrict(ctx, vaultDir)` returns a non-nil error whose message contains `Broken.md`, and the task slice is nil. This is AC5: the loud contract is unchanged.

8. Add one spec asserting `ListTasks` still returns a nil error with the same broken file (its current tolerant-but-silent behaviour is unchanged). This guards the third sibling against accidental coupling.

9. Do NOT change `ListTasksStrict`'s method body, `ListTasks`'s method body, or `walkTaskFiles`'s signature and behaviour. The relative name is computed with `filepath.Rel` against `tasksDir` inside the new method; it is not a new callback parameter.
</requirements>

<constraints>
- Copied from the spec: the tolerant listing returns `(tasks []*domain.Task, skipped []string, err error)`; it is added to the `TaskStorage` interface in `pkg/storage/storage.go`; `Storage` embeds `TaskStorage`, so both counterfeiter fakes (`mocks/task-storage.go`, `mocks/storage.go`) must be regenerated with `make generate`.
- Do not edit `ListTasksStrict` — AC5 pins its behaviour and its body; the tolerant path is a new method.
- Do not change `walkTaskFiles`'s callback contract. It already passes `(task, name, path, err)`; the two existing callers depend on this signature.
- Skip only per-file errors. A walk error must still abort (AC6). Swallowing a missing directory would turn "the vault is misconfigured" into "the vault has no tasks".
- A skipped name containing a newline or other control character is rendered with each control character replaced by a single space, so the report stays one physical line.
- `pkg/ops/` is untouched by this prompt — the operation still calls `ListTasksStrict` until the next prompt changes it.
- Use `github.com/bborbe/errors` for errors, never `fmt.Errorf`; never pass `context.Background()` where a `ctx` is available.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- Do NOT hand-bump any version string or tag (`.maintainer.yaml: release.autoRelease: true`).
</constraints>

<verification>
Run at the repo root:

```bash
make test
make precommit
```

Both must exit 0.

Then confirm the API surface and the untouched contracts:

```bash
grep -n 'func (t \*taskStorage) ListTasksTolerant' pkg/storage/task.go
grep -n 'ListTasksTolerant' pkg/storage/storage.go
grep -n 'func (fake \*TaskStorage) ListTasksTolerant' mocks/task-storage.go
grep -n 'func (fake \*Storage) ListTasksTolerant' mocks/storage.go
grep -q 'onFile func(task \*domain.Task, name string, path string, err error) error' pkg/storage/task.go
grep -q 'return errors.Wrapf(ctx, err, "read task file %s", path)' pkg/storage/task.go
grep -q 'skipped = append(skipped, skippedTaskName(tasksDir, path))' pkg/storage/task.go
```

The four `grep -n` commands must print a line each; the three `grep -q` commands must exit 0. The first two `grep -q` commands prove `walkTaskFiles` and `ListTasksStrict` still carry their original bodies; the third proves the new method appends to `skipped`.

Before finishing, re-run the verification block and confirm it passes. Walk AC2, AC5 and AC6 from the spec against the new specs and confirm each is exercised by a named spec.
</verification>
