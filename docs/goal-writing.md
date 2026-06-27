---
tags:
  - guide
  - goal
---

A goal is a personal-ambition deliverable that takes weeks-to-months to achieve and breaks down into ≤ 8 tasks. It states *what* will be true when done, not *how* to get there.

## TL;DR

- **Use for**: personal ambitions, weeks-to-months, ≤ 8 tasks
- **Create**: `/vault-cli:create-goal "<title>"`
- **Audit**: `/vault-cli:audit-goal "<title>"`
- **Sections**: Summary → Impact → Status Summary → Success Criteria → **Non-goals** → Tasks → Related
- **Forcing functions**: Non-goals enumerated; every task contributes to ≥ 1 success criterion

## Goal

Produce a goal page that an agent can scaffold, audit, and complete without ambiguity. Every section is binary-checkable; the contract holds whether the goal is operational, learning, or project-shaped.

## When to Write a Goal

| Situation | Goal? |
|-----------|------|
| Personal ambition you WANT to achieve, weeks-to-months effort | Yes |
| Specific measurable deliverable | Yes |
| Supports a theme or objective | Yes |
| Operational task taking days | No — write a task |
| Ongoing strategic direction (years, never-ending) | No — write a theme |
| Quarterly outcome (3-12 months, multi-goal umbrella) | No — write an objective |
| Lifetime aspiration | No — write a vision |

## Creating a Goal

Use the slash command:

```
/vault-cli:create-goal "<title>"
```

The command invokes `goal-creator`, which scaffolds a file in the configured vault's `goals_dir` (typically `23 Goals/`). The agent reads vault config via `vault-cli config list --output json` — never hardcode paths.

## Title & Filename

**Title = deliverable.** Filename = title (Obsidian renders the filename as the page title — no separate H1 needed).

**Rules:**

- State the outcome, not the activity ("Deploy automated trading system" beats "Work on trading system")
- Be specific about what you're creating
- Avoid ambiguous titles that could mean multiple things

**Quick test:** can someone read just the title and know exactly what "done" looks like? If no, revise.

**Title sniff test — outcome vs mechanism:** does the title describe the OUTCOME (what you get when done) or the MECHANISM (what you build)? Prefer outcome.

| Mechanism (weak) | Outcome (strong) | Why |
|---|---|---|
| "PR Reviewer Operator UX" | "On-Demand PR Review Trigger" | "Operator UX" describes the surface; "On-Demand Trigger" names what you can now do |
| "Release Agent - Extended" | (split into focused goals) | "Extended" describes phase, not outcome; can't pass the sniff test on its own |
| "Release Agent" | "Release Agent - Base" | Adding "- Base" clarifies the goal owns MVP scope, not the perpetual hardening that follows |
| "Phase-Gated Task Flow" | "Phase-Gated Task Flow" | Accepted — "phase-gated" *is* the outcome (predictable flow); mechanism and outcome coincide |

When the title still describes a mechanism after one rewrite, the goal itself may be a "big collection goal" — split first, then title each split.

This is the goal-level form of the **problem-vs-solution** principle from `task-writing.md` — same idea, scoped to weeks-of-work surface.

### Tooling-Category Exception

For goals under `category: tooling` whose deliverable IS the tool, an artifact-shaped title and tool-existence summary pass the sniff test even though they read as mechanism on the surface. Examples:

| Title | Verdict | Why |
|---|---|---|
| "Multi-Provider Claude Code Proxy" | Accepted under tooling exception | The proxy IS the deliverable — naming the artifact names the outcome |
| "Goal-Writing Assistant" | Accepted under tooling exception | The assistant IS the deliverable — reaching for it by default IS the world-after-ship |
| "Release Agent - Base" | Accepted under tooling exception | The `- Base` suffix scopes the artifact to MVP (already canonical example above) |

**Rule:** when the user's framing is `"I have a <tool> that does X"` / `"<Artifact> exists and I reach for it"`, accept it as a tooling goal — the artifact IS the outcome. **Don't bounce back to outcome-only.** Still apply Summary discipline (lead with the state-of-the-world the artifact creates, not "I am building X"), but accept the artifact noun in the title.

When NOT to apply: goals where the tool is a means to a separate outcome (e.g. "Reduce backlog by 50%" via a tool — outcome is the backlog reduction; tool is the mechanism). Tooling category is for goals where shipping the artifact = the win.

## Summary (First Sentence)

