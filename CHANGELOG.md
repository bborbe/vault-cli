# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## unreleased

- feat: `/vault-cli:plan-task` Step 5 now enforces five planning-gate checks instead of two. Adds three new non-negotiables: an e2e verify subtask for shipping-class tasks (rejects all 9 dishonest-tick phrases from `task-writing.md:122-134`); subtask-goal alignment (every `# Tasks` checkbox must map to a `# Success Criteria` outcome or be the verify subtask, else flagged as scope-creep); and a soft KISS warning when `# Tasks` has > 8 checkboxes (owner can still proceed). Step 7's phase-transition gate now requires all four hard non-negotiables to pass, not just the original two. Closes a gap where plan-task let tasks pass while missing verification subtasks (e.g. BRO-20548 closed without an e2e check).
- feat: `/vault-cli:task-status` adds an `Outcome:` line to the header — the task body's first paragraph after the frontmatter `---` separator (the canonical Summary per `task-writing.md`), truncated to ~140 chars. Sits above the volatile Status line as a contract reminder ("what's true when this is done") so the owner sees outcome + state in one glance. Omitted entirely for legacy tasks without a Summary paragraph; flat output mode is unaffected.

## v0.72.0

- feat: `/vault-cli:task-status` runs `/vault-cli:sync-progress` inline first (file is always disk-fresh before the report) and emits a grouped-checkbox status report split by `# Success Criteria` / `# Tasks` / `# Definition of Done` with verbatim `[x] / [ ] / [/]` state per item. Aggregate progress in the header, one-line `Next:` action at the bottom. Legacy flat output kept under `OUTPUT=flat` for orchestration callers. Frontmatter description now explicitly notes the sync-progress side-effect so owners aren't surprised by the file mutation.

## v0.71.0

