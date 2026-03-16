---
status: completed
summary: Added excludes config field to vault and wired it into ListDecisions to skip excluded directories during vault-wide walks
container: vault-cli-067-add-vault-excludes
dark-factory-version: v0.57.3
created: "2026-03-16T15:50:08Z"
queued: "2026-03-16T15:50:08Z"
started: "2026-03-16T15:50:09Z"
completed: "2026-03-16T15:56:00Z"
---

<summary>
- Add excludes list to vault configuration to skip directories during vault-wide operations
- Decision list no longer returns results from excluded directories like templates
- Excludes are prefix-matched against relative paths within the vault
- Empty excludes list preserves current behavior (backward compatible)
- Excluded directories are skipped entirely via fs.SkipDir (no subtree traversal)
- Helper method on baseStorage makes exclude logic reusable for future walk operations
</summary>

<objective>
Add an `excludes` config field to vaults so directories like "90 Templates" and ".claude" are skipped during vault-wide operations. Currently `decision list` walks the entire vault and returns noise from template files that happen to have `needs_review: true` in frontmatter.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/config/config.go` — `Vault` struct, add `Excludes` field
- `pkg/storage/storage.go` — `Config` struct, `NewConfigFromVault`, wire excludes
- `pkg/storage/decision.go` — `ListDecisions` uses `filepath.WalkDir` on full vault — this is the primary consumer that needs exclude filtering
- `pkg/storage/task.go` — `ListTasks` uses `os.ReadDir` on tasks_dir only — already scoped, no change needed
- `pkg/storage/page.go` — `ListPages` uses `os.ReadDir` on specific dir — already scoped, no change needed
- `pkg/ops/lint.go` — `Execute` uses `filepath.Walk` on tasks_dir only — already scoped, no change needed
- `example/config.yaml` — already updated with `excludes` field
- `README.md` — update config example

Real user config for reference (already updated):
```yaml
excludes:
  - "90 Templates"
  - ".claude"
  - ".obsidian"
```
</context>

<constraints>
- Excludes are prefix-matched against the relative path from vault root
- A file at `90 Templates/Foo.md` with relative path `90 Templates/Foo` matches exclude `"90 Templates"`
- Excludes apply to WalkDir — when a directory matches, skip the entire subtree (return `fs.SkipDir`)
- Empty excludes list means no filtering (backward compatible)
- Do NOT change `ListTasks`, `ListPages`, or lint — they already operate on scoped directories
- Use `github.com/bborbe/errors` for error wrapping
- Tests must use Ginkgo/Gomega with Counterfeiter mocks
</constraints>

<requirements>

## 1. `pkg/config/config.go` — Add Excludes field to Vault

```go
type Vault struct {
    // ... existing fields ...
    Excludes []string `yaml:"excludes,omitempty" json:"excludes,omitempty"`
}
```

Add getter method:
```go
// GetExcludes returns the list of excluded directory prefixes.
func (v *Vault) GetExcludes() []string {
    return v.Excludes
}
```

## 2. `pkg/storage/storage.go` — Wire excludes to storage Config

Add field to `Config`:
```go
type Config struct {
    // ... existing fields ...
    Excludes []string
}
```

Update `NewConfigFromVault`:
```go
func NewConfigFromVault(vault *config.Vault) *Config {
    return &Config{
        // ... existing fields ...
        Excludes: vault.GetExcludes(),
    }
}
```

## 3. `pkg/storage/decision.go` — Filter excluded paths in ListDecisions

In the `filepath.WalkDir` callback, after the `de.IsDir()` check and before processing:

```go
// Skip excluded directories
if de.IsDir() {
    rel, _ := filepath.Rel(vaultPath, path)
    for _, exclude := range d.config.Excludes {
        if strings.HasPrefix(filepath.ToSlash(rel), exclude) {
            return fs.SkipDir
        }
    }
    return nil
}
```

This skips the entire subtree when a directory matches an exclude prefix. The existing `!strings.HasSuffix(path, ".md")` check already handles non-markdown files.

IMPORTANT: The current code has `if de.IsDir() || !strings.HasSuffix(path, ".md") { return nil }`. This needs to be split — directories need the exclude check BEFORE returning nil, while non-.md files still return nil.

## 4. `pkg/storage/base.go` — Add helper for exclude checking

Add a reusable helper to `baseStorage`:

```go
func (b *baseStorage) isExcluded(vaultPath, path string) bool {
    rel, err := filepath.Rel(vaultPath, path)
    if err != nil {
        return false
    }
    relSlash := filepath.ToSlash(rel)
    for _, exclude := range b.config.Excludes {
        if strings.HasPrefix(relSlash, exclude) {
            return true
        }
    }
    return false
}
```

## 5. Tests in `pkg/storage/markdown_test.go` (or `pkg/storage/decision_test.go` if it exists)

Add test cases for exclude filtering. Target at least 80% coverage for new code:

- ListDecisions with excludes configured — files in excluded dir not returned
- ListDecisions with empty excludes — all files returned (backward compat)
- ListDecisions with exclude matching a subdirectory — entire subtree skipped
- `isExcluded` helper — matching prefix returns true, non-matching returns false, empty excludes returns false

## 6. `README.md` — Update config example

Add `excludes` to the config example:
```yaml
vaults:
  personal:
    name: personal
    path: ~/Documents/Obsidian/Personal
    tasks_dir: "24 Tasks"
    goals_dir: "23 Goals"
    daily_dir: "60 Periodic Notes/Daily"
    excludes:
      - "90 Templates"
      - ".obsidian"
```

</requirements>

<verification>
make precommit

# Verify excludes work with example config:
# Create a temp dir with excluded and non-excluded decisions
# Run vault-cli decision list and verify excluded files don't appear
</verification>
