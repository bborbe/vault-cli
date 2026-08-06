---
status: draft
spec: [026-bug-duplicate-frontmatter-key-silent-task-loss]
created: "2026-08-06T17:05:00Z"
branch: dark-factory/bug-duplicate-frontmatter-key-silent-task-loss
---

<summary>
- A task file that vault-cli cannot parse no longer disappears from `vault-cli task list` without a word.
- When the page enumeration skips an unreadable file, it now emits a warning on stderr naming the file and the parse error, instead of a debug line nobody sees at the default log level.
- The warning names the file's full path on disk, not just its display name, so the operator can act on it directly.
- The list of tasks printed on stdout is unchanged, and `--output json` stdout stays machine-parseable — the diagnostic goes to stderr only.
- The exit code of `task list` is unchanged: still 0 when one file is corrupt but the directory walk otherwise succeeds, so downstream scripts do not turn a silent bug into a loud outage.
- A missing tasks directory still yields an empty list with no warning, exactly as before.
- No interface, return type, or call signature changes — this is a log-level and log-field change only.
</summary>

<objective>
Make `pageStorage.ListPages` — the enumeration function backing `vault-cli task list` — tell the operator when it skips a file it cannot parse, by raising its existing skip log from `slog.Debug` to `slog.Warn` and logging the file's full path instead of its display name. This closes the silent-task-loss path where a corrupted task file vanishes from a listing that still exits 0.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement rules).

Read these files before making changes:

- `/workspace/pkg/storage/page.go` (80 lines) — read fully. The single line to change is inside `ListPages`:
  ```go
  fileName := strings.TrimSuffix(entry.Name(), ".md")
  filePath := filepath.Join(targetDir, entry.Name())

  page, err := p.readPageFromPath(ctx, filePath, fileName, vaultPath)
  if err != nil {
      // Log error but continue with other pages
      slog.Debug("skipping unreadable page", "file", fileName, "error", err)
      continue
  }
  ```
  Both `fileName` (basename, `.md` stripped) and `filePath` (full path) are already in scope. The `os.ReadDir` error branch above it already handles `fs.ErrNotExist` by logging at Debug and returning `(nil, nil)` — leave that branch alone.
- `/workspace/pkg/storage/base.go` — `readEntityComponentsFromPath` is what fails on a duplicate-key file. Its parse failure is returned as `errors.Wrap(ctx, parseErr, "parse frontmatter")`, where `parseErr` is `errors.Wrap(ctx, err, "unmarshal yaml frontmatter")` around the `gopkg.in/yaml.v3` error. The resulting message contains the substring `already defined`.
- `/workspace/pkg/storage/markdown_test.go` lines ~598-684 — the existing `Context("ListPages")` block. Read it for the fixture style (`os.MkdirAll` a custom pages dir under the temp vault, `os.WriteFile` page files with `0600`, then `store.ListPages(ctx, vaultPath, "Custom Pages")`). Do NOT add the new tests into this file — it is already ~900 lines; put them in a new file per requirement 3.
- `/workspace/pkg/storage/storage.go` lines ~91-95 and ~208-215 — the `PageStorage` interface and its constructor.
- `/workspace/pkg/cli/cli.go` lines ~90-98 — `PersistentPreRunE` already installs `slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))` with `level = slog.LevelWarn` unless `--verbose` is passed. This is why raising the call to `slog.Warn` is sufficient to reach stderr — no CLI change is needed.
- `/workspace/pkg/ops/list.go` line ~76 — `l.pageStorage.ListPages(ctx, vaultPath, pagesDir)` is the call site behind `task list`. It is context only; do NOT change it.
- `/workspace/CHANGELOG.md` — top versioned section is `## v0.102.3`.

Verified signatures (do not re-derive):

```go
// pkg/storage/page.go
func (p *pageStorage) ListPages(ctx context.Context, vaultPath string, pagesDir string) ([]*domain.Page, error)

// pkg/storage/storage.go
type PageStorage interface {
    ListPages(ctx context.Context, vaultPath string, pagesDir string) ([]*domain.Page, error)
}
func NewPageStorage(storageConfig *Config) PageStorage
func NewConfigFromVault(vault *config.Vault) *Config
```

Verified behavior of `gopkg.in/yaml.v3` v3.0.1 unmarshalling a duplicate top-level key into `map[string]any`:

```
yaml: unmarshal errors:
  line 4: mapping key "task_identifier" already defined at line 1
```

