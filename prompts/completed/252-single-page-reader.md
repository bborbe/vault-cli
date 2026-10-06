---
status: completed
summary: Added storage.PageStorage.ReadPage for single-page reads routed through the shared readEntityComponentsFromPath chokepoint, with byte-identity, single-file-read, missing-file, and unparseable-file tests plus regenerated counterfeiter mocks
execution_id: vault-cli-single-page-reader-exec-252-single-page-reader
dark-factory-version: v0.196.0
created: "2026-10-06T18:39:08Z"
queued: "2026-10-06T18:53:22Z"
started: "2026-10-06T18:56:30Z"
completed: "2026-10-06T19:00:03Z"
---

# Add a single-page read to PageStorage

<summary>
- The vault page store gains a second read: fetch one named page file directly, instead of listing the whole directory
- The page it returns is byte-identical to the one the directory listing produces for the same file — same parser, same metadata, same content
- Reading one page opens exactly one file: an unreadable neighbour in the same directory produces no skip warning
- Unlike the directory listing, a missing or unparseable target file is an error rather than a silent skip
- The generated test doubles are regenerated so the package and its consumers still compile
- Existing directory-listing behaviour — the skip-and-warn diagnostics, the warning cap, the cause truncation — is unchanged
- Groundwork for a caller that refreshes one index entry instead of rebuilding the folder (the caller itself lands in a follow-up prompt)
</summary>

<objective>
Add `ReadPage` to the `storage.PageStorage` interface so a consumer can re-parse exactly one page file and swap that entry in its index, instead of rebuilding the whole folder with `ListPages`. The new read must be byte-identical to the corresponding `ListPages` entry, because the consumer's index is compared against the listing path. This prompt ships the producer only — no in-repo caller is added here, and `pkg/ops/list.go` is untouched; the consumer (vault-ui's page index) lands in a follow-up prompt.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read these files before implementing:

- `pkg/storage/storage.go` — `PageStorage` is declared here with its `//counterfeiter:generate` directive. `Storage` embeds `PageStorage`, and `markdownStorage` embeds `*pageStorage`, so a new method on `*pageStorage` satisfies both composed interfaces with no extra wiring. `NewPageStorage` returns the narrow interface.
- `pkg/storage/page.go` — `ListPages` walks the directory, derives `fileName := strings.TrimSuffix(entry.Name(), ".md")` and `filePath := filepath.Join(targetDir, entry.Name())`, and calls `p.readPageFromPath(ctx, filePath, fileName, vaultPath)` once per entry. `readPageFromPath` lives on `*baseStorage` in the same file and delegates to `readEntityComponentsFromPath` and then `domain.NewPage`.
- `pkg/storage/base.go` — `readEntityComponentsFromPath` is the shared parse chokepoint: it enforces `isSymlinkOutsideVault`, reads the file, stats it for `FileMetadata.ModifiedDate`, and parses frontmatter through `parseToFrontmatterMap`. Routing through it is what makes byte-identity structural rather than coincidental.
- `pkg/storage/page_test.go` — the existing Ginkgo/Gomega suite for `ListPages`. Reuse its shape: `os.MkdirTemp` for the vault, `storage.NewPageStorage(storage.NewConfigFromVault(&config.Vault{}))` for the store, and the `logBuf` + `slog.SetDefault(slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))` capture. It also already defines the `duplicateKeyPage(n)` helper, which returns a page whose frontmatter declares `task_identifier` n times and therefore fails to parse.
- `pkg/ops/list.go` — the only production consumer of `ListPages`; it must keep working unchanged.
- `docs/development-patterns.md` — storage conventions, including the parse-chokepoint rule.
- Coding plugin `go-testing-guide.md` (Ginkgo/Gomega suite shape) and `go-mocking-guide.md` (counterfeiter regeneration) — reference rather than re-derive.
</context>

<requirements>
1. Add `ReadPage` to the `PageStorage` interface in `pkg/storage/storage.go`, immediately after `ListPages`, and leave the existing `//counterfeiter:generate` directive on the interface untouched:

   ```go
   // ReadPage returns a single page from a specific directory in the vault.
   // Unlike ListPages it reads only the named file, and it fails when that file
   // is missing or unparseable rather than skipping it.
   ReadPage(ctx context.Context, vaultPath string, pagesDir string, name string) (*domain.Page, error)
   ```

   `name` is the page name — the file's base name without the `.md` suffix — exactly the value `ListPages` passes as `name` to `readPageFromPath`.

2. Implement `ReadPage` on `*pageStorage` in `pkg/storage/page.go`, directly below `ListPages`:

   ```go
   func (p *pageStorage) ReadPage(
       ctx context.Context,
       vaultPath string,
       pagesDir string,
       name string,
   ) (*domain.Page, error) {
       filePath := filepath.Join(vaultPath, pagesDir, name+".md")
       return p.readPageFromPath(ctx, filePath, name, vaultPath)
   }
   ```

   Do NOT re-implement any parsing, and do NOT implement `ReadPage` as `ListPages`-then-filter: that reads the whole directory and reintroduces the cost this change exists to remove. Byte-identity is achieved by routing through the same `readPageFromPath` → `readEntityComponentsFromPath` → `domain.NewPage` path that `ListPages` uses.

