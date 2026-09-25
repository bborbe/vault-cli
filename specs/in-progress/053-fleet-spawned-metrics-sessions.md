---
status: prompted
tags:
    - dark-factory
    - spec
approved: "2026-09-25T16:46:01Z"
generating: "2026-09-25T17:13:03Z"
prompted: "2026-09-25T17:28:26Z"
branch: dark-factory/fleet-spawned-metrics-sessions
---

## Fleet-spawned runs land a metrics session

## Summary

- A fleet-spawned worker stamps `claude_session_id` on its task but lands **no `metrics_sessions` entry**, so the runs the manager layer exists to produce are exactly the ones missing from session-cost analytics.
- `metrics_sessions` is appended only by `vault-cli task work-on`. The documented fresh-start spawn shape (`spawn_agent(prompt='/vault-cli:work-on-task "<task>"')`) never calls that path and forbids pre-minting the session, so nothing outside the worker stamps, and the worker's own session-connect step writes only the session id.
- This spec adds **one vault-cli verb** that appends a metrics entry, and calls it from the plugin's session-connect step — the same step, and the same shape, as the existing `vault-cli task set … claude_session_id` call.
- The append stays passive (vault-cli writes the field, never the worker), accumulates (append, never replace), and is schema-identical to what the older `work-on` path writes.
- The generic write verbs (`set`, `add`, `remove`) begin refusing the field with a pointer at the new verb, so it can no longer be written in a shape every reader discards.

## Problem

Fleet-spawned workers are absent from `metrics_sessions`, so session-cost and interaction analytics under-count them — and the runs the manager layer exists to produce are precisely the ones missing. The cause is a missing write path, not a wrong value: `metrics_sessions` is appended by `pkg/ops/workon.go` `persistSessionAndMetrics`, reached only through `vault-cli task work-on`; the documented fresh-start spawn shape never calls it and deliberately forbids pre-minting the session via `work-on --mode headless`. The worker's own session-connect step therefore has nothing to call: `vault-cli task add <task> metrics_sessions '<json>'` fails with `unknown field: "metrics_sessions"` (the field is absent from the known-field lists in `pkg/ops/frontmatter_entity.go`), and `vault-cli task set <task> metrics_sessions '<json>'` writes a **scalar** — a shape the dedicated list reader discards by design, so `set` would report success while recording nothing visible. Hand-editing is barred by `work-on-task`'s own contract (*"These metrics fields are written passively by vault-cli and must not be hand-edited"*). Measured 2026-09-19: of 49 `in_progress` tasks in the Personal vault with an empty `claude_session_id`, 8 had a matching transcript and 3 were backfilled by hand. That population is a **different** one — it needs session *recovery*, and the append lives in the branch that writes the id, so this spec does not close it. The population this spec repairs is the one whose id **does** land and whose entry does not: tasks carrying a non-empty `claude_session_id` and zero `metrics_sessions` entries — measured 2026-09-25 in the Personal vault: **641 of the 1,379 tasks carrying a non-empty `claude_session_id`** hold zero `metrics_sessions` entries. The same defect class sits one surface over on the topic work-on path, which appends no metrics at all — named here as a deliberate boundary, not an oversight.

## Reproduction

Smallest flow, verified 2026-09-19 against vault-cli v0.138.1 on the Personal vault.

1. Spawn a worker with the documented shape — the prompt string unmodified, nothing chained onto it — against a task whose `claude_session_id` is absent and whose `metrics_sessions` is absent:

   ```
   mcp__supervisor__spawn_agent(prompt='/vault-cli:work-on-task "<task>"', ...)
   ```

2. The session id lands. The spawn reported `888223a2-4d21-4ece-aacf-da4c959e566a`, and the task carries it:

   ```
   $ vault-cli task get "<task>" claude_session_id
   888223a2-4d21-4ece-aacf-da4c959e566a
   ```

3. The metrics entry does not:

   ```
   $ grep -c '^metrics_sessions:' "<task file>"
   0
   ```

