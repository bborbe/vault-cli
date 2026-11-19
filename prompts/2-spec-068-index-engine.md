---
status: draft
spec: [068-pkg-index-metadata-page-index]
created: "2026-11-19T20:45:00Z"
branch: dark-factory/pkg-index-metadata-page-index
---

# Build the in-memory page index engine with the ported refresh contract

<summary>
- A caller can ask for a vault folder's pages and get them from memory, in the same order and with the same frontmatter, names, paths and modification times as vault-cli's own folder listing, with every body empty.
- The same folder can also be read as entries, which add each page's pre-parsed Open Questions without a second read and without changing the page pointers.
- A full body is still available on request, read from disk every time and never retained, so a listing cannot grow the process by the size of its files.
- A folder that has not changed is never re-read: a refresh lists the folder once, compares each file's size, modification time and status-change time, and re-reads only what differs — publishing nothing and keeping every pointer when nothing did.
- A single file can be re-read on its own, so a write or an editor event costs one file read instead of a folder walk.
- A caller can learn which pages changed after a revision it holds, and is told plainly when the set cannot be bounded.
- Concurrent readers share one build per folder; a failed rebuild keeps serving the previous snapshot; a cancelled reader does not abort work others depend on.
- A background rescan keeps every known folder current on a fixed 50-second cadence.
- A written contract document describes the metadata-only rule, the entry rule, lazy body reads, the staleness bounds, the incremental-update rules and key derivation, and the changelog records the new package.
</summary>

<objective>
Implement `pkg/index`'s snapshot engine: `PageIndex`, `NewPageIndex`, the per-key immutable snapshots, the ported build / stat-diff / per-file mark / revision / change-log / rescan contract, and the test suite that pins it against vault-cli's own listing. This is spec 068 prompt 2 of 3: Desired Behaviors 1–5, plus Acceptance Criteria 1–6, 8, 9 (all containers except `Store hydration`), 14 (the six non-store headings) and 15. It depends on prompt 1's seams.
</objective>

<context>
Read `CLAUDE.md`, `docs/development-patterns.md` and `docs/dod.md` first.

Read these files fully before writing anything:

- `specs/in-progress/068-pkg-index-metadata-page-index.md` — Summary, Goal, Non-goals, Desired Behavior 1–5, Constraints "Decisions (frozen for this spec)", "Must not change", Failure Modes, Security, AC2–AC6, AC8, AC9, AC14, AC15.
- `pkg/index/seams.go`, `pkg/index/reader.go`, `pkg/index/lister.go`, `pkg/index/key.go`, `pkg/index/metrics.go` — prompt 1's output. Do not change their exported shapes; `Entry` and `FileFingerprint` are what this engine stores.
- `pkg/storage/page.go` — `pageStorage.ListPages` is the reference listing the equivalence tests compare against; its skip rules (directories, non-`.md`, unreadable files, symlinks out of the vault) are the ones the index must reproduce.
- `pkg/ops/list.go` — `NewListOperation(pageStorage storage.PageStorage) ListOperation`, `ListOperation.Execute(...) ([]TaskListItem, error)`. AC5 runs both operations over the same fixture and compares the items.
- `pkg/domain/page.go` — `Page`, `NewPage`, `FrontmatterMap.GetString` / `GetStringSlice`, and `RawMap()`.
- `pkg/domain/file_metadata.go` — `FileMetadata{Name, FilePath string; ModifiedDate *time.Time}`.
- `pkg/ops/export_test.go` — the pattern for `_test` helpers that expose private state.
- `Makefile` (`generate`, `test`, `precommit`) and `.golangci.yml` (funlen 80/50, nestif 4, gocognit 20, dupl, maintidx 20).

