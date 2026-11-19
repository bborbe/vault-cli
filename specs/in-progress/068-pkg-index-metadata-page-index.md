---
status: prompted
approved: "2026-11-17T16:55:24Z"
generating: "2026-11-19T19:31:09Z"
prompted: "2026-11-19T19:40:53Z"
branch: dark-factory/pkg-index-metadata-page-index
---

## Summary

- vault-cli gains a shared, exported page index library: it keeps every page's frontmatter, name, path and modification time in memory per folder, but never the page body.
- Each indexed page also carries its pre-parsed Open Questions, so the one body-derived field a consumer needs on a hot path is answered without re-reading files.
- A body is read only on demand, straight from disk, and is never retained by the index.
- An optional on-disk cache lets a restarted process skip re-parsing unchanged files; it stores no bodies and recognises (and discards) the older vault-ui cache format by name.
- The library is a port of vault-ui's existing page index (same refresh/staleness contract), made body-free and free of vault-ui's logging and metrics stack, so vault-ui and vault-api can both adopt it in follow-up migrations.

## Problem

vault-ui holds about 1.5 GB of Go heap (6.5–7M objects for ~48k indexed markdown files, 481 MB raw markdown) because its page index keeps every page in memory with its full file body, and its on-disk cache persists the bodies too (~760 MB file). Only one vault-ui consumer reads the body — it derives each task's Open Questions — and every other consumer reads frontmatter, path and modification time only. vault-api separately re-reads every task and goal page with full bodies on each git HEAD change, only to discard the bodies after building typed items. The index logic lives in vault-ui alone, so vault-api cannot share it and the two read paths diverge. The memory cost makes vault-ui swap on the operator's machine.

## Goal

vault-cli ships an exported `pkg/index` package whose page index serves the same pages, in the same order, with the same frontmatter, names, paths and modification times as vault-cli's folder listing — with every body empty — plus each page's Open Questions, from immutable per-folder snapshots kept current by the ported incremental-refresh contract. A body is available on request by reading the file. An optional bolt cache persists the metadata-only records under store format version 2. A contract document in vault-cli describes the behaviour. vault-ui and vault-api can replace their own index/list paths with this package without changing what they serve.

## Non-goals