The first sentence of the goal body is the **outcome statement** — the same rule as Title & Filename, scoped to one sentence. Tells the reader what's true when the goal is done, in plain language. The *how* (mechanism, architecture, refactor steps) belongs in `# Impact` (as an "Approach" lead paragraph) or in linked design docs — never in the opening sentence.

**Rules:**

- Lead with the outcome, not the activity ("Reduce X from Y to Z" beats "Build Z by doing W")
- One sentence is enough; second sentence adds quantification or scope, not mechanism
- Avoid "via X" / "by doing Y" / "through Z" — these are mechanism leaks pretending to be outcome
- Same problem-vs-solution principle as Title & Filename, just at sentence scope

**Sniff test:** can the reader, after one sentence, picture the *world after the goal ships*? If they instead picture the *work happening*, rewrite.

| Mechanism (weak) | Outcome (strong) | Why |
|---|---|---|
| "Split monorepo + build /launch-agent plugin for new-agent scaffolding." | "Reduce new-agent creation from multi-day setup to a ~30-minute slash-command flow." | Strong version names the new state of the world (creation time drops); weak version describes the build work. |
| "Refactor the auth layer to use OAuth2 with PKCE." | "Make first-time user login work in one click instead of three forms." | "Refactor the auth layer" is mechanism; "first-time login in one click" is the user-visible outcome that motivates the refactor. |
| "Set up Prometheus + Grafana for agent observability." | "Make per-pipeline agent token spend visible within 5 minutes of any run." | The mechanism (Prometheus + Grafana) is one of several stacks that could deliver the outcome. |

When the summary still reads as activity after one rewrite, check whether the title also fails the sniff test — two failures signal an activity-shaped goal; split before rewriting either section.

## Goal Structure

### Frontmatter

```yaml
---
status: todo
page_type: goal
priority: 3                                      # optional, 1-3
category: <domain>                               # optional
timeline: 2026-MM-DD to 2026-MM-DD               # optional, ≤ 4 weeks for tactical goals
objective: "[[Parent Objective]]"                # optional
themes:                                          # optional
  - "[[Parent Theme]]"
---
```

`status` valid values: `in_progress`, `todo`, `backlog`, `hold`, `completed`, `aborted`.

### Required sections

In order:

1. `Tags: [[Goal]]` (after frontmatter, before content separator)
2. **Summary** — first paragraph after the `---` separator. 1-2 sentences stating the **outcome** (what's true when done) + benefit. See [Summary (First Sentence)](#summary-first-sentence) for the outcome-vs-mechanism sniff test and examples.
3. `# Impact` — strategic value, theme connection, quantified where possible
4. `# Status Summary` — Progress / Current / Next / Blockers (one line each)
5. `# Success Criteria` — 3-5 binary, measurable checkbox outcomes — *what we want when done*
6. `# Definition of Done` — closure steps that verify completion — *how we know we're done* (peer to Success Criteria; see [Definition of Done](#definition-of-done))
7. `# Non-goals` — 3-7 concrete deferrals (what's out of scope; link follow-up tasks/goals)
8. `# Tasks` — 1-8 task wikilinks (`[[Wikilink Task Title]]`, NOT bold text + description), business-value milestones in logical order. See [Tasks as Business-Value Milestones](#tasks-as-business-value-milestones). The 4-8 range is a soft cap, NOT a floor — 1 task is fine for small goals; don't pad to hit a number.
9. `# Related` — themes / sister goals / docs

Optional: `# Risk Management` (appendix for high-stakes goals).

### Non-goals — the scope-creep guard

Adopted from dark-factory spec convention. Forces explicit articulation of what's out of scope BEFORE the task list bloats.

**Good non-goals:**

```markdown
# Non-goals

- Auto-retry of transient agent failures — separate concern, [[Auto-Retry Transient Agent Failures Before Human Review]]
- Refactoring existing CRDs (assignee names stay as they are)
- Backfilling historical tasks — only new emissions get the new shape
```

**Bad non-goals:**

```markdown
# Non-goals

- We won't be perfect
- Anything not in tasks
```

**Rules:**
- 3-7 items typical. Fewer = scope wasn't really challenged. More = goal itself is too broad.
- Each item is a *concrete* deferral, not a vague disclaimer.
- Link follow-up goals/tasks where the deferred work lives.
- After drafting tasks: re-read Non-goals. If any task is also listed under Non-goals, fix one or the other.

When in doubt: if a reader might ask "does this goal include X?" and the answer is no, X belongs in Non-goals.

## Definition of Done

Every goal has TWO sides:

| Section | Question | Example |
|---------|----------|---------|
| `# Success Criteria` | **What is true when this goal ships?** (outcome) | "All 5 must-fix items shipped — correctness + security restored" |
| `# Definition of Done` | **How do we verify it actually shipped?** (closure) | "All PRs merged. Dev + prod tested. Goal page synced." |

The Success Criteria side says *what we want*. The DoD side says *how we know we're done*. Both are required; they are NOT alternative phrasings of the same content. Conflating them is the failure mode that lets goals close with PRs still open or prod never tested.

### Required content

The DoD section MUST contain ≥2 binary checkboxes covering closure (`PR merged`, `branches deleted`, `tested on dev`, `tested on prod`, `goal page synced`, etc.). Placeholder like "see closure patterns" without concrete steps is rejected.

### Use the canonical references

Don't re-derive the checklist from scratch. Reference the shared guides + add project-specific extras inline:

```markdown
# Definition of Done

See [[Goal Closure Checklist]] for the generic 6-section structure (Code shipped / Build+tests / Regression gate / Dev→Prod / Vault sync / Cleanup).

For the artifact this goal ships, copy the matching block from [[Closure Patterns]]:
- K8s-deployed service → use [[Closure Patterns#Pattern — K8s-Deployed Service]]
- CLI tool / binary → use [[Closure Patterns#Pattern — CLI Tool / Binary]]
- Docs / markdown → use [[Closure Patterns#Pattern — Docs / Markdown]]

## Project-specific extras

- [ ] (extras for this goal — e.g. "verify Hue lights still pair", "run TDR batch review")
```

### Migration note

Goals authored before this section existed sometimes embed closure steps INSIDE `# Success Criteria` (e.g. an SC item like "PR merged + dev deployed"). That pattern is **deprecated but accepted** — the auditor flags it as WARN, not MAJOR, so existing goals don't break. New goals MUST use the peer `# Definition of Done` section.

### What does NOT belong in DoD

- The actual work (lives in `# Tasks`)
- Outcome statements (lives in `# Success Criteria`)
- Implementation choices (lives in linked task pages or specs)

DoD is the gate, not the plan.

### Anti-pattern: soak-time DoD on personal-laptop tools

For goals shipping personal-laptop tools the operator drives interactively, **the operator IS the runtime monitor.** Avoid time-based bake-in DoD items:

| Soak-time (weak) | Exercise-now (strong) | Why |
|---|---|---|
| "Runs for 1 working day with no manual workarounds" | "All 4 providers reached in one Claude Code session — evidence: `[route] <provider> 200` log line per provider, same PID" | Operator notices breakage immediately; time-based bake adds no signal beyond exercise-now |
| "No regressions for a week of normal use" | "Three real goal-drafting sessions complete end-to-end with 0 MAJOR audit findings" | "Regression" requires immediate exercise to detect; a week of no-use doesn't bake anything |
| "Service runs unattended for 24h" | (acceptable only on prod services with silent-degradation risk — k8s, multi-user, trading hot path) | The operator's attention IS the heartbeat for laptop tools |

**Rule:** for personal-laptop tools (CLI tools, slash commands, single-user daemons, scripts), prefer **exercise-now** verification ("all paths reached", "first-pass audit clean", "real-task completed end-to-end") over **time-based bake** ("runs for N hours/days without incident"). Soak-time is appropriate ONLY when silent degradation is a real risk (production services, multi-user, autonomous overnight jobs) — flag explicitly when used.

This anti-pattern mirrors the [Adversarial Laziness Test](#adversarial-laziness-test): a soak-time DoD item passes by *not breaking*, which the laziest implementation already achieves (just don't deploy and time passes anyway). Replace with an evidence-shape SC or a concrete closure step.

## Scope Check

Before approving a goal, verify these signals:

- **Task count ≤ 8**, all tasks contribute to ≥ 1 success criterion
- **Tasks-to-criteria ratio ≤ 2.5×** (e.g. ≤ 8 tasks for 3 criteria)
- **Non-goals enumerates 3-7 deferrals** — not vague disclaimers
- **All tasks share one mental model** (one operator outcome, one domain)
- **Goal title states an outcome**, not an activity or capability (passes the outcome-vs-mechanism sniff test — see [[#Title & Filename]])
- **Summary first sentence states an outcome**, same sniff test at sentence scope (see [[#Summary (First Sentence)]])

If 3+ smells fail → goal is over-scoped. Split into multiple goals or move items to Non-goals.

## Tasks as Business-Value Milestones

Tasks under `# Tasks` are **business-value milestones**, not code-change slices (WBS rows). Each task delivers a *shippable improvement* — a usable state of the world.

### Decision rule

> If each item could be a shippable milestone → N separate tasks.
> If sequential steps within one milestone → 1 task with N inline subtasks.

| Business-value milestone (right) | WBS slice (wrong) | Why |
|---|---|---|
| "Allow Claude Code to pass through the proxy" | "Implement config-driven routing core" | First names a shippable state ("I can use it for one provider"); second is one slice of effort |
| "Add config + other providers" | "Add four provider adapters" / "Carry over fallback semantics" | First names a usable extension; second is implementation breakdown that lives inside the task |
| "Set up project skeleton at GitHub" | "Define provider config schema for YAML" | First is foundation (explicitly framed); second is a code-change inside the foundation work |
| "Dogfood `/launch-goal` on 3 real goal ideas and log observations" | "Run launch-goal once" / "Write observations file" | First is the milestone; second is the inline subtasks |

### Decomposition hierarchy (explicit)

```
Goal (file) → linked Tasks (wikilinks, separate files) → inline Subtasks (checkboxes inside the task file)
```

- **Goal-level tasks** are `[[Wikilinks]]` to separate task files.
- **Task-level subtasks** are checkboxes INSIDE the task file (`- [ ] …`). Atomic work units, no independent identity, no separate files.
- **Never** create sibling task files for subtask-shaped work. Don't recreate file-link hierarchy below the task level. See `task-writing.md` § Subtask Hierarchy for the task-side view.

### Format mandate

In the goal file body, the `# Tasks` section MUST render each task as a `[[Wikilink Task Title]]`:

```markdown
# Tasks

1. [[Allow Claude Code to Pass Through the Proxy]]
2. [[Add Config and Other Providers to the Proxy]] — context (→ SC2, SC3)
```

NOT bold text + description:

```markdown
# Tasks

1. **Allow Claude Code to pass through the proxy** — single-provider end-to-end…
2. **Add config and other providers** — yaml schema, mapping, adapters…
```

Why: Obsidian renders `[[Wikilinks]]` as clickable; clicking auto-creates the task file with the title. Bold-text + description disables the auto-create path and forces manual `/vault-cli:create-task "<title>"` invocation. The wikilink form preserves both paths.

**Title rules** (applied at goal-write time):
- Title-Case
- No `/`, `.`, backticks, `:`, `*`, `?`, `"`, `<`, `>`, `|` (Obsidian filename rules)
- Optional one-line context after the wikilink: `1. [[Task Title]] — context (→ SC2)`

### Foundation/skeleton work

Tasks that enable but don't directly advance an SC are allowed when **explicitly framed** as foundation:

```markdown
1. [[Set Up Multi-Provider Proxy Project Skeleton]] — GitHub repo, license, CI, install.sh (foundation; enables iteration)
```

The audit accepts this when the framing makes "foundation" explicit; otherwise it flags as orphan (task advances no SC).

### Soft cap, not floor

The 1-8 range is upper-bounded. **There is no minimum** — small goals can have 1 task ("Implement the proxy"). Don't pad to hit a number. If the goal genuinely has only one shippable milestone, the file has one entry under `# Tasks`.

## Evidence Shape per Success Criterion

Borrowed from `~/Documents/workspaces/dark-factory/docs/rules/spec-writing.md` § "Evidence Shape per Acceptance Criterion." Every Success Criterion must declare **what the operator will observe to confirm pass.**

### Acceptable evidence shapes

| Shape | Example phrasing in SC |
|---|---|
| Exit code | "`make precommit` exits 0" |
| Stdout / stderr match | "stdout contains `processed: 42`" |
| Log line | "log line `request_id=<uuid> status=ok`" |
| File presence | "`ls path/to/file` succeeds" |
| File content (diff / grep) | "`grep -n 'pattern' file.md` returns ≥1 line" |
| HTTP response | "`GET /api/x` returns 200 with body matching `{...}`" |
| State transition | "frontmatter `status` transitions `next → in_progress`" |
| Metric delta | "counter `foo_total{label=x}` increments by N after action" |
| Negative evidence | "`grep ERROR run.log` returns 0 lines during the test window" |
| File artifact | "task file under `tasks_dir/` exists with frontmatter `goal: [[X]]`" |

### Bad SCs (no evidence shape)

- ❌ "Tests pass" — what test, what assertion, evidence shape?
- ❌ "It works" / "Functionality verified" — narration, not observable
- ❌ "Code is clean" / "Performance improved" — aspirational; needs a metric + threshold
- ❌ "Documented properly" — what file, what content, grep target?

### Good SCs (evidence shape declared)

- ✅ "After `/vault-cli:create-task`, `cat tasks/<id>.md` shows `phase: todo` in frontmatter"
- ✅ "`kubectl -n dev logs <pod> | grep 'job spawned'` returns ≥1 match"
- ✅ "Across 3 dogfood runs, each goal's first-pass `/vault-cli:audit-goal` returns `MAJOR: 0`"

The point isn't to inline test scripts — it's to make the SC's *observable target* unambiguous. The reader (and the operator later doing the work) should know exactly what to check.

## Adversarial Laziness Test

Borrowed from `dark-factory/docs/rules/spec-writing.md` § "Adversarial Laziness Test." Before approving the goal, read your Success Criteria assuming the laziest possible implementation that still ticks every box.

> If the operator wrote `[x]` on every SC tomorrow **without doing the actual work**, would the goal feel done?

If yes — the SCs are under-specified.

### Examples

| SC | Laziest "satisfaction" | Verdict |
|---|---|---|
| "`/launch-goal` works" | `echo "works" > log` | Under-specified — needs evidence shape (which paths exercised? what audit verdict?) |
| "Goal file exists" | `touch <goal>.md` | Under-specified — needs minimum content (which sections? evidence shapes in SCs?) |
| "All 4 providers reachable from one session" | Hard to fake — requires 4 distinct `[route] <provider> 200` log lines in same PID, observable | Specified well |
| "0 MAJOR findings from `/vault-cli:audit-goal`" | Audit is mechanical — can't bypass | Specified well |

### Fix pattern

Replace artifact-existence SCs (`<file> exists`) with **behavior** SCs (`<file> contains section X with content matching Y`). Replace narration SCs (`it works`) with **observation** SCs (`grep <pattern> returns ≥1 line`).

This test runs at the same scope as the [Soak-Time DoD anti-pattern](#anti-pattern-soak-time-dod-on-personal-laptop-tools) — both reject "passes by not doing anything" SCs/DoDs. Soak-time is the time-based variant; laziness is the action-based variant.

## Preflight Checklist

Before approving:

- [ ] What strategic outcome are we achieving? (Success Criteria)
- [ ] How will we verify it actually shipped? (Definition of Done — ≥2 binary closure checkboxes)
- [ ] What's NOT in scope (Non-goals enumerated)?
- [ ] Does every task contribute to ≥ 1 success criterion?
- [ ] Is the goal title outcome-shaped (not activity-shaped)?
- [ ] Is the summary first sentence outcome-shaped (not activity-shaped)?
- [ ] Are tasks weeks-to-months in aggregate (not days, not years)?

## Audit

Always audit before committing publicly:

```
/vault-cli:audit-goal "<goal title or path>"
```

The auditor (`goal-auditor` agent) checks structure, SMART criteria, Non-goals presence, Goal Scope Fit smells (9 indicators of over-scoping; 3+ → flag), and per-task alignment to success criteria.

## Lifecycle

| Status | Meaning | Trigger to enter |
|--------|---------|------------------|
| `todo` | Defined, not started | Goal file created with required sections filled |
| `in_progress` | Actively working (limit to 3-5 in flight) | First linked task transitions to `in_progress` |
| `hold` | Blocked or paused | `blocked_by:` field populated, or operator sets manually |
| `completed` | All success criteria met | `/vault-cli:complete-goal` — checks every `# Success Criteria` checkbox is `[x]` |
| `aborted` | Abandoned without completion | Operator sets manually with reason in body |
| `backlog` | Potential future, not committed | Initial state before commitment |

Completed goals are immutable — for new outcomes, create a new goal with `parent_goal: [[Previous Goal]]` if a continuation.

## Vault-Specific Examples

This doc covers structure and conventions. For concrete examples drawn from a real vault (good vs weak goals, vault-quality assessment, common mistakes), see the per-vault writing guide:

- Personal vault: `~/Documents/Obsidian/Personal/50 Knowledge Base/Goal Writing Guide.md`

That guide is example-rich and references real goals; this doc is the generic contract.

## References

- `task-writing.md` — tasks roll up to goals
- `/vault-cli:goal-status` — progress on an active goal
- `/vault-cli:verify-goal` — mechanical structure check