4. No CLI path can append it, and the one that looks like it can writes a shape the readers discard:

   ```
   $ vault-cli task add "<task>" metrics_sessions '{"session_id":"888223a2-…"}'
   Error: unknown field: "metrics_sessions"
   exit=1

   $ vault-cli task set "<task>" metrics_sessions '{"session_id":"888223a2-…"}'
   ✅ Set metrics_sessions={"session_id":"888223a2-…"} on: <task>
   exit=0     # …and metrics_sessions then reads back as no entries at all
   ```

The defect's blast radius is every fleet run whose status flip does not route through `work-on` — that is every run on a task already `in_progress`, which is the normal fleet case (a manager spawns onto a task it has already claimed). A run on a task that is *not* `in_progress` reaches the cached `work-on` path incidentally and may land an entry; that path is not the mechanism and must not be relied on.

## Expected vs Actual

**Expected.** `commands/work-on-task.md` § Passive metrics states the contract: *"Each work-on run appends one entry to the task's `metrics_sessions` frontmatter field (session id + start timestamp). These metrics fields are written passively by vault-cli and must not be hand-edited."* A fleet-spawned worker runs `/vault-cli:work-on-task`, so its run is a work-on run: it must land an entry, written by vault-cli, without the worker touching frontmatter.

**Actual.** The entry is appended only by `vault-cli task work-on`. The fresh-start spawn shape never calls it, the field is unreachable through `add`, `set` writes a shape the readers discard, and hand-editing is barred. So the run stamps the session id and no metrics entry — the documented contract and reality disagree.

## Goal

Every run that connects a session to a task through the plugin's session-connect step records that session in the task's `metrics_sessions`, written by vault-cli through an explicit verb, appended to whatever is already there. A fleet-spawned worker's run becomes indistinguishable, in the metrics corpus, from a run started by `vault-cli task work-on`: same field, same entry shape, same accumulate-never-replace semantics. No command can write the field in a shape the readers discard.

## Non-goals

- **The topic work-on path.** `pkg/ops/topic_workon.go` writes only `claude_session_id` and appends no metrics — the same defect class one surface over, deliberately deferred here so the boundary is a decision rather than an oversight.
- **Changing the auto-resume gate.** Its condition 2 is satisfied by `claude_session_id` alone; what is missing is metrics parity, not recovery.
- **Backfilling older tasks** with an empty `claude_session_id` and no matching transcript — a different population: closed sessions and older tasks.
- **Reworking the spawn shape, or pre-minting the session** via `work-on --mode headless` — the fresh-start form exists deliberately.
- **An executor-side append** — the worker's own definition writing the frontmatter itself. Barred by the driving task's SC1 ("not by hand-editing frontmatter") and its subtask 2 ("the worker must not write frontmatter itself"). See Alternatives Considered.
- **The structural fragility of the name→task join** — session-connect joins on a fuzzy display title because it has no id yet.
- **A run whose session-connect refuses to write `claude_session_id`** — zero or multiple title matches, where the step deliberately does not guess. That run lands no entry, because the append lives in the branch that writes the id. Out of scope: this spec covers only the branch that writes it.
- Do NOT add write-side dedup, idempotence, or an "already recorded" guard to the append — the accumulator is frozen by spec 036 AC2 and spec 038's Constraints ("the accumulator must not start suppressing entries"); the duplicate-id double-count is handled on the read side. Invariant; if a future consumer demands suppression, that is a separate spec.
- Do NOT add a `--started-at` / `--at` / backfill flag to the verb — its only consumer stamps at connect time, and a backfill consumer is out of scope above. Invariant; a future backfill is a separate spec.
- Do NOT add a guard requiring the supplied session id to equal the task's current `claude_session_id` — the call site orders the id write before the append, which makes the orphan row unreachable at the only consumer, and a verb-side cross-check would couple the verb to that ordering and block any future consumer. Invariant; if a future consumer demands the cross-check, that is a separate spec.
- Do NOT make the new verb write `claude_session_id` — the id write stays `vault-cli task set`, and the auto-resume gate's field keeps its existing path.
- Do NOT add the field to the `add` / `remove` list allowlist so that `task add <task> metrics_sessions …` works — `add` comma-splits a string into a list of scalars, and this field's entries are maps (`session_id` + `started_at`); an allowlist entry would write a shape every reader discards, which is the defect this spec closes.
- Do NOT add a `clear` path for the field — removing entries stays an operator frontmatter action.
- Do NOT change the Phase 3 status-flip rule in `agents/work-on-task-assistant.md` (`work-on` when session-connect wrote the id, `task set status` when it did not) — it is reasoned, observed behaviour and not this spec's surface.
- No changes to `metrics_interaction_count`, `metrics_completed_at`, `metrics_cycles`, or any read-side consumer of `metrics_sessions[].session_id`.

