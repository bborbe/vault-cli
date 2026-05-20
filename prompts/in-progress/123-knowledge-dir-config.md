---
status: committing
summary: Added knowledge_dir config field to Vault struct with GetKnowledgeDir() accessor defaulting to '50 Knowledge Base'
container: vault-cli-exec-123-knowledge-dir-config
dark-factory-version: v0.164.0
created: "2026-05-20T21:28:15Z"
queued: "2026-05-20T21:28:15Z"
started: "2026-05-20T21:28:52Z"
---

<summary>
- Add a `knowledge_dir` config field to vault-cli's per-vault `Vault` struct, alongside the existing `tasks_dir`, `goals_dir`, `themes_dir`, `objectives_dir`, `vision_dir`, `daily_dir` fields.
- Expose a `GetKnowledgeDir()` accessor that returns the configured value, falling back to a documented default when empty.
- Surface the new field in the JSON output of `vault-cli config list --output json` so plugin commands (notably an upcoming `/vault-cli:reflect` port) can read the vault-correct Knowledge Base path without hardcoding.
- Existing vault configs that omit `knowledge_dir` continue to load without modification — the field is optional via `omitempty`.
- Add unit-test coverage matching the pattern used for the other `*Dir` accessors.
</summary>

<objective>
Add a `knowledge_dir` config field to `Vault` with `GetKnowledgeDir()` defaulting to `"50 Knowledge Base"`. Pure mirror of the existing `*Dir` pattern; no architectural change.
</objective>

<context>
Read `CLAUDE.md` at the repo root for project conventions (especially the dark-factory release flow, the four-version-string alignment rule, and the `errors.Wrapf` / `github.com/bborbe/errors` convention).

Read these files fully before editing:
- `pkg/config/config.go` — the file you are modifying. Pay attention to:
  - The `Vault` struct definition (around line 25) — note the order and spacing of the existing `*Dir` fields and their YAML/JSON tags.
  - The existing `GetTasksDir`, `GetGoalsDir`, `GetThemesDir`, `GetObjectivesDir`, `GetVisionDir`, `GetDailyDir` accessors (around lines 45–93) — these are the canonical pattern your new accessor MUST mirror.
- `pkg/config/config_test.go` — note how the existing accessors are tested with Ginkgo `Describe` / `It` blocks. Your new tests mirror this shape.
- `pkg/config/config_suite_test.go` — Ginkgo suite bootstrap; nothing to change.