Read these coding-plugin docs (they are in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-logging-guide.md` — log-level semantics and structured-logging conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>
1. In `/workspace/pkg/storage/page.go`, inside `ListPages`, change the skip log from Debug to Warn and log the full path instead of the display name. Exactly this replacement:

   ```go
   // old
   // Log error but continue with other pages
   slog.Debug("skipping unreadable page", "file", fileName, "error", err)

   // new
   // Warn and continue: the operator must be told the file was skipped,
   // otherwise the page silently disappears from listings that still exit 0.
   slog.Warn("skipping unreadable page", "file", filePath, "error", err)
   ```

   Keep the log message text `skipping unreadable page` and the attribute key `"file"` unchanged — only the level and the value change. `filePath` is already in scope at that line; `fileName` remains in use as the argument to `readPageFromPath`, so do not delete it.

2. Do NOT change anything else in the enumeration path. Specifically:
   - The `os.ReadDir` `fs.ErrNotExist` branch keeps logging at `slog.Debug` and keeps returning `(nil, nil)` — a missing tasks directory must stay silent and non-fatal.
   - A non-`ErrNotExist` `os.ReadDir` failure stays fatal (`errors.Wrap(ctx, err, ...)`).
   - `ListPages` keeps its signature, keeps returning `(pages, nil)` when some files were skipped, and keeps `continue`-ing past the unreadable file. No new interface, no new return type, no plumbing through `ListOperation`.
   - Do NOT modify `/workspace/pkg/ops/list.go`, `/workspace/pkg/cli/cli.go`, or `/workspace/pkg/storage/task.go`.

3. Create a new test file `/workspace/pkg/storage/page_test.go` in package `storage_test`, with the 2026 license header matching the other files in that directory:

   ```go
   // Copyright (c) 2026 Benjamin Borbe All rights reserved.
   // Use of this source code is governed by a BSD-style
   // license that can be found in the LICENSE file.

   package storage_test
   ```

   The package already has a Ginkgo suite bootstrap at `/workspace/pkg/storage/storage_suite_test.go`, so no new suite file is needed.

   Set up a `Describe("pageStorage.ListPages diagnostics")` with:
   - a `ctx`, a temp vault created via `os.MkdirTemp` in `BeforeEach` and removed in `AfterEach`,
   - a page store built with `storage.NewPageStorage(storage.NewConfigFromVault(&config.Vault{}))` (import `github.com/bborbe/vault-cli/pkg/config`; `ListPages` takes `vaultPath` and `pagesDir` explicitly, so the config defaults are irrelevant),
   - a pages directory named `UnreadablePages` (no spaces — `slog`'s `TextHandler` quotes attribute values containing spaces, and a space-free path keeps the substring assertions unambiguous),
   - a captured default logger so the warning can be asserted:

   ```go
   var logBuf *bytes.Buffer
   var prevLogger *slog.Logger

   BeforeEach(func() {
       logBuf = &bytes.Buffer{}
       prevLogger = slog.Default()
       slog.SetDefault(slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
   })

   AfterEach(func() {
       slog.SetDefault(prevLogger)
   })
   ```

   The handler is deliberately pinned at `slog.LevelWarn` — that is the level `pkg/cli/cli.go` installs by default, so the test proves the diagnostic actually reaches the operator without `--verbose`, not merely that some log call exists.

4. Add an `It` that covers the spec's core scenario: one parseable page and one duplicate-key page in the same directory. Write the broken file with frontmatter that defines `task_identifier` twice (the real corruption shape) and the healthy file with a single one:

   ```go
   healthy := "---\nstatus: in_progress\npage_type: task\ntask_identifier: 11111111-1111-4111-a111-111111111111\n---\n# Healthy\n"
   broken := "---\ntask_identifier: e1bc4321-7570-41f9-bfc6-a783d7aa4371\nassignee: bborbe\nstatus: completed\ntask_identifier: 9fba815b-e1bb-442d-bc3e-87722f767a1f\n---\n# Broken\n"
   ```

   Write them as `Healthy.md` and `Broken.md` in the pages directory with mode `0600`, call `store.ListPages(ctx, vaultPath, "UnreadablePages")`, and assert all of:
   - `err` is `nil` (a corrupt file does not fail the walk — this is the exit-code-stays-0 contract),
   - the returned slice has length 1 and its single element's `Name` is `Healthy`,
   - `logBuf.String()` contains `skipping unreadable page`,
   - `logBuf.String()` contains the broken file's **full path**, i.e. `filepath.Join(vaultPath, "UnreadablePages", "Broken.md")` — this is the assertion that fails if the code logs the basename,
   - `logBuf.String()` contains `already defined` (the parse error reaches the operator, not just the file name),
   - `logBuf.String()` does NOT contain `Healthy.md` (only the skipped file is reported).

5. Add an `It` asserting the negative case: a directory containing only parseable pages produces no warning. Write one healthy page, call `ListPages`, assert the returned slice has length 1 and `logBuf.String()` does not contain `skipping unreadable page`. Prefer `Expect(logBuf.String()).To(BeEmpty())` only if nothing else in the package logs at Warn during the call; if that proves flaky, assert the absence of the `skipping unreadable page` substring instead.

6. Add an `It` asserting the missing-directory path is untouched: call `store.ListPages(ctx, vaultPath, "DoesNotExist")` and assert `err` is `nil`, the returned slice is empty, and `logBuf.String()` does not contain `skipping unreadable page` (the `fs.ErrNotExist` branch still logs at Debug, which the Warn-level handler drops).

7. Add a `## Unreleased` entry to `/workspace/CHANGELOG.md` immediately after implementing, before running `make precommit`. If a `## Unreleased` section already exists (the sibling prompt for this spec may have created it), APPEND this bullet to it — do not replace the section or its existing bullets. If it does not exist, create it **below** the preamble block (the `All notable changes…` line and the three `* MAJOR / MINOR / PATCH` bullets) and **above** the `## v0.102.3` heading — never between the `# Changelog` title and the preamble.

   ```
   - fix: `task list` now warns on stderr with the file's full path and the parse error when it skips an unreadable page, instead of hiding the skip behind a debug log — a corrupted task no longer vanishes from a listing that reports success
   ```

   Do NOT bump or hand-edit any version string in `CHANGELOG.md` or in `.claude-plugin/plugin.json` / `.claude-plugin/marketplace.json` — the release agent owns those.

8. Coverage: the changed package is `pkg/storage`. The new tests in requirements 4-6 cover the skip branch, the happy path, and the missing-directory branch of `ListPages`. Do not add retroactive coverage to unrelated untested storage code, and do not modify the existing `Context("ListPages")` tests in `markdown_test.go` — they must keep passing unchanged.
</requirements>

<constraints>
- This is a log-level and log-field change only. No new interface, no new return type, no new error, no plumbing through `pkg/ops` or the CLI layer. If you find yourself editing `pkg/ops/list.go` or `pkg/cli/cli.go`, stop — the change is in `pkg/storage/page.go` alone.
- The set of pages written to stdout by `task list` and the process exit code are unchanged. `task list` still exits 0 when a file is unparseable but the walk otherwise succeeds — downstream callers (task-orchestrator, skills, scripts) treat non-zero as fatal, and failing the whole listing over one corrupt file trades a silent bug for a loud outage.
- `--output json` stdout must stay machine-parseable. The new diagnostic goes to stderr via `slog`, never to stdout.
- Every unparseable file gets exactly one warning line; the walk continues past it.
- The sibling silent-drop in `pkg/storage/task.go` (`taskStorage.ListTasks`, its own `slog.Debug("skipping unreadable task", ...)`) is a DIFFERENT function on a different call path, reached only by `ensure_task_identifiers.go` and `goal_complete`, never by `task list`. It is explicitly out of scope for this spec — do NOT change it, and do not "fix it while you're here".
- Do NOT change `pkg/ops/lint.go` — the duplicate-key repair is a separate prompt for this spec.
- Do NOT add a flag, config key, or environment variable to opt out of the warning. The behavior is invariant.
- Frontmatter values must NOT be echoed into the diagnostic — only the file path and the parse error, both of which already appear in existing error output.
- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. No `fmt.Print*` and no `os.Stdout` anywhere in this change.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string.
- Existing tests must still pass, including the existing `Context("ListPages")` cases in `pkg/storage/markdown_test.go`.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0, including the new `pkg/storage` `page_test.go` specs and the pre-existing `markdown_test.go` `ListPages` cases.

Confirm the changed call is at Warn level and logs the full path (must print the `slog.Warn` line with `filePath`, and must NOT print a `slog.Debug("skipping unreadable page"` line):

```
grep -n 'skipping unreadable page' pkg/storage/page.go
```

Confirm the out-of-scope sibling was not touched (must still show `slog.Debug`):

```
grep -n 'skipping unreadable task' pkg/storage/task.go
```

Confirm the changelog entry landed with the right prefix:

```
grep -A20 '^## Unreleased' CHANGELOG.md
```

Must show a line starting with `- fix:`.

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.

Coverage for the changed package:

```
go test -coverprofile=/tmp/cover.out ./pkg/storage/... && go tool cover -func=/tmp/cover.out | grep ListPages
```
</verification>
