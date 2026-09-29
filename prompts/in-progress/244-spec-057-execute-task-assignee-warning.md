---
status: approved
spec: [057-task-approve-assignee]
created: "2026-09-28T22:04:01Z"
queued: "2026-09-28T22:23:56Z"
---

# Warn at the execution gate when the task has no owner (spec 057, prompt 2 of 2)

<summary>
- The planning → execution gate now tells the operator when a task is about to enter execution with nobody owning it.
- The gate prints one warning line naming the empty assignee, and then proceeds exactly as before — the phase still flips.
- The warning is a notice, never a block: an empty assignee is the escalation channel an agent uses to hand work back to the operator, so the gate must not close it.
- When the task already has an owner, the gate prints nothing — the line lives inside a branch, not in the always-printed output.
- The changelog records both halves of this feature: the approval now fixes the owner, and the gate now warns when there is none.
- The plugin version is deliberately not touched — the releaser owns the version bump and the tag on this repo.
- This prompt is markdown only; it depends on prompt 1 for the wording of the rule it warns about, not for any code.
</summary>

<objective>
Make the planning → execution gate surface the one thing approval could not guarantee: a task with no owner. `/vault-cli:execute-task` inspects `assignee` and, when it is empty, prints a single `assignee is empty` line before it flips the phase — then flips the phase anyway. This prompt covers spec 057 ACs 5 and 6 and Desired Behaviors 5 and 6, and adds the `## Unreleased` changelog bullets for both prompts of this spec. It depends on prompt 1 only for wording: the warning describes the owner rule that prompt 1 establishes, and the two prompts touch disjoint files.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully before editing:

- `commands/execute-task.md` — the whole file. You are editing it. Note its shape: a numbered process (1 resolve path, 2 read status + phase, 3 refusals, 4 status entry contract, 5 the four hard non-negotiables, 6 phase transition or refusal, 7 surface first subtask + DoD), a `## Non-interactive contract` section, and a `## Notes` section. The file is written in an imperative, operator-facing voice with fenced `bash` blocks for the commands it runs; match that voice and form. **Do not renumber the existing steps** — the new content goes inside steps 2 and 6.
- `docs/task-writing.md` — the `assignee` semantics section (search for the paragraph that begins "`assignee` semantics:" and the following "two different meanings depending on how it became empty" paragraph). Prompt 1 added the approve-time owner rule there; your warning line describes that same rule, so read it so the two agree. The guide is the canonical rule source this command already cites.
- `CHANGELOG.md` — the top of the file, down to and including the `## v0.153.0` heading. Note the existing `## Unreleased` section, its single existing bullet, and the bullet house style: a lowercase type prefix (`feat:` / `fix:` / `docs:`), backticked identifiers and command names, a full sentence of what changed and why, and a trailing `Change set: <files>` clause.
- `commands/plan-task.md` — the sibling gate command, for tone and formatting conventions only. Do not edit it.

