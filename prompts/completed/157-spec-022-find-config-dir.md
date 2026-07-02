---
status: completed
spec: [022-xdg-config-migration]
summary: Added exported FindConfigDir function with XDG-first lookup and updated configLoader.Load to use it instead of hardcoding ~/.vault-cli/config.yaml
execution_id: vault-cli-xdg-config-exec-157-spec-022-find-config-dir
dark-factory-version: v0.191.0
created: "2026-07-02T12:00:00Z"
queued: "2026-07-02T11:54:16Z"
started: "2026-07-02T11:54:18Z"
completed: "2026-07-02T11:55:51Z"
branch: dark-factory/xdg-config-migration
---

<summary>
- vault-cli's config directory lookup now follows the XDG Base Directory spec
- New installs place config at `~/.config/vault-cli/config.yaml` (the modern standard location)
- Existing installs at `~/.vault-cli/config.yaml` keep working with no migration — files are never moved
- If both the XDG dir and the legacy dir exist, XDG wins
- The lookup is a standalone reusable exported function, so other callers can use it too
- No environment variable override and no CLI flag are added (explicit config path via `NewLoader` remains the override)
- No change to the config file format, the `Loader` interface, or the `NewLoader` signature
</summary>

<objective>
Add an exported `FindConfigDir(toolName string) (string, error)` function to `pkg/config/` that resolves a tool's config directory using XDG-first priority (`~/.config/<tool>/` over `~/.<tool>/`), and make `configLoader.Load` use it instead of hardcoding `~/.vault-cli/config.yaml`. This is additive lookup logic only — no filesystem migration.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read these files fully before changing anything:
- `pkg/config/config.go` — the config package. Focus on:
  - `func NewLoader(configPath string) Loader` (~line 164) — signature is frozen, do NOT change.
  - `type configLoader struct { configPath string }` (~line 170).
  - `func (c *configLoader) Load(ctx context.Context) (*Config, error)` (~line 175). The block to change is:
    ```go
    configPath := c.configPath
    if configPath == "" {
        homeDir, err := os.UserHomeDir()
        if err != nil {
            return nil, errors.Wrap(ctx, err, "get home directory")
        }
        configPath = filepath.Join(homeDir, ".vault-cli", "config.yaml")
    }
    ```
- `pkg/config/config_test.go` — existing tests; they all pass an explicit `configPath` to `NewLoader`, so they exercise the non-empty branch and must continue to pass unchanged.

Verified facts (already confirmed against the repo — do not re-derive, but you may re-verify):
- `github.com/bborbe/errors` is already imported in `config.go`. Signatures: `errors.Wrap(ctx context.Context, err error, message string) error` and `errors.Wrapf(ctx context.Context, err error, format string, args ...interface{}) error`.
- `os`, `path/filepath` are already imported in `config.go`.

Relevant guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — use `bborbe/errors`, never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment starts with the function name.
</context>

<requirements>
1. Add an exported function to `pkg/config/config.go` (place it near the top of the file, after the `Vault` accessor methods and before the `Loader` interface, or anywhere at package scope — but not inside another function):

   ```go
   // FindConfigDir returns the config directory for the given tool, applying
   // XDG-first priority. If ~/.config/<toolName>/ exists it is returned.
   // Otherwise, if the legacy ~/.<toolName>/ directory exists, it is returned.
   // When neither exists, the XDG path ~/.config/<toolName>/ is returned as the
   // default for new installs. FindConfigDir never creates directories and never
   // writes to the filesystem; it only checks directory existence via os.Stat.
   func FindConfigDir(ctx context.Context, toolName string) (string, error) {
       homeDir, err := os.UserHomeDir()
       if err != nil {
           return "", errors.Wrap(ctx, err, "get home directory")
       }
       xdgDir := filepath.Join(homeDir, ".config", toolName)
       if info, statErr := os.Stat(xdgDir); statErr == nil && info.IsDir() {
           return xdgDir, nil
       }
       legacyDir := filepath.Join(homeDir, "."+toolName)
       if info, statErr := os.Stat(legacyDir); statErr == nil && info.IsDir() {
           return legacyDir, nil
       }
       return xdgDir, nil
   }
   ```

   Notes:
   - The signature is `FindConfigDir(ctx context.Context, toolName string) (string, error)`. The spec's Desired Behavior #1 writes it as `FindConfigDir(toolName string) string`, but the spec's Failure Modes table requires returning an error when `os.UserHomeDir()` fails, and the codebase wraps all errors with `bborbe/errors` (which needs a `ctx`). Returning `(string, error)` with a `ctx` first parameter is the only form consistent with both the Failure Modes table and the repo's error-wrapping convention. This is the frozen signature for this prompt.
   - Use `info.IsDir()` so a stray *file* named `~/.config/vault-cli` does not get treated as the config dir.
   - Do NOT read `XDG_CONFIG_HOME` — the spec Non-goals forbid an env-var override, and Desired Behavior #2 pins the literal `~/.config/<toolName>/` path.

