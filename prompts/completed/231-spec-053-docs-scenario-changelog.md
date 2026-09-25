---
status: completed
spec: [053-fleet-spawned-metrics-sessions]
summary: 'Recorded the second metrics_sessions writer in README, work-on-task § Passive metrics and the session-lifecycle doc, authored scenarios/006 as draft, and reconciled the existing ## Unreleased entry'
execution_id: vault-cli-metrics-exec-231-spec-053-docs-scenario-changelog
dark-factory-version: v0.196.0
created: "2026-09-25T17:18:12Z"
queued: "2026-09-25T17:37:23Z"
started: "2026-09-25T17:54:24Z"
completed: "2026-09-25T17:58:05Z"
branch: dark-factory/fleet-spawned-metrics-sessions
---

# Record the second writer, encode the operator walk, and changelog it

<summary>
- The README lists the new verb alongside the other task commands.
- The work-on-task command's passive-metrics note now names both writers of the metrics field and says the entry accumulates rather than replaces.
- The session-lifecycle document records the second writer and why the append is non-fatal.
- A new scenario encodes the operator walk for a real fleet spawn: two throwaway fixtures in the live vault, a real spawn on each, and the entry, session-id, spawn-window and preserved-entries assertions.
- The scenario is authored `status: draft`; it flips to `active` only after a real walk succeeds, which happens on the operator rung and not here.
- The changelog carries one unreleased entry describing the change.
- Documentation, scenario and changelog only — no Go code, no test code, no agent definition, no script.
</summary>

<objective>
Record the second metrics writer where the repo records behaviour — `README.md`, `commands/work-on-task.md` § Passive metrics, `docs/work-on-session-lifecycle.md` — encode the operator walk as `scenarios/006-fleet-spawn-metrics-sessions.md`, and add the `## Unreleased` changelog entry. The mechanism itself shipped in prompts 1–3; this prompt is what makes it discoverable and walkable.
</objective>

<context>
This prompt depends on prompts 1, 2 and 3 of spec 053 already being applied: the documentation describes what actually shipped on this branch, not an intention. Read `CLAUDE.md` first for project conventions.

Then read in full:

- `README.md` § `### task` — the command list. Read the whole `### task` block before editing; the new verb belongs beside `task set` / `task clear`.
- `commands/work-on-task.md` § `## Passive metrics` — the section you rewrite. It currently reads, in full:
  ```
  ## Passive metrics

  Each work-on run appends one entry to the task's `metrics_sessions` frontmatter field
  (session id + start timestamp). These metrics fields are written passively by vault-cli
  and must not be hand-edited.
  ```
  It is the last section of the file. Do NOT change anything else in that file.
- `docs/work-on-session-lifecycle.md` — the design-decisions document you extend. Read the whole file. It is decisions only; implementation details live in the code and its doc comments. The new section belongs immediately after `## Post-exit write ordering`, which is where the two writers of the task's session fields are discussed.
- `agents/work-on-task-assistant.md` § `### Session connect (MANDATORY when Obsidian task file exists)` — READ ONLY. Read it to quote the real invocation and the real warning wording in the docs; do not edit it.
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` — **the frozen scenario shape.** Copy its structure: the `---` frontmatter with `status:`, the `# Scenario NNN: <what this proves>` title, the one-sentence `Validates that …` line, a `## Why a scenario` section, then `## Setup` / `## Action` / `## Expected` / `## Cleanup` with checkbox items and fenced bash where a real command is run. Note its practice of stating preconditions that are the most likely cause of a false FAIL, and of asserting against files on disk rather than against scrollback.
- `scenarios/002-task-lifecycle.md` — a second scenario in the same voice, for the `work-on` path.
- `CHANGELOG.md` — read the top ~15 lines. There is **no** `## Unreleased` section today and the newest is `## v0.146.5`. The preamble block (`# Changelog`, the `All notable changes…` line, the SemVer link and the `* MAJOR / MINOR / PATCH` bullets) is frozen and everything you add goes directly below it.
- `docs/releasing-vault-cli.md` — READ ONLY. The release gate that walks `scenarios/*.md` before `make install`, and the scenario-skip rule. This scenario is walked on the operator rung, not here.
- `specs/in-progress/053-fleet-spawned-metrics-sessions.md` — the spec. Read its Desired Behavior 8, Acceptance Criteria 7, 9 and 10, the Post-Deploy rung, the Scenario coverage paragraph, the Verification section, and the Failure Modes rows "The agent definition loses the invocation" and "The local clock is wrong (skew)". Every requirement below comes from them.