Two operator-side vault configs already use different KB folder names; this prompt does NOT modify them (they're outside the repo). They are listed here only so future consumers understand the field's purpose:
- Personal vault uses `50 Knowledge Base/` on disk.
- Brogrammers vault uses `50 Knowledge/` on disk.
- Trading vault has no dedicated KB folder.
</context>

<requirements>

## 1. Add the field to the `Vault` struct

In `pkg/config/config.go`, inside the `Vault` struct, add a new field named `KnowledgeDir`. Place it adjacent to the other `*Dir` fields — directly after `DailyDir`. (The struct order is `TasksDir`, `GoalsDir`, `ThemesDir`, `ObjectivesDir`, `VisionDir`, `DailyDir`, then `ClaudeScript`, `SessionProjectDir`, then the `*Template` group. Insert `KnowledgeDir` between `DailyDir` and `ClaudeScript` so all `*Dir` fields stay clustered.)

The field MUST mirror the tag style used by the surrounding fields exactly:
- YAML tag: `knowledge_dir,omitempty`
- JSON tag: `knowledge_dir,omitempty`

Reference the formatting of `TasksDir`, `GoalsDir`, `ThemesDir` for column alignment of the tags.

**Tag-column alignment is NOT auto-fixed by `gofmt`** — match the column positions of the surrounding fields by hand. After the change, struct tags should line up vertically just like the existing fields.

## 2. Add the `GetKnowledgeDir()` accessor

Add a method on `*Vault`:

- Return the `KnowledgeDir` field when non-empty.
- Return the default `"50 Knowledge Base"` when `KnowledgeDir` is the empty string.

Place it adjacent to the other directory accessors (`GetTasksDir`, `GetGoalsDir`, ...) — same group, same shape, same one-line GoDoc comment style.

DO NOT use any new import. DO NOT introduce error returns (the existing accessors all return a bare `string` — match that).

If you find yourself writing more than ~6 lines of accessor body, you've drifted from the pattern — re-read `GetTasksDir` and mirror it.

## 3. Add unit tests

In `pkg/config/config_test.go`, add Ginkgo `It` blocks under the same `Describe` group that already exercises the other accessors. Use the existing `BeforeEach` / fixture conventions in the file (do NOT introduce a new test helper).

Cover exactly these four cases (mirroring the existing accessor tests one-for-one):

1. `Vault{KnowledgeDir: ""}.GetKnowledgeDir()` returns `"50 Knowledge Base"`.
2. `Vault{KnowledgeDir: "Some Folder"}.GetKnowledgeDir()` returns `"Some Folder"`.
3. A YAML config containing `knowledge_dir: "50 Knowledge"` loads into `Vault.KnowledgeDir == "50 Knowledge"` and `GetKnowledgeDir()` reflects it.
4. A YAML config that omits `knowledge_dir` produces `Vault.KnowledgeDir == ""` and `GetKnowledgeDir() == "50 Knowledge Base"`.

Each test MUST use a literal expectation matching the cases above — do not parametrize unless an existing nearby test does so.

## 4. JSON emission (no new code; verify behavior)

The `Vault` struct already serializes via the standard JSON tags. After step 1, `vault-cli config list --output json` automatically includes `"knowledge_dir": "<value>"` for any vault that sets the field, and omits it for any vault that doesn't (because of `omitempty`).

No code change is required here. Add a single unit test verifying the marshalling behaviour for both populated and empty cases:

- `Vault{KnowledgeDir: "X"}` JSON-marshals to a payload containing `"knowledge_dir":"X"`.
- `Vault{KnowledgeDir: ""}` JSON-marshals to a payload that does NOT contain the string `knowledge_dir`.

## 5. Doc references (no doc rewrite)

If `docs/development-patterns.md` already enumerates the per-vault folder fields, append `knowledge_dir` to the list in the same row format. If it does not (verify with `grep -n 'tasks_dir' docs/development-patterns.md`), skip this step — do NOT introduce a new section.

## 6. Verify

```bash
make precommit
```

`make precommit` MUST exit 0. The release-version check is a separate step run later by `/coding:commit`; do not bump versions in this prompt.

</requirements>

<constraints>
- Field is optional via `omitempty` — existing vault configs without `knowledge_dir` MUST load and behave exactly as today.
- `GetKnowledgeDir()` MUST treat the empty string as "unset" and fall back to the default.
- Default fallback is the literal string `"50 Knowledge Base"` — do not pull this from a const, do not import anything new, mirror the inline-literal style used by the other `Get*Dir` accessors.
- DO NOT auto-detect the KB folder by Glob, do not consult the filesystem, do not validate existence. This prompt is purely a config-shape change.
- DO NOT regenerate Counterfeiter mocks — the `Loader` interface is unchanged.
- `errors.Wrapf` / `errors.Errorf` from `github.com/bborbe/errors` for any new error wrapping (none expected).
- DO NOT bump any version string (`CHANGELOG.md`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`). The release commit happens in a separate step via `/coding:commit` once this and any companion work land.
- DO NOT modify any operator-side vault config (`~/.claude/mcp-*.json`, `~/.claude/settings.json`, etc.) — those are outside the repo.
- DO NOT commit — dark-factory handles git.
</constraints>

<verification>

Field present with correct tags:
```bash
grep -n 'KnowledgeDir string' pkg/config/config.go
```
Expected: exactly one line, with `yaml:"knowledge_dir,omitempty"` and `json:"knowledge_dir,omitempty"` tags.

Accessor present:
```bash
grep -n 'func (v \*Vault) GetKnowledgeDir' pkg/config/config.go
```
Expected: exactly one line.

Accessor body is small and mirrors the others:
```bash
awk '/func \(v \*Vault\) GetKnowledgeDir/,/^}/' pkg/config/config.go | wc -l
```
Expected: ≤ 8 lines (function header + 1 conditional + return + closing brace).

Default fallback present:
```bash
grep -n '"50 Knowledge Base"' pkg/config/config.go
```
Expected: ≥ 1 line — the default literal in `GetKnowledgeDir`.

Test coverage for both branches:
```bash
grep -B2 -A4 'GetKnowledgeDir' pkg/config/config_test.go
```
Expected: assertions for both `"50 Knowledge Base"` default AND a non-empty configured value (e.g. `"50 Knowledge"`).

YAML round-trip test present:
```bash
grep -n 'knowledge_dir' pkg/config/config_test.go
```
Expected: ≥ 1 line — a YAML fixture or struct tag reference inside a test.

JSON marshalling test present:
```bash
grep -n '"knowledge_dir"' pkg/config/config_test.go
```
Expected: ≥ 1 line — verifies the JSON output contains the key when set, and does not contain it when empty.

Existing tests still pass without modification — no rename, no contract change:
```bash
git diff --stat pkg/config/config.go pkg/config/config_test.go
```
Expected: insertions, near-zero deletions (only whitespace/alignment changes in `config.go` if any).

Full precommit:
```bash
make precommit
```
Expected: exit 0.

</verification>
