---
status: completed
spec: [034-prepare-compact-plugin-command]
summary: 'Created /vault-cli:prepare-compact as a first-class plugin command (six-step port with granular capability-scoped allowed-tools, per-session checkpoint file, verdict, no closer panel) and added the ## Unreleased changelog bullet; make precommit exits 0'
execution_id: vault-cli-prepare-compact-exec-185-spec-034-prepare-compact-plugin-command
dark-factory-version: dev
created: "2026-08-19T09:47:00Z"
queued: "2026-08-19T10:23:57Z"
started: "2026-08-19T10:24:28Z"
completed: "2026-08-19T10:28:13Z"
branch: dark-factory/prepare-compact-plugin-command
---

<summary>
- Promotes the compaction-safety checklist from the per-machine commands directory into the vault-cli plugin as a first-class slash command, so it ships to every install via the normal plugin update path instead of hand-copying.
- The full six-step checklist is carried over: sync-progress call, goal/task sweep, compact-safety checks (git state, live background processes, daemon, containers, unanswered gates), a 4-field `RESUME AFTER COMPACT` block, a per-session `compact-checkpoints/<session-id>.md` resume file, and a ready/not-ready verdict.
- The command becomes capability-restricted: `allowed-tools` grants only the six read-only/status Bash subcommands the body actually invokes plus `mkdir` for the checkpoint directory — no unscoped `Bash`, no `Edit`, no `AskUserQuestion` — so it can never commit, push, or kill anything even if its prose were misread.
- It stays read-and-report-only by both prose and tool surface; the only writes are vault progress (delegated to the `vault-cli:sync-progress` sibling) and the per-session checkpoint file.
- The `sync-progress` call and the `session-close` contrast read as same-plugin sibling references; the session-close-style closer panel is forbidden and the prohibition survives verbatim.
- The changelog records the new command under `## Unreleased` with a `feat:` entry; no version strings or plugin manifests are touched (the github-releaser owns bumps, `autoRelease: true`).
- Reviewer caveat: the original command file lives outside this repository and is not readable in the execution container, so the body is reconstructed from the spec's Desired-Behavior + Acceptance-Criteria contract (which quotes every load-bearing string). Worth a one-line diff against the operator's original file before approving.
</summary>

<objective>
Create `/vault-cli:prepare-compact` as a first-class vault-cli plugin command that ships and updates exactly like `/vault-cli:session-close` and `/vault-cli:sync-progress`: a faithful port of the six-step pre-compaction checkpoint with a granular, capability-scoped `allowed-tools` list, plus a `## Unreleased` changelog bullet. No other file in the repository changes, no version strings move.
</objective>

<context>
Read `/workspace/CLAUDE.md` and `/workspace/docs/development-patterns.md` for project conventions.

**Content contract for the port.** The pre-plugin source command lives outside this repository and is NOT mounted in the execution container — there is no source file to read here. The authoritative content contract is the spec at `/workspace/specs/in-progress/034-prepare-compact-plugin-command.md` — read it in full. Its Desired Behavior section (six steps + the exact `allowed-tools` list + namespace-fit rules), its Acceptance Criteria (which quote every load-bearing string the port must contain), its Security section (session-id trust boundary), and its Constraints are the ground truth for this prompt. Reconstruct the command body from that contract; do not invent steps, tool names, a closer panel, or extra fields.

**House style — read these sibling command files in full before writing:**
- `/workspace/commands/session-close.md` — the granular `allowed-tools` list form (`- Bash(git status:*)` etc.), the inline-command preamble, the "must stay inline" note, and the state-closer panel shape that prepare-compact must NEVER emit. Its frontmatter is the closest structural model for the new file (minus the tools prepare-compact is not granted).
- `/workspace/commands/sync-progress.md` — the sibling skill invoked in step 1; note the house-style invocation `Skill: vault-cli:<name>` (see its Phase 4b) and its own granular `allowed-tools` list.
- `/workspace/commands/update-task.md` — a short sibling showing the minimal frontmatter form.

**Coding plugin** (in-container paths — the YOLO container mounts these at `/home/node/.claude/plugins/marketplaces/coding/docs/`):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `feat:` prefix for a new command, `## Unreleased` section placement rules.
- `/home/node/.claude/plugins/marketplaces/coding/docs/claude-code-skill-writing-guide.md` — slash-command file conventions (frontmatter, body structure).

**Spec verified before writing this prompt** — the relevant spec is `/workspace/specs/in-progress/034-prepare-compact-plugin-command.md` (read in full). The spec's Suggested Decomposition is a single prompt; this prompt implements it in one pass.
</context>

<requirements>

### File 1: `/workspace/commands/prepare-compact.md`