Coding-plugin docs (in-container paths — the host paths do not exist inside the container):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the `## Unreleased` format, the frozen-preamble rule, and the required conventional prefix.
- `/home/node/.claude/plugins/marketplaces/coding/docs/readme-guide.md` — README conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

**Environment facts that shape this prompt:**

1. **Every check in `<verification>` is git-free.** The daemon's executor does not check `<verification>` exit codes, so a command that dies for an environmental reason still reports a pass. The spec's git-shaped evidence is reproduced here with file reads and greps. The git forms stay on the spec's `# Verification` § "Operator-executable" rung.
2. **Do NOT bump any version field.** `.maintainer.yaml` sets `release.autoRelease: true`, so `github-releaser` owns the version bump and the tag. `CLAUDE.md` § Version Alignment still carries the older "the prompt MUST also bump the three plugin JSON fields" instruction — that is the manual FALLBACK path only, superseded by its own § Plugin Release Checklist and by `docs/releasing-vault-cli.md` § Plugin release ("Do not hand-bump the JSONs on an `autoRelease` repo"). It does not apply here. Add the `## Unreleased` section only; do NOT touch `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, and do NOT create a `## vX.Y.Z` heading. `scripts/check-versions.sh` fails when the four version strings diverge, and `<verification>` runs it explicitly — it must pass because this prompt bumps none of them.
3. **Do NOT walk the scenario.** It runs on the operator rung after merge and install, against a real `mcp__supervisor__spawn_agent` and the live vault. This prompt authors the file only, and it stays `status: draft`.
4. **This prompt is documentation only.** No Go file, no test file, no script, no agent definition changes.
</context>

<requirements>

## 1. Scope — five files

- `README.md` — MODIFIED.
- `commands/work-on-task.md` — MODIFIED (the `## Passive metrics` section only).
- `docs/work-on-session-lifecycle.md` — MODIFIED (one new section).
- `scenarios/006-fleet-spawn-metrics-sessions.md` — NEW, `status: draft`.
- `CHANGELOG.md` — MODIFIED (one `## Unreleased` section).

