---
status: completed
spec: [012-watch-rename-as-deleted]
summary: Changed mapFsnotifyOp() to return 'deleted' for fsnotify.Rename op, removed 'renamed' from watch --help text, added two integration tests verifying the mapping, and added CHANGELOG Unreleased entry.
container: vault-cli-118-spec-012-watch-rename-as-deleted
dark-factory-version: v0.156.1-1-g04f3863-dirty
created: "2026-05-14T14:30:00Z"
queued: "2026-05-14T14:29:37Z"
started: "2026-05-14T14:29:38Z"
completed: "2026-05-14T14:31:32Z"
branch: dark-factory/watch-rename-as-deleted
---

<summary>
- fsnotify `Rename` op (triggered by Obsidian trash-delete, `mv` out-of-dir, and atomic editor saves) now emits `event: "deleted"` instead of `event: "renamed"`
- The string `"renamed"` can no longer appear in any event emitted by `vault-cli watch`
- `vault-cli watch --help` documents three event types: `created`, `modified`, `deleted` — `renamed` is removed from the Long description
- Two new Ginkgo integration tests verify the mapping with real temp dirs and real fsnotify: one for `mv`-out-of-dir (Rename op) and one for `rm` (Remove op, regression guard)
- `CHANGELOG.md` gains an `## Unreleased` entry calling out the breaking event-API change (`renamed` removed; `Rename` ops now reported as `deleted`)
- `README.md` has no `"renamed"` mention to update (verified: zero grep matches); `integration/cli_test.go` has no `"renamed"` assertion to update (verified: zero grep matches)
- The debouncer key (`vault:relpath`) and window are unchanged — debounce behavior for atomic saves is preserved
</summary>

<objective>
Change `mapFsnotifyOp()` in `pkg/ops/watch.go` to return `"deleted"` for the `fsnotify.Rename` op (instead of `"renamed"`), and remove `"renamed"` from the `vault-cli watch --help` text. After this change, both fsnotify `Remove` and `Rename` ops are reported as `deleted`, and the `"renamed"` event type is eliminated from the public API. This resolves the downstream bug where Obsidian deletes (which use `os.Rename` to `.trash/`) produced unhandled `renamed` events that left task cards lingering in consumer UIs.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for Ginkgo v2/Gomega patterns.
Read `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` for which test types to write for each code change.
Read `changelog-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for the changelog entry format.

Key files to read before making changes:
- `pkg/ops/watch.go` — full file (~199 lines); contains `mapFsnotifyOp()` at lines 155–170 (the only behavioral change site), plus `WatchEvent`, `handleEvent`, and the debouncer
- `pkg/ops/watch_test.go` — full file (~240 lines); contains the `captureWatchEvents` helper (lines 22–79) and all existing Ginkgo tests; new tests go inside the existing `Describe("Execute", ...)` block
- `pkg/ops/ops_suite_test.go` — Ginkgo suite bootstrap; read package name before writing new test code
- `pkg/cli/cli.go` — lines 2103–2150 (`createWatchCommand`); the `Long:` help text at line 2116 lists `renamed` as a valid event value and must be updated; no other changes needed in this file
</context>

<requirements>
### 1. Fix `mapFsnotifyOp()` in `pkg/ops/watch.go`

At lines 165–166, change the `Rename` case from:

```go
case op.Has(fsnotify.Rename):
    return "renamed"
```

to:

```go
case op.Has(fsnotify.Rename):
    return "deleted"
```

This is the only behavioral change in `pkg/ops/watch.go`. No other lines in the file change.

After this fix the full mapping is:
- `Write` → `"modified"` (unchanged)
- `Create` → `"created"` (unchanged)
- `Remove` → `"deleted"` (unchanged)
- `Rename` → `"deleted"` (was `"renamed"`)
- `Chmod` / other → `""` (filtered, unchanged)

### 2. Update help text in `pkg/cli/cli.go`

In `createWatchCommand()` (around line 2112–2123), the `Long:` field currently includes:

```
  event  - change type: created, modified, deleted, renamed
```

Change that line to remove `renamed`:

```
  event  - change type: created, modified, deleted
