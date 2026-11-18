---
spec: ["068-pkg-index-metadata-page-index"]
status: draft
created: "2026-11-18T23:16:38Z"
---

# Build the in-memory snapshot engine for the page index

<summary>
- A vault folder's pages are served from one immutable in-memory snapshot and refreshed incrementally.
- A listing matches the existing CLI folder listing exactly, with every page body empty.
- An entries listing serves the same pages in the same order and with the same pointers, plus each page's Open Questions.
- A body is read from disk on demand, never cached and never republished.
- A stat-diff refresh re-reads only files whose size, modification time or status-change time moved; an unchanged pass publishes and allocates nothing.
- Per-file marks re-read exactly one named file; folder marks, forced reloads, revisions and the bounded change log behave as documented.
- Concurrent cold reads share one build, and a failed rebuild keeps serving the previous snapshot.
- The package documentation and changelog state the contract.
</summary>

<objective>
Implement the page index engine over the seams built in the previous prompt, so a consumer can serve a folder's page metadata and Open Questions from memory while bodies stay on disk and refreshes stay incremental.
</objective>

<context>
Read `CLAUDE.md` for project conventions and `docs/dod.md` for the definition of done this change is graded against.

Read these files before writing anything:

- `pkg/index/key.go`, `pkg/index/seams.go`, `pkg/index/reader.go`, `pkg/index/lister.go`, `pkg/index/metrics.go` — the seams this prompt builds on. `PageReader.ReadEntry` returns the metadata-only entry plus the pre-read fingerprint; `DirectoryLister.ListFiles` returns `.md` entries in `os.ReadDir` order.
- `pkg/storage/page.go` — `ListPages` is the reference listing: it returns pages in `os.ReadDir` order, skips directories, non-`.md` files and unreadable pages, and returns `nil` for a missing folder.
- `pkg/ops/list.go` — `NewListOperation(pageStorage storage.PageStorage) ListOperation`. The index implements `storage.PageStorage`, so `ops.NewListOperation(idx)` must keep working unchanged.
- `pkg/domain/page.go` — `Page` embeds `FrontmatterMap`, `FileMetadata` and `Content`; `domain.NewPage` is the constructor.
- `pkg/index/` — this package already exists in the working tree from an earlier pass, and `pkg/index` is named in the released `## v0.197.0` CHANGELOG section. Read every existing file before editing. Keep code and tests that already satisfy the contract below; add or correct what does not. Do not delete passing tests and do not recreate a file that already matches.
- The store layer (persistence, hydration, bolt) is the next prompt's scope. If store-related code (`Store`, `NewPageIndexWithStore`, a `store` field, hydration) already exists in the tree, leave it intact and keep it compiling; do not extend it here.

Coding conventions to follow:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — use `run.All` from `github.com/bborbe/run` for concurrent key builds; no raw `go func()`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md` — a long-running loop checks `ctx` non-blockingly; wrap errors with `errors.Wrap`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-time-injection.md` — never call `time.Now()` in business logic; use the injected `libtime.CurrentDateTimeGetter` and `libtime.WaiterDuration`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external test package, counterfeiter mocks.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80, nestif 4, golines 100, license headers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format and the `## Unreleased` rules.
</context>

<requirements>
1. Create `pkg/index/index.go` with the constants:

   ```go
   const RescanInterval = 50 * time.Second
   const rescanPollInterval = time.Second
   const rebuildTimeout = 2 * time.Minute
   const maxChangeLog = 64
   ```

   `RescanInterval` is exported and documented: interval + poll granularity + one stat-diff stays under the 60-second ceiling. The other three are private and documented as invariant.

