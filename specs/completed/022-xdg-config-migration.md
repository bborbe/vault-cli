---
status: completed
tags:
    - dark-factory
    - spec
approved: "2026-07-02T11:43:30Z"
generating: "2026-07-02T11:47:05Z"
prompted: "2026-07-02T11:53:47Z"
verifying: "2026-07-02T11:57:19Z"
completed: "2026-07-02T12:04:11Z"
branch: dark-factory/xdg-config-migration
---

## Summary

TL;DR — vault-cli config loading follows XDG Base Directory spec, with a smooth migration from the legacy `~/.vault-cli/` directory.

- Add an exported `FindConfigDir` function that picks the config directory using XDG-first priority with legacy fallback.
- `Load()` uses `FindConfigDir` instead of hardcoding the legacy path.
- New installs go to `~/.config/vault-cli/config.yaml`; existing `~/.vault-cli/config.yaml` continues to work.
- No config migration is forced — existing files are never moved.
- User-facing docs in the Personal vault describe `~/.config/vault-cli/config.yaml` as the primary path.

## Problem

vault-cli hardcodes its config path to `~/.vault-cli/config.yaml`, a dotfile directory in the home root. This violates the XDG Base Directory Specification (`~/.config/<app>/` is the standard location for user-level config files). Every other modern CLI tool on the system follows XDG; vault-cli is the odd one out, cluttering the home directory and forcing users to look in a non-standard location for config.

## Goal

vault-cli config loading observes XDG-first directory priority: `~/.config/vault-cli/` takes precedence over `~/.vault-cli/`. Existing installs with the legacy directory continue to work without migration. New installs land in the XDG location. The lookup logic is a standalone exported function reusable by other callers.

## Non-goals

- Do NOT move or migrate existing config files — this spec adds lookup logic only, never mutates the filesystem.
- Do NOT change the config file format (YAML schema, field names, `Loader` interface).
- Do NOT add an environment variable override (e.g., `VAULT_CLI_CONFIG_DIR`) — not requested; if a future consumer demands one, that is a separate spec.
- Do NOT add a command-line flag for config path — `NewLoader(configPath)` already accepts an explicit path; that is the override mechanism.
- Do NOT update `docs/development-patterns.md` in the repo — that is an architecture doc, not user-facing; the user-facing doc update is in the Personal vault.

## Desired Behavior

1. An exported function `FindConfigDir(toolName string) string` exists in `pkg/config/` that returns the config directory path for a tool, applying XDG-first priority.
2. When `~/.config/<toolName>/` exists, `FindConfigDir` returns the absolute path to that directory.
3. When `~/.config/<toolName>/` does NOT exist but `~/.<toolName>/` exists, `FindConfigDir` returns the absolute path to the legacy directory.
4. When neither directory exists, `FindConfigDir` returns the absolute path to `~/.config/<toolName>/` (XDG default for new installs).
5. `Load()` calls `FindConfigDir("vault-cli")` and appends `/config.yaml` when its `configPath` field is empty — no other change to `Load()`'s behavior.
6. The Personal vault page `50 Knowledge Base/vault-cli.md` in the Personal vault is updated: the config path shown in the "Locations" / config section lists `~/.config/vault-cli/config.yaml` as the primary path, with `~/.vault-cli/config.yaml` noted as legacy fallback.

## Constraints

- `Loader` interface (exported from `pkg/config/`) must NOT change its method signatures.
- `NewLoader(configPath string) Loader` constructor signature must NOT change — callers passing an explicit path continue to work.
- All existing tests in `pkg/config/config_test.go` must continue to pass unchanged.
- `FindConfigDir` must be exported (uppercase first letter, in `pkg/config/`) — its signature and package location are frozen by this spec.
- Config file YAML schema must NOT change.
- `make precommit` in the repo root must exit 0.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Both XDG dir and legacy dir exist | XDG wins (first priority rule). `FindConfigDir` returns the XDG path. Legacy dir is ignored. | Operator moves or removes the XDG config if they want the legacy path to take effect. This is a deliberate choice, not a failure. |
| `os.UserHomeDir()` fails during lookup | `FindConfigDir` returns an error. `Load()` wraps and returns the error (same pattern as today when home dir resolution fails). | Operator fixes the environment (`HOME` not set, `/etc/passwd` corrupt). Existing behavior — no new recovery path. |
| Neither directory exists, caller writes config to returned path | The returned XDG path's parent directory (`~/.config`) may not exist. This is NOT `FindConfigDir`'s concern — it returns a path, not a guarantee of writability. The caller (`Load()`) already handles missing-file (`os.IsNotExist` → default config). | Operator creates `~/.config/` directory. Same class as today: if `~/.vault-cli/` is missing, writes fail; `FindConfigDir` does not create directories. |

