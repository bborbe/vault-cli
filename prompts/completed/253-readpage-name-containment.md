---
status: completed
summary: Added an isBarePageName guard so ReadPage rejects names containing path separators, mirrored the contract on the PageStorage interface, and extended page_test.go with name-containment tests.
execution_id: vault-cli-readpage-containment-exec-253-readpage-name-containment
dark-factory-version: v0.196.0
created: "2026-10-06T19:21:56Z"
queued: "2026-10-06T19:28:54Z"
started: "2026-10-06T19:31:22Z"
completed: "2026-10-06T19:35:08Z"
---

# Reject a page name that escapes the pages directory

<summary>
- The single-page read rejects a page name that contains a path separator
- A caller can no longer reach a file outside the pages directory by passing a name with `..` in it
- The interface documents that a page name is a bare base name without the `.md` extension
- A valid name behaves exactly as before — the same page, byte-identical to the directory listing's entry
- Tests cover an escaping name, a nested name and the empty name, each asserted to fail without reading anything
- A regression test proves a file one level above the pages directory is unreachable
</summary>

<objective>
Close the containment gap a reviewer flagged on `ReadPage`: it joins a caller-supplied `name` straight into a filesystem path, so a name containing `..` reads a file outside `vaultPath`. Validate the name at the boundary and pin the contract in the interface doc, so the consumer landing in the follow-up cannot turn a caller-supplied string into an arbitrary path.
</objective>