- feat: `task-auditor` adds **Shipping Checklist** rule (criterion #11): when a task is shipping-class (signals: PR, release, deploy, plugin, slash command, etc.), require three explicit subtasks — merge, release fired (tag exists), and end-to-end verification in real environment. Flags `[x]` ticks with defer notes ("deferred to first use", "trust CI") as dishonest. Aligns with new `Shipping Checklist` section in `docs/task-writing.md`.
- feat: Add `/vault-cli:audit-graph` slash command + `graph-auditor` agent — audits Obsidian vault link-graph topology (broken wikilinks, orphan / loose cluster members, top hubs by in-degree). Two modes: full-vault (no arg) and topic-scoped via `mcp__semantic-search__search_related`. Lean v1: 3 topology checks only, no `--json`. Deferred to v2: connected components, reachability from `[[Index]]`, external bridges, semantic-vs-graph delta, bidirectional reciprocity, alias / case-insensitive link resolution.

## v0.70.1

- bump version

## v0.70.0

- feat: Rename `/vault-cli:refine-task` → `/vault-cli:plan-task`. Plan-task is phase-aware: validates Success Criteria and subtask coverage via `task-auditor`, drives a conversational fix loop, and on `phase: planning` flips the task to `phase: execution` after the auditor passes (score ≥ 8). Entry contract: on `status: next, phase: todo`, plan-task flips to `in_progress, planning` itself — no `/work-on-task` prerequisite. Hard rename; the legacy `refine-task` command is removed (low-callsite, owner-confirmed migration cost was zero).

## v0.69.0

- feat: Move task-creation consent gate from `vault-cli:work-on-task-assistant` agent to the `work-on-task` slash command — agent loses the `Skill` tool (architectural block on `Skill: vault-cli:create-task`); `Task` is retained for legitimate subagent dispatch in Phase 5 (`coding:pre-implementation-assistant`) and Phase 7 (`vault-cli:task-manager-agent`). On miss the agent emits a structured `not_found:` verdict; the slash command parses it, asks the user via `AskUserQuestion`, and on `Yes` routes to `Skill: vault-cli:create-task` before re-invoking the agent against the new task.
- feat: Add `not_found` form to `vault-cli:work-on-task-assistant` `<output_format>` so the slash command can parse the absence case (searched-source evidence + suggested task name)

## v0.68.1

- bump Go 1.26.3 → 1.26.4
- bump bborbe/* deps (collection, time, validation, run)
- bump golang.org/x deps (net, sys, text)
- bump ginkgo/v2 v2.29.0 and gomega v1.41.0
- exclude cloud.google.com/go v0.26.0

## v0.68.0

- feat: add `/vault-cli:refine-task` slash command — conversationally refines task substance (DoD, scope, subtasks, goal alignment) by invoking `task-auditor`, surfacing findings as numbered questions, applying edits, and re-auditing until score ≥ 8
- doc: cross-ref `/vault-cli:refine-task` from `work-on-task` and `create-task` so the refinement step is discoverable in the task lifecycle

## v0.67.7

- doc: sync-progress always posts Jira progress comment when ticket exists (was optional); adds 404 fallback and 1-hour dedup rule

## v0.67.6

- fix: Clear claude_session_id when completing a recurring task so the next occurrence starts with a fresh session ID

## v0.67.5

- fix: Add symlink escape protection to storage implementations (task, goal, theme, objective, vision, daily_note) - isSymlinkOutsideVault now correctly returns false for non-symlink files and true for broken symlinks

## v0.67.4

- fix: Replace all fmt.Errorf with errors.Wrapf/errors.Errorf in pkg/ops/ files to enable context-enriched error tracing

## v0.67.3

- fix: Replace all fmt.Errorf with errors.Wrapf/errors.Errorf in pkg/config/config.go to enable context-enriched error tracing

## v0.67.2

- fix: Propagate caller's context to `libtime.ParseTime` in `readDecisionFromPath` so storage operations respect context cancellation

## v0.67.1

- fix: Wrap bare `return err` with `errors.Wrapf` in walk callbacks in `pkg/storage/decision.go`, `pkg/storage/task.go`, `pkg/storage/base.go`, and `pkg/ops/lint.go` to provide context about which directory was being walked when errors occurred

## v0.67.0

- add `defer-goal` slash command wrapping `vault-cli goal defer`, mirror of `defer-task` (interactive + `--tool` JSON modes)
- restrict `defer-goal` and `defer-task` slash commands to their specific `vault-cli` subcommand via `allowed-tools`

## v0.66.13

- fix(workon): `task work-on` now exits non-zero when claude's headless session returns an actual failure (zero turns, is_error). The "claude binary missing" case still exits 0 with a warning, preserving v0.66.9 behavior. Closes spec 014 AC8 — the verifier confirmed exit 0 on the forced unknown-command repro before this fix.

## v0.66.12

- feat(workon): Use configurable `work_on_command` from vault config instead of hardcoded `/work-on-task` in `handleClaudeSession`

## v0.66.11

- fix(claude_session): Return error when `num_turns: 0` (slash command unknown, no conversation created) or `is_error: true` (claude reported an error) instead of silently returning the session_id. Error messages include the `result` field text for debugging.

## v0.66.10

- feat(config): Add configurable `work_on_command` field to vault configuration with default `/vault-cli:work-on-task`. Follows existing optional vault field pattern (e.g., `GetClaudeScript`). Allows per-vault customization of the Claude slash command used to start work-on sessions.

## v0.66.9

- fix(workon): Return error instead of silent empty session when `ClaudeSessionStarter` is nil (claude script not found in PATH). Previously `handleClaudeSession` returned `("", nil)` when starter was nil and task had no cached session ID, causing callers like task-orchestrator to receive `{"success": true, "session_id": ""}` with no diagnostic. Now returns an error that `Execute` wraps as a warning in `MutationResult.Warnings`.

## v0.66.8

- docs(sync-progress): clickable `obsidian://` links in Phase 5 report — every updated file is now grouped by category (Daily / Task / Goal / Runbook / Doc) and rendered as a clickable URL, replacing wikilinks that aren't actionable from chat. New Phase 3.5 schema captures `{path, vault, relpath, link, title, category, section}` per write so Phase 5 doesn't re-derive anything. Fixes from `/coding:audit-slash-command`: (1) `allowed-tools` scoped to `Bash(vault-cli:*)`, `Bash(grep:*)`, `Bash(command -v:*)` instead of bare `Bash`, (2) URL-encoding rule now enumerates the full unreserved-set rule plus explicit examples for em-dash, `+`, `%`, `&`, `?`, `#` (previously only listed `%20` / `%2F` while worked examples used `%E2%80%94` / `%2B` / `%25` — would have produced broken links for common task names), (3) removed misleading "UPDATED_FILES for Phase 4" claim (Phase 4 never uses it), (4) worked example `Completed:` line now has a real encoded URL instead of `obsidian://...` placeholder that violated the "never invent links" rule.

## v0.66.7

- docs(sync-progress): replace blanket "never auto-complete without confirmation" rule with a strict 4-criteria objective gate. Auto-complete fires only when ALL hold: (1) `# Success Criteria` section exists and is fully ticked, (2) zero `[ ]`/`[/]` checkboxes remain in the task file, (3) verification evidence is documented (`# Results` or `# Pull Requests` section, OR conversation cites a shipped artifact like `vX.Y.Z` / merged PR / scenario replay), (4) no unresolved blockers in conversation. Any criterion failing → AskUserQuestion fallback. Explicit "do nothing / don't ask" cases documented for incomplete tasks, blockers, and "sync"-not-"complete" intent.

## v0.66.6

- chore(release): Sync plugin manifest versions — `.claude-plugin/plugin.json` and `.claude-plugin/marketplace.json` (both `metadata.version` and `plugins[0].version`) drifted to `0.66.4` while CHANGELOG + tag advanced to `0.66.5`. Fixed by bumping all four to `0.66.6` in this release. Root cause: `make precommit` does not run `scripts/check-versions.sh` — only `make release-check` does. Follow-up candidate: wire `check-versions` into `precommit` so drift is impossible.

## v0.66.5

- fix(plugin): Add cross-vault wikilink resolution requirement to `work-on-task-assistant` Phase 6 — agent must verify `[[Wikilink]]` references via `mcp__semantic-search__search_related` (which is cross-vault by design, indexing Personal + Trading + Family + OpenClaw + workspace docs) BEFORE claiming a file is missing. Adds forbidden-phrasing list ("file doesn't appear to exist", "runbook not created yet", "only the log exists") to prevent active-vault-scoped Glob from producing false negatives. Root cause: `TRADE-4533 "Review MoneyMoney"` retest reported `[[MoneyMoney Review]]` runbook "doesn't appear to exist" when it actually lives at `~/Documents/Obsidian/Personal/65 Runbooks/MoneyMoney Review.md` — the file was reachable via the existing single semantic-search MCP, but the agent did Glob-scoped existence check in the active (Trading) vault and stopped there.

## v0.66.4

- fix(plugin): Reinforce Jira auto-assign + transition in `work-on-task-assistant` — even after the model bump to sonnet, the agent occasionally skipped the Jira mutation when input was a Jira ID (caught in `TRADE-4531`). Hardening:
  - Added `<critical_writes>` block at top of agent prompt listing the mandatory mutations explicitly.
  - **Reordered phases**: Jira auto-assign + transition is now Phase 2 (runs immediately after fetch), Obsidian work moves to Phase 3. Mutations happen before guide discovery so they cannot be forgotten mid-workflow.
  - **Added Phase 8 verification gate**: re-fetches the Jira issue after mutations and asserts `status.name == "In Progress"` and `assignee.accountId == current_user`. Retries once on failure, then reports ⚠️. Agent NEVER emits "Ready to work on this task." while Jira state is stale.
  - **Output format**: Jira mutation + verification lines are now REQUIRED (not optional) when `JIRA_MCP_AVAILABLE` and input is a Jira ID. Verification line added.
  - Success criteria updated: criterion #2 now requires post-mutation re-fetch verification; #7 mandates the verification line in the report.
- fix(plugin): Make Phase 6 (guide/runbook discovery) MANDATORY in `work-on-task-assistant` — agent must run at least one `search_related` call per task and use the verbatim task title as the primary seed. Adds three required queries (title, title+runbook, title+guide), score threshold ≥ 0.5, and explicit examples to prevent paraphrasing. Success criterion #6 now flags Phase 6 skip as FAIL. Root cause of recent miss where `TRADE-4533 "Review MoneyMoney"` returned generic Trading guides instead of the existing `MoneyMoney Review` runbook.
- fix(plugin): Bump `work-on-task-assistant` model from `haiku` to `sonnet` — haiku was short-circuiting the 8-phase workflow after Phase 1, skipping Jira auto-assign/transition (Phase 3) and guide discovery (Phase 6). Sonnet reliably executes all phases.
- chore(build): Improve `make vulncheck` — add `VULNCHECK_IGNORE` env var for whitelisting known vulnerabilities, switch output to a compact `id / module@version → fixed / summary` table instead of full govulncheck dump on failure.

## v0.66.3

- feat(plugin): Add `/vault-cli:update-task` and `/vault-cli:update-goal` slash commands — thin wrappers that delegate to the existing `vault-cli:task-manager-agent` and `vault-cli:goal-manager-agent` for in-progress checkbox updates, summary refresh, and noteworthy-progress detection. Migrated from Personal + Brogrammers vault local copies (both archived). Vault-agnostic — folders resolved from `vault-cli config list`.

## v0.66.2

- feat(plugin): Add `/vault-cli:reflect` slash command — extracts high-significance learnings from the parent Claude Code conversation and documents them in the active vault's Knowledge Base. Migrated from Personal + Brogrammers vault local copies (both archived). Inline (cannot delegate to a sub-agent because it needs the parent conversation). Graceful detection of `mcp__semantic-search__*` MCPs; falls back to `Glob` / `Grep` when absent.
- feat(config): Add `knowledge_dir` field to per-vault `Vault` config, alongside `tasks_dir` / `goals_dir` / `themes_dir` / `objectives_dir` / `vision_dir` / `daily_dir`. New `GetKnowledgeDir()` accessor with default `"50 Knowledge Base"`. Surfaced in `vault-cli config list --output json` (omitted when unset). Unblocks vault-agnostic KB-writing commands like `/vault-cli:reflect`.

## v0.66.1

- fix(plugin): Declare `mcp__atlassian__*` and `mcp__semantic-search__search_related` tools in `work-on-task-assistant` frontmatter so the agent can actually call the MCPs. Without these the agent was unable to invoke `mcp__atlassian__getJiraIssue` and fell back to direct `curl https://<host>/rest/api/3/issue/...`, which fails (no auth) and bypasses MCP credential management.
- fix(plugin): Add explicit constraint — no direct HTTP / `curl` / `gh api` fallback for Jira. If `mcp__atlassian__*` is unavailable, skip every Jira block silently.
- fix(plugin): Declare `mcp__semantic-search__search_related` on `work-on-goal-assistant` so semantic search isn't silently degraded.
- refactor(plugin): Unify Atlassian MCP namespace to the single canonical `atlassian` name across `work-on-task-assistant`, `work-on-goal-assistant`, `task-creator`, and the `next-task` / `work-on-task` / `sync-progress` slash commands. Previously the migrated agents referenced vault-specific suffixes (`atlassian-personal`, `atlassian-seibert`); now both vaults expose their Atlassian MCP under the same canonical key, so the plugin works with a single tool whitelist regardless of which Jira instance is active. Operator-side companion change (not part of this release): per-vault `~/.claude/mcp-*.json` configs each register their instance under the key `atlassian`.

## v0.66.0

- feat(plugin): Add five new slash commands — `next-steps`, `next-task`, `sync-progress`, `work-on-goal`, `work-on-task` — migrated from Personal + Brogrammers vaults to a single source of truth. Replaces per-vault divergent copies (Personal `next-task` was 562 lines, Brogrammers 170 lines, 714 diff lines).
- feat(plugin): Add two supporting sub-agents — `work-on-goal-assistant`, `work-on-task-assistant`. Both use graceful runtime detection: any `mcp__atlassian-*` namespace is supported (personal, seibert, future), Jira cloudId auto-detected via `getAccessibleAtlassianResources`, `mcp__semantic-search` optional, `gh` optional. No hardcoded hostnames, project keys, or vault paths.
- feat(plugin): Folder names read from `vault-cli config list --output json` per vault — `tasks_dir`, `goals_dir`, `themes_dir`, `objectives_dir`, `daily_dir`. Cross-vault discovery walks each entry under `~/Documents/Obsidian/`.
- feat(plugin): `work-on-goal` drops Focus-page auto-lookup — goal name is now a required argument. Vault-side wrappers can resolve their own default before invoking.
- feat(plugin): Generic `[A-Z]+-\d+` Jira regex everywhere — works for `TRADE-`, `BRO-`, or any project key.

## v0.65.2

- fix: `vault-cli task set <id> {status|phase}` accepts the legacy aliases `todo` and `in_progress` again — both are normalised to canonical (`next`, `execution`) before validation, and the canonical form is written to disk. Restores the alias acceptance documented in the rename strategy that was missing on the write path.

## v0.65.1

- test: Fix integration tests missed by spec 013 — update assertions for canonical `next` status, replace `status: next` invalid-status fixture with `status: garbage`, rewrite lint `--fix` context to assert alias-silent-acceptance
- chore: Add `-count=1` to `make test` target to prevent Go test cache from hiding integration failures when only `pkg/` source changes
- chore: Align tag with CHANGELOG — autoRelease bumped patch from prior `v0.64.2` tag instead of recognizing existing `## v0.65.0` entry, producing orphan `v0.64.3` tag. This release aligns plugin manifests + tag at `v0.65.1`.

## v0.65.0

- feat: Rename canonical task status `todo` → `next` and phase `in_progress` → `execution` to eliminate status/phase name collision. Old values (`todo`, `in_progress`) remain accepted aliases via `NormalizeTaskStatus` / `NormalizeTaskPhase` — existing vault files are untouched on disk.
- feat: Add `TaskStatusNext`, `TaskPhaseExecution`, `IsValidTaskPhase`, and `NormalizeTaskPhase` to `pkg/domain/`
- refactor: `vault-cli lint` accepts old canonical status/phase aliases silently (no longer flags `status: todo` or `phase: in_progress` as fixable issues)
- refactor: `statusFromProgress` emits `next` instead of `todo` for newly-computed default statuses

## v0.64.2

- fix: `vault-cli task work-on` advances `phase` from `todo`/missing/empty to `planning` when entering the workflow; mid-flight phases (`in_progress`, `ai_review`, `human_review`, `done`, ...) are left unchanged so resuming a task does not reset progress

## v0.64.1

- fix: Map fsnotify `Rename` op to `deleted` event in `vault-cli watch` — removes the `renamed` event type from the public API. Consumers handling `deleted` now automatically receive Obsidian trash-deletes (which use `os.Rename` internally). Breaking: any consumer expecting `event:"renamed"` will no longer receive that string.

## v0.64.0

- feat: Expose `goals` frontmatter array in `task list --output json` — enables consumers to filter tasks by goal without re-parsing the markdown source. Verbatim emission (brackets preserved); consumer strips `[[ ]]` if needed.

## v0.63.0

- feat: Add `vault-cli watch` top-level command with `--types` filter for entity kinds (task, goal, theme, objective); emit deprecation warning from `vault-cli task watch` pointing to the canonical command [spec 011 prompt 2]

## v0.62.0

- feat: Add `WatchDir` struct carrying entity `Kind` alongside directory path; extend `WatchEvent` with `type` field populated from directory→kind map lookup [spec 011 prompt 1]

## v0.61.0

- feat: Add canonical structural docs `docs/goal-writing.md` and `docs/task-writing.md` (modeled after dark-factory's `spec-writing.md` pattern)
- feat: Establish `# Non-goals` (goals) and `# Out of Scope` (tasks) as required sections — forcing function for scope-creep prevention at write-time
- feat: `goal-auditor` adds "Goal Scope Fit" smells block (8 indicators; 3+ → flag) and per-task "Task-Goal Alignment" check
- feat: `task-auditor` adds "Task Scope Fit" smells block (7 indicators) and per-goal-link "Task-Goal Alignment" check
- feat: `goal-creator` and `task-creator` scaffold the new required sections by default
- feat: `read-guides` lists the new canonical docs first, framing vault Obsidian guides as vault-specific examples
- fix: `read-guides` Glob calls used `~` paths which silently returned zero matches; replaced with `Bash(ls:*)` which correctly expands tilde
- fix: `read-guides` `allowed-tools` array literal `[Read, Glob, Bash]` replaced with comma-separated string and scoped Bash patterns
- chore: Add `color: blue` to `goal-auditor` and `color: yellow` to `task-auditor`

## v0.60.0

- feat: Unify all *_date frontmatter fields across Task, Goal, Objective, Theme, Decision to use libtime.DateOrDateTime for RFC3339 round-trip fidelity [spec 010]
- feat: Migrate Decision `reviewed_date` from plain string to *libtime.DateOrDateTime
- chore: Drop check-versions from `make precommit`; add `make release-check` for release-time gating
- docs: Update releasing-vault-cli.md for relaxed version-alignment gate (release-time only, not precommit)

## v0.59.2

- refactor: Migrate Objective and Theme `start_date` and `target_date` from `*time.Time` to `*libtime.DateOrDateTime`; update `GetField`/`SetField` to use `formatDateOrDateTime` and `setDateField`

## v0.59.1

- refactor: Migrate Goal `start_date` and `target_date` from `*time.Time` to `*libtime.DateOrDateTime`; update `GetField`/`SetField` to use `formatDateOrDateTime` and `setDateField`; remove `setDateFromString` helper

## v0.59.0

- feat: Migrate Task `completed_date`, `last_completed`/`last_completed_date` to `*libtime.DateOrDateTime`; add new `created_date` field with typed getter/setter. Dual-write window writes both `last_completed_date` (canonical) and `last_completed` (legacy) for one release cycle.

## v0.58.7

- refactor: replace local domain.DateOrDateTime with libtime.DateOrDateTime from github.com/bborbe/time@v1.27.0

## v0.58.6

- Update github.com/bborbe/time v1.25.11 → v1.27.0
- Update golang.org/x/term v0.42.0 → v0.43.0
- Update golang.org/x/sys v0.43.0 → v0.44.0
- Update github.com/bborbe/parse v1.10.11 → v1.10.12

## v0.58.5

- chore: Lock the four version strings (CHANGELOG top, `plugin.json`, `marketplace.json` metadata + plugins[0]) to a single value. Added `scripts/check-versions.sh`, wired into `make precommit` as `check-versions` target.
- docs: Added `docs/releasing-vault-cli.md` mirroring dark-factory's release-gate procedure (run all scenarios against a freshly built binary before `make install`). Updated `CLAUDE.md` with the locked-model alignment rule, the scenario gate rule, and the Plugin Release Checklist section.
- chore: Added `scenarios/helper/lib.sh` with reusable helpers (`build_binary`, `setup_example_vault`, `days_from_today`, `assert_*`, `scenario_done`) for future scripted scenario runners.
- fix: Fixed portability bugs in scenarios 002, 003, 004 — `cp -r src/ dst/` (BSD vs GNU divergence) replaced with `cp -R src/. dst/`. Fixed YAML date-quoting mismatches in scenarios 003 (`defer_date`) and 004 (`reviewed_date`); assertions now tolerate optional quotes via `grep -E`.
- fix: Corrected scenario 004 to match the binary's intentional immutable-history model — `decision ack` does NOT flip `needs_review`; it adds `reviewed: true` + `reviewed_date`. Description and assertions updated.

## v0.58.4

- fix: `vault-cli {task,goal,theme,objective} list` now returns an empty list (exit 0) when the configured pages directory does not exist, instead of erroring. All other I/O errors (permission denied, broken symlinks, ENOTDIR) still error with the original wrapped message.

## v0.58.3

- bump go 1.26.2 → 1.26.3
- update bborbe/collection, errors, time, validation deps
- update fsnotify v1.9.0 → v1.10.1

## v0.58.2

- chore(domain): move `TaskStatus` and helpers to `pkg/domain/task_status.go` (mirrors `task_phase.go`); pure refactor, no API change

## v0.58.1

- chore: Migrate to tools.env + Makefile @version pattern; remove tools.go and obsolete replace block. go.mod reduced from 452 to 49 lines

## v0.58.0

- feat: Add `/create-task` slash command and `task-creator` agent to the plugin (generic, vault-config-driven; reads `task_template`, no hardcoded paths or assignees)
- chore: Bump plugin and marketplace manifest versions to 0.58.0 (previously stuck at 0.55.3)

## v0.57.1

- Add CLAUDE.md project documentation
- Remove .idea/ IDE config from repository
- Restructure scenarios with NNN prefix and `/tmp/new-vault-cli` fresh-binary build pattern (dark-factory §2a); split task lifecycle into non-recurring + recurring scenarios

## v0.57.0

- feat: Add optional template path fields (task_template, goal_template, theme_template, objective_template, vision_template) to vault config with path resolution

## v0.56.0

- feat: make vault name lookup case-insensitive by normalizing config keys, Vault.Name, and DefaultVault to lowercase on load

## v0.55.3

- feat: add `/vault-cli:read-guides` command to load vault-cli docs + vault hierarchy writing guides (Vision/Theme/Objective/Goal/Task)
- chore: ignore `.dark-factory.log`
- chore: bump plugin manifest (`.claude-plugin/{plugin,marketplace}.json`) from 0.40.0 → 0.55.3 to match package version

## v0.55.2

- fix: add `FrontmatterMap.GetTime` helper that handles both `time.Time` (YAML-parsed) and `string` forms for date fields
- fix: route `TaskFrontmatter.DeferDate`, `PlannedDate`, `DueDate`, `LastCompleted`, `CompletedDate` through `GetTime` so YAML-native date literals no longer produce nil or corrupted `"00:00:00 +0000 UTC"` strings
- fix: route `GoalFrontmatter.DeferDate` through shared `GetTime` helper, replacing ad-hoc type assertion fallback
- refactor: extract `formatTimeAsDate` helper and simplify `formatDateOrDateTime` to delegate to it
- test: add `GetTime` unit tests covering time.Time, string, nil, empty string, wrong-type, missing-key, and unparseable-string paths
- test: add unit tests for `TaskFrontmatter` date accessors covering both YAML-native `time.Time` and string input paths
- test: add `GoalFrontmatter.DeferDate` unit tests for both input paths
- test: add integration tests asserting `task show --output json` and `task list --output json` include correct date fields from YAML-native date literals

## v0.55.1

- refactor: delete `pkg/ops/frontmatter_reflect.go` and its test — reflection-based field helpers replaced by map-based `FrontmatterMap` accessors
- refactor: remove dead `parseFrontmatter`/`serializeWithFrontmatter` methods from `pkg/storage/base.go`
- refactor: migrate `decisionStorage` to use `parseToFrontmatterMap`/`serializeMapAsFrontmatter`
- docs: update `docs/development-patterns.md` to describe map-based entity pattern with `XxxFrontmatter`, `FileMetadata`, and typed accessors

## v0.55.0

- refactor: migrate `domain.Goal`, `domain.Theme`, `domain.Objective`, `domain.Vision` from YAML-tagged structs to `XxxFrontmatter`+`FileMetadata`+`Content` embedding with typed getters/setters
- feat: add `GoalFrontmatter`, `ThemeFrontmatter`, `ObjectiveFrontmatter`, `VisionFrontmatter` typed wrappers with `GetField`/`SetField`/`ClearField` generic API preserving unknown frontmatter fields
- feat: add `Validate` method to `GoalStatus`, `ThemeStatus`, `ObjectiveStatus`, `VisionStatus` with `AvailableXxxStatuses` and `XxxStatuses` types
- refactor: update `goal.go`, `theme.go`, `objective.go`, `vision.go` storage to use `parseToFrontmatterMap`/`serializeMapAsFrontmatter`
- refactor: replace reflection-based entity list operations in `frontmatter_entity.go` with per-entity typed operations; `entityGetOperation` and `entityShowOperation` use type switch
- test: add `goal_frontmatter_test.go` covering typed getters, setters, unknown-field round-trips, date round-trips, and priority validation

## v0.54.0

- refactor: migrate `domain.Task` from YAML-tagged struct to `TaskFrontmatter`+`FileMetadata`+`Content` embedding with typed getters/setters
- feat: add `TaskFrontmatter` typed wrapper with `GetField`/`SetField`/`ClearField` generic API preserving unknown frontmatter fields through round-trips
- feat: add `Priority.Validate` method rejecting negative priorities
- refactor: replace hardcoded switch in `FrontmatterGetOperation`/`FrontmatterSetOperation`/`FrontmatterClearOperation` with `task.GetField`/`task.SetField`/`task.ClearField`
- refactor: replace reflection-based task list operations with typed `taskListOperation` using `SetGoals`/`SetTags`
- test: add `task_frontmatter_test.go` covering typed getters, setters, and unknown-field round-trips
- test: enable unknown-field preservation integration test in `cli_test.go`

## v0.53.0

- feat: add `FrontmatterMap`, `FileMetadata`, and `Content` domain types as foundation for flexible frontmatter refactor
- feat: add `parseToFrontmatterMap` and `serializeMapAsFrontmatter` methods to `baseStorage` for map-based YAML round-trips

## v0.52.2

- Update Go to 1.26.2
- Update bborbe/* deps (collection, errors, time, validation, parse)
- Update containerd, docker/cli, moby/buildkit, otel deps
- Update golang.org/x/* deps (sys, term)
- Add 60s timeout to storage test suite

## v0.52.1

- Update go-git/go-git to v5.17.1 (fix security vulnerabilities)

## v0.52.0

- feat: add path-suffix matching to `FindDecisionByName` so users can disambiguate decisions with identical short names by passing a path-containing identifier (e.g. "40 Trading/Weekly/2026-W12 - Review")

## v0.51.2

- Add GoDoc comments to TaskStatus and TaskPhase constants
- Fix go.mod dependencies

## v0.51.1

- Update dependencies (errors, time, validation, golangci-lint, osv-scanner, docker, moby, containerd, opentelemetry, etc.)
- Add .osv-scanner.toml config
- Regenerate mocks

## v0.51.0

- feat: add `task_identifier` field to `domain.Task` with UUIDv4 auto-generation in `WriteTask`, lint check `MISSING_TASK_IDENTIFIER` for tasks without a stable identity, and promote `github.com/google/uuid` to a direct dependency
- feat: add `EnsureAllTaskIdentifiersOperation` in `pkg/ops/` to backfill `task_identifier` on all tasks in a vault that are missing one, collecting modified file paths and skipping write errors non-fatally

## v0.50.0

- feat: add `GoalDeferOperation` and `vault-cli goal defer` command to set `defer_date` on goals using shared date-parsing helpers (relative days, weekday names, ISO dates, RFC3339); extract `parseDeferDate`, `isDeferDateInPast`, and `nextWeekday` as package-level helpers shared between task and goal defer operations

## v0.49.0

- feat: add `defer_date` field to `domain.Goal` struct so generic set/get/clear operations support `goal set/get/clear defer_date`; extend `frontmatter_reflect` to handle `*DateOrDateTime` pointer type

## v0.48.7

- Update bborbe/* dependencies (collection, errors, time, validation, run, math, parse)
- Update security scanner gosec v2.25.0
- Update golang.org/x/* stdlib dependencies
- Update osv-scanner v2.3.4 and related scanning tools
- Update charmbracelet UI and other indirect dependencies

## v0.48.6

- refactor: extract output formatting from LintOperation and WatchOperation so neither writes to stdout; CLI layer formats lint issues and handles exit behavior; watch CLI passes a handler callback for streaming JSON events

## v0.48.5

- refactor: extract output formatting from seven mutation operations (complete, defer, workon, update, decision-ack, goal-complete, objective-complete) so they return structured MutationResult and never write to stdout; CLI layer owns all formatting

## v0.48.4

- refactor: extract output formatting from five query operations (list, show, search, decision-list, entity-show) so they return structured results and never write to stdout; CLI layer owns all formatting

## v0.48.3

- upgrade golangci-lint from v1 to v2
- standardize Makefile: add mocks mkdir, reorder lint, use go mod tidy -e
- update .golangci.yml to v2 format
- setup dark-factory config

## v0.48.2

- fix: set phase to done when completing a non-recurring task so status and phase remain consistent

## v0.48.1

- fix: make --assignee filter case-insensitive using strings.EqualFold so localclaw, LocalClaw, and LOCALCLAW all match the same assignee

## v0.48.0

- feat: add STATUS_PHASE_MISMATCH lint check to detect inconsistent combinations of task status and phase fields (e.g. status=completed with phase=in_progress)

## v0.47.0

- feat: add optional session_project_dir vault config field so work-on can start Claude sessions in a directory different from the vault path

## v0.46.0

- feat: introduce strongly-typed TaskPhase enum with six values (todo, planning, in_progress, ai_review, human_review, done); replace free-form Phase string field with *TaskPhase, validate on set, and clear phase when completing a recurring task

## v0.45.1

- refactor: add String(), Validate(), Ptr() methods and AvailableTaskStatuses collection to TaskStatus, simplify IsValidTaskStatus and parseTaskStatus to use collection lookup

## v0.45.0

- feat: change --status flag on task list (and generic list commands) from single string to string slice, supporting repeated flags and comma-separated values (e.g. --status=in_progress --status=completed)

## v0.44.0

- feat: record completed_date on non-recurring task completion; expose completed_date in task list and task show JSON output

## v0.43.0

- feat: add ModifiedDate field to all domain types (Task, Goal, Objective, Theme, Vision) populated from file mtime; expose modified_date in task list JSON output

## v0.42.0

- feat: make ListTasks, FindTaskByName, and ReadTask discover tasks recursively in subdirectories

## v0.41.1

- fix: preserve time component in list and show JSON output for defer_date, planned_date, due_date — date-only values output as YYYY-MM-DD, datetime values output as RFC3339

## v0.41.0

- feat: extend task date fields (defer_date, planned_date, due_date) to support full RFC3339 datetime-with-timezone values alongside existing YYYY-MM-DD date-only format; defer command now accepts RFC3339 datetime strings; relative +Nd offsets preserve existing time component when present

## v0.40.2

- update go.yaml.in/yaml/v3 from v3.0.2 to v3.0.4
- cleanup go.mod exclude directives

## v0.40.1

- remove k8s.io/kube-openapi replace directive
- clean up k8s exclude blocks from go.mod

## v0.40.0

- feat: add 6 plugin agents — task-manager-agent, task-auditor, goal-manager-agent, goal-auditor, theme-auditor, objective-auditor

## v0.39.0

- feat: add 8 plugin commands — verify-task, task-status, audit-task, verify-goal, audit-goal, verify-theme, audit-theme, audit-objective

## v0.38.1

- docs: add Claude Code Plugin section to README with install instructions and command table

## v0.38.0

- feat: add Claude Code plugin commands/ directory with complete-task and defer-task

## v0.37.2

- fix: strip Obsidian wiki-link brackets `[[...]]` from name in `findFileByName` so goal lookups with bracket-wrapped names resolve correctly

## v0.37.1

- test: add integration test verifying all CLI commands and subcommands are registered via `--help` exit-0 checks

## v0.37.0

- feat: add `goal complete` command with open-task validation and --force flag
- feat: add `objective complete` command

## v0.36.0

- feat: Add GoalCompleteOperation with open-task blocking check and --force bypass, and ObjectiveCompleteOperation, both with JSON output and counterfeiter mocks

## v0.35.0

- feat: Add Completed date field to Goal and Objective domain structs; add ListTasks to TaskStorage interface and regenerate mock

## v0.34.0

- feat: Wire add/remove subcommands into task, goal, theme, objective, and vision CLI command groups using EntityListAddOperation and EntityListRemoveOperation with VaultDispatcher pattern

## v0.33.0

- feat: Add EntityListAddOperation and EntityListRemoveOperation to generic entity frontmatter ops layer, with isListField/appendToList/removeFromList reflection helpers and constructors for all five entity types (task, goal, theme, objective, vision)

## v0.32.0

- feat: Add --goal flag to task list command for filtering tasks by goal name (exact, case-sensitive match against goals frontmatter list)

## v0.31.0

- feat: Wire get/set/clear/show subcommands into goal, theme, objective, and vision CLI command groups using VaultDispatcher pattern

## v0.30.0

- feat: Add reflection-based generic frontmatter get/set/clear/show operations for goal, theme, objective, and vision entities (EntityGetOperation, EntitySetOperation, EntityClearOperation, EntityShowOperation)

## v0.29.0

- feat: Add Objective and Vision domain structs with storage layer (ReadObjective, WriteObjective, FindObjectiveByName, ReadVision, WriteVision, FindVisionByName)
- feat: Add ThemeStorage narrow interface with FindThemeByName; add ObjectiveStorage and VisionStorage narrow interfaces with counterfeiter mocks
- feat: Embed ThemeStorage, ObjectiveStorage, VisionStorage in Storage composite interface with NewThemeStorage, NewObjectiveStorage, NewVisionStorage constructors

## v0.28.0

- feat: Add `excludes` config field to vault to skip directories during vault-wide operations (e.g. `decision list`)

## v0.27.4

- fix: ReadTheme uses configured ThemesDir instead of hardcoded "Themes" path
- fix: Remove blank line between counterfeiter directives and interface declarations in show.go and watch.go

## v0.27.3

- refactor: Extract duplicated multi-vault try-each-until-success loop into VaultDispatcher in pkg/ops and replace all 9 vault loops in CLI commands with dispatcher calls

## v0.27.2

- refactor: Add ctx parameter to storage base helpers (parseFrontmatter, serializeWithFrontmatter, findFileByName) and replace fmt.Errorf with errors.Wrap/errors.Errorf throughout storage and CLI layers

## v0.27.1

- refactor: Replace fmt.Fprintf(os.Stderr) calls with log/slog structured logging; add --verbose flag to control log level (default: warn, verbose: debug)

## v0.27.0

- feat: Add due_date field to Task struct and frontmatter get/set/clear operations, list JSON output, and show JSON output

## v0.26.0

- feat: Add planned_date, recurring, last_completed, page_type, goals, and tags fields to frontmatter get/set/clear operations

## v0.25.6

- refactor: Update cli.go to construct per-domain storage instances (NewTaskStorage, NewGoalStorage, NewDailyNoteStorage, NewPageStorage, NewDecisionStorage) instead of monolithic NewStorage in all command wiring functions

## v0.25.5

- refactor: Regenerate per-domain counterfeiter mocks (TaskStorage, GoalStorage, DailyNoteStorage, PageStorage, DecisionStorage) and update all ops tests to use narrow mock types instead of monolithic Storage mock

## v0.25.4

- refactor: Update ops constructors to accept narrow per-domain storage interfaces (TaskStorage, GoalStorage, DailyNoteStorage, PageStorage, DecisionStorage) instead of monolithic Storage

## v0.25.3

- refactor: Split monolithic `pkg/storage/markdown.go` into per-domain files (task, goal, theme, daily_note, page, decision) with narrow interfaces and a shared `baseStorage` embedded struct

## v0.25.2

- fix: Resolve vaultPath through symlinks in isSymlinkOutsideVault (macOS /tmp fix)
- add: Dark-factory prompts for splitting monolithic Storage interface into per-domain structs

## v0.25.1

- docs: Rewrite README Usage section to document all commands (task, goal, theme, objective, vision, decision, search, config)

## v0.25.0

- feat: Add `vault-cli decision list` and `vault-cli decision ack` CLI commands wired into the multi-vault pattern

## v0.24.0

- feat: Add `DecisionAckOperation` that marks a decision as reviewed with today's date and optionally overrides its status field

## v0.23.0

- feat: Add `DecisionListOperation` with filter modes (unreviewed/reviewed/all), plain and JSON output, alphabetical sorting, and counterfeiter mock

## v0.22.0

- feat: Add `ListDecisions`, `FindDecisionByName`, and `WriteDecision` to `Storage` interface with recursive vault scanning, symlink path-traversal guard, ambiguous-match detection, and in-place frontmatter update

## v0.21.0

- feat: Add `Decision` domain struct with YAML frontmatter fields (`needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type`) and `DecisionID` type

## v0.20.1

- fix: Redirect warning messages from stdout to stderr in storage layer to avoid corrupting JSON output

## v0.20.0

- feat: Add `vault-cli task watch` streaming command that emits newline-delimited JSON events on stdout when task, goal, theme, or objective files change

## v0.19.0

- feat: Add `vault-cli task show <name>` command returning full task detail including content, metadata, and file modification time

## v0.18.0

- feat: Enrich task list JSON output with category, recurring, defer_date, planned_date, claude_session_id, and phase fields for external tool integration

## v0.17.1

- fix: Increase claude session timeout from 60s to 5m for longer-running tasks
- fix: Remove hardcoded `--max-turns 1` limit, allow unlimited turns by default
- feat: Add stderr progress message when starting Claude session

## v0.17.0

- feat: Add optional `claude_script` field to `Vault` config so each vault can specify a custom Claude wrapper script for sessions, defaulting to "claude"

## v0.16.0

- feat: Add Claude session management to `vault-cli task work-on` — starts or resumes a Claude coding session, with `--mode` flag (auto/interactive/headless) for TTY detection

## v0.15.0

- feat: Add `vault-cli config current-user` subcommand that prints the current user from the config file

## v0.14.0

- feat: Add `vault-cli config list` command to list configured vaults with plain and JSON output formats

## v0.13.0

- feat: Add `--version` flag to `vault-cli` reporting the installed build version (git tag or "dev")

## v0.12.0

- feat: Add `RecurringInterval` type with `ParseRecurringInterval` supporting named aliases (`quarterly`, `yearly`) and numeric shorthand (`3d`, `2w`, `2m`, `1q`, `2y`) for recurring tasks

## v0.11.1

- fix: Change `DeferDate` and `PlannedDate` in Task domain model from `*time.Time` to `*libtime.Date` so YAML serialization produces date-only values (`2026-03-08`) instead of full timestamps

## v0.11.0

- feat: Make date argument optional in `vault-cli task defer`, defaulting to `+1d` when omitted

## v0.10.8

- go mod update

## v0.10.7

- Add recurring task support to complete command (reset checkboxes, bump defer_date, keep in_progress)

## v0.10.6

- Fix frontmatter serialization: exclude Name, Content, FilePath from YAML output via `yaml:"-"` tags

## v0.10.5

- Remove root-level command aliases (complete, defer, list, lint) — use `task` subcommand instead

## v0.10.4

- Add context-aware error wrapping with github.com/bborbe/errors

## v0.10.3

- Improve test coverage for pkg/storage

## v0.10.2

- Improve test coverage for pkg/ops (complete, update operations)

## v0.10.1

- Improve test coverage for pkg/ops (lint, validate operations)

## v0.10.0

- Add `vault-cli task validate <task-name>` command for single-task linting

## v0.9.0

- Add `vault-cli task get <name> <key>` to read frontmatter field values
- Add `vault-cli task set <name> <key> <value>` to write frontmatter field values
- Add `vault-cli task clear <name> <key>` to remove frontmatter field values
- Add Phase and ClaudeSessionID fields to Task domain type

## v0.8.0

- Add `--output plain|json` flag for all commands
- Add JSON output with vault field and warnings in response body

## v0.7.0

- Add `--status` filter flag for all list commands (task, goal, theme, objective, vision)

## v0.6.1

- Improve test coverage for pkg/ops, pkg/config, pkg/domain, pkg/storage

## v0.6.0

- Add `vault-cli task work-on <task-name>` command (sets in_progress + assigns current user)
- Add `current_user` field in config

## v0.5.0

- Add `--assignee` flag for all list commands

## v0.4.0

- Fix priority parsing to handle invalid string values gracefully (use -1 instead of skipping)

## v0.3.0

- Run all commands across all configured vaults by default
- Add `--vault` flag to restrict output to a single vault

## v0.2.0

- Add lint subcommand for goal, theme, objective, and vision entity types

## v0.1.0

- Add `vault-cli list` command with `--status` and `--all` flags
- Add `vault-cli lint` command with `--fix` flag
- Detect MISSING_FRONTMATTER, INVALID_PRIORITY, DUPLICATE_KEY, INVALID_STATUS
- Auto-fix INVALID_PRIORITY and DUPLICATE_KEY issues
