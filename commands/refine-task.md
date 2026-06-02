---
description: Conversationally refine a task's substance (DoD, scope, subtasks, goal alignment) using task-auditor findings as input
argument-hint: <task-file-path-or-name>
allowed-tools: [Task, Read, Edit, Glob, Bash, AskUserQuestion]
---

Refine a task that isn't ready to work on — when DoD is weak, subtasks are vague, scope has crept, or the goal link doesn't fit. Runs `task-auditor` for findings, asks you targeted questions, applies your answers to the file, and re-audits until the task is ready.

## When to use

Reach for this when `/vault-cli:work-on-task` opens a task and you realize it's not actually workable yet. `work-on-task` is content-agnostic by design (decided 2026-06-02) — it sets status and finds guides, it does not question your task. `refine-task` is the dedicated tool for sharpening substance.

```bash
/vault-cli:refine-task "Add vault-cli refine-task slash command"
/vault-cli:refine-task 24\ Tasks/Some\ Task.md
/vault-cli:refine-task TRADE-1234   # if you use Jira-style identifiers
```

## Process

1. **Validate input**
   - If no argument: `❌ Pass a task identifier or name.` and STOP
   - Resolve to a task file path:
     - If path-like, use as-is
     - Otherwise `Glob` `24 Tasks/*<arg>*.md` (vault-dependent; respect `~/.vault-cli/config.yaml` vault root)
     - If multiple matches: list them and STOP with `Be more specific.`
     - If zero matches: STOP with `❌ Task not found.`

2. **Run the auditor (read-only)**
   ```
   Task tool with:
     subagent_type: 'vault-cli:task-auditor'
     prompt: 'Audit <resolved-path>. Return: score (1-10), Task Scope Fit findings, Critical Issues, Task-Goal Alignment, top 5 Recommendations.'
   ```

3. **Early-exit on already-good tasks**
   - If score ≥ 8 AND no Critical Issues AND no Task Scope Fit smells → print `✅ Task is ready. Score: X/10. No refinement needed.` and STOP

4. **Surface gaps as numbered questions**
   - Translate findings into at most **3 questions per turn** (respects global UX rule: no either/or, single-select y/digit answers)
   - Each question should:
     - Quote the offending line/section verbatim (so the user sees what triggered the question)
     - Offer 2–4 numbered options OR a single yes/no
     - Lead with a recommendation marked `(Recommended)` (per global preference)
   - Use `AskUserQuestion` for the actual ask
   - Example shapes:
     - DoD missing → "What does 'done' look like?" with 2–3 candidate criteria drafted from the task body
     - Subtask vague → "Subtask `- [ ] Improve X` — concretize as:" + numbered options
     - Scope smell (6+ SCs, 3 repos) → "Split into N tasks?" 1=yes / 2=keep+add DoD per SC
     - Goal orphan → "Re-link to [[Goal A]] / [[Goal B]] / drop link?"

5. **Apply answers**
   - For each answer, `Edit` the task file to apply the chosen change
   - Update `last_updated` frontmatter to today's date
   - If a structural change (e.g. add `# Definition of Done` section), insert in the canonical position per Task Writing Guide

6. **Re-run auditor**
   - Re-invoke `task-auditor` on the edited file
   - Print delta: `Score: X → Y` + list of newly-resolved issues + any remaining

7. **Loop or exit**
   - If score < 8 AND remaining gaps are addressable → return to step 4 (next batch of questions)
   - If score < 8 BUT remaining gaps need decisions you don't want to make now → exit with `⚠ Task improved to X/10. Remaining: <bullet list>. Re-run when ready.`
   - If score ≥ 8 → exit with `✅ Task ready. Score: X/10.`

## Notes

- **Reuses `task-auditor` — does not duplicate audit logic.** Auditor stays read-only; refine-task is the write-back wrapper.
- **Conversational on purpose.** The user is the judge of substance — refine-task never silently rewrites the goal link, DoD, or scope. Every change comes from an explicit answer.
- **Loop discipline.** Stop at score ≥ 8 OR when the user indicates "good enough" — nice-to-haves are optional, don't chase a 10/10.
- **Does NOT gate `work-on-task`.** Opt-in only. Invoke when *you* feel the task isn't ready.
- **Mechanical fixes stay in `audit-task` → manual edit.** This skill is for substance (DoD, scope, alignment), not formatting (missing tags, frontmatter typos).

## Integration

Task lifecycle:
1. `/vault-cli:create-task` → first draft
2. **`/vault-cli:refine-task`** → sharpen substance (this command)
3. `/vault-cli:work-on-task` → set status + find guides
4. `/vault-cli:update-task` / `/vault-cli:sync-progress` → log progress
5. `/vault-cli:complete-task` → close

Output ends with one of:
- `✅ Task ready. Score: X/10.` (success)
- `⚠ Task improved to X/10. Remaining: <bullets>. Re-run when ready.` (partial)
- `❌ Task not found.` / `❌ Pass a task identifier or name.` (input error)