## Security / Abuse Cases

- No user input flows into `FindConfigDir` — its only parameter is a caller-controlled `toolName` string that is joined into a path. Since `toolName` is passed through `filepath.Join` and `os.Stat`, path traversal is not possible (Go's `filepath.Join` cleans `..` segments).
- No file writes in `FindConfigDir` — it only checks directory existence via `os.Stat`.
- No new network I/O.

## Acceptance Criteria

- [ ] `FindConfigDir` exists as an exported function in `pkg/config/` — evidence: `grep -n 'func FindConfigDir' pkg/config/*.go` returns at least one line.
- [ ] When `~/.config/vault-cli/` exists (test creates it), `FindConfigDir("vault-cli")` returns a path ending in `/.config/vault-cli` — evidence: unit test assertion passes (test asserts return value suffix, no failure).
- [ ] When only `~/.vault-cli/` exists (test creates it, XDG dir absent), `FindConfigDir("vault-cli")` returns a path ending in `/.vault-cli` — evidence: unit test assertion passes.
- [ ] When neither directory exists, `FindConfigDir("vault-cli")` returns a path ending in `/.config/vault-cli` — evidence: unit test assertion passes.
- [ ] `Load()` with an empty `configPath` resolves its config file through `FindConfigDir` — evidence: unit test creates config in an XDG-shaped temp directory with no legacy dir, calls `Load()`, and successfully reads the config.
- [ ] All existing tests in `pkg/config/config_test.go` pass unchanged — evidence: `make test` in the repo root exits 0.
- [ ] The file `~/Documents/Obsidian/Personal/50 Knowledge Base/vault-cli.md` contains `~/.config/vault-cli/config.yaml` as the primary documented config path, with `~/.vault-cli/config.yaml` present as a fallback notation — evidence: `grep -n '\.config/vault-cli/config\.yaml' ~/Documents/Obsidian/Personal/50\ Knowledge\ Base/vault-cli.md` returns at least one line, and `grep -n 'fallback\|legacy\|also supported\|if.*not.*present' ~/Documents/Obsidian/Personal/50\ Knowledge\ Base/vault-cli.md` returns at least one line (case-insensitive, OR of the four patterns).

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
```

### Operator-executable (runs on the host after PR merge)

```
grep -n '\.config/vault-cli/config\.yaml' ~/Documents/Obsidian/Personal/50\ Knowledge\ Base/vault-cli.md
```

## Do-Nothing Option

Continue hardcoding `~/.vault-cli/config.yaml`. The tool works today. Cost: every new install places config in a non-standard location; the code accumulates a divergence from XDG that will be harder to fix later (more docs, more users with legacy paths). The change is low-risk (additive lookup, no migration) and the cost of delay is cumulative.

## Verification Result

**Verified:** 2026-07-02T12:03:36Z (HEAD 405c4e8)
**Binary:** /Users/bborbe/Documents/workspaces/go/bin/dark-factory (dark-factory v0.191.0)
**Scenario:** Spec declares inline verification — 67/67 config tests pass, `make precommit` clean (0 lint, 0 vuln), Personal vault doc updated
**Evidence:**
- `grep -n 'func FindConfigDir' pkg/config/config.go` → line 160 (exported, in pkg/config/)
- `make test` → 67/67 config specs pass: XDG-first, legacy-fallback, neither-exists-default, both-exist-XDG-wins, file-not-dir-falls-through, Load-via-FindConfigDir
- `make precommit` → all packages pass, 0 golangci-lint issues, 0 vulns (osv-scanner + trivy), ready to commit
- `grep -n '\.config/vault-cli/config\.yaml' ~/Documents/Obsidian/Personal/50\ Knowledge\ Base/vault-cli.md` → line 53: XDG primary path with `~/.vault-cli/config.yaml` as legacy fallback
**Verdict:** PASS