1. **Create the file** at `/workspace/commands/prepare-compact.md`. The frontmatter must be exactly this block, verbatim (including the em-dash in `description`):

   ```yaml
   ---
   description: Pre-compaction checkpoint — sync vault progress, surface live background state, write a resume block
   allowed-tools:
     - Skill
     - Read
     - Write
     - Glob
     - Bash(git status:*)
     - Bash(git log:*)
     - Bash(git rev-parse:*)
     - Bash(pgrep:*)
     - Bash(dark-factory status:*)
     - Bash(docker ps:*)
     - Bash(mkdir:*)
   ---
   ```

   This list is a hard contract, not a suggestion:
   - Non-Bash tools are exactly `Skill`, `Read`, `Write`, `Glob`. Do NOT add `Grep`, `Edit`, `Task`, `AskUserQuestion`, or any other non-Bash tool. (`Write` covers the step-5 checkpoint file; `Skill` covers the step-1 sibling invocation.)
   - Bash tools are exactly the six scoped read-only/status subcommands above plus `Bash(mkdir:*)`. There must be NO bare `Bash` entry and no other `Bash(...)` scope — the command must not be able to commit, push, or kill anything.
   - Do NOT add a `vault-cli:*` Bash scope; step 1 delegates to the `Skill` invocation, it does not shell out.
   - `description` MUST start with the literal prefix `Pre-compaction checkpoint` (an Acceptance Criterion greps `^description: Pre-compaction checkpoint`).

