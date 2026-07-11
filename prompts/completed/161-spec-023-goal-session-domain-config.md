---
status: completed
spec: [023-goal-work-on]
summary: Added claude_session_id accessor trio to GoalFrontmatter and work_on_goal_command config field with /vault-cli:work-on-goal default
execution_id: vault-cli-exec-161-spec-023-goal-session-domain-config
dark-factory-version: v0.191.0
created: "2026-07-11T08:40:00Z"
queued: "2026-07-11T08:29:33Z"
started: "2026-07-11T08:32:53Z"
completed: "2026-07-11T08:36:49Z"
---

<summary>
- Goals can now carry a Claude session id in their frontmatter, just like tasks already can.
- The session id round-trips through the same generic get/set surface every other goal field uses, and unknown frontmatter fields still survive untouched.
- Projects gain a configurable "work-on-goal" command that defaults to `/vault-cli:work-on-goal`, separate from the existing task work-on command which stays unchanged.
- This is pure leaf plumbing — no behavior change yet; it unblocks the goal work-on operation that lands next.
- `make precommit` passes.
</summary>

<objective>
Add a `claude_session_id` accessor trio to the Goal domain frontmatter and a `work_on_goal_command` config field (with a `/vault-cli:work-on-goal` default), so the upcoming `goal work-on` operation has the domain and config primitives it needs. No operation or CLI changes in this prompt.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external `_test` package convention.

Read these files before implementing:
- `pkg/domain/task_frontmatter.go` — the EXACT pattern to mirror onto goals:
  - `ClaudeSessionID()` (line 99): `func (f TaskFrontmatter) ClaudeSessionID() string { return f.GetString("claude_session_id") }`
  - `SetClaudeSessionID` (line 204): `func (f *TaskFrontmatter) SetClaudeSessionID(v string) { f.Set("claude_session_id", v) }`
  - `ClearClaudeSessionID` (line 211): `func (f *TaskFrontmatter) ClearClaudeSessionID() { f.Delete("claude_session_id") }`
  - `GetField` case (line 352): `case "claude_session_id": return f.ClaudeSessionID()`
  - `SetField` case (line 450): `case "claude_session_id": f.SetClaudeSessionID(value)`
- `pkg/domain/goal_frontmatter.go` — the file to edit. It embeds `FrontmatterMap` (has `GetString`, `Set`, `Delete`, `Get`). Note the existing `GetField`/`SetField` switch statements (around lines 197 and 233) that this change extends. Goal already has `Assignee()`/`SetAssignee()` here.
- `pkg/domain/goal_frontmatter_test.go` — existing Ginkgo suite; note the "SetField / GetField - unknown field round-trip" Describe block (around line 51) — extend it, do not rewrite it.
- `pkg/config/config.go` — the `Vault` struct (lines 25-44) with column-aligned yaml+json struct tags, and the `GetWorkOnCommand()` accessor (line 114) that returns `/vault-cli:work-on-task` when `WorkOnCommand` is empty. Mirror this accessor exactly for the goal command.
- `pkg/config/vault_test.go` — the `GetWorkOnCommand` Describe block (line 125) and the `work_on_command` JSON-marshalling It blocks (lines 152-164). Mirror both for the new field.
</context>

<requirements>
1. In `pkg/domain/goal_frontmatter.go`, add three methods mirroring the task equivalents (place the reader near the other readers, the setters near the other setters):
   ```go
   // ClaudeSessionID reads "claude_session_id" key as string.
   func (f GoalFrontmatter) ClaudeSessionID() string { return f.GetString("claude_session_id") }

   // SetClaudeSessionID stores the claude_session_id in the map.
   func (f *GoalFrontmatter) SetClaudeSessionID(v string) { f.Set("claude_session_id", v) }

   // ClearClaudeSessionID removes the claude_session_id key from the map.
   func (f *GoalFrontmatter) ClearClaudeSessionID() { f.Delete("claude_session_id") }
   ```

2. In the same file's `GetField(key string) string` switch, add a case before the `default`:
   ```go
   case "claude_session_id":
       return f.ClaudeSessionID()
   ```

