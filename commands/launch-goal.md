---
description: Interview-driven goal framing — discovery → fan-out exploration → outcome candidates → sharpen → draft-to-disk → parallel verify → audit + status flip. Parallels /launch-agent. Resolves the "create-goal jumps straight to writing" failure mode by forcing an outcome-sentence confirmation gate before the file is touched.
argument-hint: "[rough idea]"
allowed-tools: [Task, Read, Write, Edit, Glob, Grep, Bash, AskUserQuestion, mcp__semantic-search__search_related]
---

<objective>
Frame a goal through interview-driven discovery before drafting the file. Uses fan-out / fan-in patterns at exploration, framing, verification, and audit to surface more signal in parallel without lengthening the user-facing path.
</objective>

<references>
- `docs/goal-writing.md` — generic contract (Title sniff test, Summary sniff test, Tasks as business-value milestones, Soak-time DoD anti-pattern, Evidence shape per SC, Adversarial Laziness Test, Scope Check, required sections)
- `docs/task-writing.md` — subtask hierarchy (linked Tasks → inline Subtasks)
- Per-vault writing-guide extension (if present in the vault's `knowledge_dir`) — vault-specific examples and conventions
- `~/Documents/workspaces/dark-factory/docs/rules/spec-writing.md` — source of evidence-shape vocab + Adversarial Laziness Test (borrowed concepts, not the whole framework)
- `[[Goal Closure Checklist]]`, `[[Closure Patterns]]` — referenced from the Definition of Done section
- `$ARGUMENTS` — optional rough idea; if empty, start Phase 1 from scratch
</references>

<design_principles>
- **KISS over rigor.** Goals are conversation anchors, not autonomous-agent contracts. Borrow the cheap-and-high-signal spec patterns (evidence shape, laziness test); skip the spec-grade ceremony (formal decomposition matrix, hedge-word audits).
- **Fan-out where parallel branches genuinely produce different signal** (exploration, framing, verification, audit). **Stay linear** where the work is a single user-focused thread (interview, section drafting).
- **No silent advance through gates.** Outcome confirmation, draft approval, forcing-test pass, scope check — each is an explicit user-visible checkpoint.
</design_principles>

<process>

## Phase 1 — Understand (linear, conversational)

Goal: surface the real outcome, the user it serves, and a list of scope-creep candidates for Phase 4 Non-goals.

Ask ONE question per turn (not a list). Keep going until you have enough signal — typically 3-5 turns. Always include the **"so that..." probe** somewhere:

> "If this ships, what is true that isn't today — and why does that matter?"

If the user struggles to answer the "so that" cleanly, that's a red flag the goal isn't well-understood yet. Loop one more probe before advancing.

Other starter prompts (pick what fits):
- "What's broken or missing today?"
- "Who feels the change — you, an operator, an agent, a downstream user?"
- "How will you know it worked?" (proto-success-criteria signal)
- Listen for adjacent-but-out-of-scope work — note these as Phase 4 Non-goal candidates.

**Hard gate to Phase 2**: you can state the rough outcome in one sentence without using "via", "by", "through", or naming a mechanism. If you can't, ask one more question.

## Phase 2 — Explore (fan-out: 5 parallel semantic searches + linkage check)

**Fan-out**: in a single message, issue parallel `mcp__semantic-search__search_related` calls:

1. `"<rough idea> theme"` → parent theme candidates from the vault's `themes_dir`
2. `"<rough idea> objective"` → parent objective candidates from the vault's `objectives_dir`
3. `"<rough idea>"` → existing or duplicate goals from the vault's `goals_dir`
4. `"<rough idea> adjacent"` → sibling/related goals worth linking
5. `"<rough idea> example"` → vault examples or guide entries worth quoting back during framing

**Fan-in**: build a context bundle:
- Top theme candidate(s) (≤3)
- Top objective candidate(s) (≤2)
- Goals scoring ≥ similarity threshold → flagged as **possible duplicates**
- Linked-goal candidates (for `# Related` section)
- Example phrasings for Phase 3 framing

### Duplicate-check gate (hard)

If the search returned any goal with high outcome-overlap (similar title OR similar Summary first sentence), STOP and present via AskUserQuestion:

- **1 (Recommended on close match)** — Extend the existing goal `[[<title>]]` instead of creating a new one (open it, suggest where to add the new SC/task)
- **2** — Create a separate goal (the existing one is adjacent but distinct; explain in one sentence why)
- **3** — Abort (the existing goal already covers this)

Do NOT advance past this gate silently. If 2 is picked, mention the adjacent goal in `# Related` automatically.

### Optional: code or web exploration

Only when the goal references an external system the user can't characterize from memory (rare). Skip by default.

## Phase 3 — Frame (fan-out: 3 lens subagents → fan-in: top-3 candidates)

**Fan-out**: spawn 3 parallel `Task` calls (general-purpose agent), each with a different framing lens. Each returns ONE outcome sentence and a one-line rationale.

- **Lens A — user-impact**: "Frame this goal as an outcome from the perspective of who feels the change. Name the actor + the new capability or relief, no mechanism. ≤1 sentence."
- **Lens B — system-state**: "Frame this goal as a before/after world state. Name what is true after that isn't now. No mechanism. ≤1 sentence."
- **Lens C — theme-alignment**: "Given parent theme `<top theme candidate>`, frame this goal as the specific delta on that theme this goal will deliver. No mechanism. ≤1 sentence."

Pass each subagent: the Phase 1 transcript + the Phase 2 context bundle + the outcome-vs-mechanism sniff-test rules from `docs/goal-writing.md` § Summary.

**Fan-in** (main loop):
- Discard any candidate that fails the sniff test (mechanism leak: "via", "by", "through", verb-first activity opening like "Build / Refactor / Set up / Migrate")
- Dedupe semantic overlap (keep the strongest phrasing)
- If <3 distinct sniff-test-passing candidates survive, run one more Phase 1 probe and retry — do NOT show weak options
- Rank: prefer the lens most aligned with the user's "so that…" answer from Phase 1

Present top 3 via AskUserQuestion (single-select), #1 recommended, with "Other (counter-propose)" implicit.

- "Other" with counter-proposal → run through sniff test → if passes, lock; if fails, rewrite and re-confirm
- **Tooling-category exception**: when the user counter-proposes a tool-existence-shaped framing ("I have an assistant that …" / "I have a proxy that …"), don't bounce back to outcome-only — accept it as a tooling goal (the artifact IS the outcome), tighten phrasing if needed, and lock. See `docs/goal-writing.md` § Tooling-Category Exception.

**Hard gate to Phase 4**: user confirmed an outcome sentence. **No silent advance.**

## Phase 4 — Sharpen (linear)

Derive from the locked outcome + Phase 1 transcript + Phase 2 context bundle.

- **Title** — outcome-shaped, passes the Title sniff test from `docs/goal-writing.md` § Title (mechanism table). Tooling-category exception accepted for artifact-shaped titles when the artifact IS the outcome.
- **Success Criteria** — 3-5 binary checkboxes; each declares an **evidence shape** (one phrase, vocab from `docs/goal-writing.md` § Evidence Shape per Success Criterion):

  | Shape | Example |
  |---|---|
  | exit code | "`make precommit` exits 0" |
  | log line | "log line `request_id=<uuid> status=ok`" |
  | file content / diff | "`grep -n 'pattern' file.md` returns ≥1 line" |
  | HTTP response | "`GET /api/x` returns 200" |
  | state transition | "frontmatter `status` transitions `next → in_progress`" |
  | metric delta | "counter `foo_total` increments by N" |
  | negative evidence | "`grep ERROR run.log` returns 0 lines" |
  | file artifact | "task file under `tasks_dir/` exists with frontmatter `goal: [[X]]`" |

  Not closure steps. Not "tests pass". Not "it works".

- **Definition of Done** — ≥2 binary closure checkboxes. Reference `[[Goal Closure Checklist]]` + the matching `[[Closure Patterns]]` block for the artifact type (k8s service / CLI tool / docs). Add ≥2 project-specific extras inline. **Anti-pattern: avoid soak-time DoD** ("runs N hours/days without incident", "one real working day's worth of use", "no regressions for a week") for personal-laptop tools the operator drives interactively. The operator IS the runtime monitor and notices breakage immediately. Prefer **exercise-now** verification ("all paths reached in one session, evidence: log line per path") over time-based bake. Soak-time DoD is appropriate only for production services with silent-degradation risk (prod k8s, multi-user, trading hot path) — flag explicitly when used. See `docs/goal-writing.md` § Soak-Time DoD Anti-Pattern.

- **Non-goals** — surface the Phase 1 scope-creep candidates via AskUserQuestion (multiSelect): *"Which of these are OUT of scope?"* Target 3-7 concrete deferrals. Link follow-up goals/tasks where the deferred work will live (use the Phase 2 adjacent-goal results to suggest links).

- **Tasks** — **business-value milestones**, not code-change slices. Each task delivers a shippable improvement: "Allow Claude Code to pass through the proxy" YES; "Implement config-driven routing core" NO — that's a WBS slice. The 4-8 range from `docs/goal-writing.md` is a soft cap, NOT a floor: **small goals can have 1 task** ("Implement the proxy"). Don't pad to hit a number. Each task visibly traces to ≥1 SC; foundation/skeleton work that enables but doesn't advance an SC is allowed if explicitly framed ("foundation; enables iteration"). **Decomposition hierarchy** (encode this when drafting):

  ```
  Goal (file) → linked Tasks (wikilinks, separate files) → inline Subtasks (checkboxes inside the task file)
  ```

  **Decision rule**: if each could be a shippable milestone → N separate tasks. If sequential steps within one milestone → 1 task with N inline subtasks. Implementation breakdown (schemas, adapters, refactors, types) lives as inline subtasks inside the task file (or in a dark-factory spec for code-heavy work) — **never as sibling task files**. Subtasks are atomic work units, no independent identity, no separate files. Don't recreate file-link hierarchy below the task level. See `docs/goal-writing.md` § Tasks as Business-Value Milestones + `docs/task-writing.md` § Subtask Hierarchy.

  **Format rules:**
  - MUST render as `[[Wikilink Task Title]]` in the goal file body, NOT bold text + description
  - Obsidian auto-creates the task file when the operator clicks the wikilink — primary task-creation path
  - Closing summary surfaces `/vault-cli:create-task "<title>"` as the alternative CLI path
  - Title-Case names, no `/`, `.`, backticks, `:`, `*`, `?`, `"`, `<`, `>`, `|`
  - Optional one-line context after the wikilink (e.g. `1. [[Task Title]] — context (→ SC2)`)

- **Parent theme + objective** — pre-fill from Phase 2 top candidates; confirm via AskUserQuestion if multiple plausible matches.

- **Impact** (one paragraph) — strategic value + theme connection. **Lead the paragraph with the user's verbatim "so that" answer from Phase 1 if it's memorable.** The Phase 3 lens subagents tend to sanitize human framing into clinical phrasing — preserve the original voice; do not rewrite for tone.

- **Status Summary** — `Progress: 0%` / `Current: Goal drafted` / `Next: <first task title>` / `Blockers: None`.

### Write draft to disk + show Obsidian link

**Do NOT render the full draft in chat.** Markdown walls in chat are unreadable, lose clickable wikilinks, and force the user to dictate edits back through chat. Write the file to disk with `status: draft` and let the user review in Obsidian's native rendering.

1. **Resolve target vault** via `vault-cli config list --output json` (don't hardcode paths)
2. **Write `<goals_dir>/<Title>.md`** with all Phase 4 content, frontmatter `status: draft` (NOT `in_progress` yet — flipped on audit PASS in Phase 6). Canonical section order from `docs/goal-writing.md` § Required sections:
   - Frontmatter: `status: draft`, `page_type: goal`, `themes:` (confirmed in Phase 4), `objective:` (confirmed in Phase 4), `created: <today>`, optional `category`, `priority`, `timeline`
   - `Tags: [[Goal]]` (+ theme tags)
   - Summary paragraph (the Phase 3 locked sentence + one optional quantification sentence)
   - `# Impact`
   - `# Status Summary`
   - `# Success Criteria` (with evidence shapes inline)
   - `# Definition of Done`
   - `# Non-goals`
   - `# Tasks` (as `[[Wikilinks]]`, NOT bold text)
   - `# Related` (linked sibling goals from Phase 2)
3. **Show the link inline** (single message, no chat-render of file content):

   ```
   Draft written: [<Title>](obsidian://open?vault=<vault>&file=<encoded-path>)
   Review in Obsidian — edit any section directly. Say "go" to advance to verify + audit.
   ```

4. **Wait for user "go"** before advancing.

**Hard gate to Phase 5**: file written on disk with `status: draft` + user said "go" (file may have been edited in Obsidian between draft-write and go — Phase 6 re-reads before audit).

## Phase 5 — Verify (fan-out: 3 parallel forcing tests → fan-in: PASS or fix-list)

**Fan-out**: in a single message, run 3 parallel forcing tests as `Task` calls (or inline if simpler):

1. **Adversarial Laziness Test** — *"If the operator wrote `[x]` on every Success Criterion tomorrow without doing the actual work, would the goal feel done?"* If yes → list which SCs are under-specified. See `docs/goal-writing.md` § Adversarial Laziness Test.
2. **Outcome traceability** — *"Does every Success Criterion verify the locked outcome sentence (Phase 3)?"* If no → list SCs that drifted off-target.
3. **Hedge-word grep** — scan Summary + SCs + Tasks for `should / appropriate / reasonable / as needed / if necessary / proper / correct`. Distinguish deferral from descriptive use — flag only deferrals.

**Fan-in**: consolidate into one of:
- **PASS** — all three tests clean → advance
- **FIX** — list specific edits needed → loop back to Phase 4 draft (re-show, re-approve)

### Scope check (linear, follows verify)

After verify passes, run the Scope Check from `docs/goal-writing.md` § Scope Check:
- Task count ≤ 8 (soft cap, not floor — 1 task is fine for small goals)
- Tasks-to-criteria ratio ≤ 2.5×
- All tasks share one mental model (one operator outcome, one domain)
- Title + Summary both pass their sniff tests

If 3+ signals fail → goal is over-scoped. Use AskUserQuestion (KISS bias: collapse before split):

- **1 (Recommended)** — Collapse fragmented tasks into broader milestones. Goals usually want fewer-broader tasks, not more granular ones. Most ratio failures dissolve when 3 small tasks become 1 named milestone.
- **2** — Add Success Criteria the existing tasks already serve. Often the tasks are fine; the SC list under-counts what's actually being delivered.
- **3** — Split into N goals. Use ONLY when the goal is genuinely multi-outcome (two separate "so that" answers). On split, loop Phase 4 per split (Phase 1-3 stay reusable — the umbrella context still applies).

## Phase 6 — Audit + status flip

### Re-read draft + audit + flip status on PASS

The file already exists from Phase 4 with `status: draft`. The user may have edited it in Obsidian between draft-write and "go" — re-read what's actually on disk so audit operates on the user's edits, not the original draft.

1. **Re-read** `<goals_dir>/<Title>.md` to capture any in-Obsidian edits
2. **Run audit fan-out** (see below) — on the disk content, not the original draft
3. **On audit PASS** (0 MAJOR): Edit frontmatter `status: draft` → `status: in_progress`
4. **On audit FAIL** (≥1 MAJOR): leave `status: draft`; surface MAJOR findings; user can fix in Obsidian directly (or via chat) and say "re-audit" — loop back to step 1

### Audit (fan-out: parallel auditors + late dup-check)

In a single message, run in parallel:

- `Task(subagent_type: "vault-cli:goal-auditor", ...)` — full guide-compliance audit
- `Task(subagent_type: "vault-cli:graph-auditor", ...)` (or `hierarchy-auditor` where available) — orphan + broken wikilink check, theme/objective backlink integrity
- `mcp__semantic-search__search_related` against the just-written outcome sentence → final dup-check (catches duplicates the rough-idea search missed once the outcome is locked)

**Fan-in**: consolidate MAJOR (must-fix) and WARN (note) findings into a single block.

### Print closing summary — rich launchpad

Final output is the operator's action panel. Goal as clickable `obsidian://` URL (file exists). Tasks as plain `[[wikilinks]]` — match what's in the goal file body, and clicking them in Obsidian auto-creates the task file (primary path). The closing summary surfaces `/vault-cli:create-task` as the alternative CLI path for operators who prefer scripted creation. Related links as clickable `obsidian://` URLs (files exist).

```
✅ Goal: [<Title>](obsidian://open?vault=<vault>&file=<encoded-path>)

Audit: MAJOR <n> / WARN <n><one-line WARN summary if non-trivial>
Theme backlink: [[<primary theme>]] # Sub-Goals ✓

📋 Tasks to create (<N>, in dependency order):
  1. [[<Task 1 title>]]
  2. [[<Task 2 title>]]
  ...

Related (existing — click to navigate):
  - [<Linked Title>](obsidian://...) — <one-line context>
  - ...

Next:
  → /vault-cli:create-task "<Task 1 title>" (first in dependency order)
  → or /vault-cli:create-task <pick-by-name> for a different task
  (if MAJOR > 0: fix in Obsidian + say "re-audit" — task creation gates on clean audit)
```

**URL encoding rules**:
- Use `%20` for spaces, `%2F` for `/`
- Strip `.md` from file paths in `obsidian://` URLs
- Vault name is case-sensitive — use the value from `vault-cli config list`

**Skip the Related block** if there are no existing-file links to surface (all-new namespace). Empty sections are noise.

</process>

<success_criteria>
- User explicitly confirmed an outcome sentence in Phase 3 (no silent advance)
- Duplicate-check gate ran in Phase 2 — either no high-overlap match, OR user chose extend/separate/abort
- Goal file exists with all 9 required sections populated from the interview
- Each Success Criterion declares an evidence shape inline
- Tasks rendered as `[[Wikilinks]]`, NOT bold text + description
- Phase 5 forcing tests (laziness + traceability + hedge) all PASS before audit
- Final audit (Phase 6 fan-out) returns 0 MAJOR findings; on PASS, `status` flips `draft` → `in_progress`
</success_criteria>

<notes>
- **Mirrors `/launch-agent`** in shape (interview → scaffold → checklist), differs in scope (one markdown file vs. a whole repo).
- **Position vs `create-goal`**: `create-goal` is the template fast-path (scaffolds from frontmatter; no discovery). `launch-goal` is the rigorous front door (discovery + framing + verify + audit). Both keep their seats; pick `create-goal` when you already know the outcome, `launch-goal` when you're still framing.
- **KISS guardrail:** if a phase starts feeling like ceremony in real use, cut it. Adversarial Laziness Test + evidence shape + outcome traceability are the three load-bearing forcing functions; everything else is supporting machinery.
- **Voice:** stay terse during the interview. One question per turn, no preambles. Sharp questions, not a form.
</notes>
