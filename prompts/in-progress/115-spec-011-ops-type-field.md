---
status: committing
spec: [011-promote-task-watch-to-vault-watch]
summary: Added WatchDir struct with Kind field, extended WatchEvent with Type field populated from directory→kind map lookup, updated buildDirMap and handleEvent, updated CLI caller and all tests including new multi-kind test.
container: vault-cli-115-spec-011-ops-type-field
dark-factory-version: v0.156.1-1-g04f3863-dirty
created: "2026-05-10T00:00:00Z"
queued: "2026-05-10T22:07:30Z"
started: "2026-05-10T22:07:32Z"
branch: dark-factory/promote-task-watch-to-vault-watch
---

<summary>
- The `WatchTarget` struct gains a typed directory descriptor (`WatchDir`) so each watched path carries its entity kind (`task`, `goal`, `theme`, `objective`) alongside its directory name
- Every emitted `WatchEvent` now includes a `type` field (JSON key `"type"`) derived from the kind the caller registered for that directory — no path-string inference, no frontmatter reads
- The directory-to-kind mapping is built once at watcher startup from the `WatchDir` entries; per-event derivation is a map lookup
- File events from directories not in the watch map continue to be silently ignored (existing defensive behavior, unchanged)
- The `createTaskWatchCommand` call site in the CLI is updated to use the new `WatchDir` struct format with correct kind strings for all four entity types
- Existing `pkg/ops/watch_test.go` tests are updated to use the new struct format and gain assertions on the emitted `Type` field
- The `WatchOperation` interface signature is unchanged — no mock regeneration required, but the executor should verify it compiles
- All pre-existing test assertions continue to pass; the `Type` field addition is additive
</summary>

