# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## Unreleased

- change(lifecycle): decouple the `planning → execution` flip so each lifecycle command has one job — `plan-task` now validates and hands off without flipping phase; `execute-task` is the sole command that flips `planning → execution`; `work-on`, `work-on-task`, and `work-on-goal` stop auto-chaining `plan-task`/`execute-task` and instead print an explicit plan → execute → complete next-step signal. Fixes `execute-task` running as a silent no-op in the auto-chain. Updated `docs/task-writing.md` phase table and `README.md` accordingly

## v0.96.4

- build: add `scripts/check-changelog.sh` (wired into `make check` → `precommit`) that fails the build when a `##` section is placed above the CHANGELOG preamble — guards against the malformed-CHANGELOG class of error a changelog edit can otherwise introduce undetected

## v0.96.3

- docs(dod): specify `## Unreleased` placement in the Definition of Done — must sit below the preamble and above the newest version section, never above the preamble (prevents the malformed-CHANGELOG class of error a dark-factory prompt can otherwise introduce)

## v0.96.2

- fix(workon): append `--non-interactive` to the headless `claude --print` bootstrap prompt in `handleClaudeSession` to prevent the 5-minute hang when `work-on` command needs input that cannot be answered headlessly

## v0.96.1

- refactor(plugin): rename orchestration-mode flag `--tool` → `--non-interactive` and internal label `MODE=tool` → `MODE=non_interactive` across create-task, create-goal, complete-task, complete-goal, defer-task, defer-goal commands and task-creator, goal-creator, goal-manager-agent agents; the command/agent instructions still accept `--tool` as a deprecated alias for one release. These flags are Claude Code slash-command arguments interpreted by the agent instructions — not vault-cli binary flags.
- feat(plugin): `/vault-cli:work-on-task` accepts `--non-interactive` — still orients the task (assistant sets status, tracks the daily note, discovers guides) but skips the create-task prompt (Phase 4) and the plan-task/execute-task sharpening chain (Phase 5), so headless callers (e.g. `vault-cli work-on`'s `claude --print` bootstrap) orient and stop instead of hanging on `AskUserQuestion`

## v0.96.0

- feat: Config loading follows XDG Base Directory spec — new FindConfigDir prefers ~/.config/vault-cli/ over legacy ~/.vault-cli/ with no forced migration

## v0.95.0

- feat(domain): add `ResolveResult` domain type for name-resolution probe outcomes, serializing to `{"type":"task|goal|","name":"...","found":true|false}` JSON contract
- feat(ops): add `ResolveOperation` in `pkg/ops/resolve.go` — probes task storage first, then goal storage, returning `ResolveResult` with task-first priority; miss returns `found:false` with no error
- feat(cli): add `vault-cli resolve <name>` top-level command — resolves a name to its entity type (task or goal) using `ResolveOperation`, JSON-only output via `--output json`, plain mode silent no-op; supports `--vault` flag and multi-vault first-success dispatch
- feat(plugin): add `/vault-cli:work-on` slash command — auto-detects task vs goal from argument (Jira-ID regex, `vault-cli resolve` probe), dispatches to correct assistant; not_found flow offers create-task or create-goal

## v0.94.0

- feat(session-close): add **Phase 8.7 — detect self-improve-worthy signals**, the friction-scoring sibling of Phase 8's reflect detection. Scores general behavior corrections, repeated instructions, misfired commands/agents, documented-rule violations, and manual multi-step workflows with no command; at score ≥ 3 surfaces a suggest-only `/coding:self-improve` nudge in Phase 9 (mode 2 + outstanding list). Never auto-runs — mirrors the reflect contract. Reflect captures durable knowledge; self-improve captures tooling friction.

## v0.93.0

- feat(work-on-goal): `work-on-goal-assistant` now promotes a goal's `status` to `in_progress` when work starts (via `vault-cli goal set`), mirroring `work-on-task-assistant`. Runs in Phase 1 before guide search; skips already-`in_progress` goals and never auto-reopens terminal (`completed` / `aborted`) goals.

## v0.92.1

- fix: use `stderrors.New` instead of `errors.New(context.Background(), ...)` for `ErrStarterUnavailable` sentinel in `pkg/ops/errors.go`
- fix: replace `fmt.Errorf("%w: ...", validation.Error, ...)` with `errors.Wrapf(ctx, validation.Error, ...)` in `GoalStatus.Validate`, `ThemeStatus.Validate`, `VisionStatus.Validate`, `ObjectiveStatus.Validate`, and `TaskStatus.Validate` — enables proper stack-trace wrapping via `github.com/bborbe/errors`
- refactor: extract `expandVaultPaths` helper in `pkg/config/config.go` to deduplicate tilde expansion and template path resolution between `GetVault` and `GetAllVaults`
- refactor: introduce `domain.Page` type to eliminate type contract violation where `PageStorage.ListPages` returned `[]*domain.Task` for all entity types; `ListPages` now returns `[]*domain.Page` and `ops/list.go` uses it directly
- refactor: move all `regexp.MustCompile` calls in `lint.go` to package-level `var` declarations — eliminates per-call regex recompilation when linting many files
- refactor: thread `goalsDir` through the lint pipeline (`Execute` → `lintFile` → `collectLintIssues` → `detectOrphanGoals`) instead of hardcoding `filepath.Join(vaultPath, "Goals")`; `LintOperation.Execute` now accepts a `goalsDir` parameter
- refactor(cli): extract `runMutation` helper in `pkg/cli/cli.go` to deduplicate vault-iteration + dispatcher boilerplate across complete/defer/update mutation commands — eliminates ~100 lines of duplicated `//nolint:dupl` code

## v0.92.0

- feat(work-on-task): Phase 5's past-planning branch (`phase: ai_review` / `human_review` / `done`) now points the operator at the close-out pair (`/vault-cli:sync-progress` then `/vault-cli:session-close`) instead of just printing "no kickoff needed". Surfaces the lifecycle's natural next step when work-on-task is invoked on a task whose work is already done.

## v0.91.2

- refactor(checkbox): DRY out duplicated checkbox parser regex — promote `checkboxRegex` in `pkg/storage` to exported `CheckboxRegex`, add sibling `CheckboxCompleteRegex` and `CheckboxUncompleteRegex` for the force-complete / force-uncomplete rewriters, and replace seven inline `regexp.MustCompile` call sites across `pkg/ops/{update,complete,defer,workon}.go` with references to the shared vars. No behavior change; lint.go's intentionally-broader `[ xX]` regex shape is left untouched.

## v0.91.1

- fix(checkbox): accept `*` as Markdown list marker alongside `-` in checkbox regex across storage and ops packages — vault files using `* [ ]`, `* [/]`, and `* [x]` are now correctly parsed and rewritten by goal-completion, task-completion, task-update, task-work-on, and task-defer operations

## v0.91.0

- feat(work-on-task): add Phase 5 auto-sharpen + auto-gate chain — after the assistant returns `Ready to work on this task.`, automatically invoke `Skill: vault-cli:plan-task` (sharpen), then read the resulting phase: if `execution`, invoke `Skill: vault-cli:execute-task` to print the kickoff (`🎯 Start with: …` + `📋 When done, verify: …`); if `planning`, stop with a nudge to re-run plan-task when the owner has answers; if past planning (`ai_review` / `human_review` / `done`), skip kickoff. End state after `/work-on-task` is always either `phase: planning` (gaps remain) or `phase: execution` (kickoff printed). Removes the operator step of manually chaining the three commands on routine recurring tasks whose plan is already clean.

## v0.90.0

- feat(goal): align `AvailableGoalStatuses` with task statuses — `next, in_progress, backlog, hold, aborted` accepted alongside legacy `active, completed, on_hold` (kept as backward-compat aliases). `goal set status in_progress` etc. now succeed; existing vault files using either set continue to validate. Unblocks task-orchestrator drag-and-drop on the Goals view (bborbe/task-orchestrator#19).

## v0.89.0

- feat(launch-goal): add new `/vault-cli:launch-goal` interview-driven goal framing command — discovery → fan-out exploration (5 parallel semantic searches + duplicate-check gate) → 3-lens framing (parallel subagents → top-3 candidates) → sharpen → draft-to-disk with `status: draft` + Obsidian link → parallel verify (Adversarial Laziness Test + outcome traceability + hedge-word grep) → audit fan-out (goal-auditor + graph-auditor + late dup-check) → status flip on PASS. Resolves the "create-goal jumps straight to writing" failure mode by forcing an outcome-sentence confirmation gate before file creation. Mirrors `/launch-agent` shape; positions as the rigorous front door beside `create-goal`'s template fast-path
- docs(goal-writing): add § "Tooling-Category Exception" extending Title sniff test — artifact-shaped titles ("Multi-Provider Claude Code Proxy", "Goal-Writing Assistant") accepted for goals where the artifact IS the deliverable; rule: don't bounce tool-existence framings back to outcome-only
- docs(goal-writing): add § "Tasks as Business-Value Milestones" — tasks are shippable outcomes, not WBS slices; explicit decomposition hierarchy (Goal → linked Tasks (wikilinks, separate files) → inline Subtasks (checkboxes in task file)); MUST render as `[[Wikilinks]]`, NOT bold text; 1-8 is soft cap NOT floor; foundation/skeleton work allowed when explicitly framed
- docs(goal-writing): add § "Evidence Shape per Success Criterion" — borrowed from `dark-factory/docs/rules/spec-writing.md`; every SC declares observable evidence (exit code / log line / file content / state transition / metric delta / negative evidence / file artifact); kills "tests pass" / "it works" vagueness
- docs(goal-writing): add § "Adversarial Laziness Test" — borrowed from `dark-factory/docs/rules/spec-writing.md`; read SCs assuming laziest possible implementation; if `[x]` everywhere tomorrow would feel "done" without doing the work, SCs are under-specified
- docs(goal-writing): add § "Anti-pattern: soak-time DoD on personal-laptop tools" under Definition of Done — operator IS the runtime monitor for laptop tools; prefer exercise-now verification ("all paths reached in one session") over time-based bake ("runs N days without incident"); soak-time reserved for prod services with silent-degradation risk; updates Tasks-section bullet in Required sections to mandate `[[Wikilink]]` format
- docs(task-writing): add § "Subtask Hierarchy" under Task Structure — Goal → linked Tasks (wikilink files) → inline Subtasks (checkboxes inside task file); subtasks are atomic work units with no independent identity, no separate files; decision rule: shippable milestone → N tasks, sequential steps within one milestone → N inline subtasks; never recreate file-link hierarchy below task level
- feat(goal-creator): step 9 `# Tasks` body composition now requires `[[Wikilink]]` format when tasks supplied at creation time (NOT bold text + description); step 12 audit checks expanded to grep for bold-text task entries and flag soak-time DoD anti-pattern phrases
- feat(goal-auditor): item 8 "Tasks Quality" extended with WARN flags for (a) bold-text tasks instead of `[[Wikilinks]]` (disables Obsidian auto-create-on-click), (b) WBS-shaped task titles (≥3 tasks starting with `Implement`/`Define`/`Add <noun>`/`Refactor`/`Migrate`/`Wire`/`Configure`); count guidance updated to "1-8 soft cap, NOT a floor" — don't flag 1-3 tasks as under-count. Item 12 "Definition of Done Quality" extended with WARN flag for soak-time DoD phrases (`runs for N hours/days`, `one real working day's worth`, `no regressions for a week`) on tooling-category goals; don't flag on production-service goals where soak-time is appropriate

## v0.88.0

- feat(auditors): goal-auditor + task-auditor flag missing/empty `# Definition of Done` section as MAJOR; severity matrix uses `DOD_REQUIRED_AS_OF=2026-06-26` constant with grandfathering (pages `created` before cutoff → WARN, not MAJOR); task-auditor adds dev → prod ladder check for multi-environment shipping-class artifacts (detection requires explicit `dev` + `prod` co-occurrence, not container keywords); 4 new dishonest-tick phrases added to anti-pattern list (`tested on dev only`, `ci passed = tested`, `auto-release tagged ≠ shipped`, `deferred to follow-up goal`)
- docs(task-writing): add `# Definition of Done` as required section for shipping-class tasks + tasks with aspirational SCs (peer to Success Criteria); split Shipping Checklist's end-to-end verification into explicit `Tested on dev` + `Tested on prod` ladder for multi-environment artifacts; references `[[Closure Patterns]]` for per-artifact copy-paste blocks
- docs(goal-writing): add `# Definition of Done` as required section (peer to Success Criteria); new subsection explains two-sided framing (what we want vs how we verify); references `[[Goal Closure Checklist]]` + `[[Closure Patterns]]`; migration note accepts existing 'DoD under SC' pattern as WARN-grandfathered
- docs(task-writing): sharpen `hold` status — reserve for weeks-long blocks (external dependency, unresolved upstream); short waits (hours/days for doc, callback, review) stay `in_progress` with `[/]` subtask. Prevents premature `hold` hiding active work from rotations

## v0.87.0

- feat: Pass `-n "<task-name>"` to `claude` when `task work-on --mode headless` mints a session, so the session's custom-title and agent-name carry the task title from turn 1 (inherited by all later resumes)

## v0.86.0

- feat(work-on-task-assistant): extend Phase 7.5 readiness nudge with phase-aware decision table — covers terminal statuses (`completed`/`aborted` → `/vault-cli:sync-progress` + `/vault-cli:session-close`), review phases (`ai_review`/`human_review` → 🔵 review-feedback nudge with `/vault-cli:execute-task` re-run path), and `phase: done` (→ `/vault-cli:complete-task`). First-match-wins ordering; STATUS short-circuits PHASE which short-circuits SC checks. Adds `SC_HAS_CHECKBOXES` flag to disambiguate "all ticked" vs "section has no checkboxes at all" — the latter now correctly emits a planning nudge instead of a misleading complete-task nudge. Output format updated with all variants spelled out (no `<reason>` placeholder collapse). Success-criteria #9 now lists 🔵 alongside ✅/⚠.

## v0.85.1

- chore(deps): bump ginkgo v2.29.0→v2.31.0, gomega v1.41.0→v1.42.0, golang.org/x/term v0.43.0→v0.44.0, sentry-go v0.46.2→v0.47.0, bborbe/math, parse, run, and assorted x/* transitive deps
- chore(go.mod): drop obsolete `exclude (cloud.google.com/go v0.26.0)` directive

## v0.85.0

- docs(goal-writing): add `## Summary (First Sentence)` section with outcome-vs-mechanism sniff test and 3-example table (parallel to the existing Title section); cross-reference from `## Goal Structure` and `## Scope Check`; new Preflight Checklist item; closing-sentence logic now reads forward ("if title also fails, the goal is activity-shaped — split before rewriting either")
- feat(goal-auditor): expand Section 5 "Summary Quality (First Sentence)" with outcome-vs-mechanism check + mechanism-leak anti-patterns + escalation rule; add 9th Goal Scope Fit smell (mechanism-shaped summary) with explicit escalation when combined with a title that also fails the sniff test; add matching positive-signal bullet; normalize mechanism-leak separator to `/` across all examples
- feat(goal-creator): step 9 body composition now requires outcome-shaped first sentence (rephrase mechanism-phrased input BEFORE writing — applies in both interactive and tool mode, do not rely on step 12 audit which is interactive-only); step 12 audit checks expanded to ban verb-first openings ("Build X" / "Refactor Y" / "Set up Z" / "Migrate Y") in both title and summary
- chore(commands): add `allowed-tools: [Task]` to `audit-goal.md` and `verify-goal.md` frontmatter (pure delegation commands; satisfies agent-cmd MUST rule)

## v0.84.0

- feat(work-on-task-assistant): green Readiness now recommends the next gate — `✅ Readiness: looks execution-ready. Run /vault-cli:execute-task to start.` (was just the bare green checkmark, no breadcrumb). Closes the gap where operators had to remember the next command after a clean readiness pass. Warning branch unchanged — still points to `/vault-cli:plan-task`.

## v0.83.1

- docs: sync `## Integration` section across `/vault-cli:work-on-task`, `/vault-cli:plan-task`, `/vault-cli:execute-task` to one canonical 8-step Task lifecycle. Plugs the missing-commands gap in `plan-task` (was missing `work-on-task` + `execute-task` entries). Adds explicit step 5 "Start work" with the three in-execution helpers (`update-task` / `task-status` / `next-steps`), plus `complete-task` (step 7) and `session-close` (step 8) as the proper end-of-flow bookends. All three command pages now show the same numbered list — only the bolded "this command" marker differs.
- docs: README "Where this fits" pipeline updated to match the canonical 8-step lifecycle — adds the in-execution helper trio and `session-close` to the previously 6-step arrow chain.
- docs(execute-task): drop `name:` frontmatter for sibling-command consistency. Zero sibling commands in `commands/` use a `name:` field — filename is the de-facto command identifier; slash invocation prefix comes from the marketplace context.
- docs: surface `/vault-cli:execute-task` in README command table + spelled out the full phase-gated lifecycle in usage examples.

## v0.83.0

- feat: new `/vault-cli:execute-task` slash command — the **hard gate** between planning and execution. Resolves the task via the 4-priority detection chain (explicit arg → recent `/create-task` or `/plan-task` output → most-recent `[[wikilink]]` task subject → daily-note's first `[/]` → MRU file in `<tasks_dir>/`). Promotes `status: next/backlog/hold → in_progress` as a resume signal *before* running the gate (silent state mutation operators should be aware of). Then re-runs `/vault-cli:plan-task`'s 4 hard non-negotiables (Success Criteria defined, subtasks reach goal, e2e verify subtask [shipping-class tasks only — PR/release/deploy/plugin/agent/library publish; skipped for pure research / decision / doc-only], subtask-goal alignment); on pass, flips `phase: planning → execution` and prints first unchecked `# Tasks` subtask + `# Definition of Done` reminder (or `✅ All subtasks complete — run /vault-cli:complete-task` if zero unchecked). Refuses on `phase: todo` / empty (planning non-skippable), `phase: done`, `status: completed/aborted` (closed tasks need explicit reopen), or any hard check failure (points to `/vault-cli:plan-task`). Idempotent on `phase: execution` / `ai_review` / `human_review` — re-prints work block + DoD without mutation. Closes the lifecycle's last operational gap: every transition (`create → plan → execute → complete`) now has an enforced command. Stronger sibling of `/vault-cli:work-on-task`'s informational readiness nudge.

## v0.82.0

- feat: `work-on-task-assistant` emits a one-line readiness nudge for Obsidian tasks (`✅ looks execution-ready` / `⚠ phase=planning / no Success Criteria / ... — run /vault-cli:plan-task first`). Shallow file-level check only — substance still belongs to `/vault-cli:plan-task`. Preserves work-on-task's content-agnostic core (no questions, no edits, no blocking) while closing the gap where a user starts work on a half-baked task without being nudged toward the planning gate.

## v0.81.0

- feat: Date fields on Task / Goal / Objective / Theme frontmatter now flow as typed `*libtime.DateOrDateTime` end-to-end — setters store the typed value (no pre-stringification), `FrontmatterMap.GetTime` handles `time.Time` / `libtime.DateOrDateTime` / `string` shapes, JSON projection uses the type's own `String()` / `MarshalJSON`. Both `formatDateOrDateTime` helpers removed (`pkg/domain/task_frontmatter.go` + `pkg/ops/frontmatter.go`); the type itself is now the single source of truth for on-disk + on-wire format. Closes the silent-divergence risk between two independently-maintained format helpers.
- feat: **On-disk format change** for `completed_date` / `last_completed_date` (any field set via `time.Now()`). Previously emitted as RFC3339 (second precision); now emitted as RFC3339Nano (nanosecond precision) because `DateOrDateTime.String()` preserves the sub-second component. Date-only fields (`defer_date`, `planned_date`, `due_date`, `start_date`, `target_date`) are unchanged — still `YYYY-MM-DD`. Existing vault files re-write with longer precision on next mutation; expect one-time format-only diffs across vault repos. All parsers (vault-cli `ParseTime`, Obsidian YAML, `bborbe/time`) accept both formats — no functional break.
- v0.80.0 byte-identical regression baseline checked in at `pkg/ops/testdata/v0.80.0-baseline/` (scenarios 002/003/004 + task list/show JSON) — future date-format drift gets caught immediately.

## v0.80.0

- feat: Add `## Title & Filename` section to `docs/task-writing.md` codifying the **problem-vs-solution** title principle. Problem-framed titles (e.g. "Concurrent writes to legacy tasks cause merge conflicts") persist across plan-execute-review; solution-framed titles ("Make X deterministic") lock in a chosen approach and silently drift when the design pivots. Includes a 5-row before/after table, explicit carve-outs for routine ops / mandated solutions / action-IS-deliverable cases, and a "rename-on-pivot" sniff test. Scope Check and Preflight Checklist updated to match.
- feat: Add `## Title & Filename` section to `docs/goal-writing.md` codifying the **outcome-vs-mechanism** title principle (the goal-level form of the same idea — mechanism describes what you build; outcome describes what you get when done). Includes the 4-row sniff-test table and a "big collection goal" anti-pattern callout. Scope Check updated to match.
- feat: Add `docs/theme-writing.md`, `docs/objective-writing.md`, `docs/vision-writing.md` — the previously-missing three of the five Vision→Theme→Objective→Goal→Task writing guides. Each is the generic contract for the page type, including a Title & Filename section appropriate to its scope (themes: present-tense direction, "5-year sniff test"; objectives: outcome at horizon-end with explicit time horizon, "verifiable yes/no on end-date sniff test"; visions: `Be …` / `Help …` identity statement, "still describes who you want to become after life changes" sniff test). Each guide ends with a `Vault-Specific Examples` pointer to the per-vault extension page (matching the convention already used in task-writing.md and goal-writing.md).
- fix: `agents/task-auditor.md` title-rule alignment. Replaces blanket "action-verb-led title" smell with the new problem-vs-solution rule: problem-framed titles AND action-verb titles for true deliverables both pass; only vague-noun and abstract-capability titles fail. Prevents the agent from flagging the new problem-framed titles as regressions immediately after this release.

## v0.79.0

- feat: Add `INVALID_TASK_IDENTIFIER` lint check in `pkg/ops/lint.go` — surfaces when `task_identifier` is present but does not parse as a UUID (catches the literal `<uuid>` placeholder from `90 Templates/Task Template.md`, typos, and truncated values). Closes the gap that let template placeholders ship as real values — `MISSING_TASK_IDENTIFIER` only fires on empty/absent values, so a forgotten `<uuid>` placeholder would otherwise pass lint and then get backfilled to a random UUID by the `WriteTask` fallback on the next write, reintroducing the concurrent-write merge-conflict race on legacy tasks. Non-fixable on purpose: operator must replace with a fresh UUIDv4 (auto-fix would itself become a hidden UUID creation site, defeating the rule's purpose).

## v0.78.1

- fix: `vault-cli task work-on` no longer silently overrides a teammate's `assignee` on the task. New blank/equal/different matrix: blank → set to current user; already equals current user → no-op (file not dirtied); different non-blank user → preserved, warning emitted in `MutationResult.Warnings`. CLI surfaces the warning with a `⚠️` line; JSON output exposes it via the existing `Warnings` field — no struct change. Status mutation still proceeds independently. Documented in `README.md` and `docs/task-writing.md`. Implementation: `pkg/ops/workon.go` Execute matrix + 3 new Ginkgo contexts in `pkg/ops/workon_test.go`. Closes the root cause of the assignee-drift gap (was: `task.SetAssignee` called unconditionally).

## v0.78.0

- feat: Add `STATUS_DATE_MISMATCH` lint check in `pkg/ops/lint.go` — surfaces when `status: next` or `status: backlog` coexists with any of `planned_date`, `defer_date`, or `due_date` (calendar dates are commitments; only `in_progress` and terminal statuses are compatible with a date on an unstarted task). Detector powers both `vault-cli task lint` and `vault-cli task validate` through shared `collectLintIssues`. `lint --fix` auto-promotes `next`/`backlog` to `in_progress` and leaves the date field byte-identical.
- feat: `vault-cli task defer` on a `next` or `backlog` task now also writes `status: in_progress` in the same file write — closing the create-side leak at write-time. Auto-promote is gated to `next` and `backlog` only; `in_progress`, `completed`, `aborted`, and `hold` are left untouched. `defer` on an already-`in_progress` task is idempotent (status line is not re-written — only `defer_date` is set). Existing defer semantics (past-date validation, planned_date clearing when before target, daily-note updates) continue to work unchanged.
- feat: Enforce calendar-as-commitment rule on task status — tasks with any of `planned_date`, `defer_date`, or `due_date` must have `status: in_progress` (or terminal). Enforced at file creation (`task-creator` agent emits `in_progress` when a date field is set), at date assignment (`task defer` auto-promotes `next`/`backlog` to `in_progress` in the same write), and at audit (`task lint` reports `STATUS_DATE_MISMATCH`; `task lint --fix` promotes status, never strips the date). Lint and validate share a single detector.

## v0.77.0

- feat: `/vault-cli:sync-progress` (new Phase 6) and `/vault-cli:complete-task` (MODE=interactive step 2e) now emit a `⚪ DONE` state-closer panel recommending `/vault-cli:session-close` after a task is completed in the session. Prevents the prior drift where Claude invented a closer pointing at `/vault-cli:next-task` — wrong for the one-task-per-session orchestrator workflow (queued daily-note items get fresh Claude sessions via the orchestrator, never appended to the current one). `complete-task` MODE=tool path is explicitly guarded — JSON output stays clean. PR-only / progress-only sync paths skip the closer.
- feat: `/vault-cli:session-close` (new Phase 4.5) now scans the session's touched vault tasks; any task still `status: in_progress` surfaces as outstanding before close with concrete next-actions (`/vault-cli:complete-task`, `/vault-cli:defer-task`, or status hold/aborted). Scoped to TOUCHED tasks only — untouched `[/]` items on the daily note belong to other sessions / the orchestrator and are intentionally NOT flagged. Closes the loop opposite to the closer change: complete-task / sync-progress tell you to close the session; session-close refuses to call it clean if the anchor task isn't actually done.
- refactor: `/vault-cli:complete-task` MODE=interactive no longer uses AskUserQuestion when task has incomplete items — replaced with abort-with-`--force`-hint message. Reduces friction on the common path (just complete) while keeping the safety gate for partial completions explicit.
- feat: `/vault-cli:complete-task` adds `--force` flag — bypasses the incomplete-items gate. MODE=tool is unchanged (always sets `phase: human_review` on incomplete, never completes).
- fix: `/vault-cli:session-close` Phase 4.5 now surfaces `vault-cli task get` failures as outstanding instead of silently skipping. Each unverified task gets its own outstanding line citing exit code + stderr first line. A failed status lookup means the anchor-task safety gate is unverified — exactly the failure mode this phase guards against. Bot PR-reviewer finding (MAJOR) from PR #18.

## v0.76.0

- fix(task-auditor): relax over-strict gates that gave shipping-class tasks 5/10 verdicts while they shipped clean. Four targeted relaxations: (1) `goals:` field is RECOMMENDED, not REQUIRED — `themes:` link is acceptable for operational/infra/follow-up tasks with no clean parent goal; only flag MAJOR when both absent. (2) Success Criteria count is GUIDANCE not a cap — 2-4 typical for focused tasks, 5-8 normal for shipping checklists (PR / merge / release / deploy / verify per env); `(optional)` markers allowed for conditional items. (3) Definition of Done is required only when SC items are aspirational ("Code is clean"); when each SC already encodes its own verification command + expected result, a blanket DoD sentence is acceptable — not MAJOR. (4) Shipping-class "release fired" subtask has an auto-release carve-out: when the repo auto-releases on merge (CI workflow, dark-factory `autoRelease: true`, conventional-commits action) and the tag is cited as evidence in `# Results` / `# Pull Requests` / merge subtask body, a separate "verify tag exists" subtask is bookkeeping and missing it is MINOR not MAJOR. Mirrored in `docs/task-writing.md` (canonical contract).

## v0.75.0

- feat: Add `/vault-cli:session-close` slash command — end-of-session safety check ported from `~/.claude/commands/session-close.md`. Verifies progress is synced, git state is clean, no orphan worktrees, no in-flight dark-factory work, and surfaces reflect/runbook/link-hygiene signals. Inline command (analyzes parent conversation; sub-agent cannot see it). All vault-specific paths driven by `vault-cli config` — `tasks_dir`, `goals_dir`, `daily_dir`, `knowledge_dir`. Runbook folders auto-discovered via `^[0-9]+ [Rr]unbooks$` regex (no config field). Cross-surface checks (git, dark-factory, `gh`, TaskList) degrade silently when absent — coworker-installable across any vault registered with `vault-cli config`. Completes the per-session lifecycle bookend alongside the existing per-task (`work-on-task` → `sync-progress` → `complete-task`) and per-day (`start-day` → `complete-day`) trinities.

## v0.74.0

- feat: `/vault-cli:plan-task` Step 5's E2E verify subtask check now also rejects *vague* verify subtasks. The body must describe both *what to do* and *what to expect* — at least one concrete shape (procedure to execute, observable to check, or artifact to inspect) plus a result a reader could independently confirm. Bare promises like *"Verify the endpoint"* fail; procedure-only steps like *"run a check on the endpoint"* also fail (no expected result); concrete steps like *"curl /widgets, confirm 200 + body matches schema"* pass. LLM quality call (no verb list or regex). Closes the *vague-verify* hole that PR #15's *missing-verify* fix left open.

## v0.73.0

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