2. Define `PageIndex` in `pkg/index/index.go` as a public interface embedding `storage.PageStorage` plus exactly these methods (the embedded interface supplies `ListPages` and `ReadPage`; declare `ReadPage` again only as documentation if you prefer, but it must keep the `storage.PageStorage` signature):

   ```go
   type PageIndex interface {
       storage.PageStorage
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

   Document every method with the behaviour in requirements 5–12. `go doc ./pkg/index PageIndex` must name the embedded `storage.PageStorage` and all ten methods.

3. Add the constructors and the warning sink in `pkg/index/index.go`:

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

   - `NewPageIndex` returns a private `*pageIndex` with an empty `map[Key]*entry`. A nil `metrics` becomes `NewNoopMetrics()`. The warning sink defaults to `func(format string, args ...any) { slog.Warn(fmt.Sprintf(format, args...)) }` from `log/slog`.
   - `SetWarnf` type-asserts `idx` to `*pageIndex`, ignores a nil sink and an index not built by this package, and replaces the sink. Document that it must be called before the index reads anything.
   - Import `libtime "github.com/bborbe/time"`. The constructor signature is frozen by the spec: do not reorder or rename parameters.

4. Model the per-key state. A private `entry` per `Key` holds: the published snapshot (an ordered `[]*Entry`), `hasSnapshot`, recorded fingerprints (`map[string]FileFingerprint`), per-file applied-read sequence (`map[string]uint64`), `requestSeq`, `dirtySeq`, `dirtyResolvedSeq`, `reloadSeq`, `reloadResolvedSeq`, pending write marks (`map[string]uint64`), `revision`, a bounded change log, and at most one in-flight build plus one queued follow-up. The index holds one mutex, the reader, the lister, the clock, the waiter, the metrics, the warning sink, and a single `readSeq` counter that orders single-file reads.

5. Build coalescing and cancellation:
   - Every `ListPages` / `ListEntries` call resolves pending work recorded before the call, then serves the published snapshot. A mark recorded while a read is in flight stays pending for the next reader.
   - At most one build runs per key. Concurrent callers wait on it. If a caller needs work the in-flight build does not cover, queue exactly one follow-up; the caller that owns the follow-up waits until it is promoted, then runs it.
   - A build runs under `context.WithTimeout(context.WithoutCancel(ctx), rebuildTimeout)`: a caller's cancellation must never abort work other readers depend on. An owned follow-up runs even when its caller's ctx is cancelled. A waiting caller's cancellation returns a wrapped error without touching the build.
   - `Build` warms every key concurrently with `run.All`; a per-key failure is logged and does not fail the build, and only an already-cancelled ctx makes `Build` return an error.
   - `Refresh` requests a stat-diff that started after the call and blocks until it is applied. It returns nil whether the listing succeeded or failed, and a wrapped error only when ctx was cancelled first.
   - A failed listing publishes nothing, logs the error with the key, and leaves the mark pending so the next read retries. A failed first build returns an error to the caller; a failed rebuild keeps the previous snapshot serving.

6. Cold build and stat-diff:
   - A cold build (no snapshot yet) lists the folder and reads every listed file.
   - A stat-diff lists the folder, compares each entry's fingerprint with the fingerprint recorded at its last read, and re-reads only names that are new, differ, or were force-marked. Comparison is inequality (`recorded != current`), never "newer", so a restored older modification time is still detected; the status-change time catches a same-size rewrite with a restored modification time.
   - An unchanged pass — no changed name, no forced name, the same name set and the same missing/empty state — publishes nothing, keeps the snapshot slice and the recorded fingerprint map in place, and allocates nothing beyond the one folder listing. This is the allocation contract; do not rebuild the fingerprint map or copy the entries slice on that path.
   - A changed pass re-reads exactly the changed, new and forced names and updates the recorded fingerprints for those names in place. A file that vanished from the listing is dropped; a file that is listed but unreadable is excluded from the snapshot while its non-zero fingerprint is retained, so it is warned about once per fingerprint change and not re-read on every pass.
   - A publication swaps in a new immutable snapshot slice. Entries carried forward keep their `*Entry` and `*domain.Page` pointers; only a re-read file gets a new pointer. Concurrent readers see either the whole old snapshot or the whole new one.

7. `ListPages` and `ListEntries`:
   - `ListPages` returns the snapshot's `[]*domain.Page` directly; the pages are the metadata-only pages built by `ReadEntry` (empty `Content`).
   - `ListEntries` returns the same snapshot's `[]*Entry` — same order, same pointers as `ListPages`.
   - A warm read touches no disk. Return the snapshot slice, not a copy (the contract documents that published values are read-only).

8. Marks and revision:
   - `MarkDirty(keys...)` records a folder-level mark on each known key; an unknown key is ignored. A folder-level mark records an unbounded change-log advance.
   - `MarkFileDirty(key, name)` applies only when the exactness rule holds: strip a `[[` / `]]` wrapper from the id; require a non-empty plain base name with no path separator and no `..`; require the key to have a snapshot holding a page whose name equals it byte-for-byte; require `<name>.md` to exist on disk at mark time. If any check fails, widen to a folder-level mark. A valid mark records `name + ".md"` in the pending write marks and records a bounded change-log advance naming that file.
   - `ForceReload()` marks every known key so its next read re-reads every file regardless of fingerprints. It is the only fingerprint-ignoring path and records an unbounded advance.
   - `Revision(key)` returns the key's revision (0 for an unknown key). It advances on every publication and on every mark, including folder marks and forced reloads.

9. `ChangedPagesSince(key, revision)`:
   - Return the `.md` filenames changed by advances after `revision`, de-duplicated, in a fresh slice the caller may mutate.
   - Return `ok == false` for: an unknown key; a revision ahead of the key's current revision; a revision the retained change window no longer covers; any folder-level mark in the range; a forced reload in the range; and the key's first publication (a cold build or a hydrate publishes the whole snapshot and cannot be bounded).
   - When refusing, call `metrics.ChangedPagesUnbounded(reason)` exactly once with the matching `Unbounded*` constant from `pkg/index/metrics.go`.
   - Bound the change log at `maxChangeLog` advances, dropping the oldest.

10. `RefreshFile(ctx, key, filename)`:
    - Validate `filename`: non-empty, `filepath.Base(filename) == filename`, no `filepath.Separator`, no `".."`, and a `.md` suffix. An invalid name returns `errors.Errorf` and reads nothing.
    - If the key has no published snapshot, delegate to `Refresh(ctx, key)`.
    - Otherwise take a new `readSeq`, read that one file under `context.WithTimeout(context.WithoutCancel(ctx), rebuildTimeout)` through `PageReader.ReadEntry`, and apply the result only if its sequence is not older than the name's last applied read. Present and readable replaces or inserts the entry at its filename-ordered position; absent or unreadable removes it. Publish a new snapshot, advance the revision, and record a bounded advance naming the file.
    - Call `metrics.FileRead(ReadReasonEvent)` once per read. A vanished file (zero fingerprint) deletes its recorded fingerprint; an excluded-but-present file keeps its non-zero fingerprint. Warn about an unreadable file only when it is new or its fingerprint changed.

11. `ReadPage(ctx, vaultPath, pagesDir, name)`:
    - Reject an empty name, a name with a path separator, or `..`, with `errors.Errorf(ctx, "invalid page name %q", name)`.
    - Delegate to `PageReader.ReadPage` (bare base name, no `.md`) and wrap the error.
    - Never cache the body, never touch the published snapshot and never move the revision.

12. `Rescan(ctx)`:
    - Loop until ctx is done. Wait `rescanPollInterval` through the injected `libtime.WaiterDuration`; when the injected clock shows `RescanInterval` has passed since the last rescan, call `Refresh` for every known key concurrently with `run.All`.
    - Return nil when ctx is done or a rescan fails; do not return the loop's error. A failed refresh keeps serving the previous snapshot.

13. Add `pkg/index/export_test.go` in `package index` exposing the test seams the engine tests need:
    - `CountingMetrics` with `NewCountingMetrics() *CountingMetrics`, `FileRead(reason string)`, `ChangedPagesUnbounded(reason string)`, `Reads(reason string) int` and `TotalReads() int`, all mutex-guarded.
    - `var TestMetrics = NewCountingMetrics()` — shared by test indexes; callers assert deltas, never absolutes.
    - `NewPageIndexWithWarnf(reader PageReader, lister DirectoryLister, currentDateTimeGetter libtime.CurrentDateTimeGetter, waiter libtime.WaiterDuration, warnf func(format string, args ...any)) PageIndex` — builds over `TestMetrics` and calls `SetWarnf`.
    - `FingerprintSetIdentity(idx PageIndex, key Key) string` and `SnapshotIdentity(idx PageIndex, key Key) string` — return `fmt.Sprintf("%p", ...)` for the recorded fingerprint map and the published entries slice, or `""` for an unknown/unpublished key. These let the allocation test prove the unchanged fast path keeps both in place.

14. Write the engine tests in `package index_test`, driving the production reader and lister over real temp vaults. The following descriptions are frozen — the verifier greps `-ginkgo.v` output for them:

    - `Describe("Snapshot equivalence")` with these exact specs:
      - `It("returns pages and entries with empty Content and populated frontmatter, name, path and modification date")`
      - `It("lists the same pages as vault-cli storage with only Content excluded")` — over the awkward-files fixture described below; compare with `reflect.DeepEqual` against `storage.NewPageStorage(nil).ListPages` with `Content` zeroed on the reference side. Include a positive control proving the fixture is non-empty.
      - `It("produces the same list-operation items as vault-cli storage")` — compare `ops.NewListOperation(idx)` against `ops.NewListOperation(storage.NewPageStorage(nil))`.
      - `It("reads the full body on ReadPage without caching it in the snapshot")` — the returned content is byte-for-byte the file; a following `ListPages` still has empty `Content` and the same `*domain.Page` pointers.
      - `It("republishes a body-only edit and names it in ChangedPagesSince")` — edit only the Open Questions section, then `RefreshFile` (and separately `Refresh`); the entry has a new page pointer and changed `OpenQuestions`, and `ChangedPagesSince(previousRevision)` returns that `.md` name with `ok == true`.
      - `It("matches vault-cli on an existing folder with no page files")`, `It("AC3 stays equal to vault-cli through create, modify, delete and rename")`, `It("matches vault-cli on a missing folder")`, `It("refuses a ReadPage name that must never become a file path")`.

      The awkward-files fixture is: a plain page, a page with a bare wikilink frontmatter value, a file without frontmatter, a file with invalid YAML, a non-`.md` file, a subdirectory, a symlink pointing out of the vault, a broken symlink, a symlink to a page inside the vault, and a non-ASCII filename.

    - `Describe("Stat-diff rescan")` with at least: `It("AC4(a) reads nothing and republishes nothing when nothing changed")`, `It("AC4(b) re-reads exactly the modified and the added file")`, `It("AC4(c) detects a same-size rewrite whose modification time was restored")`, `It("AC5(ii)/8 lists once and re-reads only what changed after a folder mark")`, `It("AC4/failure 9 keeps the snapshot and the mark pending when the listing fails")`, `It("empties and refills the snapshot when the folder is deleted and recreated")`.

    - `Describe("Allocation")` with `It("allocates nothing beyond its folder listing on an unchanged stat-diff")`. Skip the spec when the race detector is enabled — add build-tagged constants `raceDetectorEnabled` in `pkg/index/race_enabled_test.go` (`//go:build race`) and `pkg/index/norace_enabled_test.go` (`//go:build !race`). Measure `runtime.TotalAlloc` deltas around one bare `lister.ListFiles` call and around the unchanged `Refresh`, then assert the pass is at most 1.35× the listing and at most 5.5 MB, and that `FingerprintSetIdentity` and `SnapshotIdentity` are unchanged. Use 5000 files so a per-page structure would dominate.

    - `Describe("Per-file write marks")` with `It("re-reads a write-marked file even when no fingerprint changed")`. This is a behavioural spec (the reader must be asked for exactly that name); it must run under `-race` too, so do not add an allocation assertion or a race skip to it.

    - `Describe("ChangedPagesSince")` with specs covering: unknown key, revision ahead, an unbounded mark inside the range, first cold-build publication, and first hydrate publication (the hydrate case may be skipped or written once the store prompt lands; if the store code already exists, include it).

    - `Describe("Cold read")` with `It("shares a single build across concurrent cold reads")` — assert the reader was asked once per file, not once per caller.

    - `Describe("Failed rebuild")` with `It("keeps the previous snapshot and retries while the key stays dirty")` and `It("returns an error when the very first build fails")`.

    - `Describe("PageIndex")` containing `Describe("ListPages")` (a repeat read of a clean key touches no storage), `Describe("Refresh")` (rebuilds only the requested key; keeps serving while a rebuild is blocked; coalesces a burst into one follow-up), `Describe("dirty marks")` (rebuild once on the next read; two concurrent reads share one rebuild; a reader is forced onto a rebuild that started after the mark; `ForceReload` re-reads every file of every key; an unknown key's mark is ignored), `Describe("Rescan")` (refreshes every known key once the interval passed; the interval stays inside the 60-second ceiling), and `Describe("context cancellation")` (a waiting reader fails without aborting the build; an owned follow-up runs even when its caller's ctx is cancelled; `Build` errors on an already-cancelled ctx).

    Use a recording `PageReader` decorator in the tests to count per-file reads; the store prompt will reuse it. A `fastWaiter` helper that returns immediately is acceptable where the test drives `Refresh` directly.