<objective>
Extend the ops-layer watch types so that each `WatchDir` entry carries an entity kind alongside its path, and populate a `type` field on every emitted `WatchEvent`. This is Prompt 1 of 2 for spec 011. Prompt 2 depends on the new struct shapes being in place.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read `go-patterns.md` in `~/.claude/plugins/marketplaces/coding/docs/` for interface/struct patterns.
Read `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for Ginkgo/Gomega test patterns.
Read `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` for which test types to write for each code change.

Key files to read before making changes:
- `pkg/ops/watch.go` — full file (~189 lines); contains `WatchTarget`, `WatchEvent`, `WatchOperation`, `vaultInfo`, `buildDirMap`, `handleEvent`
- `pkg/ops/watch_test.go` — full file; tests using `captureWatchEvents` helper that will need `WatchDirs` format updated
- `pkg/cli/cli.go` — lines 2015–2050 (`createTaskWatchCommand`) — the only CLI caller of `WatchTarget`
- `mocks/watch-operation.go` — generated mock; read to confirm it compiles after `WatchTarget` field change (interface signature is unchanged, so regeneration should not be needed)
</context>

<requirements>
### 1. Add `WatchDir` struct to `pkg/ops/watch.go`

Add a new exported struct immediately before `WatchTarget`:

```go
// WatchDir pairs a vault-relative directory name with the entity kind it contains.
type WatchDir struct {
	Dir  string
	Kind string
}
```

### 2. Change `WatchTarget.WatchDirs` from `[]string` to `[]WatchDir`

In `pkg/ops/watch.go`, change the `WatchTarget` struct from:

```go
type WatchTarget struct {
	VaultPath string
	VaultName string
	WatchDirs []string
}
```

to:

```go
type WatchTarget struct {
	VaultPath string
	VaultName string
	WatchDirs []WatchDir
}
```

### 3. Extend `vaultInfo` to carry `kind`

In `pkg/ops/watch.go`, change the private `vaultInfo` struct from:

```go
type vaultInfo struct {
	vaultPath string
	vaultName string
}
```

to:

```go
type vaultInfo struct {
	vaultPath string
	vaultName string
	kind      string
}
```

### 4. Update `buildDirMap` to iterate `[]WatchDir` and populate `kind`

Change the inner loop in `buildDirMap` from iterating `[]string` to iterating `[]WatchDir`:

```go
func buildDirMap(watcher *fsnotify.Watcher, vaults []WatchTarget) map[string]vaultInfo {
	dirToVault := make(map[string]vaultInfo)
	for _, target := range vaults {
		for _, wd := range target.WatchDirs {
			absDir := filepath.Join(target.VaultPath, wd.Dir)
			if _, err := os.Stat(absDir); err != nil {
				slog.Debug("watch skipping missing directory", "dir", absDir)
				continue
			}
			if err := watcher.Add(absDir); err != nil {
				slog.Warn("watch failed", "dir", absDir, "error", err)
				continue
			}
			dirToVault[absDir] = vaultInfo{
				vaultPath: target.VaultPath,
				vaultName: target.VaultName,
				kind:      wd.Kind,
			}
		}
	}
	return dirToVault
}
```

### 5. Add `Type` field to `WatchEvent`

In `pkg/ops/watch.go`, change `WatchEvent` from:

```go
type WatchEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Vault string `json:"vault"`
	Path  string `json:"path"`
}
```

to:

```go
type WatchEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Vault string `json:"vault"`
	Path  string `json:"path"`
	Type  string `json:"type"`
}
```

The new field is last; existing JSON consumers parsing by field name are unaffected.

### 6. Update `handleEvent` to populate `ev.Type`

In `handleEvent`, after constructing the `ev` struct, set `ev.Type = info.kind`:

```go
ev := WatchEvent{
	Event: eventType,
	Name:  strings.TrimSuffix(filepath.Base(absPath), ".md"),
	Vault: info.vaultName,
	Path:  relPath,
	Type:  info.kind,
}
```

No other changes to `handleEvent` are needed.

### 7. Update `createTaskWatchCommand` in `pkg/cli/cli.go`

In `createTaskWatchCommand` (around line 2015), change the `WatchDirs` field from a `[]string` slice to a `[]ops.WatchDir` slice with correct kind values for all four entity types:

```go
targets = append(targets, ops.WatchTarget{
	VaultPath: vault.Path,
	VaultName: vault.Name,
	WatchDirs: []ops.WatchDir{
		{Dir: vault.GetTasksDir(), Kind: "task"},
		{Dir: vault.GetGoalsDir(), Kind: "goal"},
		{Dir: vault.GetThemesDir(), Kind: "theme"},
		{Dir: vault.GetObjectivesDir(), Kind: "objective"},
	},
})
```

No other changes to `createTaskWatchCommand` are needed (deprecation warning is added in Prompt 2).

### 8. Update `pkg/ops/watch_test.go`

Update all occurrences of `WatchDirs: []string{"Tasks"}` (or similar) to use the new `[]ops.WatchDir` format. For any test that creates a `WatchTarget` with a `Tasks` directory, use `Kind: "task"`:

```go
targets := []ops.WatchTarget{
	{
		VaultPath: vaultDir,
		VaultName: "personal",
		WatchDirs: []ops.WatchDir{
			{Dir: "Tasks", Kind: "task"},
		},
	},
}
```

In the test "emits a JSON event with correct fields when an .md file is created", add an assertion on the `Type` field:

```go
Expect(ev.Type).To(Equal("task"))
```

Place this alongside the existing assertions on `ev.Name`, `ev.Vault`, `ev.Path`, `ev.Event`.

For all other tests that use `WatchDirs`, update them to `[]ops.WatchDir` format. Keep the `Kind` value consistent with the directory being watched (use `"task"` for `"Tasks"` directories in all existing tests — the tests are not testing kind derivation for other entity types).

**Add a new test that exercises all four kinds** (`task`, `goal`, `theme`, `objective`) — this verifies the directory→kind lookup works for every kind, not just the alias-checked `"task"` value (a typo like `Kind: "tasks"` plural in CLI wiring would not be caught by single-kind tests):

```go
It("emits the correct Type for each entity kind", func() {
    vaultDir := filepath.Join(tmpDir, "vault-multikind")
    for _, sub := range []string{"Tasks", "Goals", "Themes", "Objectives"} {
        Expect(os.MkdirAll(filepath.Join(vaultDir, sub), 0755)).To(Succeed())
    }

    targets := []ops.WatchTarget{
        {
            VaultPath: vaultDir,
            VaultName: "v",
            WatchDirs: []ops.WatchDir{
                {Dir: "Tasks", Kind: "task"},
                {Dir: "Goals", Kind: "goal"},
                {Dir: "Themes", Kind: "theme"},
                {Dir: "Objectives", Kind: "objective"},
            },
        },
    }

    events := captureWatchEvents(targets, func() {
        for sub, name := range map[string]string{"Tasks": "T", "Goals": "G", "Themes": "Th", "Objectives": "O"} {
            Expect(os.WriteFile(filepath.Join(vaultDir, sub, name+".md"), []byte("x"), 0644)).To(Succeed())
        }
    })

    typeByName := map[string]string{}
    for _, ev := range events {
        typeByName[ev.Name] = ev.Type
    }
    Expect(typeByName["T"]).To(Equal("task"))
    Expect(typeByName["G"]).To(Equal("goal"))
    Expect(typeByName["Th"]).To(Equal("theme"))
    Expect(typeByName["O"]).To(Equal("objective"))
})
```

### 9. Verify mock compilation

The `WatchOperation` interface signature (`Execute(ctx, []WatchTarget, func(WatchEvent) error) error`) has not changed — only the internal shape of `WatchTarget` and `WatchEvent` changed. The generated mock at `mocks/watch-operation.go` references `[]ops.WatchTarget` as a parameter type and should compile without regeneration.

Verify by running:
```
go build ./mocks/...
```

If compilation fails, regenerate with:
```
go generate ./pkg/ops/...
```
</requirements>

<constraints>
- The `WatchOperation` interface signature must NOT change — only the types `WatchTarget`, `WatchDir`, and `WatchEvent` are modified
- The existing JSON keys (`event`, `name`, `vault`, `path`) on `WatchEvent` must remain unchanged; `type` is an additional key
- `WatchEvent.Type` is populated from `vaultInfo.kind` which comes from `WatchDir.Kind` — no path-string inference, no frontmatter reads
- File events from directories not in the `dirToVault` map continue to be silently ignored (the `!found` guard in `handleEvent` is unchanged)
- Do NOT add any stdout writes to `pkg/ops/watch.go` — the ops layer remains I/O-free; stdout is the caller's responsibility via the handler callback
- All existing watch tests must continue to pass (updated struct syntax is not breaking their intent)
- Do NOT commit — dark-factory handles git
- Prompt 2 (CLI watch command + deprecation warning) depends on this prompt completing first
</constraints>

<verification>
```
make precommit
```

```
# Confirm WatchDir type exists
grep -n 'type WatchDir struct' pkg/ops/watch.go
# expected: one line

# Confirm WatchEvent has Type field
grep -n '"type"' pkg/ops/watch.go
# expected: one line with json tag

# Confirm WatchTarget uses []WatchDir
grep -n 'WatchDirs \[\]WatchDir' pkg/ops/watch.go
# expected: one line

# Confirm vaultInfo has kind field
grep -n 'kind.*string' pkg/ops/watch.go
# expected: one line in vaultInfo struct

# Confirm CLI caller is updated
grep -n 'WatchDir{' pkg/cli/cli.go
# expected: four lines (one per entity type)

# Confirm mock compiles
go build ./mocks/...
```
</verification>