Nothing else. In particular `agents/work-on-task-assistant.md`, `pkg/**`, `integration/**`, `scripts/**`, `Makefile`, `go.mod`, `go.sum`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json` and every other `scenarios/*.md` are untouched.

## 2. `README.md` — list the verb

In the `### task` fenced bash block, add one line immediately after the `vault-cli task clear …` line:

```bash
vault-cli task append-metrics-session "Build vault-cli Go Tool" <session-id>   # Append one metrics_sessions entry (use this, not set/add/remove)
```

The line must contain the literal `append-metrics-session`. Do NOT restructure the block, reorder the existing lines, or add a second occurrence of the verb anywhere in the file.

## 3. `commands/work-on-task.md` § `## Passive metrics` — name both writers

Replace the section's body with prose that states, in the file's voice:

- Each work-on run appends one entry to the task's `metrics_sessions` frontmatter field (session id + start timestamp).
- **Two writers append it, and both accumulate** — an entry is never replaced, and a session id that already has an entry is appended again rather than suppressed:
  - `vault-cli task work-on` appends an entry for the session it starts or resumes.
  - The session-connect step in `agents/work-on-task-assistant.md` appends an entry through `vault-cli task append-metrics-session "<task>" "<session-id>"` after it writes `claude_session_id`. This is the writer a fleet-spawned run uses.
- The field's entries are maps (`session_id` + `started_at`), so `task set`, `task add` and `task remove` refuse the field and point at `task append-metrics-session`.
- The existing sentence `These metrics fields are written passively by vault-cli and must not be hand-edited.` survives verbatim — it is the contract the spec quotes. **Write it on one physical line; do not wrap it:** `<verification>` matches it with a line-based `grep -cF`, and the file currently wraps it across two lines (`commands/work-on-task.md:134–135`), so copying today's wrapping reads as a missing contract. Wrap the rest of the section normally — `<verification>` also asserts the section body is at least 10 lines, which an unwrapped five-bullet rewrite can miss.

The section must contain the literal `append-metrics-session` and the literal `task work-on`. Do NOT touch any other section of the file, and do NOT restate the four-value phase enum, the close-out flags or anything else outside this section.

## 4. `docs/work-on-session-lifecycle.md` — record the second writer

Add one new section immediately after `## Post-exit write ordering` and before `## Why stream-json was rejected`, headed exactly:

```
## The second writer: the session-connect append
```

It must record, as decisions rather than as a tutorial:

- `vault-cli task work-on` is no longer the only writer of a task's `metrics_sessions`. The plugin's session-connect step in `agents/work-on-task-assistant.md` § Session connect invokes `vault-cli task append-metrics-session "<task>" "<session-id>"` in the branch that writes `claude_session_id`, after that write has landed. The verb is `pkg/ops/metrics_session_append.go`; it appends through the same domain append (`domain.TaskFrontmatter.AppendMetricsSession`) the work-on path uses, so the two producers emit one shape.
- Why the second writer exists: a fleet-spawned worker's session-connect is the only party that can record a session the spawn shape deliberately does not pre-mint, so without this append the runs the manager layer exists to produce are exactly the ones missing from session-cost analytics.
- **The entry accumulates; it never replaces.** A session id already present is appended again rather than suppressed — the accumulator is frozen by spec 036 AC 2 and spec 038's Constraints, and the duplicate-id double-count is handled on the read side (the interaction counter's per-distinct-session dedupe), never by a write-side guard.
- **The append is non-fatal by decision.** A non-zero exit from the verb is a warning, not a `⚠️` failure: the `claude_session_id` write stays in place and the run continues. The id is the load-bearing field — an id on disk means the session is resumable — while the metrics entry is analytics. The append is deliberately not in the agent definition's `<critical_writes>` list.
- **The generic write verbs refuse the field.** `task set`, `task add` and `task remove` refuse `metrics_sessions` before any mutation, with no `--force` bypass, because `set` stores a scalar and `add` / `remove` comma-split into a list of scalars — both shapes the dedicated list reader discards. That refusal lives in `pkg/ops/metrics_session_write.go`.
- **The concurrent-append race is accepted.** Two processes appending to the same task file are last-write-wins, the same read-modify-write race the existing `work-on` path has; a lost row is re-appended by re-running the verb. No lock, no re-read and no dedup were added.

Do NOT rewrite or reword any existing section, and do NOT document the topic work-on path — the spec names its absence of metrics as a deliberate boundary.

## 5. `scenarios/006-fleet-spawn-metrics-sessions.md` — the operator walk

Create the file with `status: draft` frontmatter and the shape of `scenarios/005`:

```markdown
---
status: draft
---

# Scenario 006: a fleet-spawned run lands a metrics session

Validates that a real fleet spawn through `/vault-cli:work-on-task` records its session in the task's `metrics_sessions`, and preserves the entries already there.
```

Then these sections, each with checkbox items, in this order:

**`## Why a scenario`** — must name, in prose:

- The behaviour lives in `agents/work-on-task-assistant.md` § Session connect, a Claude Code agent definition with no test harness, executed only by a real LLM in a real spawned worker behind `mcp__supervisor__spawn_agent`. An integration test can exercise the verb; it can never exercise the call site firing. `scripts/metrics-append-call-site-test.sh` pins the invocation's presence, not that it fires.
- It is load-bearing for the fleet journey: the fresh-start fleet spawn is the manager layer's core journey, and metrics parity for it is this spec's entire point.
- `scenarios/005` covers `work-on` resume and does not cover this.
- The regression risk, concretely: an edit to § Session connect silently drops the invocation and fleet runs vanish from metrics again, exactly as they are absent today.
- **This scenario is not sandboxed, by necessity.** The spawn uses the **installed** plugin and the **live** vault, so a temp-vault sandbox would be fiction. It mutates two clearly-named throwaway fixtures and today's daily note, and both are cleaned up below. Note the counterpart to `scenarios/005`'s TTY warning: this walk needs no TTY, but it does need the vault directory to be already trusted by Claude Code, because the first-run trust prompt would be an approval turn inside the spawned worker.

**`## Setup`** — builds both fixtures in the live vault, and records:

- The vault path resolved from `vault-cli config list --output json` by name (pinned explicitly, as `scenarios/005` does), the tasks directory, the installed plugin version, and the wall-clock window start.
- **Fixture 1** — a throwaway task with `status: in_progress`, `page_type: task`, a `priority`, and **neither** `claude_session_id` nor `metrics_sessions`.
- **Fixture 2** — a throwaway task with the same base keys, **no** `claude_session_id`, and a `metrics_sessions` block holding `N ≥ 1` entries authored in the writer's own shape (a 4-space-indented block sequence, `started_at` quoted) so the "byte-identical before and after" comparison is against a canonical baseline. Record `N`.
- A precondition asserting the two fixture files carry no `claude_session_id` and the expected `metrics_sessions` counts before the walk, and that the vault directory is already trusted.

**`## Action`** — the two real spawns:

- `mcp__supervisor__spawn_agent(prompt='/vault-cli:work-on-task "<fixture 1>"', …)` — **the prompt string unmodified, nothing chained onto it** — then, after the run, the same for fixture 2. Record the session id `spawn_agent` reported for each.
- An explicit instruction that the verifier issues **no manual invocation of the verb** at any point in the window. State both exclusions and what they rule out: chaining the verb onto the spawn prompt is excluded by the unmodified prompt, and running it by hand is excluded by the verifier issuing none. Together they are what makes the landing entry attributable to the call site.

**`## Expected`** — per fixture, asserting against the file on disk:

- Fixture 1: the file carries **exactly one** `metrics_sessions` entry whose `session_id` equals the id `spawn_agent` reported, and whose `started_at` falls inside the spawn window (≥ the spawned transcript's first timestamp, ≤ its last).
- Fixture 2: the file carries `N + 1` entries, with the original `N` byte-identical.
- Call-site provenance is an **artifact** check, not a log-absence one — state why: session-connect runs inside the worker's own session, so the invocation legitimately appears in that transcript. The check is instead `grep -n 'append-metrics-session' ~/.claude/plugins/cache/vault-cli/vault-cli/<version>/agents/work-on-task-assistant.md`, returning ≥1 line **inside § Session connect** of the **released** plugin in the load path.
- A note that `status:` flips from `draft` to `active` only after the first successful walk, and that the flip happens on the operator rung — not in this prompt.

**`## Cleanup`** — removes both fixture files and the fixtures' tracking lines from today's daily note, and records the fixture paths so the operator can find them.

Do NOT add a `## Why a scenario` claim that the script already covers the firing, and do NOT claim the scenario runs in a sandbox.

## 6. `CHANGELOG.md` — the `## Unreleased` entry

Insert a `## Unreleased` section directly below the frozen preamble block and directly above the current newest heading (`## v0.146.5` at authoring time — verify rather than trusting that number). It does not exist today, so create it; if it does exist when you run, append to it instead of creating a second one.

The section carries exactly one flat bullet, with the `feat:` prefix, naming the verb and the refusal. Model it on the repo's existing entries — specific, naming types, commands and packages, no date suffix, no `### Added` sub-headings:

```
## Unreleased

- feat: `vault-cli task append-metrics-session <task> <session-id>` appends one `{session_id, started_at}` entry to a task's `metrics_sessions` through the existing domain append, so a run whose session-connect does not route through `task work-on` — a fleet-spawned worker — records its session in the same shape and the same accumulate-never-replace semantics as the older path; the plugin's session-connect step invokes it after writing `claude_session_id`, and a non-zero exit is a warning that leaves the id write in place. `task set`, `task add` and `task remove` now refuse the field before any mutation, with a message naming the field and pointing at the new verb, because a scalar or a comma-split list is a shape every reader discards; `--force` does not bypass the refusal.
```

Never insert anything above or inside the preamble — `make check-changelog` guards that structure. Do NOT bump `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, and do NOT create a versioned heading.

## 7. Self-check before finishing

- Confirm each of the four greps in spec AC 7 and AC 9 holds: `grep -n 'append-metrics-session' README.md`, `commands/work-on-task.md` and `docs/work-on-session-lifecycle.md` each return ≥1 line; `docs/work-on-session-lifecycle.md` carries the new section heading and the accumulate-not-replace statement; `commands/work-on-task.md` § Passive metrics names both writers.
- Confirm `scenarios/006-fleet-spawn-metrics-sessions.md` exists, carries `status: draft`, and has all six sections (`Why a scenario`, `Setup`, `Action`, `Expected`, `Cleanup`) plus the title and the `Validates that …` line.
- Confirm `CHANGELOG.md` has exactly one `## Unreleased` heading, that it sits below the preamble line and above the newest `## vX.Y.Z` heading, and that the bullet carries the `feat:` prefix.
- Walk spec 053's Acceptance Criteria 7, 9 (CHANGELOG half) and 10 (scenario content) and state in your completion report which requirement satisfies each, and note explicitly that the scenario's walk and its `draft` → `active` flip are the operator rung and are not claimed here.
- Confirm each check in `<verification>` passes by **running** it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 053 — non-goals (hard vetoes).** Do NOT document or change the topic work-on path — its absence of metrics is a deliberate boundary. Do NOT document a write-side dedup, idempotence or "already recorded" guard, a `--started-at` / `--at` / backfill flag, a verb-side session-id cross-check, or a `clear` path for the field. Do NOT rework the spawn shape or document pre-minting the session via `work-on --mode headless`. Do NOT change the auto-resume gate. Do NOT add or extend a scenario for the topic path. Do NOT touch `agents/work-on-task-assistant.md`, `scripts/**`, any Go file or any test file.
- **Copied from spec 053 — constraints.** The `## Unreleased` CHANGELOG entry goes below the preamble block and above the newest `## vX.Y.Z`; the four version strings must stay aligned, so do NOT bump any of them — `.maintainer.yaml` sets `release.autoRelease: true` and `github-releaser` owns the bump and the tag. Read-side consumers are unchanged and must not be documented as changed: `metrics_sessions[].session_id` unions, `metrics_interaction_count`, `metrics_completed_at`, `metrics_cycles`. The scenario follows the repo's existing release-gate shape (`scenarios/*.md`, walked by the operator before `make install`), is **not** sandboxed, uses the installed plugin and the live vault, and cleans up its throwaway fixtures. `make precommit` must pass in the repo root.
- **Frozen strings.** The verb `append-metrics-session`; the section heading `## The second writer: the session-connect append`; the scenario file `scenarios/006-fleet-spawn-metrics-sessions.md` with `status: draft`; the CHANGELOG heading `## Unreleased` with a `feat:` bullet. All are grep targets in `<verification>`.
- **The `## Passive metrics` contract sentence survives verbatim**: `These metrics fields are written passively by vault-cli and must not be hand-edited.` Do not reword it — the spec quotes it.
- **Documentation and scenario only.** No behaviour, signature, test or script change. Every quoted command, error wording and field name must be copied from the real artifacts on this branch (`agents/work-on-task-assistant.md`, `pkg/ops/metrics_session_append.go`, `pkg/ops/metrics_session_write.go`), never invented.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command. Do NOT spawn an agent or walk the scenario.
- Do NOT bump version fields, do NOT tag, and do NOT rename `## Unreleased` to a version.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code. Absence is asserted with `! grep -q`, never with a `grep -c` that must print `0`. Never pipe a test command.

**The second writer is recorded where the repo records behaviour — spec AC 7:**

```
test "$(grep -c -- 'append-metrics-session' README.md)" -ge 1
test "$(grep -c -- 'append-metrics-session' commands/work-on-task.md)" -ge 1
test "$(grep -c -- 'append-metrics-session' docs/work-on-session-lifecycle.md)" -ge 1
test "$(grep -cF -- 'task work-on' commands/work-on-task.md)" -ge 1
test "$(grep -cF -- 'These metrics fields are written passively by vault-cli and must not be hand-edited.' commands/work-on-task.md)" = "1"
test "$(grep -cF -- '## The second writer: the session-connect append' docs/work-on-session-lifecycle.md)" = "1"
awk '/^## The second writer: the session-connect append$/{f=1;next} f && /^## /{exit} f' docs/work-on-session-lifecycle.md > /tmp/spec053-second-writer.txt
grep -qF -- 'The entry accumulates; it never replaces.' /tmp/spec053-second-writer.txt
grep -qF -- 'AppendMetricsSession' /tmp/spec053-second-writer.txt
grep -qF -- 'metrics_session_append.go' /tmp/spec053-second-writer.txt
grep -qF -- 'critical_writes' /tmp/spec053-second-writer.txt
grep -qF -- 'task append-metrics-session' /tmp/spec053-second-writer.txt
test "$(wc -l < /tmp/spec053-second-writer.txt)" -ge 12
```

**The `## Passive metrics` section is rewritten, and its neighbours are untouched:**

```
awk '/^## Passive metrics$/{f=1;next} f && /^## /{exit} f' commands/work-on-task.md > /tmp/spec053-passive-metrics.txt
grep -qF -- 'append-metrics-session' /tmp/spec053-passive-metrics.txt
grep -qF -- 'task work-on' /tmp/spec053-passive-metrics.txt
grep -qF -- 'accumulate' /tmp/spec053-passive-metrics.txt
test "$(wc -l < /tmp/spec053-passive-metrics.txt)" -ge 10
test "$(grep -cF -- 'claude_session_id' commands/work-on-task.md)" -ge 1
```

**The new section sits after `## Post-exit write ordering` and before `## Why stream-json was rejected`:**

```
test "$(grep -nF -- '## Post-exit write ordering' docs/work-on-session-lifecycle.md | cut -d: -f1)" -lt "$(grep -nF -- '## The second writer: the session-connect append' docs/work-on-session-lifecycle.md | cut -d: -f1)"
test "$(grep -nF -- '## The second writer: the session-connect append' docs/work-on-session-lifecycle.md | cut -d: -f1)" -lt "$(grep -nF -- '## Why stream-json was rejected' docs/work-on-session-lifecycle.md | cut -d: -f1)"
```

**The scenario exists with the frozen shape and status — spec AC 10:**

```
test -f scenarios/006-fleet-spawn-metrics-sessions.md
test "$(grep -c '^status: draft$' scenarios/006-fleet-spawn-metrics-sessions.md)" = "1"
test "$(grep -c '^# Scenario 006: ' scenarios/006-fleet-spawn-metrics-sessions.md)" = "1"
test "$(grep -c '^Validates that ' scenarios/006-fleet-spawn-metrics-sessions.md)" = "1"
test "$(grep -c '^## Why a scenario$' scenarios/006-fleet-spawn-metrics-sessions.md)" = "1"
test "$(grep -c '^## Setup$' scenarios/006-fleet-spawn-metrics-sessions.md)" = "1"
test "$(grep -c '^## Action$' scenarios/006-fleet-spawn-metrics-sessions.md)" = "1"
test "$(grep -c '^## Expected$' scenarios/006-fleet-spawn-metrics-sessions.md)" = "1"
test "$(grep -c '^## Cleanup$' scenarios/006-fleet-spawn-metrics-sessions.md)" = "1"
test "$(grep -cF -- 'mcp__supervisor__spawn_agent' scenarios/006-fleet-spawn-metrics-sessions.md)" -ge 1
test "$(grep -cF -- '/vault-cli:work-on-task' scenarios/006-fleet-spawn-metrics-sessions.md)" -ge 1
test "$(grep -cF -- 'append-metrics-session' scenarios/006-fleet-spawn-metrics-sessions.md)" -ge 1
test "$(grep -cF -- 'status: in_progress' scenarios/006-fleet-spawn-metrics-sessions.md)" -ge 1
test "$(grep -cF -- 'not sandboxed' scenarios/006-fleet-spawn-metrics-sessions.md)" -ge 1
test "$(grep -cF -- 'metrics_sessions' scenarios/006-fleet-spawn-metrics-sessions.md)" -ge 1
test "$(grep -cE '^- \[ \]' scenarios/006-fleet-spawn-metrics-sessions.md)" -ge 8
```

**The scenario file is `draft`, not `active`, and no other scenario moved:**

```
! grep -q '^status: active$' scenarios/006-fleet-spawn-metrics-sessions.md
test "$(grep -c '^status: active$' scenarios/005-work-on-resume-auto-invokes-subtask.md)" = "1"
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "6"
```

**The CHANGELOG section is added below the preamble and above the newest version — spec AC 9:**

```
test "$(grep -c '^## Unreleased$' CHANGELOG.md)" = "1"
test "$(grep -n -m1 '^## ' CHANGELOG.md | cut -d: -f1)" -gt "$(grep -n -m1 '^All notable changes to this project' CHANGELOG.md | cut -d: -f1)"
test "$(grep -n -m1 '^## Unreleased$' CHANGELOG.md | cut -d: -f1)" -lt "$(grep -nE '^## v[0-9]+\.[0-9]+\.[0-9]+$' CHANGELOG.md | head -1 | cut -d: -f1)"
test "$(awk '/^## Unreleased$/{f=1;next} f && /^## /{exit} f' CHANGELOG.md | grep -cF -- 'append-metrics-session')" -ge 1
test "$(awk '/^## Unreleased$/{f=1;next} f && /^## /{exit} f' CHANGELOG.md | grep -cE '^- feat: ')" = "1"
```

**The changelog structure script passes, and no version field was bumped:**

```
bash scripts/check-changelog.sh
bash scripts/check-versions.sh
```

**No forbidden artifact was touched by this prompt** — the agent definition, the script and the Go layer are prompts 1–3's artifacts and must survive this prompt byte-for-byte in the parts that matter:

```
! grep -qF -- 'append-metrics-session' scripts/struck-row-rule-test.sh
test "$(grep -cF -- 'vault-cli task append-metrics-session' agents/work-on-task-assistant.md)" = "1"
test "$(grep -cF -- 'ℹ️ Metrics: not recorded' agents/work-on-task-assistant.md)" = "1"
test -f scripts/metrics-append-call-site-test.sh
bash scripts/metrics-append-call-site-test.sh
test "$(grep -cF -- 'func metricsSessionsWriteRefusal(' pkg/ops/metrics_session_write.go)" = "1"
test "$(grep -cF -- 'type AppendMetricsSessionOperation interface' pkg/ops/metrics_session_append.go)" = "1"
```

**The whole gate:**

```
make precommit > /tmp/spec053-precommit.log 2>&1; test "$?" = "0"
```

Finally, walk spec 053's Acceptance Criteria 7, 9 (CHANGELOG half) and 10 (scenario content) against the change and state in your completion report which requirement and which spec satisfies each one. Note explicitly that the scenario's walk against a real fleet spawn, and its `draft` → `active` flip, are the operator rung and are not claimed here.
</verification>
