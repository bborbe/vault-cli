---
status: draft
spec: [054-rollup-weekly-baseline-delta]
created: "2026-09-26T15:48:55Z"
---

# `vault-cli config set-baseline <vault> <path>`: the config key, its validation and its atomic write (spec 054, prompt 1 of 3)

<summary>
- A vault's config entry gains a `baseline` key naming a baseline file inside that vault.
- A new `vault-cli config set-baseline <vault> <path>` verb writes that key, alongside the existing `config list` and `config current-user`.
- The verb reads the config file before it writes anything: a malformed or unparseable config fails with nothing written.
- The verb refuses a vault the config does not name, and refuses a path that is absolute or that escapes the vault root — in both cases the config file is untouched.
- The config file is written atomically: after any failure it holds either its previous content or the new content, never a truncated or half-merged file.
- The path validation is a shared helper, so the same rule can guard the read side when prompt 3 wires the rollup to the config key.
- An existing `config.yaml` with no `baseline` key parses and behaves exactly as before; no other per-vault key changes meaning.
- The README's `### config` block names the new verb, and the integration help table gains its row.
- No rollup output changes in this prompt, and no changelog entry is written here.
</summary>

<objective>
Land the config-side half of the rollup baseline: a `baseline` key on a vault's config entry, a `config set-baseline <vault> <path>` verb that persists it read-before-write and atomically, and a shared vault-relative path validator that the rollup's read side will reuse. This is spec 054's first prompt: it covers Desired Behavior 1 and depends on nothing — prompts 2 and 3 build on it.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/development-patterns.md` and `docs/dod.md` (the Definition of Done — it names the changelog, the README and the integration command-registration table explicitly).

Read these files fully before making changes:

- `pkg/config/config.go` — the file you extend. `type Config struct` (`CurrentUser`, `DefaultVault`, `Vaults map[string]Vault`, `Notification`) and `type Vault struct` (the per-vault keys: `Path`, `Name`, `TasksDir`, `GoalsDir`, `ThemesDir`, `ObjectivesDir`, `VisionDir`, `DailyDir`, `KnowledgeDir`, `TopicsDir`, `ClaudeScript`, `SessionProjectDir`, `WorkOnCommand`, `WorkOnGoalCommand`, `TaskTemplate`, `GoalTemplate`, `ThemeTemplate`, `ObjectiveTemplate`, `VisionTemplate`, `Excludes`). Every field carries BOTH a `yaml:` and a `json:` tag, and every optional one carries `,omitempty` on both — e.g. `TasksDir string \`yaml:"tasks_dir,omitempty" json:"tasks_dir,omitempty"\``. `FindConfigDir(ctx, toolName)` applies the XDG-first resolution. `NewLoader(configPath)` / `type configLoader struct { configPath string }` / `func (c *configLoader) Load(ctx)` — `Load` resolves the config path itself (explicit `configPath`, else `FindConfigDir(ctx, "vault-cli")` joined with `config.yaml`), returns `getDefaultConfig` when the file does not exist, and otherwise reads, `yaml.Unmarshal`s, lowercases `DefaultVault` and lowercases every vault map key. Note `//#nosec G304 -- user-controlled config path` on the `os.ReadFile` at line 242 — mirror that comment form and reason for any user-controlled path you read or write.
- `pkg/config/config_suite_test.go` — the `config_test` suite entry point. **This package has exactly ONE `func TestSuite(t *testing.T)` calling `RunSpecs`. A second `func Test*` calling `RunSpecs` panics, so do not add one.**
- `pkg/config/config_test.go` and `pkg/config/find_config_dir_test.go` — the existing `config_test` style: Ginkgo v2, `os.MkdirTemp` for a temp dir, `configPath = filepath.Join(tempDir, "config.yaml")`, `config.NewLoader(configPath)`, raw YAML written with `os.WriteFile`.
- `pkg/cli/cli.go` — read `NewRootCommand` in full: the `configLoader config.Loader`, `vaultName`, `configPath`, `outputFormat`, `verbose` locals; the `PersistentPreRunE` that runs `configLoader = config.NewLoader(configPath)`; the persistent flags; and the `configCmd` block near the end (`configCmd.AddCommand(createConfigListCommand(...))`, `configCmd.AddCommand(createConfigCurrentUserCommand(...))`, `rootCmd.AddCommand(configCmd)`). The file is 2834 lines — do NOT add new command code inline here. Add exactly ONE registration line and put the new command builder in a new `pkg/cli/config.go`.
- `pkg/cli/cli.go` `createConfigListCommand` and `createConfigCurrentUserCommand` — the two existing `config` verbs, and the shape your new builder mirrors.
- `pkg/cli/output.go` — `OutputFormat`, `IsJSON()`, `PrintJSON(v any) error`.
- `pkg/cli/cli_suite_test.go` — the `cli_test` suite entry point. **One `func TestSuite`; do not add a second `func Test*` calling `RunSpecs`.**
- `pkg/cli/watch_test.go` — the existing in-process `pkg/cli` test style: `cli.Run(ctx, []string{"--config", configPath, ...})` against a temp config, and `os.CreateTemp` config fixtures.
- `integration/cli_test.go` — read the `DescribeTable("exits 0 for --help", ...)` and its `Entry(...)` rows, in particular the `// Config subcommands` group (`Entry("config list", "config", "list")`, `Entry("config current-user", "config", "current-user")`). The table appends `--help` to each row's args and expects exit 0.
- `docs/baseline-file.md` — the baseline contract this feature implements. Its `## Where it lives` section states the rules your validator enforces: vault-relative, absolute-or-escaping rejected, recorded on the vault's config entry under `baseline`, one baseline per vault, set by `vault-cli config set-baseline <vault> <path>`.
- `specs/in-progress/054-rollup-weekly-baseline-delta.md` — the spec. Read Desired Behavior 1, the Constraints, the Failure Modes table and the Security section.
- `README.md` § Usage — the `### config` fenced `bash` block with its inline `#` comments.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — public interface + private struct + `New*` constructor; `github.com/bborbe/errors` wrapping.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)` / `errors.Wrapf(ctx, err, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err` for a newly constructed error.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-security-linting.md` — gosec rules: file permissions `0600`, `#nosec` with a reason, fix on the first attempt.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits (`funlen` 80 lines / 50 statements, `gocognit` 20, `nestif` 4, `maintidx` 20) and license headers.

Two environment facts:

1. **Make no git calls anywhere, including in `<verification>`.** The daemon does not check `<verification>` exit codes, so a git command that dies (`fatal: not a git repository`) reports a false pass. Every check below is git-free.
2. **`make generate` deletes and regenerates the whole `mocks/` tree via `go generate ./...`.** Your new `//counterfeiter:generate` annotation is picked up automatically — never hand-write a mock file.
</context>

<requirements>

## 0. Scope — exactly eight files you write, plus one generated mock

- `pkg/config/config.go` — modify: one new field, one extracted helper.
- `pkg/config/baseline.go` — new: the writer, the validator and the atomic write.
- `pkg/config/baseline_test.go` — new: the `config_test` suite for both.
- `pkg/cli/config.go` — new: the `config set-baseline` command builder.
- `pkg/cli/config_test.go` — new: the `cli_test` suite for the verb.
- `pkg/cli/cli.go` — modify: one registration line in `NewRootCommand`.
- `integration/cli_test.go` — modify: one `Entry` row.
- `README.md` — modify: one line in the `### config` block.
- `mocks/baseline-writer.go` — generated by `make generate`; do not write it by hand.

Nothing else. Do NOT touch `pkg/ops/`, `pkg/storage/`, `pkg/domain/`, `pkg/cli/rollup.go`, `integration/rollup_baseline_test.go`, or `CHANGELOG.md` — prompts 2 and 3 own those, and the changelog bullet is prompt 3's alone. Do NOT add a scenario file (`ls scenarios/*.md | wc -l` must still print `6`).

## 1. `pkg/config/config.go` — the `baseline` key

Add exactly one field to `type Vault struct`, as the LAST field, after `Excludes`, matching the dual-tag convention of its siblings:

```go
	// Baseline is the vault-relative path of the vault's hand-authored baseline
	// file. Empty means the vault has no baseline: `rollup weekly` then prints
	// exactly what it printed before the baseline feature existed.
	Baseline string `yaml:"baseline,omitempty" json:"baseline,omitempty"`
```

Do not reorder, retag or remove any existing field. `Baseline` is additive: an existing `config.yaml` with no `baseline` key parses and behaves exactly as before.

Also extract the config-path resolution out of `func (c *configLoader) Load` into a package-level helper so the writer resolves the path by the same rule. Replace the first block of `Load` (the `configPath := c.configPath` / `if configPath == ""` / `FindConfigDir` / `filepath.Join(dir, "config.yaml")` sequence) with a call to it, and add:

```go
// resolveConfigPath returns the config file path: the explicit configPath when
// given, otherwise <FindConfigDir(ctx, "vault-cli")>/config.yaml. It never
// creates a directory and never writes.
func resolveConfigPath(ctx context.Context, configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}
	dir, err := FindConfigDir(ctx, "vault-cli")
	if err != nil {
		return "", errors.Wrap(ctx, err, "find config dir")
	}
	return filepath.Join(dir, "config.yaml"), nil
}
```

`Load` must keep its current observable behaviour byte for byte — the same resolution order, the same `getDefaultConfig` for a missing file, the same lowercasing. This is a pure refactor; do not change any other line of `Load`.

## 2. `pkg/config/baseline.go` — the validator, the writer and the atomic write

New file, `package config`, BSD license header matching its siblings (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` — use the current year as `pkg/config/baseline_test.go`'s siblings do; `make addlicense` normalises it).

### 2a. The shared path validator

```go
// ValidateBaselinePath validates a vault-relative baseline path. It rejects an
// empty path, an absolute path, and any path whose cleaned form escapes the vault
// root. The same rule guards the write (`config set-baseline`) and the read
// (`rollup weekly`): a path accepted here is a path both sides can resolve
// against the vault root, and a path rejected here is rejected by both.
func ValidateBaselinePath(ctx context.Context, baselinePath string) error
```

Behaviour, in this order, each returning immediately:

1. `baselinePath == ""` → `errors.Errorf(ctx, "baseline path is empty")`.
2. `filepath.IsAbs(baselinePath)` → an error naming the rejected path verbatim, e.g. `errors.Errorf(ctx, "baseline path must be vault-relative, got absolute path %s", baselinePath)`.
3. `filepath.Clean(baselinePath)` is `".."` or begins with `".." + string(filepath.Separator)` → an error naming the rejected path verbatim, e.g. `errors.Errorf(ctx, "baseline path escapes the vault root: %s", baselinePath)`.
4. Otherwise `nil`.

Accepted: `baseline.md`, `notes/baseline.md`, `./notes/baseline.md`, `notes/../baseline.md`, `60 Baseline/2026-09-12.md`. Rejected: `""`, `/etc/passwd`, `../../outside.md`, `a/../../b.md`, `..`.

The error message MUST contain the rejected path as the caller passed it — the acceptance criteria grep stderr for it.

### 2b. The writer

```go
//counterfeiter:generate -o ../../mocks/baseline-writer.go --fake-name BaselineWriter . BaselineWriter

// BaselineWriter persists a vault's baseline file path into the vault-cli config
// file. It is the only writer of the config file; `rollup weekly` never writes.
type BaselineWriter interface {
	// SetBaseline writes baselinePath onto the named vault's config entry under the
	// key "baseline". The config file is read and validated before anything is
	// written: a missing or unparseable config file, a vault the config does not
	// name, or a rejected path leaves the file exactly as it was.
	SetBaseline(ctx context.Context, vaultName string, baselinePath string) error
}

// NewBaselineWriter creates a BaselineWriter for the config file at configPath.
// An empty configPath resolves the config file by the loader's own rule.
func NewBaselineWriter(configPath string) BaselineWriter {
	return &baselineWriter{configPath: configPath}
}

type baselineWriter struct {
	configPath string
}
```

`SetBaseline`, in this order — read-before-write, and no write on any failure:

1. `if err := ValidateBaselinePath(ctx, baselinePath); err != nil { return err }` — validate BEFORE touching the file.
2. `configPath, err := resolveConfigPath(ctx, b.configPath)`; wrap a failure with `errors.Wrap(ctx, err, "resolve config path")`.
3. `data, err := os.ReadFile(configPath)` with `//#nosec G304 -- user-controlled config path` on that line, mirroring `pkg/config/config.go` line 242. On error return `errors.Wrapf(ctx, err, "read config file %s", configPath)`. **A missing config file is an error here, not a default config** — this verb requires a config file to write into, so `os.ReadFile`'s failure is returned as-is and nothing is created.
<!-- OPEN QUESTION: the spec's Failure Modes table does not pin what `config set-baseline` does when the config file does not exist. The loader's own `Load` returns a default config in that case, so the alternative reading is "treat a missing file as the default config and create one". This prompt commits to the error, because it makes "the config file is unchanged on every failure" trivially true and never silently creates a config the operator did not author. A reviewer who prefers the create-on-missing reading should say so at audit time. -->
4. `var cfg Config; if err := yaml.Unmarshal(data, &cfg); err != nil { return errors.Wrapf(ctx, err, "parse config file %s", configPath) }` — a malformed config exits non-zero and nothing is written.
5. Find the vault by name, **case-insensitively, preserving the file's own key spelling**:
   ```go
   var matchedKey string
   for key := range cfg.Vaults {
       if strings.EqualFold(key, vaultName) {
           matchedKey = key
           break
       }
   }
   if matchedKey == "" {
       return errors.Errorf(ctx, "vault not found in config: %s", vaultName)
   }
   ```
   A nil or absent `Vaults` map makes the loop body never run and the lookup miss — that is the correct outcome, not a panic. Do not add a nil check.
6. `vault := cfg.Vaults[matchedKey]; vault.Baseline = baselinePath; cfg.Vaults[matchedKey] = vault` — write the path **verbatim**, exactly as the caller passed it. Do not clean, absolutise or re-case it.
7. `out, err := yaml.Marshal(&cfg)`; wrap a failure with `errors.Wrap(ctx, err, "marshal config")`.
8. `return writeFileAtomic(ctx, configPath, out)`.

The write re-serialises the parsed `Config` struct, so the file's key order becomes the struct's field order and any YAML comment or key the `Config`/`Vault` structs do not model is not preserved. That is the accepted behaviour for this verb: the config file is the tool's own, and the spec's failure-mode requirement is atomicity, not byte-preservation of the old file. Do not attempt a surgical text edit, and do not add a third parse type.

### 2c. The atomic write

```go
// writeFileAtomic writes data to path atomically: it writes a temp file in path's
// own directory and renames it over path, so a reader observes either the previous
// content or the new content and never a truncated or partially merged file. On
// any failure path is untouched and the temp file is removed.
func writeFileAtomic(ctx context.Context, path string, data []byte) error
```

- `dir := filepath.Dir(path)`; `tmp, err := os.CreateTemp(dir, ".config-*.yaml")` — same directory, so the rename stays within one filesystem. `os.CreateTemp` creates the file `0600`, which is the permission the config file needs.
- `if _, err := tmp.Write(data); err != nil` → close, `os.Remove(tmp.Name())`, return `errors.Wrapf(ctx, err, "write temp file %s", tmp.Name())`.
- `if err := tmp.Close(); err != nil` → `os.Remove(tmp.Name())`, return `errors.Wrapf(ctx, err, "close temp file %s", tmp.Name())`.
- `if err := os.Rename(tmp.Name(), path); err != nil` → `os.Remove(tmp.Name())`, return `errors.Wrapf(ctx, err, "rename temp file to %s", path)`.
- Otherwise `nil`.

Do NOT use `os.WriteFile` anywhere in this file: it truncates in place, which is exactly the partial-file state the spec's failure-mode table forbids. Run `make lint` and, if gosec flags a new call, add the minimal `//#nosec G304 -- user-controlled config path` (or the matching rule) with a reason, mirroring the existing precedent.

## 3. `pkg/cli/config.go` — the verb

New file, `package cli`, BSD license header. One exported-from-package builder, named because it is a grep target:

```go
// createConfigSetBaselineCommand returns the "config set-baseline" leaf command.
// It writes the vault-relative path of a vault's baseline file onto the vault's
// config entry, so `rollup weekly` can find the file the vault's figures are
// compared against.
func createConfigSetBaselineCommand(
	ctx context.Context,
	configPath *string,
) *cobra.Command
```

- `Use: "set-baseline <vault> <path>"`, `Short: "Set a vault's baseline file path"`, `Args: cobra.ExactArgs(2)`.
- `RunE`:
  ```go
  writer := config.NewBaselineWriter(*configPath)
  if err := writer.SetBaseline(ctx, args[0], args[1]); err != nil {
      return errors.Wrap(ctx, err, "set baseline")
  }
  return nil
  ```
- `configPath *string` is the same late-bound local the loader is built from in `PersistentPreRunE` — the `--config` persistent flag is parsed by cobra at Execute time, so the value is only correct inside `RunE`. Do not read the flag yourself and do not add a new flag; `--config`, `--vault`, `--output` and `--verbose` are inherited persistent flags.
- **Print nothing on success.** Exit 0 is the signal; the acceptance criteria read the value back with `config list --output json`. Do not print JSON, do not print a confirmation line, and do not add an `--output`-dependent branch — a write verb has no output shape, and inventing one is scope this spec did not ask for.
- Add no other verb to `pkg/cli/config.go`.

## 4. `pkg/cli/cli.go` — one registration line

In `NewRootCommand`, immediately after `configCmd.AddCommand(createConfigCurrentUserCommand(ctx, &configLoader))`, add:

```go
	configCmd.AddCommand(createConfigSetBaselineCommand(ctx, &configPath))
```

One line. Do not touch anything else in the file, and do not move the existing `config` commands into `pkg/cli/config.go` — that refactor is out of scope.

## 5. `pkg/config/baseline_test.go` — the config-side suite

New file, `package config_test`, Ginkgo v2 + Gomega. **This package has one `func TestSuite` in `pkg/config/config_suite_test.go`; do not add a second `func Test*` calling `RunSpecs`.** Use `os.MkdirTemp` for a temp dir and write `config.yaml` into it with `os.WriteFile`, exactly as `pkg/config/config_test.go` does.

Write a local helper that returns the config file's bytes, so the "unchanged on failure" assertions are real byte comparisons:

```go
readBytes := func(path string) []byte {
	b, err := os.ReadFile(path)
	Expect(err).To(BeNil())
	return b
}
```

Use these spec names verbatim; they are grep targets in `<verification>`:

`Describe("ValidateBaselinePath", …)`
1. `"accepts a vault-relative path"` — a `DescribeTable` or loop over `baseline.md`, `notes/baseline.md`, `./notes/baseline.md`, `notes/../baseline.md`; each `ValidateBaselinePath(ctx, value)` returns no error.
2. `"rejects an empty path"` — `ValidateBaselinePath(ctx, "")` errors.
3. `"rejects an absolute path and names it"` — `ValidateBaselinePath(ctx, "/etc/passwd")` errors and the message contains `/etc/passwd`.
4. `"rejects a path escaping the vault root and names it"` — each of `../../outside.md`, `a/../../b.md`, `..` errors and the message contains the rejected value.

`Describe("BaselineWriter", …)`
5. `"writes the baseline key onto the named vault"` — a config with a `personal` vault carrying `tasks_dir: "25 Tasks"`; `SetBaseline(ctx, "personal", "60 Baseline/2026-09-12.md")` succeeds; `config.NewLoader(configPath).GetVault(ctx, "personal")` then reports `vault.Baseline` equal to that exact string. Reading the value back through the loader is the point of this spec — it proves the key round-trips through the loader's own parse.
6. `"preserves the vault's other keys"` — after the write, the loader's `GetVault(ctx, "personal")` still reports the original `tasks_dir` and `path`, and the file still parses as a `config.Config`.
7. `"matches the vault name case-insensitively and preserves the file's key spelling"` — a config whose key is `Personal:`; `SetBaseline(ctx, "personal", "b.md")` succeeds; the file's raw bytes still contain `Personal:` and not `personal:`.
8. `"rejects an absolute path and leaves the file unchanged"` — `SetBaseline(ctx, "personal", "/etc/passwd")` errors, the message contains `/etc/passwd`, and `readBytes(configPath)` is byte-equal before and after.
9. `"rejects a path escaping the vault root and leaves the file unchanged"` — `SetBaseline(ctx, "personal", "../../outside.md")` errors, the message contains `../../outside.md`, and the bytes are unchanged.
10. `"rejects a vault absent from the config and leaves the file unchanged"` — `SetBaseline(ctx, "nope", "b.md")` errors naming `nope`, and the bytes are unchanged.
11. `"rejects a malformed config and leaves the file unchanged"` — write `vaults: [` (unparseable YAML) into the config path; `SetBaseline(ctx, "personal", "b.md")` errors and the bytes are unchanged.
12. `"leaves no temp file behind on a successful write"` — after a successful `SetBaseline`, `os.ReadDir(tempDir)` returns exactly one entry, the config file itself. A `os.WriteFile` implementation passes this trivially; an implementation that leaks its temp file does not.
13. `"writes the path verbatim without normalising it"` — `SetBaseline(ctx, "personal", "./notes/baseline.md")` succeeds and the loader reports `vault.Baseline` as exactly `./notes/baseline.md`.

## 6. `pkg/cli/config_test.go` — the verb's in-process suite

New file, `package cli_test`, Ginkgo v2 + Gomega, driving the real command through `cli.Run(ctx, args)` against a temp config, in the style `pkg/cli/watch_test.go` already uses. **One `func TestSuite` in `pkg/cli/cli_suite_test.go`; do not add a second `func Test*` calling `RunSpecs`.**

Fixture: `os.MkdirTemp` + a `config.yaml` naming a `test` vault with `path`, `tasks_dir: Tasks`.

Use these spec names verbatim:

1. `"persists the baseline key for a named vault"` — `cli.Run(ctx, []string{"--config", configPath, "config", "set-baseline", "test", "60 Baseline.md"})` returns no error, and `config.NewLoader(configPath).GetVault(ctx, "test")` reports `vault.Baseline == "60 Baseline.md"`.
2. `"returns an error for a vault absent from the config and leaves the file unchanged"` — the run returns an error and the config bytes are unchanged.
3. `"returns an error for an absolute path and leaves the file unchanged"` — `set-baseline test /etc/passwd` returns an error containing `/etc/passwd`, and the config bytes are unchanged.
4. `"returns an error for a path escaping the vault root and leaves the file unchanged"` — `set-baseline test ../../outside.md` returns an error containing `../../outside.md`, and the config bytes are unchanged.
5. `"requires a vault name and a path"` — `cli.Run(ctx, []string{"--config", configPath, "config", "set-baseline", "test"})` returns an error (cobra's `ExactArgs(2)`).

## 7. `integration/cli_test.go` — one table row

In the existing `DescribeTable("exits 0 for --help", …)`, inside the `// Config subcommands` group, add one row after `Entry("config current-user", "config", "current-user")`:

```go
			Entry("config set-baseline", "config", "set-baseline"),
```

One line. Do not touch any other row, helper or `Describe` in this file. `docs/dod.md` requires the registration table to carry every new subcommand.

## 8. `README.md` — one line in `### config`

In the `### config` fenced `bash` block, add one line beneath `vault-cli config current-user  # Print the current user`, matching the block's comment-aligned style:

```bash
vault-cli config set-baseline personal "60 Baseline.md"  # Point a vault at its baseline file
```

Follow the block with one short sentence stating that the path is vault-relative (an absolute path or one escaping the vault root is refused), that the key is written to the vault's config entry, and that `docs/baseline-file.md` documents the file's frontmatter contract. Do not document a `config unset-baseline` verb — the spec's Non-goals forbid one. Do not reword or reorder any other line in the file.

## 9. Failure modes and security — what each row carries

Map the spec's Failure Modes table onto this change and state the mapping in your completion report:

- **The config file is malformed or unparseable when `config set-baseline` runs** → step 4 returns the parse error wrapped with the path; nothing is written. Covered by `pkg/config/baseline_test.go` spec 11.
- **The config write fails partway (disk full, permission denied, interrupted)** → `writeFileAtomic` renames only after a complete write and close, so the file holds its previous content or the new content and never a truncated or merged file; the temp file is removed on every failure path. Covered by spec 12 (no temp file left behind) plus the atomic-write structure itself.
- **`config set-baseline` names a vault absent from the config** → step 5 errors naming the vault; the file is untouched. Covered by specs 10 and 6-2.
- **`config set-baseline` given an absolute path or one escaping the vault root** → `ValidateBaselinePath` rejects it before any file is read; the message names the rejected path; the file is untouched. Covered by specs 8, 9, 6-3, 6-4.
- **Two `config set-baseline` calls race on the config file** → the temp-file-plus-rename write means one rename wins and the file is never interleaved or corrupt; the loser's rename either lands wholly or fails. Nothing extra to build; do not add locking.
- **A missing config file** → `os.ReadFile` fails and the error names the resolved path; nothing is created. This is the deliberate reading of the spec's "the verb reads the config file before it writes anything".
- **Clock skew / timezone** → no effect; this prompt reads no clock and writes no date.

Security: the untrusted input is the config-supplied path and the config file's content. Validate the path before any read or write; never build a shell command, never follow the path, never `EvalSymlinks` or otherwise touch the filesystem outside the config file itself. The baseline file is not opened by this prompt at all. No network, no credentials, no retry loop.

## 10. Self-check before finishing

- Re-read the changed hunks and confirm: `pkg/config/baseline.go` contains no `os.WriteFile`; `SetBaseline` validates before it reads; the vault lookup is case-insensitive and preserves the file's key spelling; `pkg/cli/cli.go` gained exactly one line; `pkg/cli/rollup.go`, `pkg/ops/`, `pkg/storage/` and `CHANGELOG.md` are untouched.
- Walk `docs/dod.md`: every exported identifier has a doc comment, errors use `github.com/bborbe/errors` with context wrapping, tests are Ginkgo v2 / Gomega in external `_test` packages, the README names the new verb, the integration registration table gained its row.
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 054 — the `baseline` key is additive.** An existing `config.yaml` with no `baseline` key parses and behaves exactly as before, and no existing per-vault key changes meaning.
- **Copied from spec 054 — the rollup never writes.** No vault file, no baseline file, no config file is written by `rollup weekly`. Only `config set-baseline` writes, and it writes only the config file, atomically.
- **Copied from spec 054 — the path is validated before use.** An absolute path, or one whose cleaned form escapes the vault root, is rejected with a non-zero exit and no write, and the rejection names the path. The same rule guards the read side.
- **Copied from spec 054 — one baseline per vault.** Do NOT add `config unset-baseline`, an `--unset` flag, a `--clear` flag, or any second baseline key. Removing a baseline is a config-file edit; the spec's Non-goals forbid an unset verb explicitly.
- **Copied from spec 054 — no new knob beyond the one key.** No environment variable, no default baseline path, no per-vault toggle, no validation opt-out. `baseline` and `set-baseline` are the whole surface.
- **Copied from spec 054 — no scenario file.** The spec's Non-goals rule one out (the four-condition scenario test fails on its first condition, matching spec 049's decision). `ls scenarios/*.md | wc -l` must still print `6`.
- **Copied from spec 054 — Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`.
- **Do NOT write the changelog.** `CHANGELOG.md` is prompt 3's alone, so the bullet describes the shipped feature once rather than three partial increments. Do not bump any version string and do not create a tag: `.maintainer.yaml` sets `release.autoRelease: true` and the post-merge releaser owns version bumps.
- **Do NOT add a dependency** and do NOT run `go mod vendor`; never write `-mod=vendor` in a verification command. `gopkg.in/yaml.v3` is already a direct dependency.
- **Tests.** Ginkgo v2 + Gomega in external `_test` packages; one `func TestSuite` per package, never a second `RunSpecs` entry point. Every new Go file keeps its BSD license header.
- **Linter limits** the new files must respect: `funlen` 80 lines / 50 statements, `gocognit` 20, `nestif` 4, `maintidx` 20, `gocyclo` default. Split `SetBaseline` into small named helpers rather than growing one function past the limit, and use the comma-ok form for every type assertion.
- **Do NOT run** `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. The daemon does not check `<verification>` exit codes — read each line's exit status yourself and report `"status":"failed"` if any is non-zero. The checks below are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, so a check that silently stops matching still surfaces as a non-zero exit.

**The focused suites, with their frozen spec names.** `-args -ginkgo.v -ginkgo.no-color` is mandatory: Ginkgo v2's default reporter prints only dots on a green run, so without `-ginkgo.v` every name grep below returns zero matches against a correct suite, and with colour on Ginkgo injects ANSI escapes between the container and It texts.

```
go test ./pkg/config/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/spec054-config.log 2>&1; test "$?" = "0"
grep -F -q -- 'accepts a vault-relative path' /tmp/spec054-config.log
grep -F -q -- 'rejects an empty path' /tmp/spec054-config.log
grep -F -q -- 'rejects an absolute path and names it' /tmp/spec054-config.log
grep -F -q -- 'rejects a path escaping the vault root and names it' /tmp/spec054-config.log
grep -F -q -- 'writes the baseline key onto the named vault' /tmp/spec054-config.log
grep -F -q -- "preserves the vault's other keys" /tmp/spec054-config.log
grep -F -q -- "matches the vault name case-insensitively and preserves the file's key spelling" /tmp/spec054-config.log
grep -F -q -- 'rejects an absolute path and leaves the file unchanged' /tmp/spec054-config.log
grep -F -q -- 'rejects a path escaping the vault root and leaves the file unchanged' /tmp/spec054-config.log
grep -F -q -- 'rejects a vault absent from the config and leaves the file unchanged' /tmp/spec054-config.log
grep -F -q -- 'rejects a malformed config and leaves the file unchanged' /tmp/spec054-config.log
grep -F -q -- 'leaves no temp file behind on a successful write' /tmp/spec054-config.log
grep -F -q -- 'writes the path verbatim without normalising it' /tmp/spec054-config.log
```

```
go test ./pkg/cli/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/spec054-cli.log 2>&1; test "$?" = "0"
grep -F -q -- 'persists the baseline key for a named vault' /tmp/spec054-cli.log
grep -F -q -- 'returns an error for a vault absent from the config and leaves the file unchanged' /tmp/spec054-cli.log
grep -F -q -- 'returns an error for an absolute path and leaves the file unchanged' /tmp/spec054-cli.log
grep -F -q -- 'returns an error for a path escaping the vault root and leaves the file unchanged' /tmp/spec054-cli.log
grep -F -q -- 'requires a vault name and a path' /tmp/spec054-cli.log
```

**The config key, the verb and the atomic write exist and are wired:**

```
grep -F -q 'yaml:"baseline,omitempty"' pkg/config/config.go
grep -F -q 'json:"baseline,omitempty"' pkg/config/config.go
test "$(grep -cE '^[[:space:]]+Baseline[[:space:]]+string' pkg/config/config.go)" = "1"
grep -F -q 'func resolveConfigPath' pkg/config/config.go
grep -F -q 'func ValidateBaselinePath' pkg/config/baseline.go
grep -F -q 'func NewBaselineWriter' pkg/config/baseline.go
grep -F -q 'func writeFileAtomic' pkg/config/baseline.go
grep -F -q 'os.Rename' pkg/config/baseline.go
test "$(grep -c 'os.WriteFile' pkg/config/baseline.go)" = "0"
grep -F -q 'counterfeiter:generate' pkg/config/baseline.go
grep -F -q 'set-baseline' pkg/cli/config.go
grep -F -q 'cobra.ExactArgs(2)' pkg/cli/config.go
test "$(grep -c 'configCmd.AddCommand(createConfigSetBaselineCommand' pkg/cli/cli.go)" = "1"
test "$(grep -c 'Entry("config set-baseline", "config", "set-baseline")' integration/cli_test.go)" = "1"
grep -F -q 'config set-baseline' README.md
```

The two tag greps are deliberately separate substrings rather than one `yaml:"…" json:"…"` pattern: gofmt aligns the tag columns across the struct, so the two tags are separated by padding spaces on the real line and a single-line pattern would never match. The `Baseline` field grep tolerates that padding for the same reason. The `os.WriteFile` line is the atomicity guard: an implementation that writes the config in place passes every behaviour spec and fails this check, because truncate-in-place is exactly the partial-file state the spec's failure-mode table forbids.

**The generated mock exists (regenerate it first, never hand-write it):**

```
make generate
test -f mocks/baseline-writer.go
grep -F -q 'type BaselineWriter struct' mocks/baseline-writer.go
```

**Nothing out of scope was touched:**

```
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "6"
test "$(grep -c '^## Unreleased' CHANGELOG.md)" = "1"
test "$(grep -cE '^- feat:.*(baseline|set-baseline)' CHANGELOG.md)" = "0"
test "$(grep -c 'Baseline' pkg/cli/rollup.go)" = "0"
test "$(grep -c 'Baseline' pkg/ops/rollup_weekly.go)" = "0"
```

**The whole suite, then formatting:**

```
make test
test -z "$(gofmt -e -l pkg/config/config.go pkg/config/baseline.go pkg/config/baseline_test.go pkg/cli/config.go pkg/cli/config_test.go pkg/cli/cli.go integration/cli_test.go 2>&1)"
```

If a check fails, fix the cause and re-run only that check. Do NOT run `make precommit` in this prompt — prompt 3 lands last and owns the full gate. In your completion report, walk the spec's Desired Behavior 1 and the Failure Modes mapping from section 9.
</verification>