Coding-plugin docs (in-container path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the `## Unreleased` bullet conventions for this repository family.
</context>

<requirements>
1. **Read the task's `assignee` in step 2 of `commands/execute-task.md`.** Step 2 ("Read status + phase") already runs two `vault-cli task get` commands with `--output json`. Add a third read in the same style:

   ```bash
   vault-cli task get "<name>" assignee --output json
   ```

   The value is needed by step 6's owner check. Do not add a new numbered step.

2. **Print the warning in step 6, on the pass path only, and keep the transition.** In step 6's "**If all hard checks pass AND `phase: planning`**" branch — the branch that runs `vault-cli task set "<name>" phase execution` — add an owner check immediately before the phase-flip command:

   - **When `assignee` is empty**, print exactly one line containing the literal string `assignee is empty`, for example:

     ```
     ⚠️ assignee is empty — "<name>" has no owner; assign one with `vault-cli task set "<name>" assignee "<owner>"`.
     ```

   - **Then proceed**: the phase transition still runs, and the rest of step 6 and step 7 are unchanged. Spec Desired Behavior 5 — the gate warns, never blocks. An empty `assignee` is the escalation channel an agent uses to hand a task back to the operator (see `docs/task-writing.md` § `assignee`), so blocking it would close that channel; the surrounding prose must say this, so the reader knows the omission is deliberate.
   - **When `assignee` is non-empty, print nothing.** Spec Desired Behavior 6 — the line lives inside a branch, never in the always-printed output.

3. **Keep the branch condition on the adjacent lines.** Spec AC 6 is checked with a bounded context grep (`grep -B2 -A2 'assignee is empty'`), so the line immediately above the literal (and the line above that, if the lead-in wraps) must name the empty-assignee condition — a short bolded lead-in such as "**Owner check (warn, never block).** If `assignee` is empty, print one line and continue:" placed directly above the print. Do not bury the literal in a paragraph of unrelated prose, and do not print it unconditionally.

4. **Leave the idempotent branch alone.** Step 6's "**If `phase: execution` / `ai_review` / `human_review`**" branch is out of scope — it neither flips the phase nor checks anything, and it must stay that way. The warning belongs to the transition, not to a re-entry that performs none.

5. **Add the `## Unreleased` changelog bullets for both prompts of spec 057.** In `CHANGELOG.md`, under the existing `## Unreleased` heading (leave the existing bullet in place; add yours beneath it), add two bullets in the file's established style:

   - the approval now fixes the owner: `vault-cli task approve` resolves an assignee and writes it in the same single write that moves a task `todo → planning` — filling an empty `assignee` from `current_user`, keeping an already-set assignee, and refusing (writing nothing) when neither the new `--assignee <name>` flag nor the task's existing assignee nor `current_user` names an owner; an empty `--assignee` is treated as "flag not given", and clearing an assignee after approval is unchanged so the escalation channel stays open. `Change set:` names `pkg/ops/task_approve.go`, `pkg/ops/task_approve_test.go`, `pkg/cli/cli.go`, `mocks/task-approve-operation.go`, `integration/cli_test.go`, `docs/task-writing.md`.
   - the execution gate now warns on an empty assignee: `/vault-cli:execute-task` prints one `assignee is empty` line before it flips `planning → execution` when the field is blank, and prints nothing when it is set; the transition still happens. `Change set:` names `commands/execute-task.md`.

   Read the code prompt 1 wrote before describing it — the bullet must be true of the shipped behaviour, not of the spec's intention.

6. **Do not bump any version and do not tag.** Spec Constraints: this repo is `autoRelease: true` via `.maintainer.yaml`, so the releaser owns the version bump and the tag. Leave `CHANGELOG.md`'s top `## vX.Y.Z` heading, `.claude-plugin/plugin.json`, and both version fields in `.claude-plugin/marketplace.json` exactly as they are. Adding an `## Unreleased` section does not move the top `## v` heading, so the four version strings stay aligned and `check-versions` is unaffected.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Note that in-container `git` does not work in this project directory (`.git` is a `gitdir:` pointer file): any `git` command fails with `fatal: not a git repository` and the daemon records a pass regardless. Use non-git checks; never stage or commit.
- **Warn, never block** (spec Non-goal: "Blocking (rather than warning) at the planning → execution gate" is out of scope). The phase transition must still happen on an empty assignee — spec AC 5 asserts the phase flips.
- **The warning is conditional.** With `assignee` set, the gate's output contains zero occurrences of the line (spec AC 6).
- **Do not change the `assignee` semantics** documented in `docs/task-writing.md` — the unclaimed-inbox-at-creation and park-at-clear readings stay exactly as they are (spec Non-goal). This prompt writes markdown that *describes* the rule prompt 1 established; it does not change the rule.
- **Do not change the four hard non-negotiables, the refusal cases, the status entry contract, or the subtask classification** in `commands/execute-task.md`. The only additions are the `assignee` read in step 2 and the owner check in step 6.
- **Do not hand-bump the plugin manifests or `git tag`.** The four version strings stay untouched; the releaser owns the bump.
- **Do not edit any Go file.** This prompt is markdown only: `commands/execute-task.md` and `CHANGELOG.md`.
- Existing tests must still pass.
</constraints>

<verification>
PRIMARY GATE — evidence greps. Absence rows are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -n 'assignee is empty' commands/execute-task.md                  # >= 1 (spec AC 5)
grep -B2 -A2 'assignee is empty' commands/execute-task.md             # the surrounding lines name the empty-assignee condition (spec AC 6)
grep -n 'assignee' commands/execute-task.md                           # >= 2 (the step 2 read and the step 6 check)
grep -n 'task get "<name>" assignee' commands/execute-task.md         # >= 1 (the read step exists)
grep -c '^## Unreleased' CHANGELOG.md                                 # exactly 1 (the section is not duplicated)
```

CHANGELOG PLACEMENT — the new bullets must sit under `## Unreleased`, not folded into a released `## vX.Y.Z` section. Match only the two new bullets' own distinctive wording — the bare token `--assignee` also appears in two pre-existing released entries (`## v0.48.1`, `## v0.5.0`), so a pattern matching it reads as a false fold. The section-walking form is authoritative:

```
awk '/^## /{sec=$0} /resolves an assignee|assignee is empty/{print sec" :: "$0}' CHANGELOG.md
```
Every printed line must read `## Unreleased :: ...`. If a line reads a released heading, that bullet landed in the wrong section — move that bullet back under `## Unreleased`. Never move a pre-existing released entry: only lines matching the two new bullets' wording are yours.

VERSION-STRING GUARD — the manifests and the top released heading must be untouched. Use non-git checks only: this project directory is itself a git worktree (`.git` is a `gitdir:` pointer file), so in-container git fails with `fatal: not a git repository` and the daemon records a pass regardless — a `git` row that dies is worse than no row, because it reads as a pass.
```
grep -c '"version": "0.153.0"' .claude-plugin/plugin.json        # exactly 1
grep -c '"version": "0.153.0"' .claude-plugin/marketplace.json   # exactly 2
grep -m1 '^## v' CHANGELOG.md        # still prints ## v0.153.0 — the Unreleased section did not move it
```

FULL GATE — `make precommit` at the repo root must exit 0. It includes `check-changelog`, which verifies that the `# Changelog` title and the "All notable changes…" preamble still precede every `## ` section — adding bullets must not disturb that order. `make precommit` does **not** run `check-versions` (release-time only, `make release-check`), which is exactly why the version strings must stay as they are.

SELF-CHECK before finishing: re-run the PRIMARY GATE greps and the FULL GATE, and walk spec 057's ACs 5 and 6 against the change — AC 5 the warning line exists (the phase-flip half is operator-side per spec 057's Verification ladder), AC 6 the line is inside a branch so a task with an owner produces none. Confirm no Go file was touched and the four version strings are unchanged.
</verification>