## Assumptions

- The fleet's normal case is a task whose `status` is already `in_progress` and whose `claude_session_id` is absent — session-connect's title match returns exactly one live UUID.
- The call site orders the id write before the append; that ordering is what makes the verb's absent id cross-check unreachable at its only consumer.
- A run that pre-set the id already appended its own entry, so the already-connected branch invokes nothing and does not double-record.
- The field's entries are maps (`session_id` + `started_at`), not scalars — which is why `add`'s comma-split cannot carry them.
- `started_at` is stamped at invocation time by the verb; no caller supplies it.

## Alternatives Considered

| Alternative | Why rejected |
|---|---|
| The worker's agent definition writes the frontmatter itself (an executor-side append) | Barred twice by the driving task: SC1 requires the mechanism to be "not by hand-editing frontmatter", and subtask 2 requires "the worker must not write frontmatter itself". It would also bypass the domain append, losing the accumulate-never-replace guarantee. |
| One combined verb that sets `claude_session_id` and appends the entry in a single write | Re-decides the id write path, which is deliberately left as `vault-cli task set`. The id is the field the auto-resume gate and the Vault UI read, and its write path carries a documented compensating-clear and per-session lock; leaving it untouched keeps the regression surface minimal. |
| Make the append idempotent — append only when the session id is absent | Contradicts spec 036 AC2 (two work-on runs against one session append two entries) and spec 038's Constraints ("the accumulator must not start suppressing entries"). Spec 038 fixed the resulting double-count on the read side; a write-side guard would silently change the corpus for every existing consumer. |
| Add `metrics_sessions` to the `add` / `remove` list allowlist instead of a verb | `add` comma-splits its value into a list of scalars; the field's entries are maps. The command would report success while writing a shape every reader discards — the same silent-divergence hole the `blocked_by` write path closes. |
| Do it in the supervisor at spawn time | The session id does not exist before the worker starts, and the documented fresh-start shape forbids pre-minting it. |
| Rely on the incidental `work-on` call in the status flip | It fires only when the task is not yet `in_progress`, so it misses every run on an already-claimed task — the normal fleet case — and it makes the entry a side effect of a status change rather than of connecting a session. |
| Do nothing | See the Do-Nothing Option below. |

## Acceptance Criteria

Fixture vault for ACs 1–5: a temp vault with `Tasks/Alpha.md` and `Tasks/Beta.md`, plus a config naming `tasks_dir: Tasks` (the shape the existing `integration/` harness builds). `<bin>` is the binary built from HEAD — `gexec.Build` inside `integration/`, or `/tmp/new-vault-cli` on the host. `Alpha`'s frontmatter holds the base keys `status: in_progress`, `page_type: task`, `priority: 1`, `task_identifier: <a well-formed UUID>`, and `metrics_sessions` in whichever shape the AC under test names (absent for AC 1, a two-entry list for AC 2). No other key is present, so any key the verb adds or drops is visible. `S1` / `S2` / `S3` below are distinct well-formed UUIDs.

