---
description: Validate that a task has Success Criteria and the subtasks needed to reach its goal; conversationally fill gaps; on phase=planning, transition to execution.
argument-hint: <task-file-path-or-name>
allowed-tools: [Task, Read, Edit, Glob, Bash, AskUserQuestion]
---

Drive a task to *execution-ready* through conversation. Checks that the task has Success Criteria defined and subtasks that lead from now to the goal. Runs `task-auditor` for findings, asks targeted questions, applies answers, loops until ready. On a task in `phase: planning`, flips to `phase: execution` as the final step.

## When to use

Right after `/vault-cli:create-task` (capture lenient → plan strict), or any time a task feels incomplete. Replaces `/vault-cli:refine-task` — same workflow plus a phase-aware tail.

```bash
/vault-cli:plan-task "Some Task Name"
/vault-cli:plan-task 24\ Tasks/Some\ Task.md
```

## Process

### 1. Validate + resolve

- No argument → `❌ Pass a task identifier or name.` STOP.
- Resolve to file path (vault-cli config respected): exact path, else `Glob 24 Tasks/*<arg>*.md`. Multiple → list and STOP. Zero → STOP.

### 2. Read status + phase

```bash
vault-cli task get "<name>" status --output json
vault-cli task get "<name>" phase --output json
```

### 3. Entry contract — flip if needed

- `status` in `next`/`todo`/`backlog` → `vault-cli task set "<name>" status in_progress`
- `phase` is `todo`/empty → `vault-cli task set "<name>" phase planning`
- Already past planning → continue without flip; step 7 will skip the phase transition.

### 4. Run task-auditor

```
Task tool with:
  subagent_type: 'vault-cli:task-auditor'
  prompt: 'Audit <resolved-path>. Return: score (1-10), Critical Issues, Task Scope Fit findings, Task-Goal Alignment, top 5 Recommendations.'
```

### 5. Check the two non-negotiables

Two checks beyond the auditor's general scoring:

- **Success Criteria defined** — `# Success Criteria` section exists with ≥ 2 binary checkboxes
- **Subtasks reach the goal** — `# Tasks` section (or equivalent) lists concrete steps that, if completed, produce the SC outcomes. If subtasks are missing or vague ("Implement feature" alone), flag.

Either failing → mandatory question in step 6; can't exit on auditor score alone.

### 6. Surface gaps + fix loop

Translate findings (auditor + non-negotiable checks) into questions. Rules:

- Max 3 questions per turn
- Each question is short (one sentence) + tight options (single yes/no OR 2-4 numbered options)
- Lead with `(Recommended)` per global UX
- Quote the offending line/section so owner sees what triggered the question
- Use `AskUserQuestion` for the actual ask

Apply each answer via `Edit`. Re-run auditor after each batch. Print delta `Score: X → Y`. Loop until score ≥ 8 AND both non-negotiables pass OR owner says "good enough."

### 7. Exit / phase transition

**Phase was `planning` AND score ≥ 8 AND non-negotiables pass:**

```bash
vault-cli task set "<name>" phase execution
```

Print: `✅ Task ready. Score: X/10. Phase: planning → execution. Next: <first unchecked SC>`

**Phase is anything else (execution / ai_review / human_review / done):**

Print: `✅ Task sharpened. Score: X/10. Phase unchanged (was <phase>).`

**Owner abort OR score < 8 after loop:**

Print: `⚠ Task improved to X/10. Phase unchanged. Remaining: <bullets>. Re-run when ready.`

## Notes

- **Scope is minimal on purpose.** Plan-task's job is "task has SC + has subtasks to reach the goal + structurally sound per auditor." Rich heuristics (MVP framing, KISS pass, Out-of-Scope capture, evidence shape, verification depth) belong in `task-auditor` and `task-writing.md` as canonical rules — not as forced workflow steps here. Letting the auditor enforce them keeps `/plan-task` short and consistent across vaults.
- **Questions stay tight.** Short setup → short options → quote the offending line for context. No paragraphs of preamble per question. Owner sees what's being asked and what each answer changes.
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
