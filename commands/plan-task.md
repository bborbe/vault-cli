---
description: Validate that a task has Success Criteria and the subtasks needed to reach its goal; conversationally fill gaps; on phase=planning, transition to execution.
argument-hint: <task-file-path-or-name> (or detects from conversation)
allowed-tools: [Task, Read, Edit, Glob, Bash, AskUserQuestion]
---

Drive a task to *execution-ready* through conversation. Checks that the task has Success Criteria defined and subtasks that lead from now to the goal. Runs `task-auditor` for findings, asks targeted questions, applies answers, loops until ready. On a task in `phase: planning`, flips to `phase: execution` as the final step.

This command **must stay inline** — it analyzes the parent conversation when no argument is given; a sub-agent cannot see the conversation.

## When to use

Right after `/vault-cli:create-task` (capture lenient → plan strict), or any time a task feels incomplete. Replaces `/vault-cli:refine-task` — same workflow plus a phase-aware tail.

```bash
/vault-cli:plan-task                              # detects from conversation (e.g. just after /create-task)
/vault-cli:plan-task "Some Task Name"
/vault-cli:plan-task 24\ Tasks/Some\ Task.md
```

## Process

### 1. Resolve task path

**With argument:** exact path if path-like, else `Glob` `<tasks_dir>/*<arg>*.md` (vault-cli config respected). Multiple matches → list and STOP. Zero → STOP.

**Without argument — detect from conversation** (in priority order):

1. **Most recent `/create-task` output** — scan the parent conversation for `/vault-cli:create-task "<name>"` (with or without slash) or its result line (`✅ Created task: <name>` / file-path output). If found and unambiguous, use that name.
2. **Most recent `[[Task Name]]` wikilink** referenced in the conversation as a task subject (not as a generic mention in prose). Match against `<tasks_dir>/`.
3. **Daily note's first `[/]` checkbox** — `{daily_dir}/YYYY-MM-DD.md`; the first item marked `[/]` (in-progress) is the active task.
4. **Most recently modified file in `<tasks_dir>/`** — final fallback.

Resolve the detected name via `Glob` same as the with-argument path. Multiple matches → list candidates and ask owner via `AskUserQuestion` (single-question, short options). Zero → `❌ No task detected. Pass a task identifier or name.` STOP.

When detection succeeds without explicit argument, print the resolved task name on first line of output (`Detected task: <name>`) so the owner can interrupt if wrong before any state mutation.

### 2. Read status + phase

```bash
vault-cli task get "<name>" status --output json
vault-cli task get "<name>" phase --output json
```

### 3. Entry contract — flip if needed

The goal is to land at `status: in_progress, phase: planning` for fresh tasks; respect a deliberate post-planning phase setting.

- `status` in `next`/`todo`/`backlog` → flip status AND phase together: `vault-cli task set "<name>" status in_progress` + `vault-cli task set "<name>" phase planning` (if phase is empty/`todo`/`planning`). Skip the phase flip if phase is `execution` / `ai_review` / `human_review` / `done` (treat as deliberate — sharpen but don't move phase backward).
- `status` already `in_progress` and `phase` is `todo`/empty → `vault-cli task set "<name>" phase planning`
- `status` already `in_progress` and `phase` is past planning → continue without flip; step 7 will skip the phase transition.

### 4. Run task-auditor

```
Task tool with:
  subagent_type: 'vault-cli:task-auditor'
  prompt: 'Audit <resolved-path>. Return: score (1-10), Critical Issues, Task Scope Fit findings, Task-Goal Alignment, top 5 Recommendations.'
```

### 5. Check the non-negotiables

Five checks beyond the auditor's general scoring — first four are hard (any failure → mandatory question in step 6, can't exit on auditor score alone), fifth is a soft warning.

**Hard:**

- **Success Criteria defined** — `# Success Criteria` section exists with ≥ 2 binary checkboxes.
- **Subtasks reach the goal** — `# Tasks` section (or equivalent) lists concrete steps that, if completed, produce the SC outcomes. If subtasks are missing or vague ("Implement feature" alone), flag.
- **E2E verify subtask present** — for shipping-class tasks (PR / release / plugin update / agent / deploy / library publish; or subtasks reference a git repo / marketplace / registry — see `task-writing.md` "Shipping Checklist"), `# Tasks` must include a subtask that runs the shipped artifact in its real environment. Two sub-checks on that subtask:

    1. **No dishonest-tick phrases.** Reject if the body contains a case-insensitive substring match of any phrase from `task-writing.md:122-134`:
        - *"deferred to first use"*
        - *"deferred — will validate"*
        - *"will check next session"*
        - *"will verify on first use"*
        - *"first deployment will test"*
        - *"trust the audit"*
        - *"trust CI"*
        - *"trust the tests"*
        - *"will validate later"*

    2. **Concrete procedure, not just a promise.** The body must describe HOW verification happens AND what result counts as success — a reader must know both *what to do* and *what to expect*. Three shapes count as concrete (any one is sufficient; combinations are stronger):
        - a **procedure to execute** — `curl /widgets`, `kubectl get pod foo`, `open the rendered page`, `run make docs-build`, `gh release list`
        - an **observable to check** — `HTTP 200`, `exit 0`, `log contains "X"`, `table renders without overflow`, `tag v0.74.0 exists`
        - an **artifact to inspect** — `output matches schema docs/widget-response.schema.json`, `marketplace.json version equals git tag`, `rendered README has working Code-Of-Conduct link`

        A verify subtask passes when its body covers (a) at least one of the three shapes AND (b) a result a reader could independently confirm. Vague promises fail: *"Verify the endpoint"* names a target but no action and no expected result; *"Verify it works"* names neither. Concrete examples pass — HTTP: *"curl /widgets, confirm 200 + body matches schema"*; CLI: *"run `scenarios/release.md`, confirm exit 0"*; doc: *"open the rendered README, confirm the install table renders + Code-Of-Conduct link works"*; K8s: *"kubectl get pod foo, confirm Running + log contains 'startup complete'"*.

        LLM quality call (no verb list, no regex) — the rule above IS the anchor. Re-read it when in doubt; the procedure / observable / artifact taxonomy defines what concrete means here.

    Skip this whole check for non-shipping-class tasks (pure research, decision, doc-only with no published artifact).
