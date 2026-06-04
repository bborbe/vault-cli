---
status: completed
spec: [016-work-on-task-move-create-gate-to-slash-command]
summary: Moved task-creation consent gate from work-on-task-assistant agent to work-on-task slash command — agent loses Skill/Task tools and emits structured not_found verdict; slash command parses the verdict, asks via AskUserQuestion, and routes to vault-cli:create-task on Yes
container: vault-cli-ask-gate-exec-137-spec-016-move-create-gate-to-slash-command
dark-factory-version: v0.175.0
created: "2026-06-04T10:31:18Z"
queued: "2026-06-04T10:46:21Z"
started: "2026-06-04T10:46:23Z"
completed: "2026-06-04T10:49:56Z"
branch: dark-factory/work-on-task-move-create-gate-to-slash-command
---

<summary>
- `vault-cli:work-on-task-assistant` agent silently auto-created Obsidian task files when a requested task was missing, bypassing its own "ASK before creating" contract (spec 014-class consent failure)
- The agent's `tools:` frontmatter currently includes `Skill` and `Task` — capability is on, prose gate is the only thing standing between an unknown title and a materialised file
- This prompt strips `Skill` and `Task` from the agent's `tools:` — architecturally the agent CAN NO LONGER create tasks, regardless of caller phrasing
- Phase 1 "Task not found" branch in the agent is rewritten to emit a structured `not_found` verdict (searched-source evidence + suggested name) and STOP — no AskUserQuestion, no Skill call
- A new `not_found` form is added to the agent's `<output_format>` section so the slash command can parse the verdict
- `commands/work-on-task.md` gets a new `## Phase 4 — Handle not_found` section that asks the user via AskUserQuestion in the main session, then on Yes invokes `Skill: vault-cli:create-task` and re-invokes the agent against the new task
- `<constraints>` and `<critical_writes>` in the agent drop the create-task references (gate moves to the slash command)
- CHANGELOG gets a `## Unreleased` entry (this is a new architectural capability, prefix `feat:`)
- No Go changes, no new tools, no MCP changes, no test rewrites — two `.md` edits plus a changelog line
</summary>

<objective>
Spec 016 is a Single Responsibility split on task creation: the agent stops being able to create Obsidian task files (its `Skill` and `Task` tools are removed), and the slash command becomes the controller for the absence case. The agent emits a structured `not_found` verdict when no task matches; the slash command parses the verdict, asks the user via `AskUserQuestion`, and on Yes routes to the existing `vault-cli:create-task` skill before re-invoking the agent.

This is a 1-prompt change because the two markdown edits (agent + slash command) are tightly coupled — the agent's new `not_found` verdict only has meaning if the slash command knows how to parse it, and the spec's ACs verify both files together.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions. The `docs/releasing-vault-cli.md` version-alignment rule is NOT in scope here — no `.claude-plugin/*.json` files change; only `agents/`, `commands/`, and `CHANGELOG.md` change.

Read these files in full before making changes:

**`/workspace/agents/work-on-task-assistant.md`** (the agent being modified) — note:
- Line 5: frontmatter `tools:` line currently lists `Skill` and `Task`; these must be removed.
- Lines 30–38: `<constraints>` section contains the `ASK: before creating a new Obsidian task file` bullet to be removed; also contains `READ-ONLY except: status frontmatter + daily-note tracking + (via Skill) task creation` — the "(via Skill) task creation" parenthetical must also be removed.
- Lines 17–28: `<critical_writes>` block currently does not list create-task as a mutation (it lists Jira assign/transition and Obsidian status set). Verify nothing needs to change there beyond what the spec AC requires; the spec AC says `<critical_writes>` no longer mentions task creation — if there is no mention, this AC is automatically satisfied.
- Lines 92–93: Phase 1 "Task not found" branch (the `AskUserQuestion → "Create new task?" — Yes invokes Skill: vault-cli:create-task; No shows manual search tips and STOPS` line) — this is the line to be replaced with the new `not_found` verdict behaviour.
- Lines 205–256: `<output_format>` section is a fenced ```markdown``` block with the existing `found` form. The new `not_found` form will be a separate fenced block (matching the existing fenced-block style) added AFTER the existing one.

