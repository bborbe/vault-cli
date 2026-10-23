---
description: Mark task as complete (normal or recurring)
argument-hint: "<task-name-or-path> [--non-interactive] [--force]"
allowed-tools:
  - Bash(vault-cli:*)
  - Read
  - Skill
---

<objective>
Mark task as complete using vault-cli. Handles normal and recurring tasks appropriately.
</objective>

<process>
1. Parse arguments:
   - If contains `--non-interactive` (or deprecated `--tool`) → MODE=non_interactive, remove flag from args
   - Otherwise → MODE=interactive
   - If contains `--force` → FORCE=true, remove flag from args (interactive mode only — `--non-interactive` overrides as defined in step 3)
   - Extract task name from remaining args

2. **MODE=interactive (default):**

   a. Read task file to check completion state:
      - Find task: `vault-cli task get "{task_name}" status --output json`
      - Parse checkboxes (count `[x]`, `[/]`, `[ ]`) with
        `grep -cE '^\s*-\s+\[[ x/]\]' "<task-file>"`
      - Never use `vault-cli task show` here: it returns the whole body, and a task
        with a long `# Progress` log makes that a large read for one field. Step 4.5
        of `session-close.md` already uses the cheap form.

   b. If incomplete items (pending > 0 or in-progress > 0) AND FORCE=false:
      - Print completion status (`X/Y checkboxes, N%`)
      - List specific incomplete items
      - Print: `❌ Task has incomplete items. Finish them first, or re-run with --force to complete anyway.`
      - STOP. Do NOT call `vault-cli task complete`. No interactive prompt.

   c. Run vault-cli. Pass `--force` through whenever FORCE=true — the CLI enforces
      the same incomplete-checkbox guard, so omitting it makes the flag a no-op and
      the run fails with `incomplete subtasks: N pending`:
      ```bash
      vault-cli task complete "{task_name}"            # no incomplete items
      vault-cli task complete "{task_name}" --force    # incomplete items + FORCE=true
      ```

      **`--force` is for items that were never this task's work** — follow-ups filed
      for a separate task, or notes that are checkboxes only by formatting. It is not
      a way to close unfinished work. If the unchecked items are genuinely part of the
      task, finish them.

   d. Show report:
      ```
      ✅ Task completed: [[{task_name}]]
      ```
      - If warnings in output, show them

   e. **Emit closer panel** — **normal tasks only.** Two cases skip both the panel and the self-close; check them first:

      - **Recurring tasks.** Probe before emitting — `vault-cli task get "{task_name}" recurring --output json`; a `value` other than `null` and non-empty means recurring (the values are `daily` / `weekly` / `monthly` / `quarterly`). This probe is required because neither other source carries it: step 2a reads only `status`, and the interactive completion prints just `✅ Task completed: {task_name}` with no recurring field. `⚪ DONE` is false for a task that stays `in_progress`, and `session-close` Phase 4.5 hard-flags an `in_progress` anchor into mode 3, which ends on an `approve:` line this panel promises does not exist. Report the completion and stop.
      - **Nested callers.** `sync-progress` Phase 4b invokes this command from inside its own run, and `session-close` Phase 2 invokes `sync-progress` — so an unconditional close recurses (`session-close → sync-progress → complete-task → session-close`). Phase 4b therefore passes `--non-interactive`, whose JSON-only path never reaches this step.

      Otherwise append below the report, verbatim, no rewording:
      ```
      ⚪ DONE
      👤 You: nothing
      ⏰ Next: /vault-cli:session-close — this session runs it now
      ```

      Then run `/vault-cli:session-close` yourself — no approval line, no pick menu.

      Rationale for this closer (one-task-per-session contract, why never `/vault-cli:next-task` here, why the "no end-of-day suggestions" global rule does NOT override): see [`sync-progress.md` Phase 6](sync-progress.md) — same contract applies. Single source of truth lives there to prevent drift.

      MODE=non_interactive MUST NOT emit this panel (see step 3 — JSON only).

3. **MODE=non_interactive (--non-interactive flag):**

   a. Read task file to check completion state
   b. If incomplete items:
      ```bash
      vault-cli task set "{task_name}" phase human_review
      ```
      Return: `{"success": false, "reason": "incomplete items"}`
      STOP.

   c. If complete:
      ```bash
      vault-cli task complete "{task_name}" --output json
      ```
      Return: `{"success": true, "path": "..."}`
      STOP.

   d. Never ask questions, never prompt.

   e. **Never emit the `⚪ DONE` closer panel** — MODE=non_interactive output is JSON only. The closer panel from step 2e is interactive-mode only.

4. Task types (handled by vault-cli internally):
   - Normal tasks: status→completed, today's daily note checked. The goal's `# Tasks` list is **not** written — it is a derived copy of the task's `goals:` frontmatter, and rollup reads the frontmatter (see `pkg/ops/complete.go`)
   - Recurring: Reset checkboxes, update defer_date, keep status in_progress
</process>

<success_criteria>
- vault-cli task complete invoked (NOT Edit tool for frontmatter)
- **MODE=non_interactive**: Returns JSON only, sets phase=human_review if incomplete
- **MODE=interactive**: Shows completion %, aborts with `--force` hint if incomplete (no prompts), reports result on success
- Goal state is **not** written — the goal's `# Tasks` list is a derived copy; rollup reads the task's `goals:` frontmatter (deliberate, `pkg/ops/complete.go:144`)
- Today's daily note updated (by vault-cli) — a task tracked only in an earlier day's note is not touched there
</success_criteria>

## Passive metrics

Completion writes `metrics_completed_at` and, when the task was worked on,
`metrics_interaction_count` and `metrics_operator_prompt_count`. Recurring completion
archives the finished cycle into `metrics_cycles` and resets the accumulator. These
metrics fields are written passively by vault-cli and must not be hand-edited.
`metrics_interaction_count` counts each distinct `session_id` in `metrics_sessions`
exactly once — a session reused across multiple work-on runs is counted a single time.
`metrics_operator_prompt_count` counts only the prompts the operator actually typed,
excluding tool results, `isMeta` injections, non-human `turnOrigin` values, `system`
`promptSource` entries, and the known non-prompt text prefixes — legacy noise,
attention-board answers, `!` shell input and rendered prompt echoes (`<command-name>`
and `<command-message>` slash commands still count).