15. Create `docs/page-index.md` with the contract. The document must open with an introduction stating: the package keeps metadata only; it implements `storage.PageStorage`, so `ops.NewListOperation(idx)` works unchanged; one immutable snapshot per key; published entries and pages are read-only for every consumer; and the package imports no glog and no prometheus. Then add exactly these six `##` sections, in this order:

    - `## Metadata-only records`
    - `## Entries and open questions`
    - `## Lazy body reads`
    - `## Staleness bounds`
    - `## Incremental updates`
    - `## Key derivation`

    Content to state (prose or bullets): full frontmatter retained, empty `Content`, equivalence to `storage.NewPageStorage(nil).ListPages` with only `Content` zeroed over the awkward-files set; entries carry `[]domain.OpenQuestion{Index, Text}` projected from `storage.ParseOpenQuestions`, never nil, every text cloned; `ReadPage` reads from disk on demand and never caches; warm reads touch no disk, rescan interval 50 s plus 1 s poll plus one stat-diff, each storage call bounded by 2 min, cold reads share one build, failed rebuild keeps the previous snapshot, `ForceReload` is the only fingerprint-ignoring path; stat-diff compares by inequality, unchanged pass publishes and allocates nothing, body-only edit re-reads and is named by `ChangedPagesSince`, marks and the bounded change log (64) and every `ok == false` case; `NewKey` cleans both parts so the read side and the event side agree.

    The next prompt appends `## Store format version 2`. Do not add that heading here.