- [ ] `<bin> --config <cfg> task append-metrics-session Alpha S1` exits 0, and `Tasks/Alpha.md` then carries a `metrics_sessions` list holding exactly one entry whose `session_id` is `S1` and whose `started_at` is a quoted RFC 3339 timestamp carrying a non-zero time-of-day; every base key of `Alpha` is byte-identical before and after — evidence: exit code 0 plus file content (`grep -A3 '^metrics_sessions:' Tasks/Alpha.md`), plus negative evidence on the base keys.
- [ ] `Alpha` whose `metrics_sessions` already holds entries for `S1` and `S2`; `<bin> --config <cfg> task append-metrics-session Alpha S3` exits 0, and the file then holds **three** entries in the order `S1`, `S2`, `S3`, with the `S1` and `S2` entries byte-identical to their pre-run form — evidence: file content (entry count 3, order) plus negative evidence (the two pre-existing entry blocks compare byte-equal before and after).
- [ ] Schema parity with the older path: `Beta`'s frontmatter carries `claude_session_id: <S1>`; `<bin> --config <cfg> task work-on Beta --mode headless` appends an entry through the existing `work-on` path (the cached-session path, no spawn), and the entry the new verb wrote for `Alpha` carries the **same key set and the same value shapes** as that entry — both a list of maps under `metrics_sessions`, each map carrying exactly `session_id` (a UUID string) and `started_at` (a quoted RFC 3339 timestamp with a non-zero time-of-day) — evidence: file artifact, the two files' entry key sets and shapes compared (values differ by design).
- [ ] The generic write verbs refuse the field instead of writing a shape the readers discard: `<bin> --config <cfg> task set Alpha metrics_sessions 'x'`, `<bin> --config <cfg> task add Alpha metrics_sessions 'x'`, and `<bin> --config <cfg> task remove Alpha metrics_sessions 'x'` each exit non-zero, each message names the field `metrics_sessions` and the verb `append-metrics-session`, and `Tasks/Alpha.md` is byte-identical after all three — evidence: three exit codes, stdout/stderr match on both names, and negative evidence (file diff empty).
- [ ] A session id the verb cannot honour is refused with nothing written: `<bin> --config <cfg> task append-metrics-session Alpha ""`, `… Alpha "not-a-uuid"`, and `… Alpha "../escape"` each exit non-zero with a message naming the required shape (a well-formed UUID), and `Tasks/Alpha.md` is byte-identical after all three — evidence: three exit codes, stdout/stderr match, and negative evidence (file diff empty).
- [ ] The call site carries the invocation, in the branch that writes the id: `grep -n 'vault-cli task append-metrics-session' agents/work-on-task-assistant.md` returns ≥1 line **that is an actionable step line inside that branch (bulleted sub-item or numbered), not a comment and not a prose mention**; that line lies inside the section headed `### Session connect (MANDATORY when Obsidian task file exists)` (checked section-scoped, not file-wide); and its line number falls **between** the `If EXACTLY ONE UUID is returned` bullet and the `If zero OR multiple UUIDs are returned` bullet of that section (the invocation is a **new line inserted between those two bullets**; the `zero OR multiple` bullet shifts down one line) — so it sits in the branch that writes `claude_session_id`, ordered after that write, and not in the branch where the id is deliberately left unwritten. The section also instructs that a non-zero exit from the verb is reported as a warning naming `metrics` and the reason, with the `claude_session_id` write left in place (`grep -nE 'warning.*metrics|metrics.*warning'` over that same section returns ≥1 line) — evidence: file content (line numbers, both branch bounds), the section-scoped check, and the warning-line grep. A test wired into `make test` pins this: `bash scripts/metrics-append-call-site-test.sh` exits 0 with the invocation present, and its self-check strips the invocation from one artifact in a throwaway copy and requires exactly that artifact to be reported (the script exits non-zero in that mode) — evidence: exit codes in both directions.
- [ ] The second writer is recorded: `README.md` names the verb, `commands/work-on-task.md` § Passive metrics names both writers (the `work-on` path and the session-connect append), and `docs/work-on-session-lifecycle.md` records the second writer and that the entry accumulates rather than replaces — evidence: file content (`grep -n 'append-metrics-session' README.md` returns ≥1 line; the other two files each carry the named statement).
- [ ] The accumulator is not suppressed on the write side: on a task whose `claude_session_id` is `S1` and whose `metrics_sessions` already holds an entry for `S1`, `<bin> --config <cfg> task work-on <task> --mode headless` leaves the file with one **more** entry, two of them carrying `S1` — evidence: file content (entry count N+1; the count of entries whose `session_id` is `S1` is 2).
- [ ] Ship readiness: `make precommit` exits 0; the new command appears in the integration command-registration table (`grep -n 'append-metrics-session' integration/cli_test.go` returns ≥1 line) and the suite passes; a `## Unreleased` section is **added** below the preamble block and above the newest `## vX.Y.Z` — it does not exist today — carrying the change — evidence: exit code, grep line ≥1, file content.
- [ ] **Post-Deploy (Rung-2):** a real fleet spawn through the documented shape lands the entry, and preserves what was already there — on a throwaway fixture task in the live vault whose `claude_session_id` is absent, whose `metrics_sessions` is absent, and whose `status` is `in_progress`, spawn a worker with `mcp__supervisor__spawn_agent(prompt='/vault-cli:work-on-task "<task>"')` — the prompt string unmodified, nothing chained onto it — and issue no manual invocation of the new verb yourself at any point in the window; after the run the file carries **exactly one** `metrics_sessions` entry whose `session_id` equals the id `spawn_agent` reported and whose `started_at` falls inside the spawn window (≥ the transcript's first timestamp, ≤ its last). Then repeat on a second fixture whose `metrics_sessions` already holds N≥1 entries and whose `claude_session_id` is absent: after the run the file carries N+1 entries with the original N byte-identical. The two exclusions together rule out both shortcuts: chaining the verb onto the spawn prompt (excluded by the unmodified prompt) and running it by hand (excluded by the verifier issuing none). Call-site provenance is an **artifact** check, not a log-absence one — session-connect runs inside the worker's own session, so the invocation legitimately appears in that transcript; instead grep the **released** plugin in the load path: `grep -n 'append-metrics-session' ~/.claude/plugins/cache/vault-cli/vault-cli/<version>/agents/work-on-task-assistant.md` returns ≥1 line inside § Session connect. Evidence: file artifact (both fixtures, before/after), the `spawn_agent` report, the transcript's first and last timestamps, and the load-path grep.
  - `deploy_check:` `bash -lc 'b=$(vault-cli --version | grep -oE "[0-9]+\.[0-9]+\.[0-9]+" | head -1); p=$(for d in ~/.claude/plugins/cache/vault-cli/vault-cli/*/; do sed -n "s/.*\"version\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$d/.claude-plugin/plugin.json"; done | sort -V | tail -1); git fetch --tags -q; t=$(git describe --tags --abbrev=0 origin/master | sed "s/^v//"); [ -n "$b" ] && [ "$b" = "$p" ] && [ "$b" = "$t" ] && echo "$b" || exit 1'`
  - `deploy_target:` `$(git describe --tags --abbrev=0 origin/master | sed 's/^v//')`

**Scenario coverage: one new scenario.** Justified on all four conditions. (a) Unit and integration tests genuinely cannot reach it: the behaviour lives in `agents/work-on-task-assistant.md` § Session connect, a Claude Code agent definition with no test harness, executed only by a real LLM in a real spawned worker behind `mcp__supervisor__spawn_agent` — an integration test can exercise the verb, never the call site firing. (b) Load-bearing for an essential journey: the fresh-start fleet spawn is the manager layer's core journey, and metrics parity for it is this spec's entire point. (c) No existing scenario covers it — `scenarios/005` covers `work-on` resume. (d) Concrete named regression risk: an edit to the agent definition silently drops the invocation and fleet runs vanish from metrics again, exactly as they are absent today. The scenario follows the repo's existing release-gate shape (`scenarios/*.md`, walked by the operator before `make install`); it is **not** sandboxed, because the spawn uses the installed plugin and the live vault, and it cleans up its throwaway fixtures.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — format + generate + test + check, exits 0
- `make test` — full suite, including the new call-site script, exits 0
- `go test ./integration/... -v` — exits 0; the command-registration table lists `task append-metrics-session`, and the verb-level ACs (1–5, 8) run here
- `bash scripts/metrics-append-call-site-test.sh` — exits 0 with the invocation present; exits non-zero when the invocation is stripped from a throwaway copy
- `grep -n 'append-metrics-session' agents/work-on-task-assistant.md README.md commands/work-on-task.md docs/work-on-session-lifecycle.md` — ≥1 line per file, and in the agent definition the line sits inside § Session connect

### Operator-executable (runs on the host after merge, spec verification ladder)

- `go build -o /tmp/new-vault-cli .` — fresh binary from HEAD; replay ACs 1–5 and 8 against a temp vault with it
- Merge to `bborbe/vault-cli` master, then let `github-releaser-agent` cut the tag (`.maintainer.yaml` carries `release.autoRelease: true`) — do **not** hand-cut it
- Install **both halves** — the mechanism is a Go CLI verb, so the plugin update alone leaves it unreachable: `make install` (the binary must win on PATH over any shadowing copy) **and** `claude plugin update vault-cli@vault-cli`; then confirm the released version in the load path `~/.claude/plugins/cache/vault-cli/vault-cli/<version>/`
- Walk `scenarios/006-fleet-spawn-metrics-sessions.md` against the installed version — a real `mcp__supervisor__spawn_agent` on the fixture, the entry/id/window checks, then the N≥1 fixture, then cleanup

## Desired Behavior

1. A vault-cli verb — `task append-metrics-session <task-name> <session-id>` — appends exactly one entry to the named task's `metrics_sessions`, carrying the supplied session id and the invocation time, and preserves every other frontmatter key on the task (known and unknown) through the write. The name is frozen because it is the interface between the CLI and the agent definition, and both artifacts must agree on it. (fires AC 1)
2. The append accumulates: every entry already present stays byte-identical and in order, and a session id that already has an entry is appended again rather than suppressed. (fires AC 2, AC 8)
3. The appended entry is schema-identical to one written by the older `vault-cli task work-on` path — same key set, same value shapes — so the fleet path yields parity rather than a lookalike. (fires AC 3)
4. The generic write verbs refuse the field before any mutation: `task set`, `task add`, and `task remove` on `metrics_sessions` each fail with a message naming the field and the append verb, and leave the file byte-identical. `--force` does not bypass the refusal, mirroring the existing refusal-before-mutation shape of the `blocked_by` write path. (fires AC 4)
5. A session id the verb cannot honour — empty, not a well-formed UUID, or carrying a path separator — is refused before the frontmatter write, with a message naming the required shape and nothing written. (fires AC 5)
6. The plugin's session-connect step in `agents/work-on-task-assistant.md` invokes the verb in the branch that writes `claude_session_id`, **after** that write has landed; when the field is already set the step invokes nothing (the run that pre-set the id already appended its own entry); and a non-zero exit is reported as a warning naming metrics and the reason, without rolling back the id write — the id is the load-bearing field, the metrics entry is analytics. (fires AC 6, AC 10)
7. A test wired into `make test` pins the call site: it asserts, section-scoped, that § Session connect carries the invocation, and proves it measures rather than recites by stripping the invocation from one artifact in a throwaway copy and requiring exactly that artifact to be reported. (fires AC 6)
8. The second writer is recorded where the repo records behaviour — `README.md`, `commands/work-on-task.md` § Passive metrics, `docs/work-on-session-lifecycle.md` — and a new `scenarios/006-fleet-spawn-metrics-sessions.md` (the next free number) encodes the operator walk: `status: draft` frontmatter at authoring time (flipped to `active` in the operator rung, after the first successful walk), a "Why a scenario" section naming the agent-definition / real-LLM gap, a setup building both fixtures (zero entries; N≥1 entries) in the live vault with cleanup, and the walk asserting the entry, the id `spawn_agent` reported, the spawn window, and the preserved entries. (fires AC 7, AC 10)

## Constraints

- **The accumulator's semantics are frozen.** Entries accumulate (spec 036 AC2), a duplicate session id is appended again, and the write side never starts suppressing entries (spec 038 Constraints). The read side — the interaction counter's per-distinct-session dedupe (spec 038) — is unchanged.
- **The session-id write path is unchanged.** `claude_session_id` is still written by `vault-cli task set`; the new verb never writes it; the auto-resume gate and its condition 2 are untouched; the compensating clear on a failed spawn and the per-session lock keep their behaviour.
- **The write stays passive.** The worker invokes a command; it never edits a task file. No `Edit` / `Write` of a task file appears in the call site's step.
- **The write goes through the existing domain append and storage write path**, so prior entries survive byte-for-byte and unknown frontmatter keys round-trip (the repo's map-based frontmatter invariant).
- **The verb resolves the task exactly as `task set` does** — vault resolution plus first-success dispatch across the configured vaults — so the call site needs no new flags and behaves identically under `--vault` and under all-vaults.
- **Output contract mirrors `task set`**: a plain confirmation line by default, and an `--output json` object whose `success` reflects the outcome; command files never import `encoding/json` (they use the `PrintJSON` helper).
- **Read-side consumers unchanged**: `metrics_sessions[].session_id` unions (`commands/task-status.md`, `docs/output-formatting.md` § Anchor pair) and `metrics_interaction_count` / `metrics_completed_at` / `metrics_cycles`.
- **The field stays out of the `add` / `remove` list allowlist**; the refusal is the behaviour, not an allowlist entry.
- **The Phase 3 status-flip rule in `agents/work-on-task-assistant.md` is unchanged.**
- **The append is not a MANDATORY mutation.** It is not added to the `<critical_writes>` list in `agents/work-on-task-assistant.md`; a non-zero exit is a warning, not a ⚠️ failure, and never blocks "Ready to work on this task." The `<constraints>` READ-ONLY whitelist is likewise unchanged — the write is performed by vault-cli, not by the agent's own edit surface.
- **Existing tests pass**; the six InteractionCounter specs and spec 038's dedup specs are unmodified.
- **`make precommit` passes**, with a `## Unreleased` CHANGELOG entry below the preamble block.

## Failure Modes

| Trigger | Expected behavior | Detection | Reversibility | Recovery |
|---------|-------------------|-----------|---------------|----------|
| The agent definition loses the invocation (a later edit to § Session connect) | Fleet runs land no metrics entry again — the defect returns silently | The call-site test fails in `make test`; the scenario walk fails | Reversible | Restore the invocation in § Session connect; re-run `make test` |
| The verb exits non-zero (task not found, vault mismatch, write error) | The step reports a warning naming metrics and the reason; `claude_session_id` stays written; this run has no entry | The warning line in the run's report; `grep -c '^metrics_sessions:' <task>` below the number of runs | Reversible | Re-run `vault-cli task append-metrics-session "<task>" "<id>"`; confirm the entry in the file |
| A caller passes a session id that is not the task's current session | A corpus row for an unrelated session; the counter can attribute that session's turns to this task | The entry's `session_id` differs from the task's `claude_session_id` | Reversible (one row, recoverable from git) | Correct or remove that entry in the task's frontmatter — the passive-write rule governs the mechanism, not an operator repairing a bad row |
| Two processes append to the same task concurrently (the worker's session-connect and a manager's `work-on` on the same file) | Last write wins; one entry can be lost — the same read-modify-write race the existing `work-on` path has | The entry count is lower than the number of runs | Partial (a lost row; no corruption) | Re-run the append for the missing id |
| A run on a task whose `status` is not `in_progress` | The status flip routes through `vault-cli task work-on`, whose cached path appends a second entry for the same session id — two entries, one session | Two entries carrying the same `session_id` | Reversible | None needed: this is the accumulation contract (spec 036 AC2) and the counter dedupes per distinct session (spec 038). Do NOT suppress it on the write side |
| The `work-on` path's entry shape changes later while the verb is not updated | The two writers diverge and the corpus holds two shapes | The parity comparison (AC 3) fails on the key set | Reversible | Bring the verb's entry to the new shape, or share the append |
| The local clock is wrong (skew) | The entry's `started_at` falls outside the spawn window while the entry itself is valid | The window comparison in the Post-Deploy AC fails while the entry exists | Reversible | None for the data — the window check is a verification instrument, not a data requirement |
| The task file is absent or unreadable | Non-zero exit, nothing written | The exit code and the step's warning | Reversible | Fix the task name or vault selection; re-run |
| A malformed id reaches the verb (empty, non-UUID, newline-bearing, path separator) | Refused before the YAML write; nothing written | Non-zero exit plus output naming the required shape | Reversible | Pass the UUID the session-connect step detected |

No network I/O is involved (the verb reads and writes one local file), so external-system unavailability and rate limiting are not applicable. The append is a single read-modify-write over one task file, so a very long `metrics_sessions` list is handled in linear time with no unbounded memory growth.

## Security / Abuse Cases

- **Attacker-controlled input is the session id argument.** The caller is a worker process (an LLM), and the value originates from a transcript filename. It is validated as a well-formed UUID before it reaches the YAML writer, so a value carrying a newline, a `---` block break, or a path separator cannot be injected into the frontmatter block or the file layout; an invalid id is refused with nothing written. The frontmatter is YAML, so this validation is the boundary that keeps a caller-supplied string from becoming structure.
- **Trust boundary.** The verb writes the operator's vault with the operator's permissions; the calling worker is untrusted. Its only mutation is one list entry on one task, resolved by name through the existing find-by-name path — no new path is constructed from input, no arbitrary file is written, and the write is confined to the resolved task file.
- **Nothing can hang or retry forever.** No network, no subprocess, no retry loop: one read-modify-write per invocation. The call site invokes the verb once per session-connect and continues on failure, so a failing vault never stalls the worker's session-connect.
- **An id must never describe a session that is not live.** The call site passes only an id it detected from a live transcript under the existing `LIVE_WINDOW` rule; the verb does not widen that window, guess, or fall back to a name, so a dead session is never recorded as this task's run.
- **No secret material.** The entry carries a session id and a timestamp; nothing beyond the id is logged, and the write touches only the vault — the read-only contract on `~/.claude` is unchanged.

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | The append verb: the ops operation exposing the existing domain append, its CLI registration, the refusal of `set` / `add` / `remove`, the id guard, the integration-test entry and the verb-level integration tests | 1, 2, 3, 4, 5 | 1, 2, 3, 4, 5, 8, 9 (registration half) | — |
| 2 | The session-connect call site in `agents/work-on-task-assistant.md` — invoke after the id write, in the branch that writes it, with the warning on non-zero exit | 6 | 6 (call-site half) | prompt 1 (the frozen verb name) |
| 3 | The call-site regression lock: `scripts/metrics-append-call-site-test.sh` (section-scoped assertion plus the strip-one-artifact self-check) wired into `make test` via the `Makefile` target that already runs `scripts/struck-row-rule-test.sh` | 7 | 6 (script half) | prompt 2 |
| 4 | Record the second writer and the walk: `README.md`, `commands/work-on-task.md` § Passive metrics, `docs/work-on-session-lifecycle.md`, `scenarios/006-fleet-spawn-metrics-sessions.md`, `CHANGELOG.md` | 8 | 7, 9 (CHANGELOG half), 10 (scenario content) | prompts 1, 2 |
| 5 | No code prompt — operator rung: release, install both halves, walk the scenario against a real fleet spawn on both fixtures | — | 10 | prompts 1–4 |

Rationale: prompt 1 is the entire capability and is self-contained in the ops/CLI layer with its integration tests; prompt 2 is the Direct-layer agent-definition edit and depends only on the verb name being frozen by prompt 1; prompt 3 mechanises prompt 2's prose; prompt 4 records the behaviour where the repo keeps its prose and encodes the walk; prompt 5 is the operator-executable rung that the fleet behaviour demands and is not a prompt. This is one spec rather than two because the behaviour is one deployable change — fleet-spawned runs land a metrics entry — and every seam above is a facet of it: splitting the call site from the verb would ship a verb nothing calls, and splitting the refusal from the verb would ship a field still writable in a shape the readers discard. The `DB × AC` product exceeds 50 for that reason, and this table is the answer to it.

## Do-Nothing Option

Fleet-spawned runs stay invisible to `metrics_sessions`, so session-cost and interaction analytics keep under-counting them, and the under-count grows with the fleet — the runs the manager layer exists to produce are the ones missing from the numbers that justify it. The source task's SC3 stays unmet, and the metrics ranking that motivates the fleet keeps reading a population that excludes fleet work. The current approach is acceptable only if session-cost analytics are not used for fleet decisions; the driving task's Impact section says they are.