3. In the same file's `SetField(ctx context.Context, key, value string) error` switch, add a case before the `default`:
   ```go
   case "claude_session_id":
       f.SetClaudeSessionID(value)
   ```
   (Follow the existing no-error cases like `assignee` — set then fall through to `return nil`.)

4. In `pkg/config/config.go`, add a `WorkOnGoalCommand` field to the `Vault` struct with the same column-aligned tag style as its neighbours (place it directly after `WorkOnCommand`):
   ```go
   WorkOnGoalCommand string   `yaml:"work_on_goal_command,omitempty" json:"work_on_goal_command,omitempty"`
   ```
   Run `gofmt`/`gofumpt` so the struct-tag columns stay aligned across all fields (the linter enforces alignment).

5. In `pkg/config/config.go`, add the accessor next to `GetWorkOnCommand`:
   ```go
   // GetWorkOnGoalCommand returns the Claude slash command for starting goal work-on
   // sessions, defaulting to /vault-cli:work-on-goal if not configured.
   func (v *Vault) GetWorkOnGoalCommand() string {
       if v.WorkOnGoalCommand != "" {
           return v.WorkOnGoalCommand
       }
       return "/vault-cli:work-on-goal"
   }
   ```
   Do NOT touch `WorkOnCommand` / `GetWorkOnCommand` or its `/vault-cli:work-on-task` default.

6. In `pkg/domain/goal_frontmatter_test.go`, add specs (a new Describe or extend the round-trip block):
   - Sets `claude_session_id` via the generic `SetField(ctx, "claude_session_id", "sess-abc")` and reads it back via `GetField("claude_session_id")` == `"sess-abc"` and via the typed `ClaudeSessionID()` == `"sess-abc"`.
   - `ClearClaudeSessionID()` after a set leaves `Get("claude_session_id")` == nil.
   - An unknown field set alongside the session id still round-trips (preserves the existing unknown-field guarantee).

7. In `pkg/config/vault_test.go`, add specs mirroring the `work_on_command` ones:
   - `GetWorkOnGoalCommand` Describe: returns the custom value when `WorkOnGoalCommand` is set; returns `/vault-cli:work-on-goal` when empty.
   - JSON marshalling: includes `"work_on_goal_command":"/cmd"` when set; omits `work_on_goal_command` when empty.
   - YAML round-trip (the real production boundary — vault config loads from `.yaml` on disk, so the yaml tag must be exercised, not just json): marshal a `Vault{WorkOnGoalCommand:"/cmd"}` and assert the output contains `work_on_goal_command: /cmd`; marshal an empty one and assert `work_on_goal_command` is absent (omitempty). This closes a gap the existing `work_on_command` test also has — a wrong yaml tag would pass the JSON test and fail silently at config load.

8. Add a CHANGELOG entry under `## Unreleased` in `CHANGELOG.md` (e.g. `- add(goal): goals carry a \`claude_session_id\` frontmatter field and vaults accept a \`work_on_goal_command\` (default \`/vault-cli:work-on-goal\`)`). Do NOT bump any version strings — the release bot versions `## Unreleased` on merge.
</requirements>

<constraints>
- Error handling: `github.com/bborbe/errors` wrapping with `ctx`; never `fmt.Errorf`; never `context.Background()` in non-test `pkg/` code (the existing `goal_frontmatter.go` uses `context.Background()` only inside legacy date parsers — do not add new uses).
- Do NOT change `TaskFrontmatter`, `WorkOnCommand`, or any existing goal field behavior.
- Do NOT add a `ClearField` special-case — the existing generic `ClearField(key)` already deletes any key including `claude_session_id`.
- Keep struct-tag columns aligned (gofumpt) — the linter fails otherwise.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `make precommit` — must pass (lint + format + generate + test + version checks).
Run `go test ./pkg/domain/ ./pkg/config/` — both suites pass.
Run `grep -n "ClaudeSessionID" pkg/domain/goal_frontmatter.go` — ≥3 lines (reader + setter + clear).
Run `grep -n "GetWorkOnGoalCommand\|work_on_goal_command" pkg/config/config.go` — ≥2 lines.
</verification>
