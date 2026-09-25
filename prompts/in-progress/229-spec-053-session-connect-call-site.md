---
status: approved
spec: [053-fleet-spawned-metrics-sessions]
created: "2026-09-25T17:18:12Z"
queued: "2026-09-25T17:37:23Z"
branch: dark-factory/fleet-spawned-metrics-sessions
---

# The session-connect call site: invoke the append verb in the branch that writes the session id

<summary>
- A fleet-spawned worker's session-connect step now records the session it just connected, instead of stamping only the session-id field.
- The append is invoked in exactly one branch — the branch that writes the session id — and only after that write has landed.
- A run whose session id is already set invokes nothing, because the run that set it already appended its own entry.
- A refused or failed append is a warning, not a failure: the session-id write stays in place and the run continues.
- The session id stays the load-bearing field; metrics remain analytics, so a missing metrics entry never blocks "Ready to work on this task."
- No frontmatter is edited by the agent itself — the write is performed by vault-cli through its own verb.
- The mandatory-mutation list, the read-only whitelist and the Phase 3 status-flip rule are all untouched.
</summary>

<objective>
Wire the `vault-cli task append-metrics-session` invocation into the plugin's session-connect step in `agents/work-on-task-assistant.md`, as a new actionable line inside the branch that writes `claude_session_id`, ordered after that write. This is the call site whose absence makes fleet-spawned runs invisible to `metrics_sessions`; the verb it calls shipped in prompt 1.
</objective>

<context>
This prompt depends on prompt 1 of spec 053 already being applied: the verb name is frozen by prompt 1, and this prompt is the only other artifact that must agree on it. Read `CLAUDE.md` first for project conventions.

Then read in full:

- `agents/work-on-task-assistant.md` — the file you edit. Read the whole file, not just the section. The pieces that matter:
  - `<critical_writes>` (the `MANDATORY mutations` block) — **frozen, must not change.** The append is deliberately NOT a mandatory mutation.
  - `<constraints>` — the line `READ-ONLY except: status frontmatter + \`claude_session_id\` frontmatter + daily-note tracking` is **frozen, must not change.** The metrics write is performed by vault-cli, not by the agent's own edit surface, so it is not added to that whitelist.
  - `## Phase 3: Find Obsidian task and set status` and its bullet `If \`status != in_progress\`: flip the status **without** \`work-on\` unless session-connect above wrote a \`claude_session_id\` for this session …` — **frozen, must not change.** It is reasoned, observed behaviour and not this spec's surface.
  - `### Session connect (MANDATORY when Obsidian task file exists)` — the section you edit. Its numbered steps are 1 (read the field), 2 (detect on empty), 3 (already set), 4 (the rename suggestion). Step 2's two branch bullets are the insertion point: `- If EXACTLY ONE UUID is returned: \`vault-cli task set "<task_name>" claude_session_id "<uuid>"\`` and `- If zero OR multiple UUIDs are returned (…)`. Note that the surrounding prose lines are long single physical lines — this file does not hard-wrap prose, and the checks in `<verification>` are line-based, so your inserted instruction must be one physical line.
- `commands/work-on-task.md` § `## Passive metrics` — READ ONLY in this prompt. It states the contract (*"Each work-on run appends one entry to the task's `metrics_sessions` frontmatter field … written passively by vault-cli and must not be hand-edited"*). Prompt 4 updates it to name both writers; do not touch it here.
- `pkg/ops/metrics_session_append.go` — created by prompt 1. If this file does NOT exist, STOP and report `status: failed` with the message "append-metrics-session verb not yet deployed (prompt 1)" — do NOT create it here. Read `Execute`'s signature and confirm the verb's argv shape: `vault-cli task append-metrics-session <task-name> <session-id>`, resolved through the same vault dispatcher `task set` uses, so the call site needs no new flags.
- `docs/session-liveness.md` — READ ONLY. The `LIVE_WINDOW` rule the detection step already uses; this prompt does not widen it or add a second window.
- `specs/in-progress/053-fleet-spawned-metrics-sessions.md` — the spec. Read its Goal, Non-goals, Desired Behavior 6, Acceptance Criterion 6, Constraints and the Failure Modes rows "The agent definition loses the invocation", "The verb exits non-zero" and "A caller passes a session id that is not the task's current session". Every requirement below comes from them.