**`/workspace/commands/work-on-task.md`** (the slash command being modified) — note:
- Lines 17–31: `## Process` section currently invokes the agent via `Task` and reports "Done." A new `## Phase 4 — Handle not_found` section must be added AFTER this entire `## Process` section.
- Lines 52–56: `## Notes` section — the one-sentence explanation of the SRP split ("The agent searches; the slash command asks before creating.") is appended here.

**`/workspace/commands/create-task.md`** (the existing interactive create flow that the new Phase 4 routes to) — note:
- This is the command the slash command invokes via `Skill: vault-cli:create-task`. It already has an interactive flow that asks the user for title, parent goal, priority, category, defer date, etc. Phase 4 only needs to provide the seed title — the create-task skill handles the rest.

**`/workspace/agents/task-creator.md`** (the agent behind `vault-cli:create-task`) — read the first 60 lines to understand the interactive flow and the filename conventions (Title Case, optional Jira-key prefix).

**Coding plugin** (in-container paths — the YOLO container mounts at `/home/node/.claude/plugins/marketplaces/coding/docs/`):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `feat:` prefix for new architectural capability, `## Unreleased` immediately above the highest `## vX.Y.Z` heading (currently `## v0.68.1`).

**Spec verified before writing this prompt** — the relevant spec is `/workspace/specs/in-progress/016-work-on-task-move-create-gate-to-slash-command.md` (read in full). The spec's Open Questions section defers the exact `not_found` verdict surface form to the prompt — fenced ```markdown``` block (matching the existing `<output_format>` style) is chosen here.
</context>

<requirements>

### File 1: `/workspace/agents/work-on-task-assistant.md`

1. **Strip `Skill` and `Task` from the `tools:` frontmatter line (line 5).** The line currently is:
   ```
   tools: Read, Glob, Bash, Edit, AskUserQuestion, Task, Skill, mcp__semantic-search__search_related, mcp__atlassian__getAccessibleAtlassianResources, mcp__atlassian__atlassianUserInfo, mcp__atlassian__getJiraIssue, mcp__atlassian__editJiraIssue, mcp__atlassian__getTransitionsForJiraIssue, mcp__atlassian__transitionJiraIssue, mcp__atlassian__lookupJiraAccountId
   ```
   After edit, the `tools:` value (and any wrap continuations if the line is broken) must contain NEITHER `Skill` NOR `Task`. The other tools (`Read`, `Glob`, `Bash`, `Edit`, `AskUserQuestion`, all `mcp__semantic-search__*` and `mcp__atlassian__*` entries) stay.
   - Verify with: `grep -nE '^tools:' agents/work-on-task-assistant.md` — the matched line + any wrap continuations must contain neither `Skill` nor `Task` as a whole-token match. Note that `mcp__atlassian__*` contains the substring `Task` (`...atlassian__getJiraIssue...` etc. do NOT, but verify by eye).

2. **Rewrite Phase 1 "Task not found" branch (lines 92–93).** The current text is:
   ```
   **Task not found**:
   - AskUserQuestion → "Create new task?" — Yes invokes `Skill: vault-cli:create-task`; No shows manual search tips and STOPS
   ```
   Replace with:
   ```
   **Task not found**:
   - Emit a structured `not_found` verdict in the report with the searched-source evidence (Jira: hit/miss/skipped, daily-note: hit/miss, semantic-search: top-3 misses with scores, Glob: paths tried) and a `Suggested task name:` line derived from the input argument (or, if input is a Jira ID, from the Jira issue summary returned by the Jira lookup; fall back to the raw input string if neither is available).
   - STOP — do NOT propose a fix, do NOT call AskUserQuestion, do NOT invoke `Skill: vault-cli:create-task`.
   - The `not_found` verdict is parsed by the calling slash command (`vault-cli:work-on-task`) which owns the create-gate.
   ```
   - The replacement text must contain the literal string `not_found`.
   - The replacement text must NOT contain `AskUserQuestion` (the agent no longer owns the consent gate).