```

No other changes to `createWatchCommand`, `parseWatchTypes`, `watchTypeIsValid`, or `watchEventMatchesFilter` are needed — those functions deal with the entity `--types` flag (task/goal/theme/objective), not with the event type strings.

### 3. Add Ginkgo integration tests in `pkg/ops/watch_test.go`

Add two new `It(...)` cases inside the existing `Describe("Execute", ...)` block (which is nested inside `Describe("WatchOperation", ...)`). Use `Label("integration")` on both new tests.

The existing `captureWatchEvents` helper signature (lines 22–79):

```go
func captureWatchEvents(
    ctx context.Context,
    watchOp ops.WatchOperation,
    targets []ops.WatchTarget,
    triggerFn func(),
    timeout time.Duration,
) ([]ops.WatchEvent, error)
```

It waits 50ms for the watcher to start, then calls `triggerFn()`, then collects events until `timeout` elapses.

**Critical setup pattern for both tests:** Create the target `.md` file BEFORE calling `captureWatchEvents`. This way the watcher starts with the file already present and generates no `"created"` event for it. Only the subsequent file-system operation (rename / remove) triggers events, keeping the assertion clear.

**Test 3a: `mv` out-of-dir emits `event:"deleted"`**

```go
It("emits event:deleted when an .md file is moved out of the watched directory", Label("integration"), func() {
    outsideDir, err := os.MkdirTemp("", "vault-watch-outside-*")
    Expect(err).NotTo(HaveOccurred())
    DeferCleanup(func() { Expect(os.RemoveAll(outsideDir)).To(Succeed()) })

    // Create file before watcher starts so no "created" event is captured.
    mdPath := filepath.Join(tasksDir, "Moved Task.md")
    Expect(os.WriteFile(mdPath, []byte("content"), 0600)).To(Succeed())

    targets := []ops.WatchTarget{
        {
            VaultPath: vaultDir,
            VaultName: "personal",
            WatchDirs: []ops.WatchDir{{Dir: "Tasks", Kind: "task"}},
        },
    }

    events, err := captureWatchEvents(ctx, watchOp, targets, func() {
        dest := filepath.Join(outsideDir, "Moved Task.md")
        Expect(os.Rename(mdPath, dest)).To(Succeed())
    }, 600*time.Millisecond)
    Expect(err).NotTo(HaveOccurred())

    var deletedEvs []ops.WatchEvent
    for _, ev := range events {
        if ev.Name == "Moved Task" && ev.Event == "deleted" {
            deletedEvs = append(deletedEvs, ev)
        }
    }
    Expect(deletedEvs).To(HaveLen(1), "expected exactly one deleted event for the moved file")
    Expect(deletedEvs[0].Path).To(Equal(filepath.Join("Tasks", "Moved Task.md")))
    Expect(deletedEvs[0].Vault).To(Equal("personal"))

    // The string "renamed" must never appear in any event.
    for _, ev := range events {
        Expect(ev.Event).NotTo(Equal("renamed"), "renamed event must not be emitted after fix")
    }
})
```

**Test 3b: `rm` still emits `event:"deleted"` (regression guard)**

```go
It("emits event:deleted when an .md file is removed with os.Remove", Label("integration"), func() {
    // Create file before watcher starts so no "created" event is captured.
    mdPath := filepath.Join(tasksDir, "Deleted Task.md")
    Expect(os.WriteFile(mdPath, []byte("content"), 0600)).To(Succeed())

    targets := []ops.WatchTarget{
        {
            VaultPath: vaultDir,
            VaultName: "personal",
            WatchDirs: []ops.WatchDir{{Dir: "Tasks", Kind: "task"}},
        },
    }

    events, err := captureWatchEvents(ctx, watchOp, targets, func() {
        Expect(os.Remove(mdPath)).To(Succeed())
    }, 600*time.Millisecond)
    Expect(err).NotTo(HaveOccurred())

    var deletedEvs []ops.WatchEvent
    for _, ev := range events {
        if ev.Name == "Deleted Task" && ev.Event == "deleted" {
            deletedEvs = append(deletedEvs, ev)
        }
    }
    Expect(deletedEvs).To(HaveLen(1), "expected exactly one deleted event for the removed file")
    Expect(deletedEvs[0].Path).To(Equal(filepath.Join("Tasks", "Deleted Task.md")))
})
```

No new imports are needed — `os`, `filepath`, `time`, and the `ops` package import are already present in `watch_test.go`.

### 4. CHANGELOG entry

Open `CHANGELOG.md`. If `## Unreleased` already exists at the top (above the topmost released version), append to it. If not, create it.

Add this entry:

```markdown
- fix: Map fsnotify `Rename` op to `deleted` event in `vault-cli watch` — removes the `renamed` event type from the public API. Consumers handling `deleted` now automatically receive Obsidian trash-deletes (which use `os.Rename` internally). Breaking: any consumer expecting `event:"renamed"` will no longer receive that string.
```

Do NOT bump any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json` — the dark-factory `autoRelease` pipeline handles versioning.

### 5. Verify no `"renamed"` references remain

After all changes, run:

```bash
grep -rn '"renamed"' pkg/ integration/
```

Expected: zero matches. If any remain, fix them before proceeding to `make precommit`.
</requirements>

<constraints>
- Event JSON shape on stdout must not change beyond the removal of the `"renamed"` value — the `event`, `name`, `vault`, `path`, `type` JSON fields remain identical in structure
- The debouncer key (`vault:relpath`) and window are unchanged
- Existing tests for `created`, `modified`, `deleted` must continue to pass without modification
- Do NOT modify the `WatchOperation` interface, the `WatchEvent` struct fields, or the `WatchTarget`/`WatchDir` structs
- Do NOT add stdout writes to `pkg/ops/watch.go` — the ops layer is I/O-free; stdout is the caller's responsibility via the handler callback
- Follows the Interface → Constructor → Struct → Method pattern from `docs/development-patterns.md` and existing `pkg/ops/` rules
- `integration/cli_test.go` does not mention `"renamed"` — no changes needed there (grep confirms zero matches)
- `README.md` does not mention `"renamed"` — no changes needed there (grep confirms zero matches)
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```bash
make precommit
```

```bash
# Confirm renamed is gone from the ops layer
grep -n '"renamed"' pkg/ops/watch.go
# expected: zero matches

# Confirm renamed is gone from the CLI help text
grep -n 'renamed' pkg/cli/cli.go
# expected: zero matches

# Confirm the two new integration tests exist
grep -n 'Label("integration")' pkg/ops/watch_test.go
# expected: exactly 2 lines (zero before this change; both added by Test 3a + Test 3b)

# Run the ops package tests to confirm all pass
go test -v ./pkg/ops/... -timeout 30s
```
</verification>
