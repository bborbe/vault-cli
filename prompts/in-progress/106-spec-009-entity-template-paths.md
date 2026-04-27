---
status: committing
spec: [009-entity-templates]
summary: Added five optional template path fields (task_template, goal_template, theme_template, objective_template, vision_template) to Vault struct with path resolution helper, typed accessors, resolution in GetVault and GetAllVaults, comprehensive Ginkgo tests, and CHANGELOG entry.
container: vault-cli-106-spec-009-entity-template-paths
dark-factory-version: v0.135.19-1-gc08c946
created: "2026-04-27T10:00:00Z"
queued: "2026-04-27T10:02:50Z"
started: "2026-04-27T10:03:45Z"
branch: dark-factory/entity-templates
---

<summary>
- Five optional template path fields added to the `Vault` config struct, one per entity type (task, goal, theme, objective, vision), with `omitempty` YAML and JSON tags
- Typed accessor method per entity type returns the resolved absolute path, or empty string when the field is unset or empty
- Path resolution follows the same rule as `vault.Path` and `session_project_dir`: tilde expands to home directory, absolute paths pass through unchanged, relative paths resolve against the (already-expanded) vault root path
- Resolution happens inside `GetVault` and `GetAllVaults` so accessors are simple field readers — no resolution logic in accessor methods
- Existing vault configs without any `*_template` fields parse, serialize, and round-trip identically to today — all new fields carry `omitempty` so they are omitted from YAML when empty
- All existing tests continue to pass; new Ginkgo tests cover all five accessor methods and all resolution cases (empty, unset, relative, absolute, tilde-prefixed)
- CHANGELOG.md gets an entry under `## Unreleased`
</summary>

<objective>
Add five optional template path fields (`task_template`, `goal_template`, `theme_template`, `objective_template`, `vision_template`) to the `Vault` struct in `pkg/config/config.go`. Each field has a typed accessor that returns the resolved absolute path — or empty string when unset — using the same path resolution already applied to `vault.Path` and `SessionProjectDir`. No CLI commands, entity domain types, or storage code are changed.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read the relevant coding guides from `~/.claude/plugins/marketplaces/coding/docs/`:
- `go-patterns.md` — struct field conventions, accessor naming
- `go-testing-guide.md` — Ginkgo/Gomega test patterns

Key files to read fully before making any changes:

- `pkg/config/config.go` — the `Vault` struct, existing `*_dir` fields with `omitempty` tags, `GetVault` (lines ~166–202) and `GetAllVaults` (lines ~204–233) where path resolution for `Path` and `SessionProjectDir` already lives, and the accessor methods (`GetTasksDir`, `GetSessionProjectDir`, etc.)
- `pkg/config/config_test.go` — the Ginkgo test file showing vault config test patterns; do NOT modify the suite bootstrap `pkg/config/config_suite_test.go`

Study the resolution pattern in `GetVault` (lines ~185–199): `vault.Path` gets tilde-expanded, then `SessionProjectDir` gets the same treatment. The new template fields need the same treatment PLUS relative-path resolution against the expanded `vault.Path`.

The existing `Get*Dir()` accessors (e.g. `GetTasksDir`) return plain directory names — no resolution — because those fields are directory names relative to vault root, not file paths. The new `Get*Template()` accessors are different: they return fully resolved absolute paths (or empty string).
</context>

<requirements>
### 1. Add five template fields to `Vault` struct in `pkg/config/config.go`

After the `SessionProjectDir` field and before `Excludes`, add:

```go
TaskTemplate      string   `yaml:"task_template,omitempty"       json:"task_template,omitempty"`
GoalTemplate      string   `yaml:"goal_template,omitempty"       json:"goal_template,omitempty"`
ThemeTemplate     string   `yaml:"theme_template,omitempty"      json:"theme_template,omitempty"`
ObjectiveTemplate string   `yaml:"objective_template,omitempty"  json:"objective_template,omitempty"`
VisionTemplate    string   `yaml:"vision_template,omitempty"     json:"vision_template,omitempty"`
```

### 2. Add five typed accessor methods on `*Vault` in `pkg/config/config.go`

Add these below `GetSessionProjectDir()`. Each accessor simply returns the (post-resolution) field value:

```go
// GetTaskTemplate returns the resolved absolute path to the task template, or empty string if not set.
func (v *Vault) GetTaskTemplate() string {
    return v.TaskTemplate
}

// GetGoalTemplate returns the resolved absolute path to the goal template, or empty string if not set.
func (v *Vault) GetGoalTemplate() string {
    return v.GoalTemplate
}

// GetThemeTemplate returns the resolved absolute path to the theme template, or empty string if not set.
func (v *Vault) GetThemeTemplate() string {
    return v.ThemeTemplate
}

// GetObjectiveTemplate returns the resolved absolute path to the objective template, or empty string if not set.
func (v *Vault) GetObjectiveTemplate() string {
    return v.ObjectiveTemplate
}

// GetVisionTemplate returns the resolved absolute path to the vision template, or empty string if not set.
func (v *Vault) GetVisionTemplate() string {
    return v.VisionTemplate
}
```

### 3. Add a helper function for template path resolution

Add a package-level unexported helper in `pkg/config/config.go` to avoid repeating the resolution logic five times:

```go
// resolveTemplatePath resolves a template field value to an absolute path.
// Returns empty string if value is empty.
// Expands a leading ~ to the user home directory.
// Joins a relative path against vaultPath (already an absolute path).
// Returns an absolute path unchanged.
func resolveTemplatePath(value, vaultPath string) (string, error) {
    if value == "" {
        return "", nil
    }
    if len(value) > 0 && value[0] == '~' {
        homeDir, err := os.UserHomeDir()
        if err != nil {
            return "", fmt.Errorf("get home directory: %w", err)
        }
        return filepath.Join(homeDir, value[1:]), nil
    }
    if filepath.IsAbs(value) {
        return value, nil
    }
    return filepath.Join(vaultPath, value), nil
}
```

### 4. Call `resolveTemplatePath` inside `GetVault`

In `GetVault`, after the existing `SessionProjectDir` tilde expansion block (around line 199), add resolution for all five template fields. At this point `vault.Path` is already fully expanded. Use the helper:

```go
templateFields := []*string{
    &vault.TaskTemplate,
    &vault.GoalTemplate,
    &vault.ThemeTemplate,
    &vault.ObjectiveTemplate,
    &vault.VisionTemplate,
}
for _, f := range templateFields {
    resolved, err := resolveTemplatePath(*f, vault.Path)
    if err != nil {
        return nil, fmt.Errorf("resolve template path: %w", err)
    }
    *f = resolved
}
```

### 5. Call `resolveTemplatePath` inside `GetAllVaults`

In `GetAllVaults`, after the existing `SessionProjectDir` tilde expansion block (inside the vault loop), add the same resolution block. At this point `v.Path` is already fully expanded:

```go
templateFields := []*string{
    &v.TaskTemplate,
    &v.GoalTemplate,
    &v.ThemeTemplate,
    &v.ObjectiveTemplate,
    &v.VisionTemplate,
}
for _, f := range templateFields {
    resolved, err := resolveTemplatePath(*f, v.Path)
    if err != nil {
        return nil, fmt.Errorf("resolve template path: %w", err)
    }
    *f = resolved
}
```

### 6. Write tests in `pkg/config/config_test.go`