3. **Add a `not_found` form to `<output_format>` (after line 256, before the closing of the section).** The existing `<output_format>` is a fenced ```markdown``` block. Append a second fenced ```markdown``` block (same fence style) labelled `not_found` with the searched-source evidence and a `Suggested task name:` line. Use this exact form:
   ````
   ```markdown
   not_found:
   📋 Task: <input> [(<jira_id>)]
   Status: not_found

   Searched:
   - Jira: <hit: summary> | <miss> | <skipped: not in input pattern>
   - Daily note ({{today}}): <hit: line> | <miss>
   - Semantic search: <top-3 misses with scores, e.g. "0.42 — <hit title>"> | <skipped: MCP unavailable>
   - Glob ({{tasks_dir}}/*{keyword}*.md): <paths tried, e.g. "24 Tasks/*foo*.md → 0 matches"> | <skipped>

   Suggested task name: <derived title — Jira summary if Jira ID input, else input string verbatim>
   ```
   ````
   - The string `not_found:` must appear in the new block.
   - The string `Suggested task name:` must appear in the new block.
   - The string `Searched:` must appear in the new block.

4. **Remove the `ASK: before creating a new Obsidian task file` bullet from `<constraints>` (line 33).** Verify with: `grep -n 'ASK: before creating' agents/work-on-task-assistant.md` — must return 0 lines after edit. The other ASK rules (Jira/Obsidian status auto-mutations are NOT asking rules — they are AUTO rules, leave them) stay.

5. **Remove the `(via Skill) task creation` parenthetical from the `READ-ONLY except:` bullet in `<constraints>` (line 35).** The current text is:
   ```
   - READ-ONLY except: status frontmatter + daily-note tracking + (via Skill) task creation
   ```
   After edit:
   ```
   - READ-ONLY except: status frontmatter + daily-note tracking
   ```

6. **Verify `<critical_writes>` (lines 17–28) does NOT mention `Skill: vault-cli:create-task` or any task-creation mutation.** If the section already doesn't mention it (verify by reading), this requirement is automatically satisfied — no edit needed. The current text lists only Jira assign/transition and Obsidian status set as mandatory mutations, so no edit should be required. Verify with: `grep -n 'Skill: vault-cli:create-task' agents/work-on-task-assistant.md` — must return 0 lines after edit (this passes if no edit is made).

### File 2: `/workspace/commands/work-on-task.md`

6b. **Update the `allowed-tools` frontmatter on line 4.** Current value is `allowed-tools: Task` (scalar form). Change to list form: `allowed-tools: [Task, AskUserQuestion, Skill]` — matching `commands/create-task.md` line 4's list style. The new Phase 4 invokes `AskUserQuestion` (consent) and `Skill: vault-cli:create-task` (route to create flow); the existing Phase 2 invokes `Task` (sub-agent dispatch). All three are required, or Phase 4 fails at runtime — every static AC would pass but the runtime repro would fail.
    - Verify with: `grep -nE '^allowed-tools' commands/work-on-task.md` — the value must include `Task`, `AskUserQuestion`, AND `Skill` (any order, list form).

