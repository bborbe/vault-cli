---
description: Validate that a task has Success Criteria and the subtasks needed to reach its goal; conversationally fill gaps; leaves the task at phase=planning and hands off to execute-task (never flips phase itself).
argument-hint: "<task-file-path-or-name> [--non-interactive] (or detects from conversation)"
allowed-tools: [Task, Read, Edit, Write, Glob, Bash, AskUserQuestion, ListAgents, SendMessage]
---

Drive a task to *execution-ready* through conversation. Checks that the task has Success Criteria defined and subtasks that lead from now to the goal. Runs `task-auditor` for findings, asks targeted questions, applies answers, loops until ready. Leaves the task at `phase: planning` and points to `/vault-cli:execute-task` to begin — **plan-task never flips the phase itself**; `execute-task` owns the `planning → execution` transition.

This command **must stay inline** — it analyzes the parent conversation when no argument is given; a sub-agent cannot see the conversation.

## Non-interactive contract

If the arguments contain `--non-interactive`, strip that token and run under **NO-ASK**: never call `AskUserQuestion` — not for the ambiguous-match prompt in step 1, not for the gap questions in step 6, not anywhere. The caller is headless (`claude --print` via `vault-cli work-on`, or the Vault UI Start button) and cannot answer; an ask is a hang, not a question.

Under NO-ASK, every point that would ask instead **reports and stops**:

- **Step 1 ambiguity** (multiple `Glob` matches) → print the candidates and `❌ Ambiguous task identifier — pass an exact path.` STOP.
- **Step 6 gaps** → do not enter the fix loop, do not `Edit` the task from assumed answers. Skip to step 7 and exit on the `⚠` branch, listing each unresolved gap as a bullet phrased as the question you would have asked, so the operator can answer it on resume.
- **Step 5 soft KISS warning** → print it as a note; it never blocks exit on its own.

Everything else is unchanged: entry-contract flips (step 3), the auditor run (step 4), and the five gate checks (step 5) all behave identically. A task whose gates pass clean produces the same `✅ Plan ready` in both modes — NO-ASK only changes what happens to a task that has real gaps.

## When to use

Right after `/vault-cli:create-task` (capture lenient → plan strict), or any time a task feels incomplete. Replaces `/vault-cli:refine-task` — same workflow plus an `execute-task` handoff.

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

### 2. Read status + phase + ownership

```bash
vault-cli task get "<name>" status --output json
vault-cli task get "<name>" phase --output json
```

**Ownership gate.** Classify the task's `claude_session_id` per [`docs/session-liveness.md`](../docs/session-liveness.md) — the single liveness definition; do not restate it here.

- Owner is `live` **and** names a session other than this one → stop before any mutation:
  - **ASK mode** → surface the owner and require an explicit answer before continuing.
  - **NO-ASK** (`--non-interactive`) → print `❌ <name> is live under session <id8> — refusing to plan it.` and STOP. Never call `AskUserQuestion`; a headless caller cannot answer, and an ask is a hang.
- `quiet`, `indeterminate`, `none` → continue. `indeterminate` is **not** a block — it cannot be proven dead, so it proceeds, but say so (`ℹ️ Session state: indeterminate — owner unproven`) so the operator can abort.

**Re-run this gate immediately before every mutation** — the step 3 entry-contract flips and every step 6 `Edit`. The step 2 read is stale by construction: a claim arriving mid-run is exactly the case this guards, and it is also the only case that matters under NO-ASK, where step 6 is skipped but step 3 still writes.

### 3. Entry contract — flip if needed

The goal is to land at `status: in_progress, phase: planning` for fresh tasks; respect a deliberate post-planning phase setting.

