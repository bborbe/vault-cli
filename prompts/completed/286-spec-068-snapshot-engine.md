---
status: completed
spec: [068-pkg-index-metadata-page-index]
summary: Verified the existing pkg/index metadata-only snapshot engine against every requirement and added the missing CHANGELOG Unreleased entry naming pkg/index; make precommit and all 150 pkg/index specs pass.
execution_id: 0f7075d0-18aa-5c24-9281-829682b06650-exec-286-spec-068-snapshot-engine
dark-factory-version: dev
created: "2026-11-19T18:35:00Z"
queued: "2026-11-19T18:48:23Z"
started: "2026-11-19T18:48:25Z"
completed: "2026-11-19T18:58:29Z"
branch: dark-factory/pkg-index-metadata-page-index
---

# Add the in-memory metadata-only page-index snapshot engine

<summary>
- A folder's pages are held in one immutable in-memory snapshot whose bodies are always empty.
- Reading a folder list is answered from memory; no vault file is opened on a warm read.
- Each listed page also carries its pre-parsed Open Questions, served from the same snapshot and the same page pointers as the plain list.
- A full page body is still available on request and always comes from disk.
- A stat-diff refresh re-reads only files whose size, modification time or status-change time differ, and publishes nothing when nothing changed.
- One file can be re-read and spliced into a new snapshot without listing the folder.
- Folder-level and per-file dirty marks are resolved before the next read serves, and the per-file mark applies only to an exact page name.
- Every key has a revision and a bounded change log, so a consumer can ask which pages changed since a revision.
- A rescan loop refreshes every known key on a fixed cadence, and concurrent cold reads of one key share a single build.
- A contract document describes the metadata-only rule, entries, lazy body reads, staleness, incremental updates and key derivation.
</summary>

<objective>
Build the shared index engine on top of prompt 1's reader seams: immutable per-folder snapshots, the ported stat-diff refresh contract, per-file marks, revisions and a bounded change log, so vault-ui and vault-api can serve list reads without holding page bodies. The engine is in-memory only; persistence lands in prompt 3.
</objective>

<context>
Read `CLAUDE.md` and `docs/dod.md` for project conventions.

Read these files fully before changing anything:

- `specs/in-progress/068-pkg-index-metadata-page-index.md` — the whole spec. Desired Behavior 1-5 and 8, Constraints, Failure Modes, Security and the prompt-2 row of Suggested Decomposition are binding.
- `pkg/index/key.go`, `pkg/index/seams.go`, `pkg/index/reader.go`, `pkg/index/lister.go`, `pkg/index/metrics.go` — prompt 1's seams. `PageReader`, `DirectoryLister`, `Entry`, `FileEntry`, `FileFingerprint`, `Metrics` and the reason constants are already defined; do not redefine them.
- `pkg/storage/storage.go` — `PageStorage` is embedded by `PageIndex`, so `ListPages` and `ReadPage` keep their exact signatures.
- `pkg/ops/list.go` — `NewListOperation(storage.PageStorage)` must accept the index unchanged.
- `pkg/domain/page.go` — `Page` and `NewPage`; `domain.Content` is the body string type.
- `github.com/bborbe/time` (`libtime`) — `CurrentDateTimeGetter` has `Now() DateTime`; `WaiterDuration` has `Wait(ctx context.Context, duration Duration) error`. Convert with `libtime.Duration(rescanPollInterval)`.
- `github.com/bborbe/run` — `run.Func` and `run.All(ctx, funcs...)`; already a direct dependency and used for concurrent key builds.
- `pkg/index/index_test.go` and siblings if present — the branch may already carry the engine and its tests.
- `docs/page-index.md` — the contract document; if present, verify and extend rather than rewrite.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — Interface → Constructor → Struct → Method.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — one build per key, shared waiting, no torn reads.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md` — rescan loop cancellation.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` with `ctx`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80 lines / 50 statements, nestif 4, gocognit 20, maintidx 20.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` section shape.
</context>

<requirements>
## 1. The exported interface — `pkg/index/index.go`

```go
const RescanInterval = 50 * time.Second

// rescanPollInterval is the granularity at which the rescan loop checks the clock.
const rescanPollInterval = time.Second

// rebuildTimeout bounds a single storage call so a hung filesystem cannot hold
// a key's rebuild — and its callers — forever.
const rebuildTimeout = 2 * time.Minute

// maxChangeLog bounds each key's retained change log.
const maxChangeLog = 64