7. **Add a new `## Phase 4 — Handle not_found` section AFTER the existing `## Process` section (after line 31, before the `## Integration` section on line 42).** The new section must be a level-2 heading (`##`) with the exact title `Phase 4 — Handle not_found`. Use the em-dash character `—` (U+2014), not a hyphen. Body of the section, as a numbered list:

   ```markdown
   ## Phase 4 — Handle not_found

   The agent (Phase 2 of this command) emits a structured `not_found` verdict when the requested task cannot be found in any source. This phase parses that verdict and asks the user before any file is created.

   1. **Parse the agent's report** for the `not_found:` marker. The agent's `<output_format>` produces a fenced markdown block with the literal `not_found:` header on its own line — match on that token.
   2. **Derive a suggested task name.** The agent's verdict includes a `Suggested task name:` line — use that value verbatim as the seed for the create step. (If the input was a Jira ID and the Jira lookup returned a summary, the agent supplies that summary; otherwise the agent supplies the input string verbatim.)
   3. **Ask the user via `AskUserQuestion`** with the `vault-cli` main-session UX channel:
      - `header`: `Create new task?`
      - `question`: `Create new Obsidian task "<suggested name>"?`
      - `options`: two entries — `Yes, create it` (description: `Run vault-cli:create-task with "<suggested name>" as the seed title, then re-invoke work-on-task-assistant`) and `No, stop here` (description: `Print manual search tips and stop — no task is created`)
   4. **On `Yes, create it`**: invoke `Skill: vault-cli:create-task "<suggested name>"` (use the same argument form as `commands/create-task.md` — pass the suggested name as a quoted argument). The create-task skill has its own interactive flow that asks for parent goal, priority, category, defer date, etc. — do not duplicate those asks.
   5. **On create success** (create-task skill returns the new task file path or reports success): re-invoke `Task tool with subagent_type: 'vault-cli:work-on-task-assistant' prompt: 'Find details and guides for: <new task title>'` — same form as the Phase 2 invocation, but with the new task title. The agent's standard Phase 2–8 prep mutations then run against the just-created task.
   6. **On `No, stop here`**: print the manual search tips and STOP. The manual search tips are:
      ```
      ❌ Task not found: "<input>"

      Searched:
      - <echo the Searched: section from the agent's verdict>
      - <echo the Glob paths tried>

      Manual search tips:
      - Check the active vault's tasks dir (`vault-cli config list` → `tasks_dir`)
      - Grep across vaults: `grep -rln "<keyword>" ~/Documents/Obsidian/`
      - Check today's daily note (`{daily_dir}/YYYY-MM-DD.md`)
      - If input looked like a Jira ID, confirm the issue exists in the Atlassian project

      No task was created.
      ```
   ```
   - Verify with: `grep -n 'Phase 4 — Handle not_found\|Phase 4 - Handle not_found' commands/work-on-task.md` — must return ≥1 line.
   - The body must contain the literal string `AskUserQuestion`, `Skill: vault-cli:create-task`, and `not_found`.
   - Use the em-dash `—` (U+2014) in the heading.

8. **Append a one-sentence `## Notes` entry explaining the architectural split.** The `## Notes` section starts at line 52. Append a new bullet (or new paragraph — match the style of the existing `## Notes` section, which uses `- ` bullets) with this exact sentence:
   ```
   - The agent searches; the slash command asks before creating.
   ```
   - Verify with: `grep -n 'The agent searches; the slash command asks before creating' commands/work-on-task.md` — must return ≥1 line.

### File 3: `/workspace/CHANGELOG.md`

9. **Add a `## Unreleased` section above the existing `## v0.68.1` heading.** Insert a new `## Unreleased` heading + bullets block IMMEDIATELY ABOVE `## v0.68.1`. The current top of the file (lines 1–9) is the frozen preamble; do NOT modify it. After the bullets, the file structure should read:
   ```markdown
   # Changelog
   ...frozen preamble (lines 1–9)...

   ## Unreleased

   - feat: Move task-creation consent gate from `vault-cli:work-on-task-assistant` to `vault-cli:work-on-task` slash command — agent loses `Skill` and `Task` tools, emits a structured `not_found` verdict when the requested task is missing; slash command parses the verdict, asks the user via `AskUserQuestion`, and on `Yes` routes to `Skill: vault-cli:create-task` before re-invoking the agent
   - feat: Add `not_found` form to `vault-cli:work-on-task-assistant` `<output_format>` so the slash command can parse the absence case (searched-source evidence + suggested task name)

   ## v0.68.1
   ...
   ```
   - Verify with: `grep -nE '^## Unreleased' CHANGELOG.md` — must return ≥1 line, and that line must be ABOVE the `## v0.68.1` heading.
   - Verify with: `grep -nE 'work-on-task|ask gate|consent gate|SRP|not_found' CHANGELOG.md` — must return ≥1 line ABOVE the next `## v` header.
   - Use the `feat:` prefix (this is a new architectural capability, minor bump per the changelog guide).
   - The frozen preamble (everything from `# Changelog` through the MAJOR/MINOR/PATCH bullet list) must NOT be modified, moved, or have anything inserted above it.