Add a new `Describe("Vault template accessors")` block inside the existing `Describe("Loader")` block (or as a sibling — follow the file's existing structure). Do NOT add a new suite bootstrap file.

Cover all of the following cases using `GetVault` as the entry point (so resolution logic is exercised):

**Accessor returns empty string when field is unset:**
```go
Describe("GetTaskTemplate", func() {
    Context("when task_template is not set", func() {
        BeforeEach(func() {
            configData := `vaults:
  main:
    name: main
    path: /vault/main
`
            // write configData to configPath, create loader
        })
        It("returns empty string", func() {
            vault, err := loader.GetVault(ctx, "main")
            Expect(err).To(BeNil())
            Expect(vault.GetTaskTemplate()).To(Equal(""))
        })
    })
})
```

Cover the same empty/unset case for the remaining four entity types (goal, theme, objective, vision) — one `It` each.

**Absolute path passes through unchanged** (test for at least `task_template`):
```yaml
task_template: /absolute/path/task.md
```
Expected: `GetTaskTemplate()` returns `/absolute/path/task.md`.

**Relative path resolves against vault root** (test for at least `task_template`):
```yaml
path: /vault/main
task_template: 90 Templates/Task Template.md
```
Expected: `GetTaskTemplate()` returns `/vault/main/90 Templates/Task Template.md`.

**Tilde-prefixed path expands to home directory** (test for at least `task_template`):
```yaml
task_template: ~/Templates/task.md
```
Expected: `GetTaskTemplate()` does not contain `~`, starts with the user home directory. Use:
```go
homeDir, err := os.UserHomeDir()
Expect(err).To(BeNil())
Expect(vault.GetTaskTemplate()).To(HavePrefix(homeDir))
```

**All five fields present in a single config parse correctly:**
Write one test with all five `*_template` fields set to absolute paths and assert all five accessors return the correct values.

**Existing config without any `*_template` fields is unchanged:**
Use the existing `Load` test configs (no `*_template` fields) and confirm `GetTaskTemplate()` etc. return `""`. This can be a simple `It` added to an existing `Context`.

**Round-trip serialization (YAML marshal/unmarshal):**
Construct a `Vault` with only `task_template` set. Marshal to YAML with `yaml.Marshal`. Assert the output contains `task_template:`. Then marshal a `Vault` with no template fields set; assert the output does NOT contain `task_template` (omitempty).

**`GetAllVaults` resolves template paths:**
Add at least one test that calls `loader.GetAllVaults(ctx)` against a config with a relative `task_template`, and asserts the returned vault's `GetTaskTemplate()` is the absolute resolved path. This ensures the resolution block in `GetAllVaults` (req #5) is exercised.

### 7. Update CHANGELOG.md

Add under `## Unreleased` (create the section if it does not exist, immediately after the `# Changelog` heading):

```markdown
- feat: Add optional template path fields (task_template, goal_template, theme_template, objective_template, vision_template) to vault config with path resolution
```
</requirements>

<constraints>
- New fields and accessors must mirror the existing per-entity-type directory config pattern: same struct, same `omitempty` YAML and JSON tags, same accessor naming convention (`Get<Entity>Template`)
- Field naming must follow the YAML snake_case `{entity_type}_template` convention
- Path resolution must use the helper `resolveTemplatePath` — no duplicated logic per field
- All existing `vault-cli` operations and tests must pass without modification
- No new dependencies introduced
- No CLI commands added or changed
- No changes to entity domain types, storage, or operations
- Do NOT commit — dark-factory handles git
- Template fields must use `omitempty` on both yaml and json tags so existing configs without them are unaffected
- The accessor methods must be simple field readers — all resolution happens in `GetVault` and `GetAllVaults`
- Unlike the existing `Get*Dir()` accessors which return defaults (e.g. `"Tasks"`, `"21 Themes"`), the new `Get*Template()` accessors return empty string when the field is unset — there is no default template path
</constraints>

<verification>
Run `make precommit` — must pass.

```bash
# Confirm five template fields exist in Vault struct
grep -n 'task_template\|goal_template\|theme_template\|objective_template\|vision_template' pkg/config/config.go
# expected: at least 5 lines (struct fields + resolution calls)

# Confirm accessor methods exist
grep -n 'func.*GetTaskTemplate\|func.*GetGoalTemplate\|func.*GetThemeTemplate\|func.*GetObjectiveTemplate\|func.*GetVisionTemplate' pkg/config/config.go
# expected: 5 lines

# Confirm resolveTemplatePath helper exists
grep -n 'func resolveTemplatePath' pkg/config/config.go
# expected: 1 line

# Confirm resolution is called in GetVault and GetAllVaults
grep -n 'resolveTemplatePath' pkg/config/config.go
# expected: multiple lines (definition + calls in GetVault and GetAllVaults)

# Confirm tests cover template fields
grep -n 'GetTaskTemplate\|GetGoalTemplate\|GetVisionTemplate' pkg/config/config_test.go
# expected: multiple lines

# Run tests for config package specifically
go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/config/... && go tool cover -func=/tmp/cover.out | grep config
# expected: pass, coverage includes new code
```
</verification>