//counterfeiter:generate -o ../../mocks/index-page-index.go --fake-name IndexPageIndex . PageIndex
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

Document every method on the interface with the contract in requirements 3-6; `go doc ./pkg/index PageIndex` must print the embedded `storage.PageStorage` plus all ten method names.

## 2. Frozen constructors and the store seam

```go
type StoredEntry struct {
	Filename    string
	Entry       *Entry
	Fingerprint FileFingerprint
}

//counterfeiter:generate -o ../../mocks/index-store.go --fake-name IndexStore . Store
type Store interface {
	Load(ctx context.Context, key Key) (entries []StoredEntry, ok bool, err error)
	Write(ctx context.Context, key Key, puts []StoredEntry, deletes []string) error
	Path() string
	Close() error
}

func NewPageIndex(
	reader PageReader,
	lister DirectoryLister,
	currentDateTimeGetter libtime.CurrentDateTimeGetter,
	waiter libtime.WaiterDuration,
	metrics Metrics,
) PageIndex

func NewPageIndexWithStore(
	reader PageReader,
	lister DirectoryLister,
	currentDateTimeGetter libtime.CurrentDateTimeGetter,
	waiter libtime.WaiterDuration,
	metrics Metrics,
	store Store,
) PageIndex

func SetWarnf(idx PageIndex, warnf func(format string, args ...any))
```

- `libtime` is `github.com/bborbe/time`.
- A nil `metrics` becomes `NewNoopMetrics()`.
- `NewPageIndex` delegates to `NewPageIndexWithStore` with a nil store.
- A nil store disables persistence. The bolt implementation and the hydrate/write-through wiring are prompt 3; this prompt stores the field and tests the nil-store path.
- `SetWarnf` installs the per-index warning sink; a nil `warnf` or an index not built by this package is ignored. The default sink is `slogWarnf` from prompt 1. Warnings are best-effort: no warning path returns an error.

## 3. Snapshots and list reads

- One immutable `published` value per key: `pages []*domain.Page` and `entries []*Entry`, built once and swapped atomically. `entries[i].Page` is the same pointer as `pages[i]`; order is the directory listing's order (filename ascending from `os.ReadDir`).
- `ListPages(ctx, vaultPath, pagesDir)` resolves any pending mark first, then returns the snapshot's pages. Content is always empty.
- `ListEntries` returns the snapshot's entries — same order, same page pointers.
- A warm read touches no disk. `ListPages` / `ListEntries` on a clean key call neither `DirectoryLister` nor `PageReader`.
- `ReadPage(ctx, vaultPath, pagesDir, name)` rejects an empty name, a name containing a path separator or `..`, and otherwise delegates to `PageReader.ReadPage`. It never caches, never mutates or republishes the snapshot, and never moves the key's revision.
- A key with no snapshot yet is built cold on first read. Concurrent cold reads of one key share exactly one build.
- A failed first build returns an error. A failed rebuild keeps serving the previous snapshot, logs the error with the key, and leaves the mark pending so the next read retries.
- Readers are served the whole old snapshot or the whole new one; never a torn mix.
- Published pages and entries are read-only for consumers. Say so in the doc comments and in `docs/page-index.md`.

## 4. Builds and the stat-diff contract

Work is per key. A key has at most one build in flight; a read that needs a build started after its own call joins the in-flight build or queues one follow-up, never both.

- `Build(ctx, keys)` warms the keys concurrently with `run.All`. A per-key failure is logged and does not fail the build; only a cancelled `ctx` makes `Build` return an error.
- Cold build: list the folder, read every file through `PageReader.ReadEntry`, publish the metadata-only snapshot.
- Stat-diff (`Refresh` and the rescan): list the folder, compare each entry's fingerprint with the one recorded at its last read, and re-read only files whose fingerprint differs (inequality, not "newer") or that are new. Drop entries whose file vanished.
- An unchanged stat-diff reads nothing, publishes nothing, keeps the snapshot pointer and the recorded fingerprint set, and allocates nothing beyond the folder listing.
- A body-only edit changes the fingerprint, so the file is re-read, republished with a new page pointer, and named by `ChangedPagesSince`.
- Every file read records a fingerprint, including files that end up excluded (unreadable, bad YAML, symlink out of the vault, deleted between list and read). An excluded file is warned about once per fingerprint change, and the warning count per pass is capped.
- Each storage call is bounded by `rebuildTimeout` (2 min). A hung filesystem must not hold a key or its waiting readers forever.
- Reads count through `Metrics.FileRead` with the reason of the call site: cold build `ReadReasonBuild`, per-file refresh `ReadReasonEvent`, write mark `ReadReasonWrite`, rescan `ReadReasonRescan`, forced reload `ReadReasonReload`.