### Verification gates

10. **Run the static checks listed in `## Verification` of the spec.** Each must pass:
    - `grep -nE '^tools:' agents/work-on-task-assistant.md` — the matched line + wrap continuations must contain neither `Skill` nor `Task`.
    - `grep -n 'not_found' agents/work-on-task-assistant.md` — must return ≥1 line in the new Phase 1 text and the new `<output_format>` form.
    - `grep -n 'Phase 4' commands/work-on-task.md` — must return ≥1 line.
    - `git diff --name-only origin/master..HEAD -- commands/ agents/` — must return exactly two paths: `commands/work-on-task.md` and `agents/work-on-task-assistant.md`. (Run from the repo root. `origin/master` may not exist locally — if so, use `git diff --name-only -- commands/ agents/` against the current branch to verify only those two files changed.)
    - `git diff --name-only origin/master..HEAD -- '*.go'` — must return 0 lines. (If no Go files were touched, this is automatic.)

11. **No other `commands/*.md` or `agents/*.md` files are modified.** Verify by `git diff --name-only` (against the prior commit on this branch) — only the two files in step 7 of the spec must be in the diff.

12. **`make precommit` must exit 0.** The change touches only markdown, but `make precommit` runs the full pipeline (format, generate, test, check, addlicense). The `addlicense` target may flag the new `## Unreleased` content or the markdown edits — if it does, address per the existing addlicense pattern (the existing `CHANGELOG.md` and `.md` files in `commands/` and `agents/` are already addlicense-clean, so re-running it should be a no-op for the edits). If `make precommit` fails for an unrelated reason (preexisting test flake, vulnerability scanner noise), report it in the completion report and continue per the YOLO rules — do not rationalize a non-zero exit as success.

13. **Do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`.** The `docs/releasing-vault-cli.md` version-alignment rule is release-time only, not precommit. The plugin JSONs may lag the binary CHANGELOG entry on the feature branch; the operator bumps them at release time. Do not introduce a plugin version bump from a markdown-only change.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` — version-alignment is release-time only; plugin JSON bumps belong at release time, not on every feature branch.
- Existing tests must continue to pass (no Go code changed, so this is automatic unless the markdown edits trigger a `go test` failure, which they should not).
- No new MCP, no new tool, no new dark-factory primitive — the change is two `.md` edits plus a CHANGELOG line.
- The agent's Phases 2–8 (Jira mutations, Obsidian status set, daily-note tracking, code-task guide discovery, runbook/guide search, mutation verification) MUST NOT have their text changed. The spec lists these as "correct mutations the agent owns" and explicitly forbids text changes. The `tools:` removal is a frontmatter-only change that does NOT touch the Phase 2–8 text bodies.
- The slash command's happy path (task found, Phase 2 returns a report ending with `Ready to work on this task.`) MUST NOT change. The new `## Phase 4 — Handle not_found` section only fires when the agent's report contains the `not_found:` verdict; otherwise the existing "Done." step stands.
- `vault-cli:create-task` skill (the `commands/create-task.md` flow and `agents/task-creator.md` agent) MUST NOT change. The new Phase 4 routes to it as-is; it already has the interactive flow that asks for parent goal, priority, category, defer date, etc.
- Use the em-dash `—` (U+2014) in the section heading `## Phase 4 — Handle not_found`, not a hyphen `-`.
- The new `not_found` form in the agent's `<output_format>` is a fenced ```markdown``` block (matching the existing `<output_format>` style), not loose YAML or a bulleted list. The spec defers the exact surface form to the prompt; fenced ```markdown``` block is chosen here.
- The frozen CHANGELOG preamble (`# Changelog` → ... → MAJOR/MINOR/PATCH bullets) MUST NOT be modified, moved, or have anything inserted above or inside it. Insert `## Unreleased` immediately AFTER the last preamble line and BEFORE the existing `## v0.68.1` heading.
- CHANGELOG entry uses the `feat:` prefix (this is a new architectural capability → minor bump per the changelog guide).
- The change is fully reversible — restoring `Skill` to `tools:` is a one-line revert if the SRP split turns out to be the wrong design.
</constraints>