- Do NOT switch vault-cli's own CLI commands to the index — the CLI keeps reading files directly.
- Do NOT move typed indexing (task/goal items, filter sets, queries, readiness) out of vault-api — the shared layer is page metadata only.
- Do NOT project or intern frontmatter keys — the full frontmatter map is retained; a projection is a separate follow-up spec if heap profiling demands it.
- Do NOT shrink, compact or migrate the bolt file — a mismatching store is discarded, never converted.
- Do NOT add configuration for the store location, rescan interval, timeouts or change-log size — ported constants are invariant; a consumer needing variation is a separate spec.
- Do NOT cache bodies anywhere (memory or store) — not even an opt-in body cache.
- Do NOT port vault-ui's board-level sections (task-list snapshot, body cache, frame ordering, watcher wiring) — they stay in vault-ui.
- Live heap, pprof and pod-memory verification belong to the consumer migrations, not this spec.
- **Follow-up consumer migrations (out of this spec's prompts; tracked by the vault task "vault-ui and vault-api Share One Memory-Efficient Bolt-Backed Index Library", SC1–SC6):**
  - vault-api: bump vault-cli; index build takes a `index.PageIndex` constructed without a store, calls `Refresh` for the tasks and goals keys then `ListPages` through it, replacing its two direct `ListPages` calls; the counting storage wrapper is kept by wrapping it in `index.NewPageReader`; body reads stay on the underlying storage's `ReadPage`.
  - vault-ui: bump vault-cli from v0.171.0; replace its page-index engine with `pkg/index` (keep key derivation, wire a prometheus `Metrics` adapter with its existing metric names, `DefaultStorePath("vault-ui")` keeps the same file path); read Open Questions from `ListEntries`; drop its own page snapshots; adapt its equivalence test to exclude only Content; add pprof.

## Acceptance Criteria

Test descriptions below are frozen so the verifier can grep `-ginkgo.v` output. All criteria are container-executable unless marked otherwise.

- [ ] AC1: `make precommit` exits 0 in the vault-cli repo — evidence: exit code.
- [ ] AC2: `go doc ./pkg/index PageIndex` stdout names the embedded `storage.PageStorage` (which supplies `ListPages` and `ReadPage`) plus each of `ListEntries`, `Build`, `Refresh`, `RefreshFile`, `MarkDirty`, `MarkFileDirty`, `ForceReload`, `Revision`, `ChangedPagesSince`, `Rescan`; `go doc ./pkg/index Entry` stdout names fields `Page` and `OpenQuestions`; `go doc ./pkg/index` stdout names `NewPageIndex` and `NewPageIndexWithStore` — evidence: stdout match (1 + 10 + 2 + 2 names).
- [ ] AC3: `go test ./pkg/index/... -ginkgo.v` exits 0 and its output contains the passing spec `returns pages and entries with empty Content and populated frontmatter, name, path and modification date` (after `Build`, every page from `ListPages` and every `Entry.Page` from `ListEntries` has `Content == ""`, non-empty frontmatter for a fixture with frontmatter, and Name/FilePath/ModifiedDate equal to the file's) — evidence: exit code + stdout match.
- [ ] AC4: Same run contains passing spec `lists the same pages as vault-cli storage with only Content excluded`: over the awkward-files fixture (no frontmatter, bad YAML, non-.md file, subdirectory, symlink out of vault, broken symlink, in-vault symlink, non-ASCII name) `ListPages` is `reflect.DeepEqual` to `storage.NewPageStorage(nil).ListPages` with Content zeroed on the reference side — same set, order, frontmatter, path, mtime — evidence: stdout match.
- [ ] AC5: Same run contains passing spec `produces the same list-operation items as vault-cli storage`: `ops.NewListOperation(idx)` and `ops.NewListOperation(storage.NewPageStorage(nil))` return equal items for the same fixture — evidence: stdout match.
- [ ] AC6: Same run contains passing spec `reads the full body on ReadPage without caching it in the snapshot`: `ReadPage` returns the file's full content byte-for-byte, and a following `ListPages` still returns `Content == ""` and the same `*domain.Page` pointers as before the call — evidence: stdout match.
- [ ] AC7: Same run contains passing specs `projects open questions from ParseOpenQuestions`, `returns an empty non-nil slice when the page has no Open Questions section`, and `does not retain open-question text that shares memory with the file body` (each retained `Text` lies outside the address range of the body string the reader parsed; technique agent decides at impl time) — evidence: stdout match (3 descriptions).
- [ ] AC8: Same run contains passing spec `republishes a body-only edit and names it in ChangedPagesSince`: editing only the Open Questions section of one file, then `RefreshFile` (and separately `Refresh`), yields a new `*domain.Page` pointer and changed `OpenQuestions` for that entry, and `ChangedPagesSince(previousRevision)` returns that name with `ok == true` — evidence: stdout match.
- [ ] AC9: Same run lists Ginkgo `Describe` containers with these exact, case-sensitive texts — `Stat-diff rescan`, `Allocation`, `Per-file write marks`, `ChangedPagesSince`, `Cold read`, `Failed rebuild`, `Store hydration` — each with ≥1 passing spec ported from vault-ui's page-index tests (unchanged stat-diff publishes nothing and keeps snapshot identity; unchanged stat-diff allocates nothing beyond its folder listing; concurrent cold reads share one build; a failed rebuild keeps the previous snapshot). vault-ui's `TestUnchangedRescanAllocation` becomes a Ginkgo spec under `Allocation`, skipped when the race detector is enabled (allocation counts are meaningless under `-race`); `TestWriteMarkSkipsUnchangedFastPath` (a behavioural check, not an allocation count) becomes an unskipped spec under `Per-file write marks`; vault-ui's cold-read sharing spec (under `ListPages`) and failed-rebuild specs (under `failures`) are re-homed under `Cold read` and `Failed rebuild`; the `Allocation` container must still report ≥1 passing spec in a non-race run of `go test ./pkg/index/... -ginkgo.v` — evidence: stdout match per container.
- [ ] AC10: Same run contains passing spec `persists no Content field in the store record`: a reflection walk over the persisted record type finds no field named `Content` at any depth and finds a field named `OpenQuestions`. Passing spec `writes no body bytes to the store file`: after a cold build with a store over a fixture whose body contains the sentinel `BODY-SENTINEL-7f3a` (outside frontmatter and outside any Open Questions item), and the store closed, the bolt file's raw bytes do not contain that sentinel. Passing spec (under `Store hydration`) `restores open questions from the store without re-reading the file`: index A over a fixture with an Open Questions section publishes to the store and closes it; index B opens the same store over the unchanged files; B's `ListEntries` `OpenQuestions` equal A's and B's injected `Metrics` records zero file reads — evidence: stdout match (3 descriptions).
- [ ] AC11: Same run contains passing spec `discards a v1 store, deletes its entry buckets and warns store format mismatch with the path`: the test writes a bolt file using **literal** bytes (not package constants) — meta bucket `vault-ui-page-index-meta`, key `identity`, JSON `{"StoreFormat":1,"ParserVersion":"v0.171.0"}` (format AND parser both differ, the real vault-ui case), one entry bucket `vault-ui-page-index\x00<vault>\x00<dir>` — opens it, then asserts the entry bucket is gone and the captured warning contains `store format mismatch` and the file path. Passing spec `reports parser version mismatch as a distinct reason` (identity `{"StoreFormat":2,"ParserVersion":"v0.0.0-stale"}`) asserts a warning containing `parser version mismatch` and not `store format mismatch` — evidence: stdout match (2 descriptions).
- [ ] AC12: Same run contains passing spec `serves without a store when the bolt file is locked by another process`: with the file held open by a second bolt handle, construction returns within 10 s, logs a discard warning, and `ListPages` serves the fixture — evidence: stdout match.
- [ ] AC13: `go list -f '{{join .Imports "\n"}}' ./pkg/index | grep -E 'golang/glog|prometheus'` returns 0 lines (direct imports only; transitive imports via `bborbe/kv`/`boltkv` are allowed) — evidence: negative grep.
- [ ] AC14: `docs/page-index.md` exists and `grep -cE '^## (Metadata-only records|Entries and open questions|Lazy body reads|Store format version 2|Staleness bounds|Incremental updates|Key derivation)$' docs/page-index.md` returns 7; `go doc ./pkg/index` stdout contains `docs/page-index.md` — evidence: file content + stdout match.
- [ ] AC15: `awk '/^## /{sec=$0} /pkg\/index/{print sec}' CHANGELOG.md` prints `## Unreleased` at least once — evidence: file content.
- [ ] AC16: **Operator-executable:** `git diff --stat origin/master -- pkg/cli pkg/config pkg/domain pkg/ops pkg/storage` is empty on the PR branch — evidence: negative diff.
- [ ] AC17: **Operator-executable (after merge + release):** `go list -m github.com/bborbe/vault-cli@<released tag>` resolves, and in a scratch module requiring that tag `go doc github.com/bborbe/vault-cli/pkg/index PageIndex` exits 0 — evidence: exit code + stdout.

No new scenario: every behaviour is reachable with in-process tests over temp directories.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
go test ./pkg/index/... -ginkgo.v
go doc ./pkg/index PageIndex
go doc ./pkg/index Entry
go doc ./pkg/index
go list -f '{{join .Imports "\n"}}' ./pkg/index | grep -E 'golang/glog|prometheus'   # expect 0 lines
grep -cE '^## (Metadata-only records|Entries and open questions|Lazy body reads|Store format version 2|Staleness bounds|Incremental updates|Key derivation)$' docs/page-index.md   # expect 7
awk '/^## /{sec=$0} /pkg\/index/{print sec}' CHANGELOG.md   # expect ## Unreleased
```

### Operator-executable (runs on the host after PR merge / release)

```
git fetch origin
git diff --stat origin/master -- pkg/cli pkg/config pkg/domain pkg/ops pkg/storage   # expect empty
go list -m github.com/bborbe/vault-cli@<released tag>
# scratch module: go mod init x && go get github.com/bborbe/vault-cli@<released tag>
go doc github.com/bborbe/vault-cli/pkg/index PageIndex
```

## Desired Behavior

1. **Metadata-only snapshots.** Each folder key holds one immutable snapshot of pages whose frontmatter (full map, every key), name, file path and modification date match vault-cli's folder listing exactly, and whose Content is always empty. The file body is read through vault-cli's own page read (one parse path, one symlink-out-of-vault guard) and dropped before the page is published.
2. **Entries carry Open Questions.** `ListEntries` serves the same snapshot as `ListPages` — same order, same page pointers — where each entry adds the page's Open Questions as `[]domain.OpenQuestion{Index, Text}` (vault-cli's own type, already the one vault-ui serves) projected from `ParseOpenQuestions` (Text = the item's question). The slice is never nil (empty when the page has no section), and every retained text is a copy that shares no memory with the body.
3. **Lazy body read.** `ReadPage` always reads the file through the underlying storage and returns the full page including Content. It never caches the body, never mutates or republishes the snapshot, and never moves a key's revision.
4. **Ported refresh contract.** Build, Refresh (stat-diff), RefreshFile, MarkDirty, MarkFileDirty, ForceReload, Revision, ChangedPagesSince and Rescan keep vault-ui's documented semantics unchanged: warm reads touch no disk; stat-diff re-reads only fingerprint-differing entries and an unchanged pass allocates and publishes nothing; per-file marks require an exact name; ForceReload is the only fingerprint-ignoring path; the change log is bounded and reports `ok == false` on folder marks, reloads, first publication and out-of-window revisions; cold reads share one build; a failed rebuild keeps the previous snapshot. Constants are ported as-is: rescan interval 50 s, rebuild timeout 2 min, change-log size 64. A body-only edit changes the fingerprint, so it is re-read, republished with a new page pointer and named by `ChangedPagesSince`.
5. **Library hygiene.** The package logs through `log/slog` by default with an injectable warning sink, and reports "file read by reason" and "changed-pages unbounded by reason" through an injected `Metrics` interface; a nil `Metrics` or nil store is a no-op. Constructors are frozen: `NewPageIndex(reader PageReader, lister DirectoryLister, currentDateTimeGetter libtime.CurrentDateTimeGetter, waiter libtime.WaiterDuration, metrics Metrics) PageIndex` and `NewPageIndexWithStore(reader PageReader, lister DirectoryLister, currentDateTimeGetter libtime.CurrentDateTimeGetter, waiter libtime.WaiterDuration, metrics Metrics, store Store) PageIndex` (vault-ui's names and leading parameters, plus `metrics`; `metrics` and `store` may be nil); the warning sink is a setter or option on the returned value — its exact form agent decides at impl time (local, defaults to slog); `NewBoltStore(ctx, path, identity, warnf, vlogf)` keeps vault-ui's signature.
6. **Optional metadata-only store (format 2).** When a store is given, each key is hydrated from it and then resolves exactly one stat-diff before it is served; publications write exactly the changed and vanished entries in one transaction, outside the index mutex. A persisted record holds filename, has-page flag, frontmatter, file metadata, Open Questions and fingerprint — no body. `DefaultStorePath(app)` is a pure path helper returning `<user cache dir>/<app>/page-index.bolt`; `NewBoltStore` creates the file's directory with mode 0700 when absent.
7. **Store reconcile with distinct reasons.** On open, a store whose identity is missing/empty, damaged, written under another store format, or written by another vault-cli parser version is discarded: its entry buckets are deleted and one warning names the path and the reason — `store format mismatch (stored N, current 2)`, `parser version mismatch`, missing/empty, or damaged — each with distinct wording. Format is checked before parser version, so a file differing in both reports `store format mismatch`. Opening waits at most 5 s for the file lock; any open or reconcile failure yields a discarded store that serves nothing and never fails construction.
8. **Contract document.** `docs/page-index.md` in vault-cli states the metadata-only rule, the entry/Open Questions rule, lazy body reads, store format version 2 and the frozen store names, plus the ported staleness, incremental-update and key-derivation sections; the package documentation points to it; `CHANGELOG.md` `## Unreleased` names `pkg/index`.

## Constraints

### Decisions (frozen for this spec)

- **New exported package `pkg/index`, port of vault-ui's engine** — one implementation for vault-ui and vault-api instead of two diverging ones.
- **Full frontmatter map retained, not a projection** — consumers (list operation, vault-ui) read arbitrary keys (`flag_set_by`, `jira`, `mode`); equivalence needs DeepEqual frontmatter.
- **Open Questions precomputed into the entry** — vault-ui reads them for every task row in one listing per vault; a lazy per-row body read would put a vault-wide file walk on the hottest path.
- **`ListPages` returns empty Content** — so `ops.NewListOperation(idx)` keeps working unchanged against the index.
- **`ReadPage` always goes to disk** — the body is the memory cost being removed; caching it would reintroduce it.
- **`pkg/index` imports no glog / prometheus directly** — vault-cli logs via slog; metrics go through an injected interface so vault-ui keeps its metric names in its own adapter (transitive imports via `bborbe/kv`/`boltkv` are unavoidable and allowed).
- **Store format version 2, persisted record without Content** — v1 records carry bodies and must not be read.
- **Frozen store names, byte-identical to vault-ui's v1 store:** meta bucket `vault-ui-page-index-meta`, identity key `identity`, identity JSON fields `StoreFormat` / `ParserVersion`, entry bucket prefix `vault-ui-page-index\x00` followed by `<VaultPath>\x00<PagesDir>` — so vault-ui's existing file is recognised and discarded as a format mismatch (entry buckets deleted) instead of reading as "missing" and leaving 760 MB of old buckets behind.
- **ParserVersion = vault-cli module version from build info** — the dependency entry when vault-cli is a dependency, the main module when vault-cli is itself main, `(unknown)` otherwise — so a vault-cli parser upgrade invalidates the cache.
- **Exported seams kept:** `Key`/`NewKey` (`filepath.Clean` on both parts), `PageReader`/`NewPageReader(storage.PageStorage)` (returns the metadata-only entry plus fingerprint), `DirectoryLister`/`NewDirectoryLister` (`.md` files only, no directories, directory order), `FileFingerprint` (size, modification time, status-change time via a symlink-following stat; per-OS for darwin and linux), `Store`, `NewBoltStore`, `DefaultStorePath`, `CurrentWriterIdentity`.

### Must not change

- No change to `pkg/cli`, `pkg/config`, `pkg/domain`, `pkg/ops`, `pkg/storage` (AC 16); the CLI's behaviour and every existing test are untouched.
- The behavioural contract in vault-ui's `docs/page-index.md` sections Staleness bounds, Incremental updates, Key derivation and Page index store holds in vault-cli, except where this spec replaces it (no bodies; format 2; distinct discard reasons).
- Published snapshots and the `*domain.Page` values in them are read-only for every consumer; the doc states this.
- Builds for darwin and linux with `CGO_ENABLED=0`.
- Version strings are owned by the releaser (`make check-versions`); do not hand-bump. Ginkgo/Gomega tests, counterfeiter mocks for new interfaces.
- New module dependencies are limited to `github.com/bborbe/boltkv`, `go.etcd.io/bbolt`, `github.com/bborbe/kv` (vault-ui's existing store stack).

### Assumptions

- `storage.PageStorage.ReadPage` and `storage.ParseOpenQuestions` (which only errors on context cancellation today) stay the single parse path; an Open Questions parse error fails that pass like any read error (previous snapshot kept).
- The store codec rebuilds pages with `domain.NewPage` from the raw frontmatter map — a JSON codec on `domain.Page` silently drops frontmatter (vault-ui's documented codec trap).

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery | Reversibility / Concurrency |
|---|---|---|---|---|
| Bolt file locked by another process | Open gives up after 5 s; discarded store; index serves from full parse | One warning naming path + reason | Next start after the other process exits uses the store | Concurrency: second process never hangs startup |
| v1 or other-format store (e.g. vault-ui's existing file) | Discarded; entry buckets deleted; full parse | Warning `store format mismatch (stored 1, current 2)` + path | Next publication writes v2 records | Irreversible by design (cache only) |
| Parser version change | Discarded; full parse | Warning `parser version mismatch` + path | Automatic re-population | Cache only |
| Damaged bolt file / record decode failure | Discarded; full parse; never fails construction | Warning reason `damaged` + path | Automatic re-population | Cache only |
| Crash mid-write | Bolt transaction atomic; previous store content intact; next start hydrates then stat-diffs | none needed | One stat-diff repairs drift | Partial write impossible |
| Filesystem hang during list or read | Each storage call bounded by 2 min rebuild timeout; previous snapshot served | Error logged with key | Next mark, read or rescan retries | Readers unaffected |
| File deleted/renamed between list and read | Excluded from snapshot; fingerprint recorded | — | Next stat-diff | — |
| Unreadable file / symlink out of vault / bad YAML | Excluded (same as vault-cli listing); fingerprint stored | Warned once per fingerprint change | Fix file; next stat-diff picks it up | — |
| Clock skew / restored older mtime | Fingerprint compared by inequality, not "newer"; status-change time catches same-size-same-mtime rewrites | — | Re-read on next stat-diff | — |
| Concurrent readers during publish | Atomic snapshot swap; reader sees whole old or whole new | — | — | Concurrency: no torn reads |
| Consumer mutates a shared `*domain.Page` | Contract violation; corrupts every reader's view | Documented read-only rule | Consumer copies before mutating | Concurrency hazard documented |
| Open-question text retained as body substring | Prevented: text cloned; AC 7 guards | Test failure | — | — |
| Heap still above consumer target because full frontmatter maps dominate | Out of scope here | Consumer heap profile in the migration task | Follow-up spec for key projection/interning | — |
| Disk full on store write | Write fails; previous content intact; serving unaffected | Warning logged | Next publication retries its own entries | Cache only |

## Security / Abuse Cases

- Inputs are local vault paths and filenames. `RefreshFile`, `MarkFileDirty` and the per-file read path reject empty names, path separators, `..` and non-`.md` names (falling back to a folder stat-diff where vault-ui does); the symlink-out-of-vault guard comes from vault-cli's page read and is preserved.
- The bolt store is untrusted input: decoding never preallocates from stored counts or lengths, and any decode failure discards the store rather than serving its content.
- The store lives under the user cache directory — never inside a vault or the repository tree — with directory mode 0700.
- Nothing retries forever: rescan runs on a fixed 50 s interval, storage calls are bounded by 2 min, store open by 5 s.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Types + seams: Key/NewKey, FileFingerprint + per-OS stat, DirectoryLister, metadata-only PageReader producing Entry (full read via storage ReadPage, Open Questions projection with clone, Content dropped), Metrics interface + no-op, warn sink; reader tests | 1, 2, 5 | 7, 13 | — |
| 2 | Snapshot engine, in-memory only: port build/merge/marks/revision/change log/rescan; Build, Refresh, RefreshFile, ListPages, ListEntries, lazy ReadPage; port engine tests, equivalence test, list-operation test; `docs/page-index.md` index sections; CHANGELOG bullet | 1, 2, 3, 4, 8 | 1–6, 8, 9 (minus `Store hydration`), 14 (non-store headings), 15 | prompt 1 |
| 3 | Bolt store v2 + hydration wiring: Store, NewBoltStore, DefaultStorePath, identity reconcile with distinct reasons, codec without Content and with OpenQuestions, frozen names, 5 s open timeout, hydration + write-through in the engine; store tests incl. literal v1-file discard and locked file; go.mod deps; doc store section | 6, 7, 8 | 9 (`Store hydration`), 10 (incl. body-sentinel + open-questions round-trip), 11, 12, 14 (`Store format version 2`) | prompt 2 |

Rationale: the reader defines the record shape every later layer stores; the engine is fully testable without persistence; the store is the only part adding dependencies and touches the engine's publish path last. AC 16–17 are operator checks after the PR.

## Do-Nothing Option

vault-ui keeps ~1.5 GB of heap and a ~760 MB cache file of bodies and keeps swapping; vault-api keeps loading every body on each HEAD change; two index implementations keep diverging. Not acceptable — the body is the dominant memory cost and only one consumer field depends on it.

## Open Questions

- Is full-frontmatter retention small enough for vault-ui's <300 MB heap target? Depends on the in-flight heap profile; if not, a follow-up spec adds key projection/interning.
- vault-ui jumps vault-cli v0.171.0 → this release (25 minors); unrelated API drift risk lands in that migration.
- vault-api retaining a metadata index between builds (instead of transient full pages) could raise steady-state pod memory; guarded by the consumer task's no-regression check (607Mi); fallback = construct the index per build.