<context>
Read CLAUDE.md for project conventions, `docs/dod.md` (this repo's Definition of Done — the `validationPrompt` in `.dark-factory.yaml`) and `docs/development-patterns.md` (storage conventions).

Read these files before implementing:

- `pkg/storage/page.go` — `ReadPage` sits directly below `ListPages` and is the method under change: `filePath := filepath.Join(vaultPath, pagesDir, name+".md")`, then `p.readPageFromPath(...)`. `strings`, `filepath` and `github.com/bborbe/errors` are already imported. `ListPages` derives its `name` internally via `strings.TrimSuffix(entry.Name(), ".md")`, so it can never produce a separator — this guard is about the caller-supplied name only.
- `pkg/storage/storage.go` — the `PageStorage` interface, where `ReadPage` carries its doc comment.
- `pkg/storage/base.go` — `isSymlinkOutsideVault` (line 375) is the existing containment idiom and `readEntityComponentsFromPath` (line 303) is the shared read chokepoint that calls it. Read both before writing: it guards symlink *targets* only, returning false for a non-symlink path, so it does **not** cover a `..` segment. That is precisely why the guard belongs at the name boundary instead.
- `pkg/storage/topic.go` — `isTopicNameWithinDir` (line 28) is the closest in-package precedent: a caller-supplied-name containment guard for `FindTopicByName`, with its traversal-refusal test at `pkg/storage/topic_test.go:153`. It is prefix-based but appends the path separator (line 32), so unlike `base.go:401` it does **not** carry the `/vault` vs `/vault-evil` sibling-prefix weakness. It is deliberately *not* reused here: a topic name may carry a subpath (`sub/x` stays inside and is allowed — lines 26-27), whereas a page name is a bare base name, so the page guard is exact and rejects `sub/x`. Do not refactor `isTopicNameWithinDir`.
- Coding plugin `go-testing-guide.md` (Ginkgo v2 / Gomega suite shape) and `go-error-wrapping-guide.md` (`errors.Errorf` with `ctx`) — reference rather than re-derive.
- `pkg/storage/page_test.go` — the `Describe("pageStorage.ReadPage", ...)` block added by the previous change. Extend it rather than starting a new one; its `BeforeEach`/`AfterEach`, the `logBuf` capture and the `parseablePage(name)` helper are already there.
</context>

<requirements>
1. Add an unexported helper to `pkg/storage/page.go` next to `ReadPage`:

   ```go
   // isBarePageName reports whether name is usable as a page base name: non-empty,
   // free of path separators (both `/` and `\`), and not a relative-path segment.
   func isBarePageName(name string) bool {
       if name == "" || name == "." || name == ".." {
           return false
       }
       return !strings.ContainsAny(name, `/\`)
   }
   ```

   Appending `.md` cannot introduce a separator, so a name that passes this check cannot leave `pagesDir`. The guard is exact rather than prefix-based.

2. Reject a non-bare name at the top of `ReadPage`, before any path is built, and extend its doc comment to state the contract:

   ```go
   // ReadPage returns a single page from a specific directory in the vault.
   // Unlike ListPages it reads only the named file, and it fails when that file
   // is missing or unparseable rather than skipping it.
   //
   // name must be a bare page base name without the .md extension — the same
   // value ListPages reports in Page.Name. A name containing a path separator
   // is rejected, so a caller-supplied value cannot escape pagesDir.
   func (p *pageStorage) ReadPage(
       ctx context.Context,
       vaultPath string,
       pagesDir string,
       name string,
   ) (*domain.Page, error) {
       if !isBarePageName(name) {
           return nil, errors.Errorf(
               ctx,
               "invalid page name %q: must be a bare base name without a path separator",
               name,
           )
       }
       filePath := filepath.Join(vaultPath, pagesDir, name+".md")
       return p.readPageFromPath(ctx, filePath, name, vaultPath)
   }
   ```

   Do **not** substitute an `Abs` + `HasPrefix` containment check. The name guard is exact, whereas a prefix check inherits the sibling-prefix weakness already present at `base.go:401` (`/vault` also matches `/vault-evil`) and would still need symlink resolution on top. Leave `isSymlinkOutsideVault` untouched.

3. Mirror the contract on the interface method in `pkg/storage/storage.go`, so a consumer reading only the interface sees the precondition:

   ```go
   // ReadPage returns a single page from a specific directory in the vault.
   // Unlike ListPages it reads only the named file, and it fails when that file
   // is missing or unparseable rather than skipping it.
   //
   // name must be a bare page base name without the .md extension — the same
   // value ListPages reports in Page.Name.
   ReadPage(
       ctx context.Context,
       vaultPath string,
       pagesDir string,
       name string,
   ) (*domain.Page, error)
   ```

4. Extend the existing `Describe("pageStorage.ReadPage", ...)` block in `pkg/storage/page_test.go` with a `Context("name containment", ...)`:

   a. **An escaping name cannot reach a file outside the pages directory.** Create the pages dir, then write a *parseable* page at `filepath.Join(vaultPath, "Outside.md")` — one level above `pagesDir`. Call `ReadPage(ctx, vaultPath, "Pages", "../Outside")`. Assert a non-nil error, a nil page, and that `err.Error()` contains `invalid page name`. This is the regression lock: without the guard, that call resolves to `vaultPath/Outside.md` and returns it.

   b. **A nested name is rejected.** `ReadPage(ctx, vaultPath, "Pages", "sub/Dir")` returns a non-nil error and a nil page.

   c. **The empty name is rejected.** `ReadPage(ctx, vaultPath, "Pages", "")` returns a non-nil error and a nil page.

   d. **A bare name still reads.** In the same context, write a parseable page and confirm `ReadPage` for its bare name returns it with no error — the guard must not break the happy path.

   e. **The dot names and a backslash name are rejected.** `ReadPage(ctx, vaultPath, "Pages", ".")`, `ReadPage(ctx, vaultPath, "Pages", "..")`, and a name written as the Go raw string literal `` `a\b` `` each return a non-nil error and a nil page. The first two exercise the `name == "." || name == ".."` arm and the third the backslash arm — neither of which the separator cases in (a) and (b) reach.

5. Add a `## Unreleased` bullet to `CHANGELOG.md`, creating the section if the top heading is a released version and placing it after the "All notable changes…" preamble:

   ```
   ## Unreleased

   - fix: `storage.PageStorage.ReadPage` rejects a page name containing a path separator, so a caller-supplied name can no longer escape the pages directory; the interface now documents that `name` is a bare base name without the `.md` extension
   ```

6. Before finishing, re-run `<verification>` and confirm it passes, then walk each `<summary>` bullet against the change and confirm it still holds.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT change `ListPages`, `readPageFromPath`, `readEntityComponentsFromPath` or `isSymlinkOutsideVault`. This change adds a guard in front of the existing path; the shared read chokepoint stays as it is.
- The byte-identity contract is unchanged: a valid name must still produce a page byte-identical to the `ListPages` entry for the same file, and the specs already asserting that must keep passing untouched.
- Error handling uses `github.com/bborbe/errors` (`errors.Errorf(ctx, ...)`); never `fmt.Errorf`. `fmt` is already imported in `page.go` for the existing `ListPages` warning — do not remove it.
- Do NOT add new dependencies.
- Out of scope: `baseStorage.findFileByName` (`base.go:232`) joins a caller-supplied name unguarded for the `Find*ByName` family (`FindTaskByName`, `FindGoalByName`, `FindThemeByName`, `FindObjectiveByName`, `FindVisionByName`, `FindDecisionByName`). `pkg/storage/topic.go:39-41` records that this is deliberate — `findFileByName` is left byte-identical so the goal and task families keep their current behaviour, and only `FindTopicByName` guards its name. This prompt closes `ReadPage` only; do not widen the guard to `findFileByName` here.
- Repo-relative paths only — no absolute or home-relative paths.
- Formatting and the licence header are enforced by `make precommit`.
</constraints>

<verification>
Run `make precommit` — must pass (ensure, format, generate, test, check, addlicense).

Then confirm the guard is present:
`grep -n "isBarePageName" pkg/storage/page.go` — must print at least two lines (the helper and its call site).

Then confirm the regression test exists:
`grep -n "name containment" pkg/storage/page_test.go` — must print at least one line.
</verification>
