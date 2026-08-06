---
status: completed
spec: [026-bug-duplicate-frontmatter-key-silent-task-loss]
summary: Replaced err with errors.Cause(err) in the skipping unreadable page warning, shrinking the diagnostic from a ~2 KB stack dump to a readable two-line message
execution_id: vault-cli-dup-frontmatter-lint-exec-170-spec-026-warn-concise-cause
dark-factory-version: v0.192.9
created: "2026-08-06T20:35:00Z"
queued: "2026-08-06T18:46:26Z"
started: "2026-08-06T18:48:34Z"
completed: "2026-08-06T18:50:58Z"
branch: dark-factory/bug-duplicate-frontmatter-key-silent-task-loss
---

<summary>
- The warning that `vault-cli task list` prints when it skips an unreadable page is now one readable diagnostic instead of a wall of stack frames.
- Today that single warning is a ~2 KB blob: the error object carries two captured stack traces, and the logger prints them in full on one very long line that the terminal wraps.
- The operator still sees the file path and the underlying parse reason — only the Go stack frames are dropped.
- A vault with several corrupted files no longer buries the terminal, which is the case this diagnostic exists to serve.
- Nothing else changes: same warning level, same wording, same behavior — only the error value being printed.
</summary>

<objective>
Make the `slog.Warn("skipping unreadable page", ...)` call in `pageStorage.ListPages` log the error's root cause rather than the fully-wrapped error, so the diagnostic is a short human-readable line instead of a multi-frame stack dump.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement rules).

Read these files before making changes:

- `/workspace/pkg/storage/page.go` (~82 lines) — read fully. The line to change is inside `ListPages`, currently:
  ```go
  // Warn and continue: the operator must be told the file was skipped,
  // otherwise the page silently disappears from listings that still exit 0.
  slog.Warn("skipping unreadable page", "file", filePath, "error", err)
  ```
  `github.com/bborbe/errors` is ALREADY imported in this file — no new import is needed.
- `/workspace/pkg/storage/page_test.go` — the test file added by the sibling prompt for this spec. Its first `It` already asserts the log contains `skipping unreadable page`, the full file path, and `already defined`. Extend it per requirement 2; do not restructure it.

Verified upstream behavior (do not re-derive):

- `github.com/bborbe/errors.Wrap(ctx, err, msg)` delegates to `github.com/pkg/errors.Wrap`, which attaches a stack trace and implements the `Cause() error` interface.
- `github.com/bborbe/errors.Cause(err)` delegates to `github.com/pkg/errors.Cause`, which walks the chain to the root error.
- For a duplicate-key task file the root cause is the raw `gopkg.in/yaml.v3` error, whose `Error()` is exactly:
  ```
  yaml: unmarshal errors:
    line 5: mapping key "task_identifier" already defined at line 1
  ```
  Two lines, no stack frames.

Observed today (this is the defect being fixed) — one skipped file produces a warning containing repeated blocks like:

```
error="yaml: unmarshal errors:\n  line 5: mapping key \"task_identifier\" already defined at line 1\nunmarshal yaml frontmatter\ngithub.com/bborbe/errors.Wrap\n\t/Users/.../errors_wrap.go:17\ngithub.com/bborbe/vault-cli/pkg/storage.(*baseStorage).parseToFrontmatterMap\n\t...
```

Read this coding-plugin doc (it is in the container at this path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-logging-guide.md` — structured-logging conventions.
</context>

<requirements>
1. In `/workspace/pkg/storage/page.go`, inside `ListPages`, change only the value logged under the `"error"` key:

   ```go
   // old
   slog.Warn("skipping unreadable page", "file", filePath, "error", err)

   // new
   slog.Warn("skipping unreadable page", "file", filePath, "error", errors.Cause(err))
   ```

   Keep the message text `skipping unreadable page`, the level `slog.Warn`, the `"file"` key, and the `filePath` value exactly as they are. Keep the two-line explanatory comment above the call. Do NOT change the `continue`, the signature, or anything else in the function.

2. In `/workspace/pkg/storage/page_test.go`, extend the existing first `It` (the one writing a healthy page and a duplicate-key broken page) with assertions that the stack frames are gone. Keep every existing assertion in that `It` unchanged — including the ones for `skipping unreadable page`, the full file path, and `already defined` — and add:

   - `Expect(log).ToNot(ContainSubstring("github.com/bborbe/errors.Wrap"))`
   - `Expect(log).ToNot(ContainSubstring("errors_wrap.go"))`
   - `Expect(log).ToNot(ContainSubstring("runtime.goexit"))`

   These three are the load-bearing regression guards: each appears in today's output and must not appear after the change.

3. Do NOT change the sibling `slog.Debug("skipping unreadable task", ...)` in `/workspace/pkg/storage/task.go` — it is a different function on a different call path and is explicitly out of scope for this spec.

4. Do NOT change `/workspace/pkg/ops/list.go`, `/workspace/pkg/cli/cli.go`, or `/workspace/pkg/ops/lint.go`.

5. Add a bullet to the `## Unreleased` section of `/workspace/CHANGELOG.md` immediately after implementing, before running `make precommit`. The section already exists (two sibling prompts for this spec created and appended to it) — APPEND this bullet to it; do not create a second `## Unreleased` heading and do not replace the existing bullets:

   ```
   - fix: the `skipping unreadable page` warning now logs the root cause instead of the fully-wrapped error, shrinking the diagnostic from a ~2 KB single-line stack dump to a readable two-line message
   ```

   Do NOT bump or hand-edit any version string in `CHANGELOG.md` or in `.claude-plugin/plugin.json` / `.claude-plugin/marketplace.json` — the release agent owns those.

6. Coverage: the changed package is `pkg/storage`. The extended `It` from requirement 2 covers the changed line. Do not add retroactive coverage to unrelated untested storage code.
</requirements>

<constraints>
- This is a one-argument change plus three test assertions. If the diff in `pkg/storage/page.go` is more than that single line, stop and reconsider.
- The operator must still see BOTH the file path and the underlying parse reason. Dropping the error detail entirely (e.g. logging only the path, or a generic "could not parse") is NOT acceptable — the point is a readable cause, not a quieter one.
- Do NOT add a flag, config key, or environment variable to control verbosity of this warning.
- Do NOT introduce a helper that formats errors for logging; `errors.Cause` is sufficient and already available.
- `pkg/ops/` is a library layer — no `fmt.Print*`, no `os.Stdout` anywhere in this change.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string.
- Existing tests must still pass, including the other specs in `page_test.go` and the `Context("ListPages")` cases in `markdown_test.go`.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0, including the extended `pkg/storage/page_test.go` specs.

Confirm the changed call logs the cause (must show `errors.Cause(err)`):

```
grep -n 'skipping unreadable page' pkg/storage/page.go
```

Confirm the out-of-scope sibling was not touched (must still show `slog.Debug`):

```
grep -n 'skipping unreadable task' pkg/storage/task.go
```

Confirm exactly one `## Unreleased` heading exists and it carries three `- fix:` bullets:

```
grep -c '^## Unreleased' CHANGELOG.md
sed -n '/^## Unreleased/,/^## v/p' CHANGELOG.md | grep -c '^- fix'
```

The first must print `1`, the second must print `3`.

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