3. Regenerate the counterfeiter mock. `make precommit` runs the `generate` target, which removes `mocks/` and regenerates it from the interface directives; run `make generate` on its own if you want it earlier. `mocks/page-storage.go` carries `var _ storage.PageStorage = new(PageStorage)`, so the build fails until the mock is regenerated. `mocks/storage.go` implements `Storage`, which embeds `PageStorage`, so it must be regenerated too — `make generate` wipes all of `mocks/` and rebuilds it, which covers both.

4. Add a `## Unreleased` section to `CHANGELOG.md` carrying one bullet, placed after the "All notable changes…" preamble and above the current top `## v0.160.3` heading:

   ```
   ## Unreleased

   - feat: `storage.PageStorage` gains `ReadPage` for single-page reads, byte-identical to the corresponding `ListPages` entry
   ```

5. Add tests to `pkg/storage/page_test.go` in a new top-level `Describe("pageStorage.ReadPage", ...)` block with its own `BeforeEach`/`AfterEach` — the existing pair is scoped inside the `pageStorage.ListPages diagnostics` Describe and is not inherited — replicating that shape (temp `vaultPath`, `logBuf` + `slog.SetDefault` capture, `store`):

   a. **Byte-identity.** Write one parseable page. Call `ListPages(ctx, vaultPath, pagesDir)` and `ReadPage(ctx, vaultPath, pagesDir, name)` for the same file. Assert `ReadPage` returns no error, and that its result equals the corresponding `ListPages` entry (`Expect(*got).To(Equal(*want))`, or `reflect.DeepEqual` on the pointees). This equality is the contract the consumer's index depends on.

   b. **Single-page read — the discriminating test.** Put two files in one directory: `Target.md` (parseable) and `Broken.md` (built with the existing `duplicateKeyPage` helper, so it is unparseable). Call `ReadPage` for `Target` and assert all three of:
      - it returns the page and no error;
      - `logBuf.String()` contains **no** `"skipping unreadable page"` — a `ListPages`-and-filter implementation walks the directory, reaches `Broken.md`, and emits exactly that warning, so this assertion is what separates a genuine single-file read from a folder walk;
      - as a positive control in the same test, a following `ListPages` call on the same directory **does** emit `"skipping unreadable page"`, proving the capture is wired and the warning can fire.

   c. **A missing file is an error.** `ReadPage` for a name with no file behind it returns a non-nil error. Assert in the same test that `ListPages` on that directory returns a nil error and no pages — use `Expect(pages).To(BeEmpty())`, not an exact `[]*domain.Page{}` equality: an absent directory makes `ListPages` return `nil` (`pkg/storage/page.go:68-70`). The two methods deliberately disagree here, and the contrast is the point.

   d. **An unparseable target is an error.** `ReadPage` for a file built with `duplicateKeyPage` returns a non-nil error — assert `Expect(err).To(HaveOccurred())` and `Expect(err.Error()).To(ContainSubstring("parse frontmatter"))`. The message does **not** carry the file path: `readEntityComponentsFromPath` wraps a parse failure as `errors.Wrap(ctx, parseErr, "parse frontmatter")`, and only its read-file and symlink-guard branches name the path — so do not assert on the file name here. `ListPages` skips the same file and returns a nil error; that difference is intentional.

6. Before finishing, re-run `<verification>` and confirm it passes, then walk each `<summary>` bullet against the change and confirm it still holds.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass. `ListPages` behaviour is unchanged by this prompt, including the `maxUnreadablePageWarnings` cap, the `truncateCause` bounding, and the "no frontmatter found" diagnostic.
- Error handling uses `github.com/bborbe/errors` (`errors.Wrap(ctx, err, ...)`); never `fmt.Errorf`, never `context.Background()` in non-test code under `pkg/`. `readEntityComponentsFromPath` names the offending path on its read-file branch (`read file %s`) and its symlink guard (`symlink outside vault: %s`), but its parse-failure branch wraps with the bare message `parse frontmatter` and does not name the file — do not add a wrap layer that changes an existing message.
- `ReadPage` must keep honouring the `isSymlinkOutsideVault` guard. It does so by routing through `readEntityComponentsFromPath`; do not bypass that helper.
- Do NOT change the `Page` type, the frontmatter parse, `ListPages`' signature, or `pkg/ops/list.go`.
- Do NOT add new dependencies.
- Repo-relative paths only — no absolute or home-relative paths.
- Formatting and the licence header are enforced by `make precommit`.
</constraints>

<verification>
Run `make precommit` — must pass (ensure, format, generate, test, check, addlicense).

Then confirm the mock was regenerated with the new method:
`grep -n "ReadPage" mocks/page-storage.go` — must print at least one line.

Then confirm the new tests exist:
`grep -n "ReadPage" pkg/storage/page_test.go` — must print at least one line.
</verification>