## 5. Marks, revisions and the change log

- `MarkDirty(keys...)` records a folder-level mark; the next read resolves it with one stat-diff. Marking a key the index has never seen is a no-op — its first read builds it anyway.
- `MarkFileDirty(key, name)` records a per-file mark. The exactness rule: strip a surrounding `[[`/`]]`, then apply it only when the name has no path separator and no `..`, matches a page name in the snapshot byte-for-byte, and `<name>.md` is present. Anything else widens to a folder-level mark, which resolves with a stat-diff.
- A per-file mark re-reads exactly the marked files and lists nothing.
- `ForceReload()` marks every known key so the next read re-reads every file regardless of fingerprints. It is the only path that ignores fingerprints.
- `Revision(key)` returns 0 for an unknown key and otherwise advances on every publication and every mark (folder or file).
- `ChangedPagesSince(key, revision)` returns the `.md` base filenames (with the `.md` suffix) changed after `revision`. `ok == false` for: an unknown key; a revision ahead of the key's current one; a revision older than the retained window; a folder-level mark; a forced reload; or the key's first publication (which publishes the whole snapshot whatever the build kind). Count each refusal through `Metrics.ChangedPagesUnbounded` with the matching constant from prompt 1.
- The change log keeps at most `maxChangeLog` (64) advances.

## 6. Per-file refresh — `pkg/index/index_file.go`

`RefreshFile(ctx, key, filename)`:

- Validate `filename`: a single plain base name including `.md`. Reject empty, a path separator, `..`, and a name not ending in `.md` — with an error and no read.
- Re-read only that file and list nothing. The file's current on-disk state decides: present and readable replaces or inserts the page at its filename-ordered position; absent or unreadable removes it.
- Splice into a new slice; never mutate the slice a previous caller holds. An unrelated dirty mark stays pending.
- The read's fingerprint updates the recorded set.
- `Refresh(ctx, key)` returns nil whether its stat-diff succeeded or failed; it returns a wrapped error only when `ctx` was cancelled first.

## 7. Rescan

`Rescan(ctx)` loops until `ctx` is done: wait `rescanPollInterval` via `waiter.Wait`, and when `currentDateTimeGetter.Now()` shows at least `RescanInterval` (50 s) since the last pass, refresh every known key concurrently with `run.All` and wait for all of them. The loop returns nil on cancellation and never lets passes pile up.

## 8. Contract document — `docs/page-index.md`

Create `docs/page-index.md` with these six exact level-2 headings (prompt 3 adds the seventh, `Store format version 2`):

```
## Metadata-only records
## Entries and open questions
## Lazy body reads
## Staleness bounds
## Incremental updates
## Key derivation
```

Content, per heading:

- Opening prose: the package implements `storage.PageStorage`, so it plugs in where vault-cli reads a folder and `ops.NewListOperation(idx)` works unchanged; single files are read through `PageReader` → `storage.PageStorage.ReadPage`, so one parse path and one symlink guard serve both; published pages and entries are read-only.
- `Metadata-only records`: full frontmatter map, name, path and modification date match the folder listing exactly; `Content` is always empty; the body is dropped before publish; `ListPages` equals `storage.NewPageStorage(nil).ListPages` with only `Content` zeroed over the awkward-files fixture (no frontmatter, bad YAML, non-`.md`, subdirectory, symlink out of vault, broken symlink, in-vault symlink, non-ASCII name).
- `Entries and open questions`: `ListEntries` serves the same snapshot and pointers; `OpenQuestions` is `[]domain.OpenQuestion{Index, Text}` projected from `storage.ParseOpenQuestions`; never nil; every `Text` is a `strings.Clone` copy; a parse error fails the read.
- `Lazy body reads`: `ReadPage` takes the bare name, always reads from disk, never caches, never mutates the snapshot, never moves the revision; invalid names are rejected without a read.
- `Staleness bounds`: warm reads touch no disk; rescan interval 50 s + 1 s poll + one stat-diff stays under 60 s; each storage call is bounded by 2 min; first read after a mark resolves it; concurrent cold reads share one build; a failed listing keeps the previous snapshot and leaves the mark pending; with a store, a key is hydrated then resolves exactly one stat-diff.
- `Incremental updates`: `RefreshFile` semantics; fingerprint fields and inequality comparison; unchanged stat-diff reads/publishes/allocates nothing; excluded files warned once per fingerprint change; `MarkDirty` / `MarkFileDirty` exactness rule; `Revision` and `ChangedPagesSince` with all the `ok == false` cases.
- `Key derivation`: `NewKey` cleans both parts; the read side and event side must derive the same key from the same config entry.