- `status` in `next`/`todo`/`backlog` → flip status AND phase together: `vault-cli task set "<name>" status in_progress` + `vault-cli task set "<name>" phase planning` (if phase is empty/`todo`/`planning`). Skip the phase flip if phase is `execution` / `ai_review` / `human_review` / `done` (treat as deliberate — sharpen but don't move phase backward).
- `status` already `in_progress` and `phase` is `todo`/empty → `vault-cli task set "<name>" phase planning`
- `status` already `in_progress` and `phase` is past planning → continue without flip; step 7 will skip the phase transition.

### 4. Run task-auditor

```
Task tool with:
  subagent_type: 'vault-cli:task-auditor'
  prompt: 'Audit <resolved-path>. Return: score (1-10), Critical Issues, Task Scope Fit findings, Task-Goal Alignment, top 5 Recommendations, and — when the task carries a `# Source` footer — the `**Template**:` line plus the [template-level]/[instance-level] label on each finding (omit both otherwise).'
```

### 5. Check the non-negotiables

Six checks beyond the auditor's general scoring — first five are hard (any failure → mandatory question in step 6, can't exit on auditor score alone), sixth is a soft warning.

**Hard:**

- **Success Criteria defined** — `# Success Criteria` section exists with ≥ 2 binary checkboxes.
- **Subtasks reach the goal** — `# Tasks` section (or equivalent) lists concrete steps that, if completed, produce the SC outcomes. If subtasks are missing or vague ("Implement feature" alone), flag.
- **E2E verify subtask present** — for shipping-class tasks (PR / release / plugin update / agent / deploy / library publish; or subtasks reference a git repo / marketplace / registry — see `task-writing.md` "Shipping Checklist"), `# Tasks` must include a subtask that runs the shipped artifact in its real environment. Four sub-checks on that subtask:

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

        A verify subtask passes when its body covers (a) at least one of the three shapes AND (b) a result a reader could independently confirm. **Both clauses required** — a procedure without an expected result is still vague. Vague fails: *"Verify the endpoint"* names a target but no action and no expected result; *"Verify it works"* names neither; *"run a check on the endpoint"* names a procedure shape but no expected result (the (b) clause fails). Concrete passes — HTTP: *"curl /widgets, confirm 200 + body matches schema"*; CLI: *"run `scenarios/release.md`, confirm exit 0"*; doc: *"open the rendered README, confirm the install table renders + Code-Of-Conduct link works"*; K8s: *"kubectl get pod foo, confirm Running + log contains 'startup complete'"*.

        LLM quality call (no verb list, no regex) — the rule above IS the anchor. Re-read it when in doubt; the procedure / observable / artifact taxonomy defines what concrete means here.

    3. **Falsifiable — the evidence must distinguish pass from fail.** Ask: *if this check passed tomorrow, what would I actually know?* Reject a criterion whose stated evidence could also be produced by a broken implementation. Sub-check 2 tests whether the evidence is **specific**; this one tests whether it **discriminates** — a criterion can be perfectly concrete and still prove nothing.

        The recurring shape is an **absence** assertion over a window shorter than the period of the event it claims to rule out: *"no writes in 15 min"* when writes occur every ~25 min passes on a no-op. Same failure with *"no errors in the log"* over a window where the error path is never exercised, or *"the file is unchanged"* when nothing would have written to it anyway.

        Two fixes, apply both when the shape appears:
        - **Widen the window past one full period** of the underlying event, so a real signal is guaranteed to fall inside it.
        - **Lead with a positive assertion** — the expected signal appears N times — and keep the absence as the secondary clause. A positive count cannot be satisfied by a no-op; an absence can.

        Fails: *"deploy with the flag on, watch 15 min, confirm no light changes"* (writes fire every ~25 min — passes on a build that ignores the flag entirely). Passes: *"watch ≥40 min spanning a write boundary, confirm ≥4 `skip` log lines naming the affected checks AND 0 `applied` lines"*.

        A second shape: **a constructed probe whose input shape the system never produces.** A case built by hand is evidence only if that case can occur. Ask: *does the input I exercise exist in the real population, or did I invent a shape the system cannot emit?* A probe outside the real shape passes for reasons unrelated to the claim, and it reads as a positive control — worse than having none, because the positive control is what the criterion leans on to show the check *can* fire.

        Worked case (2026-09-11): a criterion required flagging a worktree carrying commits beyond base, and a probe was hand-built to exactly that shape. It passed. But these repos merge with merge commits only (`allow_squash_merge=false`, `allow_rebase_merge=false`), so every real orphan's tip stays an *ancestor* of master, `git merge-base HEAD origin/master` returns `HEAD` itself, and the count reads **0** — the probe's shape and the real population were disjoint. A peer review caught it; this gate did not.

        Two fixes, apply both when the shape appears:
        - **Sample the population before trusting a hand-built case** — confirm the system actually emits that shape. Here one `git log --merges origin/master` over the real fleet would have shown every orphan is merge-committed.
        - **Assert on a signal the real path emits**, not the manufactured one — draw the evidence from the population the claim quantifies over.

        Sibling test: `/supervisor:worker-drive` § "Challenge the acceptance criteria" Axis B applies the same question — but only once work is already underway. This gate is the cheaper place to catch it.

    4. **Shape-matched — the claim's shape must fit the evidence's shape.** Sub-checks 2 and 3 interrogate the **probe**; this one interrogates the **claim**. Ask: *will the evidence that will exist support a verdict, or only a mechanism / an elimination?* A criterion demanding a frequency verdict — *"is this one-off or structural"*, *"does it recur on ordinary days"* — over a population that has aged out, or that has not yet accrued, is unsound however concrete its probe.

        **Repetition is the diagnostic.** The same criterion failing audits in *different* ways is the signature that the claim, not the probe, is wrong — not three separate defects, and not three probe patches. Observed 2026-09-21: an SC2 demanding a one-off-vs-structural verdict failed three consecutive audits three different ways — no probe; then a probe naming weeks that had already aged out, making "zero further orphans" trivially true; then a fallback tickable without running the sweep. Three probe patches, each satisfying this gate as written, none producing an answerable criterion. Reframing the claim — deriving the call from the *nature* of the identified cause rather than from a count — moved the audit 6 → 8 and flipped the adversarial-laziness pass to PASS.

        Two honest repairs, whichever the evidence supports:
        - **Reframe the claim to what the evidence can carry.** Derive a verdict from the *nature* of the identified cause (a dated event vs a standing property of the config or account) rather than from a count — or record an **eliminative** result: what was ruled out, by which probe, and what remains.
        - **State the investigation depth required**, when the verdict genuinely may not be reachable. The criterion then asserts the question was pursued to a named depth, not that it was answered.

        Never accept a fallback satisfiable without running the probe: *"undetermined"* must be recorded alongside the quoted result that establishes the loss, never asserted on its own.

        **Note the scope of this item.** It sits inside a block that is skipped for non-shipping-class tasks (next line), yet the defect it catches is not shipping-specific — the 2026-09-21 case was a diagnosis task, where this gate never ran. `agents/task-auditor.md` § 15 carries the same check and runs on every task; this item is the shipping-class enforcement, not the universal home.

    Skip this whole check for non-shipping-class tasks (pure research, decision, doc-only with no published artifact).
- **Subtask-goal alignment** — every `# Tasks` checkbox must either (a) map by topic to ≥ 1 `# Success Criteria` outcome, or (b) be the e2e verify subtask. Flag any orphan as a scope-creep candidate; in step 6 the owner can link it to an SC, move it to `# Out of Scope`, or split it into a separate task.
- **Blast radius named** — if any subtask pushes to a registry, deploys, mutates a cluster, or needs a credential/secret, the task must name the external system AND the account written to (e.g. *"pushes `docker.io/bborbe/<img>` under the bborbe Docker Hub account"*). A credential requirement with no named target is a scope gap: the owner discovers what was automated at the secrets request, after the work has shipped. Observed 2026-08-27 — a publish-on-tag CI was designed, merged and released; the owner objected (*"Is the agent trying to push a docker image? I don't think that I want this"*) only when its Docker Hub secrets were requested, costing two reversal PRs for a net deletion. Flag → mandatory question in step 6.
- **Resolution steps are resolvable** — a subtask naming a *lookup, join or key* (*"resolve the gate to its store `item_id`"*, *"look up the record by X"*) must be checked against whether that key actually exists and is derivable **by the actor that will run it**. This is the one hard check that is about the plan's *premise* rather than its shape: a subtask can name a concrete verb, a concrete artifact and a concrete outcome and still be unsatisfiable by construction. Ask of each: *do the two records share a field, and does the caller hold it?* Observed 2026-09-21 — a subtask read *"the sweep resolves the gate it is about to publish to a store `item_id`"*. No such join existed: the store wrote `item_id` itself, the producer's dedup key was a hash of a value the manager never holds, and the third candidate field was documented as `<session id or pane id>` — two kinds of value in one field, so it could not anchor an identity either. The same session's e2e-verify subtask also named no procedure and no expected result. **Two unsatisfiable-by-construction subtasks in one task is the signal that this check is missing, not that the author was careless** — and the cost is paid at execution, where the premise has already collapsed and the task file needs revising mid-flight. Flag → mandatory question in step 6.

- **Premise matches the ask** — when `# Impact` quotes the operator verbatim, re-read the quote against Summary + Success Criteria. Flag when the quote admits more than one reading, or when an SC/constraint excludes a plausible reading of it (e.g. *"must NOT be named X"* when the quote says X). A plan can score 9/10 on shape and still build the wrong thing — the task writer's interpretation is not the operator's words. Observed 2026-09-23: *"add a open skill"* was read as an open-items skill (name `open` banned); the operator meant moving `/open` → `/supervisor:open` — found only after release. Flag → mandatory question in step 6, quoting both readings.

**Soft:**

- **KISS ceiling** — if `# Tasks` has > 8 checkboxes, warn: *"task may be too large for one session — consider splitting, moving items to `# Out of Scope`, or promoting to a goal."* Owner decides; task can still proceed to execution.

Any hard check failing → mandatory question in step 6; can't exit on auditor score alone. Soft check failing → surfaced as a question in step 6 but doesn't block exit if owner says proceed.

### 6. Surface gaps + fix loop

**NO-ASK short-circuit:** under `--non-interactive` no question is asked, no fix loop runs, and nothing is `Edit`ed from an assumed answer. Carry the gaps forward to step 7's `⚠` branch as bullets. **The template-verdict check below still runs** — it is a lookup, not a question, and a current verdict is what lets a headless run reach `✅ Plan ready` instead of parking. Everything else in this step applies to ASK mode only.

**Worker sessions (ASK mode only) — send each question to your manager as well.** This step is where a spawned worker most often asks, and the `AskUserQuestion` call in the rules below reaches only whoever is sitting in this tab. If a manager session watches your topic, resolve it with `ListAgents` — an explicit name given at spawn wins; otherwise the row matching your task's topic (`<Topic>` or `<Topic> Manager`; your `goals:` name the goals, and the topic page listing them is your topic). Send the question with `SendMessage` too. The ask is what unblocks you; the send is what makes the question visible without the operator visiting this tab. **Never send a permission prompt** — a peer message cannot release a harness gate. Nothing resolves, or the tools are absent → just ask in this tab.

**Template-verdict check — run before translating any auditor finding.** A CR-materialized recurring instance is audited once per period, but the artifact the auditor is judging — its schedule template — does not change between periods, so the same template-level finding re-asks every week. Resolve a verdict before entering the fix loop:

**Run this check in both modes.** It is a lookup, not a question — and under NO-ASK a current verdict is exactly what lets a headless run reach `✅ Plan ready` instead of parking on a gap report.

1. **Take the slug from the auditor's report.** On a CR-materialized task the auditor emits a `**Template**: \`<slug>\`` line and labels every finding `[template-level]` or `[instance-level]`. No `**Template**` line → the task is not CR-materialized → skip this whole block. The auditor owns detection (see `agents/task-auditor.md` § Finding Classification); do not re-derive the shape here.
2. **Hash the template as materialized** — the instance body with the substituted period values normalized out:
   ```bash
   awk 'BEGIN{n=0} /^---$/{n++; next} n>=2' "<instance-file>" \
     | sed -E 's/[0-9]{4}-[0-9]{2}-[0-9]{2}/<DATE>/g; s/[0-9]{4}W[0-9]{2}/<PERIOD>/g' | shasum
   ```
   The normalization is deliberately blunt: every ISO date in the body is masked, so a template edit that only rewrites a date literal will *not* invalidate a verdict. That is the accepted trade — hashing the raw body would instead re-ask on every period roll, which is the defect this check exists to remove.
   **Frontmatter is stripped for the same reason, and it is not optional.** The whole file would fold in the per-instance `task_identifier` UUID the creator writes, so every materialization would hash differently and a recorded verdict could never match — the comparison would be *unreachable*, not merely flaky.
   **Carry this item-2 value through — never re-hash.** A verdict must carry the hash of the *fresh materialization*, the body as it stood when item 2 ran. The most common answer to a first adjudication is "fix the instance + port to YAML", and the fix loop below applies those edits to the instance **before** the verdict is written — so "the body as it is now" is the *corrected* body, which no future materialization can reproduce. Every later period is generated from the still-uncorrected template and hashes differently, so the comparison becomes *unreachable* and the verdict re-asks every period — the exact cost this check exists to remove. Observed in production 2026-09-22: a verdict recorded after a local `ls -A` correction re-asked every period until the page was re-baselined by hand.
   **A verdict can only be baselined from a *fresh* instance — settle this before branching.** The item-2 value is stable across periods only because a fresh materialization carries the template's body verbatim: boxes unticked, `# Progress` holding nothing but its `Scheduled` line. An instance that has already been worked no longer matches what the template emits, so any verdict written or re-baselined from its body is unreachable for exactly the same reason — and a worked instance's hash necessarily *differs* from the recorded one, so it would otherwise route into the `differs` branch and re-baseline from precisely the wrong body. When `plan-task` runs against an already-worked instance, **no branch below may write or update a verdict, and no skip may be taken on the strength of a comparison made against a worked body**: print `ℹ️ Instance already worked — not a valid verdict baseline` (under NO-ASK, emit it as the single gap bullet), leave any existing verdict untouched, and ask the auditor's findings as normal.
3. **Look for the verdict file** at `<vault>/50 Knowledge Base/Recurring Template Verdicts/<slug>.md` — a vault-relative directory, so it travels with the vault on sync. Branch on what you find:
   - **Verdict present, `body_hash` matches** → the template is unchanged and already adjudicated. **Skip the auditor-derived questions**: do not translate auditor findings into questions, do not enter the fix loop for them. Print `ℹ️ Template verdict current for <slug> (recorded <date>) — auditor findings already adjudicated; no re-ask.` and go straight to step 7. A current verdict also satisfies step 7's score gate for the findings it covers; without that clause an adjudicated 7/10 would still exit on the `⚠` branch and the skip would buy nothing.
     **Order matters.** Step 5's hard non-negotiables are checked first and always apply in full — they are structural, independent of template content, and a failure there is a different defect from the one the verdict covers. Take the skip only on a clean pass.
   - **Verdict present, `body_hash` differs** → the template changed since the verdict. Raise **exactly one** question: does the new template state supersede the recorded verdict? On the answer, update `body_hash` to the **item-2 value** and the verdict text, then continue to step 7. Under NO-ASK, emit this as the single gap bullet instead of asking. This is the only case in which a verdicted template asks.
   - **No verdict** → normal path. After the fix loop completes, **record the verdict** — write `<vault>/50 Knowledge Base/Recurring Template Verdicts/<slug>.md` carrying `template_slug`, `template_source`, the **item-2 `body_hash`**, the date, and what was adjudicated. When the answer is to port the change into the source YAML, **auto-file exactly one follow-up task** naming the slug:
     ```
     Task tool with subagent_type: 'vault-cli:task-creator',
       prompt: '<slug> — port the adjudicated change into its schedule template --non-interactive'
     ```
     Dedupe by slug: if the verdict file already records a filed follow-up, or an open task already names the slug, file nothing — and later periods of that template ask nothing.

Only findings the auditor labelled `[template-level]` are verdict-coverable. An `[instance-level]` finding always asks, verdict or not.

Translate findings (auditor + non-negotiable checks) into questions. Rules:

- Max 3 questions per turn
- Each question is short (one sentence) + tight options (single yes/no OR 2-4 numbered options)
- Lead with `(Recommended)` per global UX
- Quote the offending line/section so owner sees what triggered the question
- Use `AskUserQuestion` for the actual ask

Apply each answer via `Edit` — re-running the step 2 ownership gate first (see there). Re-run auditor after each batch. Print delta `Score: X → Y`. Loop until score ≥ 8 AND all five hard non-negotiables pass OR owner says "good enough." A current template verdict (see the template-verdict check above) satisfies the score gate for the findings it covers.

### 7. Exit — hand off to execute-task (no phase flip)

**plan-task never flips the phase.** It validates and reports; `/vault-cli:execute-task` owns the `planning → execution` transition. This keeps each lifecycle command to one job and makes "start executing" a deliberate operator action.

**Phase is `planning` AND (score ≥ 8 OR a current template verdict covers the auditor findings) AND hard non-negotiables pass:**

Print: `✅ Plan ready. Score: X/10. Phase stays: planning. → Run /vault-cli:execute-task to begin execution.`

**Phase is already past planning (execution / ai_review / human_review / done):**

Print: `✅ Task sharpened. Score: X/10. Phase unchanged (was <phase>).`

**Owner abort OR (score < 8 after loop AND no current template verdict covers the findings) — OR any unresolved gap (NO-ASK mode):**

Print: `⚠ Task improved to X/10. Phase unchanged. Remaining: <bullets>. Re-run /vault-cli:plan-task when ready.`

Under NO-ASK the score is whatever the auditor returned (nothing was fixed, so there is no "improved to"); print `⚠ Plan not ready. Score: X/10. Phase unchanged. Remaining: <bullets>. Answer these on resume, then re-run /vault-cli:plan-task.` Each bullet is the question that would have been asked, so the operator can answer it directly.

## Notes

- **Scope is focused on what blocks safe execution.** Plan-task enforces five hard planning-gate checks (SC defined, subtasks reach goal, e2e verify subtask, subtask-goal alignment, blast radius named) plus the KISS ceiling as a soft sixth, because each one prevents a specific failure mode: missing outcomes, missing path, dishonest-tick verification, scope creep, an unnamed blast radius, oversize task. Other heuristics (MVP framing, Out-of-Scope capture quality, evidence shape) stay in `task-auditor` and `task-writing.md` as canonical rules — surfaced via the auditor in step 4, not promoted to dedicated gates. Letting the auditor enforce general structure while plan-task enforces the five named gates keeps the command short and the gates legible.
- **Questions stay tight, with consequence visible.** 2-3 lines of setup → short options. "Tight" doesn't mean stripping context — owner must see what each answer *changes*. Quote the offending line, name the trade-off, then options.
- **Subtask granularity = session-sized.** When proposing or sharpening `# Tasks` items, target *work-block size* (a session's worth of work), not CLI-step size. Aim for 3-6 items per task. Reject auditor-suggested over-decomposition like "run precommit / open PR / merge PR" as separate subtasks — those collapse into one "ship the change" block.
- **Reads `~/.claude/plugins/marketplaces/vault-cli/docs/task-writing.md` as the canonical rule source** — same rules `task-auditor` enforces.
- **Conversational on purpose.** Owner is the judge of substance. Plan-task never silently rewrites; every change comes from an explicit answer. This is exactly why `--non-interactive` refuses to guess: with no owner to answer, the honest move is to stop and list the gaps, never to invent answers and edit the task.
- **Entry contract.** On a fresh task (`status: next, phase: todo`), plan-task flips to `in_progress, planning` itself. No `/work-on-task` prerequisite.
- **No phase flip.** plan-task never transitions phase; it validates and hands off to `/vault-cli:execute-task`, which owns the `planning → execution` flip. Entry-contract flips (`next` → `in_progress` + `planning`) still happen in step 3.
- **Mechanical fixes stay in `/audit-task`.** This command is for substance (SC, subtasks, goal alignment), not formatting.

## Integration

Task lifecycle:

1. `/vault-cli:create-task` — capture (lenient)
2. `/vault-cli:work-on-task` — orient (status + guides + daily note), then auto-chain into this command (both modes; headless callers pass `--non-interactive` through)
3. **`/vault-cli:plan-task`** — sharpen (5 hard gates); never flips `planning → execution` — this command
4. `/vault-cli:execute-task` — gate planning → execution; flips phase + prints first subtask + DoD reminder
5. Start work — while working, use any of:
   - `/vault-cli:update-task` — log completed work, sync to daily note / parent goal
   - `/vault-cli:task-status` — grouped-checkbox status (Success Criteria / Tasks / DoD) + next step
   - `/vault-cli:next-steps` — next actionable steps; offer defer if nothing left today
6. `/vault-cli:sync-progress` — flush conversation to daily note + task pages
7. `/vault-cli:complete-task` — close task
8. `/vault-cli:session-close` — verify session is safe to end (synced, committed, no orphaned state)

Output ends with one of:
- `✅ Plan ready. Score: X/10. Phase stays: planning. → Run /vault-cli:execute-task.` (planning success)
- `✅ Task sharpened. Score: X/10. Phase unchanged (was <phase>).` (non-planning success)
- `⚠ Task improved to X/10. Phase unchanged. Remaining: <bullets>. Re-run when ready.` (partial, ASK mode)
- `⚠ Plan not ready. Score: X/10. Phase unchanged. Remaining: <bullets>. Answer these on resume, then re-run /vault-cli:plan-task.` (partial, NO-ASK mode)
- `❌ Task not found.` / `❌ Pass a task identifier or name.` / `❌ Ambiguous task identifier — pass an exact path.` (input error)