Library APIs already verified for this prompt:
- `github.com/bborbe/run` — `type Func func(ctx context.Context) error`; `run.All(ctx context.Context, funcs ...Func) error`. Use it for concurrent per-key work; never a bare `go func()`.
- `github.com/bborbe/time` (imported as `libtime`) — `libtime.CurrentDateTimeGetter` with `Now() DateTime`; `libtime.WaiterDuration` with `Wait(ctx context.Context, duration Duration) error`; `libtime.Duration`; `libtime.NewWaiterDuration()`; `libtime.NewCurrentDateTime()`.
- `github.com/bborbe/errors` — `errors.Wrapf(ctx, err, format, args...)`, `errors.Wrap(ctx, err, msg)`, `errors.Errorf(ctx, format, args...)`.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md`
</context>

<requirements>
1. **Declare the engine's public surface in `pkg/index/index.go`.** The interface is frozen (AC2 greps `go doc ./pkg/index PageIndex` for the embedded storage interface plus these ten method names):

   ```go
   type PageIndex interface {
       storage.PageStorage // supplies ListPages and ReadPage

       ListEntries(ctx context.Context, vaultPath string, pagesDir string) ([]*Entry, error)
       Build(ctx context.Context, keys []Key) error
       Refresh(ctx context.Context, key Key) error
       RefreshFile(ctx context.Context, key Key, filename string) error
       MarkDirty(keys ...Key)
       MarkFileDirty(key Key, name string)
       ForceReload()
       Revision(key Key) uint64
       ChangedPagesSince(key Key, revision uint64) (names []string, ok bool)
       Rescan(ctx context.Context) error
   }
   ```

   Copy the method semantics from the spec's Desired Behavior 4 into the GoDoc for each method — the doc comment is part of the contract. Add the counterfeiter directive above the interface producing `mocks/index-page-index.go` with fake name `IndexPageIndex`.

2. **Constructors and the warning sink** (same file):

   ```go
   func NewPageIndex(
       reader PageReader,
       lister DirectoryLister,
       currentDateTimeGetter libtime.CurrentDateTimeGetter,
       waiter libtime.WaiterDuration,
       metrics Metrics,
   ) PageIndex

   func SetWarnf(idx PageIndex, warnf func(format string, args ...any))
   ```

   `NewPageIndex` stores the five dependencies and defaults the warning sink to `log/slog` (`slog.Warn(fmt.Sprintf(format, args...))`). A nil `metrics` becomes `NewNoopMetrics()`. `SetWarnf` type-asserts the value to the private struct; a nil `warnf` or a foreign implementation is ignored (no panic).

   Do **not** add `Store`, `StoredEntry`, `NewPageIndexWithStore` or any persistence in this prompt — prompt 3 adds them and rewires `NewPageIndex` to delegate.

3. **Per-key state and immutable snapshots.** One private `pageIndex` struct guarded by a single `sync.Mutex` holds `entries map[Key]*entry` plus the five dependencies and the `warnf` field. One `entry` holds:

   - `snapshot published`, `hasSnapshot bool`
   - `fingerprints map[string]FileFingerprint` — keyed by base filename **with** `.md`, recorded at each read, including files that were excluded
   - `fileReadSeq map[string]uint64` — per filename, the sequence of the last applied single-file read; a removed file keeps its entry as a tombstone
   - `writeMarked map[string]uint64` — filename → request sequence of a pending per-file write mark
   - `requestSeq uint64`, `reloadSeq uint64`, `revision uint64`
   - `changeLog []pageChange`
   - `inflight *build`, `followUp *build`

   ```go
   type published struct {
       entries []*Entry
       pages   []*domain.Page
   }
   ```

   `entries` and `pages` hold the same order and the same page pointers; `pages` is nil exactly when `entries` is nil. A helper `publish(entries []*Entry) published` builds both slices. A published snapshot is never mutated: every publication builds a new slice, so a reader holding the old one keeps a whole consistent view (Failure Modes: no torn reads).

   ```go
   type pageChange struct {
       revision uint64
       names    []string
       bounded  bool
       reason   string
   }
   ```

   `const maxChangeLog = 64`; `recordChangeLocked(names []string, bounded bool, reason string)` appends `{revision: e.revision, ...}` and trims the log to the newest 64 entries. The caller must already hold the mutex and already have advanced `revision`.

4. **Constants** (ported as-is, invariant — no configuration): `RescanInterval = 50 * time.Second`, private `rescanPollInterval = time.Second`, private `rebuildTimeout = 2 * time.Minute`, private `maxChangeLog = 64`.

5. **`ListPages` / `ListEntries` / `list`.** Both build a `NewKey(vaultPath, pagesDir)` and resolve the key through one shared private `list(ctx, vaultPath, pagesDir) (published, error)`:

   - `ListPages` returns `snap.pages`; `ListEntries` returns `snap.entries`.
   - `list` calls `ensure(ctx, key, false)`. A cancellation of the caller's own context is returned as a wrapped error. A build error is returned as a wrapped error **only when the key has no published snapshot**; otherwise the previous snapshot is served silently (Failure Modes: a failed rebuild keeps the previous snapshot).
   - A warm read touches no disk (Staleness bounds).

6. **`ReadPage`.** Validate the bare name first with the same rule `MarkFileDirty` uses (`validFileMarkName`: non-empty, no `..`, no `filepath.Separator`, no `/`) and return `errors.Errorf(ctx, "invalid page name %q", name)` otherwise. Then delegate to `p.reader.ReadPage(ctx, vaultPath, pagesDir, name)`, wrapping a failure as `read page %s`. It must never lock, mutate or republish the snapshot and never move the revision (AC6: a following `ListPages` returns the same `*domain.Page` pointers).

7. **`Build(ctx, keys []Key) error`.** Normalise each key with `NewKey`, run one `run.All` func per key that resolves the key with `ensure(ctx, key, false)`, and return only the caller's cancellation (wrapped as `build page index`). A per-key failure is logged and does not fail the build. An already-cancelled ctx returns the wrapped error before doing any work.

8. **`Refresh(ctx, key Key) error`.** Normalise the key and call `ensure(ctx, key, true)`. Return nil whether the build succeeded or failed; return the wrapped error only when the caller's own ctx was cancelled.

9. **Build coalescing — the ordering contract.** This is the part the tests pin, so implement it exactly:

   - Each `build` carries `kind buildKind`, `reason string`, `startSeq uint64`, `started chan struct{}`, `done chan struct{}`, and result fields `snap published` / `err error` written under the mutex before `done` is closed.
   - Build kinds: `buildCold` (no snapshot yet: list and read every file), `buildStatDiff` (list and re-read only fingerprint-differing entries), `buildFileReads` (re-read exactly the write-marked files, list nothing), `buildReload` (list and re-read every file, ignoring fingerprints).
   - Per key there is at most one `inflight` build and at most one `followUp`.
   - A build covers requested work only when `build.startSeq >= needSeq` **and** the kind is compatible: cold and reload cover every kind; stat-diff covers only stat-diff; file-reads covers only file-reads. (Prompt 3 adds `buildHydrate`, which covers cold, stat-diff and hydrate.)
   - Acquiring work for a key: if `inflight` covers the request, wait on its `done`; otherwise if `followUp` covers it, wait for the follow-up to be promoted (its `started` channel closed) and then run it; otherwise create the build, set it as `inflight`, close its `started`, and run it.
   - When a build finishes: if a `followUp` exists, promote it to `inflight`, clear `followUp`, close the follow-up's `started` and run it; otherwise clear `inflight` and close `done`.
   - The build runs under `context.WithTimeout(context.WithoutCancel(ctx), rebuildTimeout)` so a caller's cancellation never aborts work other readers depend on. A waiter selects on `<-build.done` and `<-ctx.Done()`; on the latter it returns a cancellation error of its own and leaves the build running.
   - After waiting, the caller re-evaluates pending work: a mark recorded while the build was running is not covered, so the caller loops and resolves it. Bounded loops only — each iteration must consume pending work.
   - A reader never blocks on an event-driven or rescan-driven read: `ensure` with `refresh == false` blocks only while a **mark** is pending, and is served from the current snapshot otherwise.
   - Two concurrent cold reads of one key share one build; a burst of refreshes coalesces into one follow-up (not one build each).

10. **Pending-work decision (`pendingLocked`).** Under the mutex, for one key, in this order:

    - no snapshot → cold build;
    - a reload is pending (`reloadSeq` newer than the last resolved reload) → reload;
    - any write-marked filename is still unresolved → file-reads build (reason `ReadReasonWrite`);
    - a folder-level mark is pending → stat-diff (reason `ReadReasonRescan` when requested by `Refresh`, `ReadReasonWrite` when resolving a mark);
    - nothing pending → serve the current snapshot.

    A `Refresh` bumps `requestSeq` before evaluating, so it always gets a stat-diff that started after the call.

11. **Cold build and stat-diff listing (`executeListing`).** One listing through `lister.ListFiles(ctx, key.VaultPath, key.PagesDir)`:

    - cold: read every listed file through `reader.ReadEntry`; reload: the same but with reason `ReadReasonReload`; stat-diff: read only entries whose `FileFingerprint` differs from `e.fingerprints[name]` (compare with `!=` — inequality, never "newer" — so a clock-skewed or restored mtime is still detected) or that have no recorded fingerprint.
    - Merge against the current snapshot: keep the current `*Entry` pointer for a listed name that was not re-read; insert a re-read entry at its filename-ordered position (order key is `entry.Page.Name + ".md"`, the `os.ReadDir` order); drop every current entry whose name is no longer listed.
    - Replace `e.fingerprints` with a map holding exactly the listed names (dropping a name that vanished), keeping the non-zero fingerprint of an excluded-but-present file.
    - An unchanged stat-diff: no reads, no publication, no revision advance, the snapshot slice and the fingerprint map keep their identity, and the pass allocates nothing beyond the one listing.
    - A changed stat-diff: `revision++`, `recordChangeLocked(changedFilenames, true, "")` with the names that were re-read or dropped, then swap the snapshot.
    - The **first** publication of a key advances the revision and records an unbounded change (`recordChangeLocked(nil, false, UnboundedFirstPublication)`), so `ChangedPagesSince` refuses to bound it.
    - Count each single-file read with `metrics.FileRead(reason)`: `ReadReasonBuild` for a cold build, `ReadReasonReload` for a reload, `ReadReasonWrite`/`ReadReasonRescan` for a stat-diff.
    - The listing call is bounded by `rebuildTimeout`. A listing error keeps the previous snapshot, leaves the mark pending so the next read retries, logs the error with the key, and sets the build error (a first build then returns that error to its caller).

12. **`RefreshFile(ctx, key, filename)`** in `pkg/index/index_file.go`:

    - Validate the filename: non-empty, `filepath.Base(filename) == filename`, no `filepath.Separator`, no `..`, and it must end in `.md`. Reject with `errors.Errorf(ctx, "invalid page filename %q", filename)` and read nothing.
    - A key with no published snapshot has nothing to splice into: delegate to `Refresh(ctx, key)` so the folder build includes the file.
    - Otherwise take the next global read sequence under the mutex (`readSeq++`), then read the file **outside** the mutex with `reader.ReadEntry` under `context.WithTimeout(context.WithoutCancel(ctx), rebuildTimeout)`, and count it with `metrics.FileRead(ReadReasonEvent)`.
    - Apply the result under the mutex only when the read's sequence is not older than `e.fileReadSeq[filename]` (a later-started read wins). The on-disk state decides the outcome, not the caller's intent: present and readable replaces or inserts the entry at its filename-ordered position; absent or unreadable removes it. Record the fingerprint (or delete it when the stat failed and the file is gone, so the unchanged fast path is not blocked by a zero-fingerprint tombstone), advance `revision`, and record a bounded change naming exactly this filename.
    - A splice builds a new slice (`splicePage(pages []*Entry, name string, page *Entry) []*Entry`, using `sort.Search` on `pages[i].Page.Name+".md" >= name`); the input slice and its pointers are never mutated. A nil `page` removes the name.
    - Warn once per fingerprint change about an excluded-but-present file through `p.warnf`; a vanished file logs at `slog.Debug` only. The returned error is the caller's own cancellation (wrapped as `refresh file`), never a read failure.
    - Return `nil` after the result is applied even when the caller's ctx was cancelled mid-read.

13. **`MarkDirty`, `MarkFileDirty`, `ForceReload`, `Revision`.**

    - `MarkDirty(keys ...Key)` normalises each key and, for a key the index already knows, bumps `requestSeq`, advances `revision` and records an unbounded folder change with reason `UnboundedFolderMarkExplicit`. An unknown key is a no-op (its first read builds it).
    - `MarkFileDirty(key, name)` applies the exactness rule before marking anything: strip a `[[` / `]]` wrapper (`stripWikilinkName`), then require a non-empty plain base name with no `..` and no separator (`validFileMarkName`), a byte-equal page name in the current snapshot (`snapshotHasName`), and `<name>.md` present on disk (`os.Stat`). Each failure widens to a folder-level mark with its own reason: unknown/unpublished key → `UnboundedFolderMarkWidenedKey`; invalid name → `UnboundedFolderMarkWidenedNameInvalid`; name not in the snapshot → `UnboundedFolderMarkWidenedName`; file absent → `UnboundedFolderMarkWidenedFile`. On success: bump `requestSeq`, store `writeMarked[name+".md"] = requestSeq`, advance `revision`, and record a bounded change naming that one filename.
    - A folder-level mark bumps `requestSeq`, advances `revision`, and records `recordChangeLocked(nil, false, reason)`.
    - `ForceReload()` marks every known key: `requestSeq++`, `reloadSeq = requestSeq`, `revision++`, `recordChangeLocked(nil, false, UnboundedForcedReload)`. It is the only path that ignores fingerprints.
    - `Revision(key)` returns the key's revision, 0 for an unknown key.

14. **`ChangedPagesSince(key, revision) (names []string, ok bool)`.** Return a fresh copy of the names (never the internal slice). `ok == false` — and exactly one `metrics.ChangedPagesUnbounded(reason)` call — when: the key is unknown (`UnboundedUnknownKey`); `revision > e.revision` (`UnboundedRevisionAhead`); `revision == e.revision` → `nil, true`; the retained log no longer reaches back to `revision` (`len(changeLog) == 0 || changeLog[0].revision > revision+1` → `UnboundedWindowNotRetained`); or the earliest unbounded advance in the window decides the cause (its own `reason`). Names are base filenames **with** `.md`, de-duplicated, in first-seen order.

15. **`Rescan(ctx) error`.** Loop: `waiter.Wait(ctx, libtime.Duration(rescanPollInterval))`; a wait error (cancellation) returns nil. When `currentDateTimeGetter.Now().Sub(lastRescan).Duration() < RescanInterval`, continue. Otherwise refresh every known key concurrently with one `run.All` (so rescans never pile up) and loop. Never return a refresh error — a rescan keeps going.

16. **Create `pkg/index/export_test.go`** (package `index`) with:

    - `CountingMetrics` implementing `Metrics`, with `NewCountingMetrics() *CountingMetrics`, `Reads(reason string) int` and `TotalReads() int` (mutex-guarded maps).
    - `var TestMetrics = NewCountingMetrics()` — shared by every test index, so callers assert deltas, never absolutes.
    - `func NewPageIndexWithWarnf(reader PageReader, lister DirectoryLister, currentDateTimeGetter libtime.CurrentDateTimeGetter, waiter libtime.WaiterDuration, warnf func(format string, args ...any)) PageIndex` — builds with `TestMetrics` and applies `SetWarnf`.
    - `func SnapshotIdentity(idx PageIndex, key Key) string` and `func FingerprintSetIdentity(idx PageIndex, key Key) string` — return `fmt.Sprintf("%p", ...)` for the key's published entries slice / fingerprint map, `""` for an unknown or unpublished key. The allocation test uses these to prove an unchanged pass replaced nothing.

17. **Write the engine tests.** External package `index_test` unless private access is required. Required container texts (exact, case-sensitive — AC9 greps `-ginkgo.v` output) and required spec texts:

    - `index_statdiff_test.go` — `Describe("Stat-diff rescan")` containing at least: `It("AC4(a) reads nothing and republishes nothing when nothing changed")` (assert zero reads, unchanged `SnapshotIdentity` and `FingerprintSetIdentity`, unchanged revision), `It("AC4(b) re-reads exactly the modified and the added file")`, `It("AC4(c) detects a same-size rewrite whose modification time was restored")`, `It("AC5(ii)/8 lists once and re-reads only what changed after a folder mark")`, `It("AC4/failure 9 keeps the snapshot and the mark pending when the listing fails")`, `It("empties and refills the snapshot when the folder is deleted and recreated")`.
    - `index_writemark_test.go` — `Describe("Per-file write marks")` containing at least `It("AC5(i) re-reads exactly the marked file and lists nothing")`, `It("AC5(i) shares one re-read between two concurrent readers")`, `It("AC5(i) strips a [[wikilink]] wrapper from the id")`, `It("AC6(d) blocks a concurrent reader until the marked file is re-read")`, `It("re-reads a file marked again while its read was in flight")`, a `DescribeTable` `It("falls back to a folder-level mark for the id "+name, ...)` covering each widened reason, `It("counts the write, rescan and reload read sites under their own reason")`, and `It("AC5(iii) reads every file exactly once after ForceReload")`.
    - `index_allocation_test.go` — two top-level containers. `Describe("Allocation")` with `It("allocates nothing beyond its folder listing on an unchanged stat-diff", ...)` that calls `Skip("allocation measurement is only meaningful without -race")` when the race detector is enabled; and `Describe("Per-file write marks")` with `It("re-reads a write-marked file even when no fingerprint changed", ...)` which is **never** skipped. Add `pkg/index/norace_enabled_test.go` (`//go:build !race`, `const raceDetectorEnabled = false`) and `pkg/index/race_enabled_test.go` (`//go:build race`, `const raceDetectorEnabled = true`) in package `index_test`. The allocation measurement: build a real temp vault with 5000 parseable pages (`Page%05d.md`, frontmatter plus a heading), cold-list it, measure the `TotalAlloc` delta of one `lister.ListFiles` call as the baseline, then measure one unchanged `Refresh` and assert its delta is at most 1.35× the baseline and at most 5,500,000 bytes; assert the snapshot and fingerprint identities are unchanged and the reader read nothing. Then change 10 files, assert a `Refresh` reads exactly those 10 names and allocates at most 1.5× the baseline. Then remove the folder and assert a second refresh over it keeps identity and reads nothing.
    - `index_changedpages_test.go` — `Describe("ChangedPagesSince")` with a `DescribeTable("bounds the set of pages that moved", ...)` plus `It("reports unknown for a key it has never seen")`, `It("reports unknown for a revision the key has never reached")`, `It("reports unknown once an unbounded mark falls inside the range")`, `It("reports unknown after a cold build publishes the whole snapshot")`.
    - `index_revision_test.go` — `Describe("Revision")` covering: 0 for an unknown key; advances on a cold publication, a folder mark, a file mark and its folder fallback, a forced reload, a file mark of an existing file, a rescan publication and a file event; and does **not** advance when a stat-diff finds nothing changed.
    - `index_test.go` — `Describe("PageIndex")` with `Describe("ListPages")` (`It("serves repeat reads of a clean key without touching storage")`), `Describe("Cold read")` containing `It("shares a single build across concurrent cold reads")` (one listing for N concurrent readers), `Describe("Refresh")` (`rebuilds only the requested key`, `keeps serving the previous snapshot while a rebuild is blocked`, `coalesces a burst of refreshes into one follow-up`), `Describe("dirty marks")`, `Describe("Failed rebuild")` containing `It("keeps the previous snapshot and retries while the key stays dirty")` and `It("returns an error when the very first build fails")`, `Describe("Rescan")` (`refreshes every known key once the interval has passed`, `keeps the rescan interval inside the 60s ceiling`), `Describe("context cancellation")` (`fails a waiting reader without aborting the build it waits on`, `runs an owned follow-up even when the caller's ctx is cancelled`, `returns an error from Build when ctx is already cancelled`), and `Describe("NewKey")` (`cleans both parts so equivalent paths share a key`).
    - `index_file_test.go` — `Describe("RefreshFile")` covering: insert at the filename-ordered position; replace the changed page and share every other pointer; remove a deleted file; remove a page whose file is gone whatever the caller meant; keep a page whose file is still there; leave a previously obtained slice and its pointers untouched; reject a filename that is not a single plain `.md` base name; build the folder when the key has no snapshot yet; keep a dirty mark an unrelated `RefreshFile` does not clear; count reads by reason. Plus `Describe("ordering concurrent reads")` with `AC6(a) publishes the later-started read of a file`, `AC6(b) never drops one file's read for another's`, `AC6(b) keeps a file created while a folder build was listing`, `AC6(c) serves the previous snapshot while a read is blocked`, `AC6/behavior 6 lets an event beat a folder build that started earlier`, `AC6/behavior 6 lets a folder build beat an event that started earlier`, and `Describe("a cancelled caller")` with `still applies the read and reports the cancellation`.
    - `index_file_internal_test.go` — package `index`, `ginkgo.Describe("splicePage", ...)` with `inserts into an empty slice`, `inserts first`, `inserts last`, `replaces in place without mutating the input`, `removes a present page without mutating the input`, `returns the pages unchanged when removing an absent one`.
    - `equivalence_test.go` — one `Describe("Snapshot equivalence")` with these exact spec texts (AC3–AC6, AC8): `It("lists the same pages as vault-cli storage with only Content excluded", ...)`; `It("returns pages and entries with empty Content and populated frontmatter, name, path and modification date", ...)`; `It("produces the same list-operation items as vault-cli storage", ...)`; `It("reads the full body on ReadPage without caching it in the snapshot", ...)`; `It("republishes a body-only edit and names it in ChangedPagesSince", ...)`. Also add `matches vault-cli on an existing folder with no page files`, `AC3 stays equal to vault-cli through create, modify, delete and rename`, `matches vault-cli on a missing folder`, `refuses a ReadPage name that must never become a file path`.

    The equivalence fixture is the awkward-files set the spec names: a plain page, a bare wikilink page, a file with no frontmatter, a file with invalid YAML, a non-`.md` file, a subdirectory, a symlink pointing out of the vault, a broken symlink, a symlink to a page inside the vault, and a non-ASCII filename. Compare `ListPages` with `storage.NewPageStorage(nil).ListPages` where the reference side is deep-copied with `Content` zeroed (`reflect.DeepEqual`), and include a positive control (the fixture must not be empty) so a silently-empty snapshot cannot pass.

    AC8's spec covers both entry points: edit only the Open Questions section of one file, then call `RefreshFile` (and, in a second pass, `Refresh`) and assert a new `*domain.Page` pointer, changed `OpenQuestions`, and `ChangedPagesSince(previousRevision)` naming the file with `ok == true`.