2. **Body — purpose and invariant.** After the frontmatter, write an opening paragraph stating that this is the pre-`/compact` checkpoint: sync vault progress, surface live background state, write a resume block. State that this command runs inline in the parent conversation (it must stay inline, like its siblings — it reads the session's state, so a sub-agent cannot see it). Include the read-and-report-only invariant as a verbatim sentence: `Never auto-commit, auto-push, kill a daemon`. (Acceptance Criterion greps for that exact string.)

3. **Step 1 — sync-progress as an in-plugin sibling.** The first step invokes the sibling command exactly once as `Skill: vault-cli:sync-progress` (the house-style invocation form used by `sync-progress.md`'s Phase 4b). The literal string `vault-cli:sync-progress` must appear in the file. Do NOT add any fallback or graceful-degradation logic for a missing skill — the skill ships in the same plugin bundle and is guaranteed present; the call is an ordinary same-plugin sibling reference.

4. **Step 2 — goal/task sweep.** This step must contain the literal sentence fragment `Sweep every goal` (an Acceptance Criterion greps it). Content: sweep every goal and task touched in this session; for each, confirm the page's Status Summary / Progress / Next line and its checkboxes reflect reality (tick what is actually done, note what is not). This is the second and last write step — progress updates only, via the sync-progress call in step 1 and these page edits.

5. **Step 3 — compact-safety checks.** This step MUST contain the literal heading text `Compact-safety checks` (an Acceptance Criterion greps it). It checks what compaction would otherwise silently drop, in this order:
   - **Git state:** uncommitted work (`git status`) and un-pushed commits (`git log` against the upstream computed via `git rev-parse`) — each guarded so a non-git directory reports gracefully instead of aborting.
   - **Live background state:** background shells / sub-agents / watchers via `pgrep`; the dark-factory daemon via `dark-factory status`; running containers via `docker ps`.
   - Include a fenced bash snippet running the mechanical checks with real `|| echo "..."` fallbacks for absent tooling. The daemon check and the container check MUST appear as literal shell wiring inside that snippet, exactly:
     ```bash
     dark-factory status || echo "no daemon"
     docker ps || echo "no containers"
     ```
     The file must therefore contain the literal strings `|| echo "no daemon"` and `|| echo "no containers"` (both are separately grepped — they must exist as fallback wiring after the actual commands, not merely as prose).
   - Include verbatim the warning `A stopped daemon does not mean stopped work` and the narrative `A running dark-factory daemon is worth pausing for` (both are separately grepped; the second is the "worth pausing for compaction" framing).
   - Include unanswered-decision detection under the literal term `Unanswered gates` (grepped) — an inventory of decisions the session raised but never answered, so they survive the compact.
   - Failure behavior: if a tool is absent on the operator's machine, each check falls back to its `|| echo "..."` text and the checklist CONTINUES and reports the tool as absent — it must never abort the command.

6. **Step 4 — resume block.** This step MUST contain the literal heading `RESUME AFTER COMPACT` (grepped). Emit a 4-field block whose field labels appear exactly once each in the file, verbatim:
   ```
   Next action:
   Live background:
   Un-pushed / uncommitted:
   Open decision:
   ```
   These four labels are the frozen resume-block schema — do not rename, reword, add, or remove a field, and do not use any of the four label strings anywhere else in the file (an Acceptance Criterion counts exactly 4 lines matching them). Populate each from the findings of steps 2–3.

7. **Step 5 — per-session checkpoint file.** Write the resume state to a checkpoint file whose path template appears verbatim: `~/.claude/compact-checkpoints/<session-id>.md` (grepped as `compact-checkpoints/<session-id>.md`). Requirements:
   - `<session-id>` is derived from the session's own scratchpad path — never from user input, conversation text, or file content. This is the Security trust boundary: a user- or file-controlled session-id would allow writing to an arbitrary filename. (If the checkpoint directory does not exist, create it via the granted `Bash(mkdir:*)` scope.)
   - State the per-session rationale verbatim: `Per session, not a single fixed path` (grepped) — two concurrent sessions in the same project each write their own `<session-id>.md` and never clobber each other.
   - State that the file lives under `~/.claude/` — outside every repo on purpose — so it survives compacts and is never committed to any worktree.
   - The checkpoint write is performed with the `Write` tool (the granted `Write` scope); this is one of only two write paths in the command (the other being step 1–2 vault progress updates).
   - The step must instruct the session to announce the file: include verbatim `state the full path in your final message` (grepped) — a checkpoint that is written but never announced defeats its purpose.

8. **Step 6 — verdict.** End with the verdict step. It MUST contain both verdict strings verbatim: `✅ Ready to compact` and `⚠️ Not compact-safe yet` (both grepped). Trigger conditions: `Ready to compact` when all checks pass — nothing uncommitted/un-pushed, nothing live in the background, no unanswered gates — and the resume block + checkpoint are in place; `Not compact-safe yet` when any open item exists (uncommitted work, live background process, unanswered gate), and in that case the session must name the open items so they are resolved or deliberately carried.

9. **No closer panel; explicit sibling contrast.** The file MUST contain exactly one `⚪ DONE` occurrence in its entire length, and that single occurrence must live inside the prohibition sentence — there must be NO actual session-close-style closer panel anywhere. Concretely:
   - Include a prohibition sentence beginning with the literal `Do NOT emit a` (grepped) that forbids the session-close closer panel, e.g.: "Do NOT emit a session-close-style closer panel — no `⚪ DONE` block; this command reports and writes, and the session continues." This is the only place `⚪ DONE` appears.
   - The literal string `👤 You:` must appear ZERO times in the file — do NOT quote or reproduce the closer panel shape anywhere (an Acceptance Criterion counts `👤 You:` == 0).
   - Make the continuing-vs-ending contrast with the sibling explicit and correctly namespaced: the file must contain the literal `/vault-cli:session-close` (grepped), e.g. "Unlike `/vault-cli:session-close`, which ends the session, `/vault-cli:prepare-compact` only pauses it for compaction — the session continues afterward." The file must NOT contain any un-namespaced `/session-close` (always the fully-namespaced form).

10. **No other content.** Do not add any `## Phase N` heading style that implies a closer, any `vault-cli` invocation other than the step-1 sibling reference and the session-close contrast, any AskUserQuestion flow, or any Bash scope beyond the frontmatter list. The command reports and writes only; it never asks, never mutates git/docker.

### File 2: `/workspace/CHANGELOG.md`

11. **Add the `## Unreleased` bullet.** The file currently has NO `## Unreleased` section — its newest section is `## v0.112.0`. Insert a new section directly between the "All notable changes…" preamble block and `## v0.112.0`, exactly:
    ```
    ## Unreleased

    - feat: add `/vault-cli:prepare-compact` plugin command — a pre-`/compact` checkpoint that syncs vault progress (via the sibling `Skill: vault-cli:sync-progress`), sweeps touched goals/tasks, runs compact-safety checks (git status/log, live background processes, daemon state, containers, unanswered gates), emits a 4-field `RESUME AFTER COMPACT` block, writes a per-session `~/.claude/compact-checkpoints/<session-id>.md` resume file, and returns a `✅ Ready to compact` / `⚠️ Not compact-safe yet` verdict. Promoted from the per-machine commands directory into the plugin bundle; `allowed-tools` is now a granular per-capability list (no unscoped `Bash`), keeping the command read-and-report-only — it never auto-commits, auto-pushes, or kills anything.
    ```
    Do NOT create a `## vX.Y.Z` section, do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` (all four version strings stay at `0.112.0`), and do NOT tag — the github-releaser owns version bumps for this repo (`.maintainer.yaml` sets `autoRelease: true`) and converts this `## Unreleased` bullet post-merge.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Must not modify anything outside this repository — the fate of the local duplicate of this command on the operator's machine is explicitly out of scope for this spec.
- Must not widen `allowed-tools` beyond the exact list in requirement 1 — no bare `Bash`, no `Edit`, no `AskUserQuestion`, no `vault-cli:*` Bash scope. This is the capability-enforced invariant the spec exists to preserve, not a tunable.
- Must not change the 4-field resume-block schema, the checkpoint path template, or the verdict wording — these are frozen by the original command's behavior contract.
- Must not emit any session-close-style closer panel: `👤 You:` must appear zero times and `⚪ DONE` exactly once (in the prohibition sentence).
- The only files that may change are `commands/prepare-compact.md` and `CHANGELOG.md` — no other `commands/*.md`, no `.claude-plugin/*.json`, no Go files.
- No scenarios are required for this change: no binary-relevant file changes (no `.go`, `.mod`, `.sum`, or `Makefile` diffs), so the scenario-walk would add no signal. The spec's post-release `/vault-cli:prepare-compact` smoke check is an operator-side step handled after merge, not by this prompt.
- `make precommit` must exit 0.
</constraints>

<verification>
Run all of the following from `/workspace`. Every `grep` uses the exact command form given; a count that differs from the stated expectation means the requirement is not met — fix the file, then re-run.

The file exists:

```
ls commands/prepare-compact.md
```

Frontmatter is granular and complete:

```
grep -c '^  - Bash$' commands/prepare-compact.md
grep -cE 'Bash\((git status|git log|git rev-parse|pgrep|dark-factory status|docker ps)' commands/prepare-compact.md
grep -cE '^  - (Skill|Read|Write|Glob)$' commands/prepare-compact.md
grep -c 'Bash(mkdir:' commands/prepare-compact.md
grep -c '^description: Pre-compaction checkpoint' commands/prepare-compact.md
```

Expect `0`, `6`, `4`, `1`, `1` respectively.

All six steps are present:

```
grep -c 'vault-cli:sync-progress' commands/prepare-compact.md
grep -c 'Sweep every goal' commands/prepare-compact.md
grep -c 'Compact-safety checks' commands/prepare-compact.md
grep -c 'A stopped daemon does not mean stopped work' commands/prepare-compact.md
grep -c 'RESUME AFTER COMPACT' commands/prepare-compact.md
grep -cE 'Next action:|Live background:|Un-pushed / uncommitted:|Open decision:' commands/prepare-compact.md
grep -c 'compact-checkpoints/<session-id>.md' commands/prepare-compact.md
grep -c 'Per session, not a single fixed path' commands/prepare-compact.md
grep -c 'Ready to compact' commands/prepare-compact.md
grep -c 'Not compact-safe yet' commands/prepare-compact.md
grep -c 'Never auto-commit, auto-push, kill a daemon' commands/prepare-compact.md
grep -c 'Unanswered gates' commands/prepare-compact.md
grep -c 'A running dark-factory daemon is worth pausing for' commands/prepare-compact.md
grep -c 'state the full path in your final message' commands/prepare-compact.md
```

Expect `1` or higher for each of the first five and last six; the field-label line must print exactly `4`.

The graceful-degradation fallback wiring is the literal shell fallback, present:

```
grep -c '|| echo "no daemon"' commands/prepare-compact.md
grep -c '|| echo "no containers"' commands/prepare-compact.md
```

Each must print `>= 1`. The strings must be inside the fenced bash snippet (real `cmd || echo ...` fallbacks), not merely prose — verify by reading the snippet.

The no-closer-panel and namespace rules hold:

```
grep -c '⚪ DONE' commands/prepare-compact.md
grep -c 'Do NOT emit a' commands/prepare-compact.md
grep -c '👤 You:' commands/prepare-compact.md
grep -c '/vault-cli:session-close' commands/prepare-compact.md
grep -c '[^:]/session-close' commands/prepare-compact.md
```

Expect `1`, `>= 1`, `0`, `>= 1`, `0` respectively. The single `⚪ DONE` must be inside the prohibition sentence (verify by eye).

Nothing outside the two intended files moved — the sibling commands did not gain references:

```
grep -c 'prepare-compact' commands/session-close.md
grep -c 'prepare-compact' commands/sync-progress.md
grep -c 'prepare-compact' commands/update-task.md
```

Each must print `0`. And the changelog records the new command under `## Unreleased`:

```
sed -n '/## Unreleased/,/## v/p' CHANGELOG.md | grep -c prepare-compact
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```

The first must print `>= 1`. The second must still print `## v0.112.0`. The third must still show `0.112.0` (no version moved).

Finally, run the full gate once:

```
make precommit
```

Must exit 0. `make precommit` runs `check`, which includes `check-changelog` (validates the `## Unreleased` placement) and the full lint/vuln/trivy suite. If it fails, fix the issue and re-run only the failing target until it passes, then run `make precommit` once more. A non-zero exit code from `make precommit` means `"status":"failed"` in the completion report — no exceptions.
</verification>