16. Append a bullet to the existing `## Unreleased` section of `CHANGELOG.md` that names `pkg/index` and describes the shared metadata-only page index: full frontmatter, name, path and modification time per page with no body, pre-parsed Open Questions, the ported stat-diff refresh contract, lazy on-disk body reads. The section already exists — append to it, do not replace it, and do not insert a section above the changelog preamble.

17. Before finishing, re-run `<verification>` and confirm it passes; walk each requirement above against the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Do NOT change `pkg/cli`, `pkg/config`, `pkg/domain`, `pkg/ops` or `pkg/storage`; `pkg/index` is added alongside them
- Do not switch vault-cli's own CLI commands to the index — the CLI keeps reading files directly
- Do not move typed indexing (task/goal items, filter sets, queries, readiness) into this package; it is page metadata only
- Do not project or intern frontmatter keys — retain the full map
- Do not cache page bodies anywhere, in memory or on disk, not even opt-in
- Do not add configuration for the store location, rescan interval, timeouts or change-log size; the ported constants are invariant
- Do not port vault-ui's board-level sections (task-list snapshot, body cache, frame ordering, watcher wiring)
- `pkg/index` must import neither `github.com/golang/glog` nor any prometheus package (direct imports)
- Never call `time.Now()` or `time.Sleep` in the engine; use the injected clock and waiter
- New module dependencies are limited to `github.com/bborbe/boltkv`, `go.etcd.io/bbolt` and `github.com/bborbe/kv`; this prompt needs none
- Builds for darwin and linux with `CGO_ENABLED=0`
- Every new file carries the project's BSD license header, matching `pkg/storage/page.go`
- Do not run `go mod vendor`; `vendor/` is a build-time artifact
- Existing tests must still pass
</constraints>

<verification>
make precommit

go test ./pkg/index/... -ginkgo.v

go doc ./pkg/index PageIndex

grep -cE '^## (Metadata-only records|Entries and open questions|Lazy body reads|Staleness bounds|Incremental updates|Key derivation)$' docs/page-index.md

awk '/^## /{sec=$0} /pkg\/index/{print sec}' CHANGELOG.md | grep -q '^## Unreleased$'

! go list -f '{{join .Imports "\n"}}' ./pkg/index | grep -qE 'golang/glog|prometheus'
</verification>