<verification>
Run `make precommit` — must exit 0.

Targeted greps and checks (each MUST hold after edits):

```bash
# 1. tools: line excludes Skill and Task
grep -nE '^tools:' agents/work-on-task-assistant.md
# The matched line + any wrap continuations must contain neither Skill nor Task
# Verify with: grep -c 'Skill' agents/work-on-task-assistant.md → 0 (or only in comments/code blocks, not in tools:)
# Verify with: grep -c '\bTask\b' agents/work-on-task-assistant.md → 0 occurrences in the tools: line
#   (note: \bTask\b matches the whole word "Task" but not "task" — Task may still appear in <constraints>/<output_format> as the noun "task", which is fine)

# 2. ASK: before creating removed from <constraints>
grep -n 'ASK: before creating' agents/work-on-task-assistant.md
# Must return 0 lines

# 3. Skill: vault-cli:create-task removed from agent
grep -n 'Skill: vault-cli:create-task' agents/work-on-task-assistant.md
# Must return 0 lines

# 4. not_found verdict present in agent
grep -n 'not_found' agents/work-on-task-assistant.md
# Must return ≥1 line in Phase 1 text AND ≥1 line in <output_format>

# 5. Phase 4 added to slash command
grep -n 'Phase 4 — Handle not_found\|Phase 4 - Handle not_found' commands/work-on-task.md
# Must return ≥1 line. Use the em-dash form first; the hyphen form is a fallback for editors that normalize.

# 6. Notes sentence appended
grep -n 'The agent searches; the slash command asks before creating' commands/work-on-task.md
# Must return ≥1 line

# 7. Only two files modified
git diff --name-only -- commands/ agents/
# Must return exactly: agents/work-on-task-assistant.md and commands/work-on-task.md (in any order)

# 8. No Go files modified
git diff --name-only -- '*.go'
# Must return 0 lines

# 9. CHANGELOG Unreleased section
grep -nE '^## Unreleased' CHANGELOG.md
# Must return ≥1 line ABOVE the '## v0.68.1' heading
grep -nE 'work-on-task|ask gate|consent gate|SRP|not_found' CHANGELOG.md
# Must return ≥1 line in the section above the next '## v' header

# 10. make precommit
make precommit
# Must exit 0
```

Runtime repro (manual, at PR verify time by the human reviewer — NOT in the daemon container):

**Not-found path:**
```bash
# In a Claude Code session running this worktree's vault-cli plugin
/vault-cli:work-on-task "Definitely Not A Real Vault Task XYZ-impossible-string"
# Expect: agent's transcript shows the new Phase 1 not_found verdict block;
# slash command then shows an AskUserQuestion prompt to the user with the suggested name;
# NO task file is materialised in the vault before the user answers.
# Verifier MUST observe: (a) the not_found verdict block in the agent's transcript;
# (b) the AskUserQuestion prompt in the slash command's transcript;
# (c) no `*.md` file appears in 24 Tasks/ (or whatever tasks_dir resolves to) before the user answers.
```

**Found path (regression check):**
```bash
/vault-cli:work-on-task "Try Hermes Agent harness"
# Expect: standard Phase 2–8 flow runs unchanged; report ends with
# "Ready to work on this task."; task's daily-note tracking + status: in_progress are correct.
```
</verification>