18. **Write `docs/page-index.md`** (create the file). Title `# Page index` and exactly these seven `## ` headings at the end of prompt 3; this prompt writes the six non-store ones, in this order:

    ```
    ## Metadata-only records
    ## Entries and open questions
    ## Lazy body reads
    ## Staleness bounds
    ## Incremental updates
    ## Key derivation
    ```

    Content: the metadata-only rule (full frontmatter map, name, path and modification time exactly as vault-cli's listing; empty `Content`; body read and dropped; the equivalence guarantee including the awkward-file list), the entry/Open Questions rule (same order and pointers as `ListPages`, `Text` is the question, never nil, every text a `strings.Clone` copy, a parse error fails the read), lazy body reads (`ReadPage` always goes to disk, never caches, never mutates the snapshot, never moves the revision, rejects a name with a separator or `..`), staleness bounds (warm reads touch no disk; 50 s rescan plus 1 s poll plus one stat-diff, each storage call bounded by 2 min; the first read after a mark resolves it; concurrent cold reads share a build; event and rescan reads never block `ListPages`; `ForceReload` is the only fingerprint-ignoring path; a failed listing keeps the previous snapshot and the mark stays pending; a failed first build returns an error), incremental updates (`RefreshFile` rules, fingerprint contents and symlink-following stat, the stat-diff by inequality, unchanged passes read/publish/allocate nothing, `MarkDirty` / `MarkFileDirty` exactness and widening, `Revision` and `ChangedPagesSince` including every `ok == false` case and the 64-entry window), and key derivation (`NewKey` with `filepath.Clean`; a mismatched derived key silently disables event refresh).

    State in the opening section that published entries and pages are read-only for every consumer.

19. **Add the changelog entry.** Insert a `## Unreleased` section immediately after the changelog preamble and before the newest `## vX.Y.Z` section, with one bullet:

    ```
    - feat: add `pkg/index`, a shared metadata-only page index (frontmatter, name, path, mtime and pre-parsed Open Questions per page, never the body) with the ported stat-diff refresh contract and lazy on-disk body reads
    ```

    Do not bump any version string and do not tag — the releaser owns both.

20. **Self-check before finishing**: re-run the `<verification>` block, confirm it passes, and walk AC2–AC6, AC8, AC9 (every container except `Store hydration`), AC14 (the six headings) and AC15 against the result.
</requirements>

<constraints>
- Do NOT change `pkg/cli`, `pkg/config`, `pkg/domain`, `pkg/ops` or `pkg/storage` (AC16). This prompt adds `pkg/index` files, tests, docs and one changelog bullet only.
- Do NOT switch any vault-cli CLI command to the index — the CLI keeps reading files directly (spec Non-goal).
- Do NOT cache a page body anywhere. `ListPages` and `ListEntries` return pages with empty `Content`; `ReadPage` always reads the file.
- Do NOT project or intern frontmatter keys — the full map is retained.
- Do NOT add persistence, a `Store`, `StoredEntry`, `NewPageIndexWithStore` or any bolt dependency in this prompt — prompt 3 owns all of it.
- Do NOT add configuration for the store location, rescan interval, timeouts or change-log size; the ported constants are invariant (spec Non-goal).
- Do NOT port vault-ui's board-level sections (task-list snapshot, body cache, frame ordering, watcher wiring) — they stay in vault-ui (spec Non-goal).
- `pkg/index` must import neither `github.com/golang/glog` nor any `github.com/prometheus/*` package directly (AC13).
- Concurrency: use `run.All` for concurrent per-key work, never a bare `go func()`. Every wait must select on the caller's `ctx.Done()`. A build outlives a cancelled caller via `context.WithoutCancel` plus `rebuildTimeout`.
- Every new `.go` file needs the repository license header (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` + the BSD two-line notice).
- Wrap errors with `github.com/bborbe/errors`; never `fmt.Errorf`, never `github.com/pkg/errors`, never `context.Background()` inside the package for wrapping.
- Respect the linter (funlen 80/50, nestif 4, gocognit 20, dupl, maintidx 20). Split the engine across `index.go`, `index_build.go` and `index_file.go` and keep each function small; a file past ~700 lines must be split further.
- Use Ginkgo v2 / Gomega only. External package `index_test`; `index_file_internal_test.go` and `export_test.go` are the only `package index` test files.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `ROOTDIR=/workspace make precommit` — must exit 0.

Then confirm the named specs are visible and passing:

```
GOFLAGS=-buildvcs=false go test ./pkg/index/... -ginkgo.v
```

Confirm the public surface (AC2):

```
go doc ./pkg/index PageIndex
go doc ./pkg/index NewPageIndex
```

`go doc ./pkg/index PageIndex` must show `storage.PageStorage` embedded and the names `ListEntries`, `Build`, `Refresh`, `RefreshFile`, `MarkDirty`, `MarkFileDirty`, `ForceReload`, `Revision`, `ChangedPagesSince`, `Rescan`.

Confirm the six doc headings (AC14, store heading excluded until prompt 3):

```
grep -cE '^## (Metadata-only records|Entries and open questions|Lazy body reads|Staleness bounds|Incremental updates|Key derivation)$' docs/page-index.md
```

This must print `6`.

Confirm the changelog bullet landed in an unreleased section (AC15):

```
awk '/^## /{sec=$0} /pkg\/index/{print sec}' CHANGELOG.md
```

This must print `## Unreleased` at least once.

Confirm import hygiene (AC13):

```
go list -f '{{join .Imports "\n"}}' ./pkg/index | grep -E 'golang/glog|prometheus'
```

This must print zero lines.
</verification>