Add one short paragraph naming the injected `Metrics` interface and the `log/slog` warning sink, and stating that the package imports no glog and no prometheus.

## 9. CHANGELOG

Add a `## Unreleased` section below the changelog preamble and above the newest `## vX.Y.Z` section, with one bullet naming `pkg/index` and its metadata-only contract. Do not bump any version manifest.

## 10. Tests

Ginkgo/Gomega, external `index_test` package; internal helpers go in `export_test.go` (same style as `pkg/ops/export_test.go`). Use a recording `PageReader` wrapper to assert exactly which files were read. Use a temp directory per spec.

Required `Describe` container texts (case-sensitive, frozen so the verifier can grep `-ginkgo.v` output):

- `Stat-diff rescan`
- `Allocation`
- `Per-file write marks`
- `ChangedPagesSince`
- `Cold read`
- `Failed rebuild`
- `Snapshot equivalence`

`Snapshot equivalence` must contain these exact `It` descriptions:

- `returns pages and entries with empty Content and populated frontmatter, name, path and modification date` — after `Build`, every page from `ListPages` and every `Entry.Page` from `ListEntries` has `Content == ""`, non-empty frontmatter for a fixture with frontmatter, and `Name` / `FilePath` / `ModifiedDate` equal to the file's.
- `lists the same pages as vault-cli storage with only Content excluded` — over the awkward-files fixture, `ListPages` is `reflect.DeepEqual` to `storage.NewPageStorage(nil).ListPages` with `Content` zeroed on the reference side: same set, order, frontmatter, path and mtime.
- `produces the same list-operation items as vault-cli storage` — `ops.NewListOperation(idx)` and `ops.NewListOperation(storage.NewPageStorage(nil))` return equal items for the same fixture.
- `reads the full body on ReadPage without caching it in the snapshot` — `ReadPage` returns the file's full content byte-for-byte, and a following `ListPages` still returns `Content == ""` and the same `*domain.Page` pointers as before the call.
- `republishes a body-only edit and names it in ChangedPagesSince` — edit only the Open Questions section of one file, then `RefreshFile` (and separately `Refresh`): a new `*domain.Page` pointer and changed `OpenQuestions` for that entry, and `ChangedPagesSince(previousRevision)` returns that name with `ok == true`.

`Stat-diff rescan` must cover: an unchanged pass publishes nothing and keeps snapshot identity; it re-reads exactly the modified and added files; it detects a same-size rewrite whose modification time was restored (status-change time); a folder mark lists once and re-reads only what changed; a failed listing keeps the snapshot and the mark pending.