- **Subtask-goal alignment** — every `# Tasks` checkbox must either (a) map by topic to ≥ 1 `# Success Criteria` outcome, or (b) be the e2e verify subtask. Flag any orphan as a scope-creep candidate; in step 6 the owner can link it to an SC, move it to `# Out of Scope`, or split it into a separate task.

**Soft:**

- **KISS ceiling** — if `# Tasks` has > 8 checkboxes, warn: *"task may be too large for one session — consider splitting, moving items to `# Out of Scope`, or promoting to a goal."* Owner decides; task can still proceed to execution.

Any hard check failing → mandatory question in step 6; can't exit on auditor score alone. Soft check failing → surfaced as a question in step 6 but doesn't block exit if owner says proceed.

### 6. Surface gaps + fix loop

Translate findings (auditor + non-negotiable checks) into questions. Rules:

- Max 3 questions per turn
- Each question is short (one sentence) + tight options (single yes/no OR 2-4 numbered options)
- Lead with `(Recommended)` per global UX
- Quote the offending line/section so owner sees what triggered the question
- Use `AskUserQuestion` for the actual ask

Apply each answer via `Edit`. Re-run auditor after each batch. Print delta `Score: X → Y`. Loop until score ≥ 8 AND all four hard non-negotiables pass OR owner says "good enough."

### 7. Exit / phase transition

**Phase was `planning` AND score ≥ 8 AND hard non-negotiables pass:**

```bash
vault-cli task set "<name>" phase execution
```

Print: `✅ Task ready. Score: X/10. Phase: planning → execution. Next: <first unchecked SC>`

**Phase is anything else (execution / ai_review / human_review / done):**

Print: `✅ Task sharpened. Score: X/10. Phase unchanged (was <phase>).`

**Owner abort OR score < 8 after loop:**

Print: `⚠ Task improved to X/10. Phase unchanged. Remaining: <bullets>. Re-run when ready.`

## Notes

- **Scope is focused on what blocks safe execution.** Plan-task enforces five planning-gate checks (SC defined, subtasks reach goal, e2e verify subtask, subtask-goal alignment, KISS ceiling) because each one prevents a specific failure mode: missing outcomes, missing path, dishonest-tick verification, scope creep, oversize task. Other heuristics (MVP framing, Out-of-Scope capture quality, evidence shape) stay in `task-auditor` and `task-writing.md` as canonical rules — surfaced via the auditor in step 4, not promoted to dedicated gates. Letting the auditor enforce general structure while plan-task enforces the five named gates keeps the command short and the gates legible.
- **Questions stay tight, with consequence visible.** 2-3 lines of setup → short options. "Tight" doesn't mean stripping context — owner must see what each answer *changes*. Quote the offending line, name the trade-off, then options.
- **Subtask granularity = session-sized.** When proposing or sharpening `# Tasks` items, target *work-block size* (a session's worth of work), not CLI-step size. Aim for 3-6 items per task. Reject auditor-suggested over-decomposition like "run precommit / open PR / merge PR" as separate subtasks — those collapse into one "ship the change" block.
- **Reads `~/.claude/plugins/marketplaces/vault-cli/docs/task-writing.md` as the canonical rule source** — same rules `task-auditor` enforces.
- **Conversational on purpose.** Owner is the judge of substance. Plan-task never silently rewrites; every change comes from an explicit answer.
- **Entry contract.** On a fresh task (`status: next, phase: todo`), plan-task flips to `in_progress, planning` itself. No `/work-on-task` prerequisite.
- **Phase-aware tail.** Phase transition only fires on `phase: planning`. At any other phase, plan-task is a pure sharpener.
- **Mechanical fixes stay in `/audit-task`.** This command is for substance (SC, subtasks, goal alignment), not formatting.

## Integration

Task lifecycle:

1. `/vault-cli:create-task` — capture (lenient)
2. **`/vault-cli:plan-task`** — plan (this command)
3. *(execution: just code, no command)*
4. `/vault-cli:sync-progress` / `/vault-cli:update-task` — log progress
5. `/vault-cli:complete-task` — close

Output ends with one of:
- `✅ Task ready. Score: X/10. Phase: planning → execution.` (planning success)
- `✅ Task sharpened. Score: X/10. Phase unchanged (was <phase>).` (non-planning success)
- `⚠ Task improved to X/10. Phase unchanged. Remaining: <bullets>. Re-run when ready.` (partial)
- `❌ Task not found.` / `❌ Pass a task identifier or name.` (input error)