Coding-plugin docs (in-container paths — the host paths do not exist inside the container):
- `/home/node/.claude/plugins/marketplaces/coding/docs/agent-command-development-guide.md` — agent-definition structure, thin-command rules, unattended-execution patterns (`agent-cmd/no-user-prompts`, `agent-cmd/scripts-in-claude-dir`). The invocation you add must be a plain command line, not an inline script, and must never introduce a prompt.

**Environment facts that shape this prompt:**

1. **Every check in `<verification>` is git-free.** The daemon's executor does not check `<verification>` exit codes, so a command that dies for an environmental reason still reports a pass. The spec's git-shaped evidence is reproduced here with line-number arithmetic, section-scoped `awk` and greps. The git forms stay on the spec's `# Verification` § "Operator-executable" rung.
2. **This is a Direct-layer markdown edit.** No Go file, no test file, no script is created or modified here — the regression lock for this prose is prompt 3, and the documentation that names the verb is prompt 4.
3. **`make precommit` does not yet run any check on this file.** Prompt 3 adds `scripts/metrics-append-call-site-test.sh` to `make test`; at this prompt the only guards on your edit are the assertions in `<verification>`.
</context>

<requirements>

## 1. Scope — exactly one file

`agents/work-on-task-assistant.md` is the only file that changes. Do NOT touch `commands/work-on-task.md`, `README.md`, `docs/**`, `scenarios/**`, `CHANGELOG.md`, any Go file, any test file, or `scripts/**`.

## 2. Insert one new line between the two branch bullets of § Session connect

Inside the section headed `### Session connect (MANDATORY when Obsidian task file exists)`, insert this sub-bullet as a **new line between** the `- If EXACTLY ONE UUID is returned: …` bullet and the `- If zero OR multiple UUIDs are returned (…)` bullet:

```
     - Then, in this same branch and only after the id write above has landed, append the session to the task's metrics: `vault-cli task append-metrics-session "<task_name>" "<uuid>"`. This is the only way a session-connect records metrics — never `vault-cli task set` / `task add` / `task remove` on `metrics_sessions`, and never a hand-edit of the frontmatter, which is written passively by vault-cli. A non-zero exit is a warning, not a ⚠️ failure: report `ℹ️ Metrics: not recorded — <reason>` and leave the `claude_session_id` write in place. The warning names metrics and the reason; the id is the load-bearing field, the metrics entry is analytics.
```

Non-negotiable properties — each is a check in `<verification>`:

- **It is an actionable step line, not a comment and not a prose mention.** It is a bulleted sub-item of the `EXACTLY ONE` branch, indented deeper than that bullet (`     - ` against the parent's `   - `). Do NOT put it in a fenced code block, do NOT prefix it with `#`, and do NOT append it to the end of the `EXACTLY ONE` bullet's own line.
- **Its line number falls strictly between the `If EXACTLY ONE UUID is returned` bullet and the `If zero OR multiple UUIDs are returned` bullet.** Inserting it shifts the `zero OR multiple` bullet down by exactly one line; nothing else moves.
- **It contains the literal `vault-cli task append-metrics-session`.** That exact substring is the interface between this artifact and the CLI; do not shorten it, wrap it, or rename the verb.
- **It is ordered after the id write.** The sentence must say the append happens in the same branch and after the id write has landed — not before it, and not as an alternative to it.
- **The report line is `ℹ️ Metrics: not recorded — <reason>`, not a `⚠️` line.** The spec's Constraint says a non-zero exit is "a warning, not a ⚠️ failure"; in this file `⚠️` is the `<critical_writes>` failure marker ("must succeed or report ⚠️") and `ℹ️` is the non-failure marker (cf. `ℹ️ Session: not connected — … refusing to guess`). Do NOT use `⚠️` here and do NOT add the append to `<critical_writes>`.
- **The warning instruction is on the same physical line** and contains the lowercase words `warning` and `metrics` in that order, so the line-based check `grep -nE 'warning.*metrics|metrics.*warning'` over § Session connect matches. Do NOT hard-wrap this instruction across two lines: the check is line-based, and a wrap defeats it even though the prose reads fine. The phrase `The warning names metrics and the reason` is the part that satisfies that check — keep the word `metrics` lowercase there.
- **The warning names metrics and the reason, and the id write stays in place.** Do NOT instruct a rollback of `claude_session_id` on a failed append, and do NOT instruct a retry loop. The id is the load-bearing field; the metrics entry is analytics. This is the spec's Desired Behavior 6 and its Failure Modes row "The verb exits non-zero".
- **No `Edit` / `Write` of a task file appears in the step.** The write is performed by vault-cli; the agent never edits frontmatter itself. This is the spec's Constraint "The write stays passive" and its Non-goal against an executor-side append.
- **The `<uuid>` you pass is the one the branch just wrote** — the single UUID the title match returned. Do NOT instruct a fallback to the task name, a second lookup, a widened `LIVE_WINDOW`, or a "pick the newest" tiebreak.

## 3. Extend step 3 so the already-connected branch invokes nothing

Step 3 currently reads:

```
3. If `claude_session_id` is **already set**: report `ℹ️ Session: already connected (<value>)` — do NOT overwrite.
```

Replace it with exactly:

```
3. If `claude_session_id` is **already set**: report `ℹ️ Session: already connected (<value>)` — do NOT overwrite, and do NOT append a metrics entry: the run that set the id already appended its own, so an append here would double-record the same session.
```

This is the spec's Desired Behavior 6 clause "when the field is already set the step invokes nothing" and its Assumption "A run that pre-set the id already appended its own entry, so the already-connected branch invokes nothing and does not double-record." Do NOT add a second invocation anywhere in step 3, and do NOT change the `ℹ️ Session: already connected (<value>)` report wording.

## 4. What must NOT change — hard, greppable constraints

Zero changes to each of:

- The `<critical_writes>` block. It keeps exactly its four numbered mutations, and the substring `append-metrics-session` must NOT appear anywhere between `<critical_writes>` and `</critical_writes>`. The append is not a MANDATORY mutation and never blocks `Ready to work on this task.`
- The `<constraints>` line `READ-ONLY except: status frontmatter + \`claude_session_id\` frontmatter + daily-note tracking`. It must survive verbatim, and `metrics_sessions` must NOT be added to it — the agent does not gain a frontmatter edit surface from this change.
- The Phase 3 bullet starting `If \`status != in_progress\`: flip the status **without** \`work-on\` …`. Its text is unchanged.
- The detection block's `LIVE_WINDOW` rule, the `sed` strip, the `find … -mmin -5` scan, the `sort -u` uniqueness requirement, and the `zero OR multiple` branch's refusal wording.
- The `<role>`, `<runtime_detection>`, `<output_format>` and every other section of the file.
- No new `tools:` entry. The invocation is a plain Bash command; `Bash` is already in `tools:`.

## 5. Self-check before finishing

- Re-read § Session connect and confirm by line number: the `EXACTLY ONE` bullet, then your new sub-bullet, then the `zero OR multiple` bullet — three consecutive lines in that order.
- Confirm the new line is one physical line, contains `vault-cli task append-metrics-session`, states the append is after the id write, and carries the warning instruction with lowercase `warning` and `metrics` in that order, and reports through `ℹ️` rather than `⚠️`.
- Confirm step 3 carries the no-append clause and no invocation.
- Walk spec 053's Acceptance Criterion 6 and state in your completion report which requirement satisfies each half of it — the call-site half here, the warning-line half here, the line-number bounds here, and the section-scoped check here. The `bash scripts/metrics-append-call-site-test.sh` half is prompt 3; say so explicitly rather than claiming it.
- Walk spec 053's Failure Modes rows that this artifact owns: "The agent definition loses the invocation" (this prompt is the fix; prompt 3 is the detector), "The verb exits non-zero" (the warning instruction) and "A caller passes a session id that is not the task's current session" (the call site passes only the id it just wrote, in the same branch).
- Confirm each check in `<verification>` passes by **running** it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 053 — non-goals (hard vetoes).** Do NOT rework the spawn shape or pre-mint the session via `work-on --mode headless`. Do NOT write the frontmatter from the agent — the worker must not write frontmatter itself, and the write must not be by hand-editing frontmatter. Do NOT add the append to the `<critical_writes>` list: it is not a MANDATORY mutation, a non-zero exit is a warning rather than a ⚠️ failure, and it never blocks "Ready to work on this task." Do NOT add `metrics_sessions` to the `<constraints>` READ-ONLY whitelist. Do NOT change the Phase 3 status-flip rule. Do NOT change the auto-resume gate or its condition 2. Do NOT add a second metrics writer on the topic work-on path. Do NOT widen `LIVE_WINDOW`, add a process cross-check, or add a tiebreak to the title match. Do NOT change the structural name→task join. Do NOT make the step invoke anything in the `zero OR multiple` branch — that branch deliberately leaves the id unwritten, and the append lives in the branch that writes it.
- **Copied from spec 053 — constraints.** The write stays passive: the worker invokes a command and never edits a task file, so no `Edit` / `Write` of a task file appears in the step. The verb resolves the task exactly as `task set` does, so the call site needs no new flags and behaves identically under `--vault` and under all-vaults. The append is not a MANDATORY mutation and is not added to `<critical_writes>`; the `<constraints>` READ-ONLY whitelist is likewise unchanged. Existing behaviour — the detection block, the refusal wording, the `ℹ️ Session: already connected` report, the `/rename` suggestion — is preserved verbatim. `make precommit` must pass in the repo root. `agents/work-on-goal-assistant.md` carries a deliberate mirror of this session-connect (it writes `vault-cli goal set "<goal_name>" claude_session_id`), but it is out of scope and must not change: `metrics_sessions` is a task-only field (`pkg/domain/task_frontmatter_metrics.go`, `TaskFrontmatter`), so a goal has no metrics entry to append and the goal path has no call site for this verb.
- **Frozen strings.** The invocation is `vault-cli task append-metrics-session "<task_name>" "<uuid>"`; the warning report is `ℹ️ Metrics: not recorded — <reason>`; the already-connected report stays `ℹ️ Session: already connected (<value>)`. All are grep targets in `<verification>`.
- **One physical line per instruction.** This file does not hard-wrap prose, and every check in `<verification>` is line-based. Do not wrap the new bullet or the new step 3.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
- Do NOT touch `commands/**`, `docs/**`, `scenarios/**`, `CHANGELOG.md`, `README.md`, any Go file or any test file — prompt 4 owns the documentation, prompt 1 owns the Go.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite (which includes `./integration`), `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code. Absence is asserted with `! grep -q`, never with a `grep -c` that must print `0`. Never pipe a test command.

Run the blocks below **in a single shell**: `AGENT` is set in the first block and `section()` is defined in the fourth, and both are reused by the blocks after them. If you run a block on its own, re-set `AGENT` (and re-define `section()` where it is used) first.

**The invocation is present exactly once, and it is the branch line:**

```
AGENT=agents/work-on-task-assistant.md
test "$(grep -cF -- 'vault-cli task append-metrics-session' "$AGENT")" = "1"
test "$(grep -cF -- 'ℹ️ Metrics: not recorded' "$AGENT")" = "1"
test "$(grep -cF -- 'ℹ️ Session: already connected' "$AGENT")" = "2"
```

**The invocation is an actionable line, not a comment or a code fence:**

```
! grep -nF -- 'vault-cli task append-metrics-session' "$AGENT" | grep -qE '^[0-9]+: *#'
! grep -nF -- 'vault-cli task append-metrics-session' "$AGENT" | grep -qE '^[0-9]+: *```'
grep -nF -- 'vault-cli task append-metrics-session' "$AGENT" | grep -qE '^[0-9]+: {5}- '
```

**It sits strictly between the two branch bullets of § Session connect:**

```
ONE=$(grep -nF -- 'If EXACTLY ONE UUID is returned' "$AGENT" | head -1 | cut -d: -f1)
ANY=$(grep -nF -- 'If zero OR multiple UUIDs are returned' "$AGENT" | head -1 | cut -d: -f1)
INV=$(grep -nF -- 'vault-cli task append-metrics-session' "$AGENT" | head -1 | cut -d: -f1)
test -n "$ONE" && test -n "$ANY" && test -n "$INV"
test "$INV" -gt "$ONE"
test "$INV" -lt "$ANY"
test "$((ANY - INV))" = "1"
```

The last assertion is the load-bearing one: the invocation is a **new line inserted between** the two bullets, so exactly one line separates them.

**The invocation is inside § Session connect (section-scoped, not file-wide), and the section carries the warning:**

```
section() { awk -v h="$2" '$0 == h { f = 1; next } f && /^#/ { exit } f' "$1"; }
SECTION='### Session connect (MANDATORY when Obsidian task file exists)'
section "$AGENT" "$SECTION" | grep -qF -- 'vault-cli task append-metrics-session'
section "$AGENT" "$SECTION" | grep -qE 'warning.*metrics|metrics.*warning'
section "$AGENT" "$SECTION" | grep -qF -- 'claude_session_id'
```

**The warning line names metrics and the reason, and keeps the id write:**

```
grep -nF -- 'ℹ️ Metrics: not recorded' "$AGENT" | grep -qE 'warning.*metrics|metrics.*warning'
section "$AGENT" "$SECTION" | grep -qF -- 'leave the `claude_session_id` write in place'
section "$AGENT" "$SECTION" | grep -qF -- 'analytics'
```

**Step 3 invokes nothing and carries the no-append clause:**

```
grep -nF -- 'If `claude_session_id` is **already set**' "$AGENT" | grep -qF -- 'do NOT append a metrics entry'
test "$(grep -cF -- 'If `claude_session_id` is **already set**' "$AGENT")" = "1"
```

**The frozen blocks are untouched** — if any of these fails, a frozen block was edited and the edit must be reverted rather than the assertion adjusted:

```
test "$(awk '/^<critical_writes>$/{f=1;next} /^<\/critical_writes>$/{f=0} f' "$AGENT" | grep -cF -- 'append-metrics-session')" = "0"
test "$(grep -cF -- 'READ-ONLY except: status frontmatter + `claude_session_id` frontmatter + daily-note tracking' "$AGENT")" = "1"
test "$(grep -cF -- 'flip the status **without** `work-on`' "$AGENT")" = "1"
test "$(grep -cF -- 'LIVE_WINDOW' "$AGENT")" = "2"
test "$(grep -cF -- 'refusing to guess' "$AGENT")" = "1"
test "$(grep -cF -- 'Do NOT fall back to the task name' "$AGENT")" = "1"
test "$(grep -cF -- 'already connected' "$AGENT")" = "2"
test "$(grep -cF -- 'Suggest: run /rename' "$AGENT")" = "2"
test "$(awk '/^<critical_writes>$/{f=1;next} /^<\/critical_writes>$/{f=0} f' "$AGENT" | grep -cE '^[0-9]+\. ')" = "4"
test "$(grep -cE '^tools: ' "$AGENT")" = "1"
```

**No frontmatter edit surface is added, and no other artifact is touched:**

```
! grep -qF -- 'append-metrics-session' commands/work-on-task.md
! grep -qF -- 'append-metrics-session' README.md
! grep -qF -- 'append-metrics-session' docs/work-on-session-lifecycle.md
! grep -qF -- 'append-metrics-session' CHANGELOG.md
test "$(grep -cF -- 'metrics_sessions' "$AGENT")" = "1"
```

The final line is a scope guard: the file mentions `metrics_sessions` exactly once after your edit — inside the new bullet — so a second mention means a metrics instruction leaked into another section. The four `! grep -qF` lines above it pin the same boundary from the other side: at this point in the spec, no other artifact names the verb (prompt 4 adds those).

Finally, walk spec 053's Acceptance Criterion 6 against the change and state in your completion report which requirement satisfies the call-site half, the warning-line half and the line-number-bounds half, and note explicitly that the `bash scripts/metrics-append-call-site-test.sh` half is prompt 3 and is not claimed here.
</verification>