`Allocation` must contain an unchanged-stat-diff allocation spec (equivalent to vault-ui's `TestUnchangedRescanAllocation`) and must be skipped when the race detector is enabled. Add `pkg/index/race_enabled_test.go` (`//go:build race`) and `pkg/index/norace_enabled_test.go` (`//go:build !race`) each defining `const raceDetectorEnabled = true/false`, and guard the allocation spec with `Skip` when true. In a non-race run the container must still report at least one passing spec.

`Per-file write marks` must contain the behavioural check `re-reads a write-marked file even when no fingerprint changed` — unskipped under every build. Also cover: a per-file mark lists nothing and reads exactly the marked file; two concurrent readers share one re-read; a `[[wikilink]]` wrapper is stripped; an inexact name widens to a folder-level mark; a file marked again while its read is in flight is re-read.

`Cold read` must contain `shares a single build across concurrent cold reads`.

`Failed rebuild` must contain: a failed rebuild keeps the previous snapshot and retries while the key stays dirty; the very first build's failure returns an error; a reader blocked on a rebuild is still served the previous snapshot.

`ChangedPagesSince` must cover every `ok == false` reason and the bounded window (a revision older than 64 advances).

Also add engine specs for: a warm read touches no storage; `RefreshFile` insert/replace/remove/splice-without-mutating; revision advances on publication and each mark; `ForceReload` re-reads every file; `Rescan` refreshes known keys once the interval has passed and the interval stays inside the 60 s ceiling; `Build` returns an error only on a cancelled context; read counts by reason.

Boundary tests: `ops.NewListOperation` is a real consumer boundary — the list-operation equivalence spec above is required, not optional. The store codec boundary is prompt 3.

## 11. Self-check

Before finishing, run every command in `<verification>` and walk each requirement above against the code. If the engine already exists, verify each item and change only what is missing or wrong.
</requirements>

<constraints>
- **New exported package `pkg/index`.** One implementation serves vault-ui and vault-api.
- **Do NOT change** `pkg/cli`, `pkg/config`, `pkg/domain`, `pkg/ops` or `pkg/storage`. `ops.NewListOperation(idx)` must keep working through the embedded `storage.PageStorage` with no edit to `pkg/ops`.
- **Do NOT cache bodies anywhere.** `ListPages` / `ListEntries` always return empty Content; `ReadPage` always goes to disk.
- **Ported constants are invariant:** rescan interval 50 s, rebuild timeout 2 min, change-log size 64. Do not add configuration for them.
- **`pkg/index` imports no glog and no prometheus directly.** Metrics go through the injected interface; warnings through `log/slog` by default.
- **No store work in this prompt.** Do not implement `NewBoltStore`, the codec, identity reconcile or hydration; those are prompt 3. Defining the `Store` interface and both constructors is required here because the interface and constructor signatures are frozen.
- **Do not port vault-ui's board-level sections** (task-list snapshot, body cache, frame ordering, watcher wiring).
- **Published snapshots and their `*domain.Page` values are read-only for consumers.** State this in doc comments and in `docs/page-index.md`.
- **Do not hand-bump version manifests or tag.** The releaser owns version strings.
- **Existing baseline:** this branch may already contain some or all of `pkg/index`. Read the existing files first and treat them as the baseline — verify every requirement and change only what is missing or wrong.
- Errors go through `github.com/bborbe/errors` with `ctx`; never `fmt.Errorf` in production code.
- Ginkgo/Gomega for tests; counterfeiter directives for every new interface.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run each command; every one must succeed.

```
ROOTDIR=/workspace make precommit
```

Focused run, with the full log captured so spec names can be asserted:

```
set -o pipefail; go test -mod=mod ./pkg/index/... -ginkgo.v | tee /tmp/index-prompt2.log
```

Assert the frozen spec names are present in the passing output:

```
for name in \
  'returns pages and entries with empty Content and populated frontmatter, name, path and modification date' \
  'lists the same pages as vault-cli storage with only Content excluded' \
  'produces the same list-operation items as vault-cli storage' \
  'reads the full body on ReadPage without caching it in the snapshot' \
  'republishes a body-only edit and names it in ChangedPagesSince'; do
  grep -Fq "$name" /tmp/index-prompt2.log
done
```

Assert the frozen `Describe` containers are present (case-sensitive):

```
for name in 'Stat-diff rescan' 'Allocation' 'Per-file write marks' 'ChangedPagesSince' 'Cold read' 'Failed rebuild'; do
  grep -Fq "$name" /tmp/index-prompt2.log
done
```

`go doc ./pkg/index PageIndex` must name the embedded `storage.PageStorage` and all ten methods:

```
go doc ./pkg/index PageIndex | grep -q 'storage.PageStorage'
for name in ListEntries Build Refresh RefreshFile MarkDirty MarkFileDirty ForceReload Revision ChangedPagesSince Rescan; do
  go doc ./pkg/index PageIndex | grep -q "$name"
done
```

`go doc ./pkg/index` must name both constructors:

```
go doc ./pkg/index | grep -q 'func NewPageIndex('
go doc ./pkg/index | grep -q 'func NewPageIndexWithStore('
```

The six non-store document headings must exist (the seventh is prompt 3):

```
grep -cE '^## (Metadata-only records|Entries and open questions|Lazy body reads|Staleness bounds|Incremental updates|Key derivation)$' docs/page-index.md
```

The command must print `6`.

The changelog must carry an `Unreleased` section naming the package:

```
awk '/^## /{sec=$0} /pkg\/index/{print sec}' CHANGELOG.md | grep -q '^## Unreleased'
```

Both supported platforms must build without cgo:

```
GOOS=linux CGO_ENABLED=0 go build ./pkg/index/...
GOOS=darwin CGO_ENABLED=0 go build ./pkg/index/...
```
</verification>