2. Update `configLoader.Load` in `pkg/config/config.go` to use `FindConfigDir`. Replace the empty-`configPath` block quoted in `<context>` with:

   ```go
   configPath := c.configPath
   if configPath == "" {
       dir, err := FindConfigDir(ctx, "vault-cli")
       if err != nil {
           return nil, errors.Wrap(ctx, err, "find config dir")
       }
       configPath = filepath.Join(dir, "config.yaml")
   }
   ```

   Do NOT change any other part of `Load`. The missing-file → default-config path (`os.IsNotExist`), the YAML parse, and the normalization logic stay exactly as they are.

3. Do NOT change `NewLoader`, the `Loader` interface, the `Config`/`Vault` structs, YAML tags, or any other function.

4. Add a CHANGELOG entry under `## Unreleased` in `CHANGELOG.md` (create the section immediately after implementing, before `make precommit`). Read `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` for style. Suggested entry:
   `- feat: Config loading follows XDG Base Directory spec — new FindConfigDir prefers ~/.config/vault-cli/ over legacy ~/.vault-cli/ with no forced migration`

5. Tests for `FindConfigDir` and the XDG-path `Load` integration are added in a SEPARATE follow-up prompt (prompt 2). This prompt must still leave the package compiling and all existing tests green.

<!-- OPEN QUESTION for reviewer: Desired Behavior #6 and Acceptance Criterion #7 require updating the Personal vault doc at ~/Documents/Obsidian/Personal/50 Knowledge Base/vault-cli.md. That path lives on the HOST and does NOT exist inside the YOLO container, so it cannot be edited here. It is intentionally OUT OF SCOPE for these container-executed prompts and must be done by the operator on the host after merge (the spec lists it under host/operator verification). No prompt writes to that file. -->
</requirements>

<constraints>
- The `Loader` interface must NOT change its method signatures.
- `NewLoader(configPath string) Loader` must NOT change — callers passing an explicit path continue to work.
- All existing tests in `pkg/config/config_test.go` must pass unchanged.
- `FindConfigDir` must be exported and live in `pkg/config/`.
- Config file YAML schema must NOT change.
- Do NOT move or migrate existing config files — lookup logic only, never mutate the filesystem. `FindConfigDir` uses only `os.Stat`.
- Do NOT add an environment variable override (no `XDG_CONFIG_HOME`, no `VAULT_CLI_CONFIG_DIR`).
- Do NOT add a command-line flag for config path.
- Do NOT update `docs/development-patterns.md`.
- Wrap errors with `errors.Wrapf(ctx, err, ...)` from `github.com/bborbe/errors` — never `fmt.Errorf`, never `context.Background()`.
- Do NOT commit — dark-factory handles git.
- Do NOT run `make install` or release.
</constraints>

<verification>
Run in the repo root:

```
make test
make precommit
```

Both must exit 0. Additionally confirm the function exists and the legacy hardcode is gone from `Load`:

```
grep -n 'func FindConfigDir' pkg/config/config.go        # must return a line
grep -n 'FindConfigDir(ctx, "vault-cli")' pkg/config/config.go   # must return a line
```
</verification>
