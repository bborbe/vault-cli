---
status: draft
---

## Summary

- Two `vault-cli` defects found in one session: `task-auditor` recommends a goal link its own orphan test then scores as MAJOR, and a `metrics_sessions` entry cannot be removed once written.
- **The removal gap is now load-bearing.** Spec 053 deliberately left removal to a hand-edit (*"Do NOT add a `clear` path for the field"*). The shared-session rule that shipped the same evening as that spec requires clearing `claude_session_id` **and any `metrics_sessions` entry carrying the shared id** — the id set is read from both fields, so a surviving metrics entry keeps the collision alive. Hand-editing is barred by `work-on-task`'s own contract.
- The generic write verbs refuse the field **absolutely** (spec 053, commit `f71296b`) because their scalar and comma-split shapes cannot express a map entry — so the fix is a dedicated verb, not an allowlist entry.
- `task remove-metrics-session <task> <session-id>` removes every entry carrying that id, preserves every other entry, and deletes the key when the last entry goes.
- The auditor fix is one guard sentence in the Task-Goal Alignment section, so the advice and the test stop using different predicates.

**Why one spec, not two.** The two defects share no code and can be reverted independently — the ordinary reading of *"two features with different do-nothing arguments = two specs"* says split. This spec keeps them together deliberately, on the driving task's decision: the task ships both fixes in one PR, and both are `vault-cli` tool repairs found in the same session against the same field family. The bundle is a packaging choice, not a coupling claim — prompt 2 (the auditor sentence) is the Direct-layer markdown half and can be dropped from the decomposition and shipped on its own without touching prompt 1. The size signals the bundle produces are stated rather than hidden: DB × AC = 88 against a threshold of 50, and 7 code layers against a threshold of 3. The `## Suggested Decomposition` table is the mitigation.

## Problem

**Defect 1 — the auditor contradicts itself.** `agents/task-auditor.md` § Task-Goal Alignment flags as MAJOR any goal link that advances none of the goal's criteria, while the advice in the same section suggests linking a goal on family resemblance. Both rules were followed correctly on 2026-09-15, and the cost was two extra audit cycles (~4 min, ~18k tokens) plus an operator ruling to unblock a single task: run 1 recommended *"Link `[[Fleet Communication]]` (in_progress) in `goals:`"*, the operator took it, and runs 2 and 3 then scored that same link *"ORPHAN (partial) — MAJOR … The goal can be marked complete without this task."*

**Defect 2 — a metrics entry cannot be removed.** `metrics_sessions` entries are maps (`session_id` + `started_at`). `task set` stores a scalar and `task add` / `task remove` comma-split into a list of scalars; every reader discards both shapes, so spec 053 made all three verbs refuse the field before any mutation, with no `--force` bypass. That refusal is correct and stays. But it left **no** programmatic removal path at all: spec 053's own non-goal says *"removing entries stays an operator frontmatter action"*. The shared-session rule ([[Manager Session]] § Step 4) then required exactly that action, on a field whose contract says *"must not be hand-edited"*. Measured 2026-09-27: **68 of the Personal vault's 722 task files carry `metrics_sessions` ids**, so the population the rule operates on is not hypothetical.

**Why not the allowlist.** Adding `metrics_sessions` to `knownTaskListFields` cannot work, and the three reasons are mechanical, not stylistic: (1) `metricsSessionsWriteRefusal` runs at `pkg/ops/frontmatter_entity.go:744`, **before** the allowlist check at `:750`, so the refusal still fires and the entry changes nothing; (2) the dispatch switch at `:760-768` reads only `Goals()` / `Tags()` / `BlockedBy()` — all `[]string` — so a map-valued field falls through with `current == nil` and sets nothing: a silent no-op; (3) the allowlist is shared by `add` and `remove` (`mode` is a field on one `Execute`), so it would also enable `task add metrics_sessions …`. The integration suite's AC4 pins the refusal by asserting `set`, `add` and `remove` each exit 1 with a byte-identical file.

## Goal

An operator can remove exactly one session's `metrics_sessions` entries from a task through a documented verb, without touching frontmatter by hand and without weakening the refusal that protects the field. Every other entry survives byte-identical, a task whose last entry is removed carries no empty `metrics_sessions` key, and a call that matches nothing is **visible** rather than a silent no-op. Separately, `task-auditor` never recommends a goal link its own alignment check would then score as an orphan.

## Non-goals

- ⚠️ **This spec supersedes spec 053's non-goal *"Do NOT add a `clear` path for the field — removing entries stays an operator frontmatter action."*** That decision is reversed here, deliberately, because the shared-session rule created the need. A future reader finding both specs must read this one as the later ruling.
- **Do NOT weaken `metricsSessionsWriteRefusal`.** `task set`, `task add` and `task remove` keep refusing `metrics_sessions` with no `--force` bypass, and AC4 keeps passing unchanged. Exempting `remove` from the refusal was considered and rejected — see `# Problem` for the three mechanical reasons the allowlist route cannot work, and `# Suggested Decomposition`'s rationale for why the dedicated verb is the only shape that leaves the refusal intact.
- **Do NOT add `metrics_sessions` to `knownTaskListFields`.** The allowlist entry is inert (see Problem) and would additionally open `task add metrics_sessions …`.
- **Do NOT remove a task's `metrics_sessions` key wholesale.** `ClearMetricsSessions` already exists and runs on task completion (`pkg/ops/complete.go:303`); this spec adds per-entry removal only, and adds no flag that clears the field.
- **Do NOT add a `--force` / `--all` / `--started-at` flag**, and do not let the verb remove by timestamp — removal is by session id, which is the key the shared-session rule reads.
- **Do NOT change the append path.** `task append-metrics-session`, its accumulator semantics, and `AppendMetricsSession` are untouched.
- **Do NOT change `claude_session_id`'s write path.** The id write stays `vault-cli task set`; this verb never writes it.
- **Do NOT edit the plugin cache** at `~/.claude/plugins/cache/vault-cli/vault-cli/*` — install artifact, clobbered by the next `task` update.
- **Other frontmatter list fields** (`themes`, `blocked_by`) are out of scope; only `metrics_sessions` is in scope.
- **No re-audit of existing tasks** that already carry a non-advancing goal link.

## Acceptance Criteria

Fixture vault for ACs 1–6: a temp vault with `Tasks/Alpha.md` and a config naming `tasks_dir: Tasks` (the shape `integration/cli_test.go` already builds). `<bin>` is the binary built from HEAD — `gexec.Build` inside `integration/`, or `/tmp/new-vault-cli` on the host. `Alpha`'s frontmatter holds `page_type: task`, `priority: 1`, `status: in_progress`, `task_identifier: <UUID>`, and `metrics_sessions` in whichever shape the AC names. `S1` / `S2` / `S3` are distinct well-formed UUIDs.

- [ ] **Removal drops exactly one entry and preserves the rest.** `Alpha` holds entries for `S1` (started 2026-09-01T08:00:00Z) and `S2` (started 2026-09-02T08:00:00Z). `<bin> --config <cfg> task remove-metrics-session Alpha S1` exits 0, and `Tasks/Alpha.md` then holds **exactly one** `metrics_sessions` entry, whose `session_id` is `S2` and whose `started_at` is `"2026-09-02T08:00:00Z"` — the survivor keeps its own id **and** its own timestamp, so a filter that dropped the wrong entry or rewrote the survivor fails here — evidence: exit code 0, file content (`grep -c '^    - session_id:'` returns 1), and the survivor's `session_id`/`started_at` lines. The absence assertion must use the entry-line prefix (`grep -c -- '- session_id: S1'` returns 0), never a bare id substring: the fixture also carries `S1` as its `task_identifier`, so a bare substring matches the surviving base key.
- [ ] **Removing the last entry deletes the key.** From the same two-entry fixture, removing `S1` then `S2` leaves `Tasks/Alpha.md` with **no** `metrics_sessions` key at all — `grep -c 'metrics_sessions' Tasks/Alpha.md` returns 0 — while `page_type`, `priority`, `status`, `task_identifier` and the body line are unchanged — evidence: file content (zero key occurrences) plus negative evidence on the base keys.
- [ ] **A duplicate id is removed in full.** `Alpha` holds three entries, two of them carrying `S1`. `<bin> --config <cfg> task remove-metrics-session Alpha S1` exits 0 and leaves exactly **one** entry, the one carrying `S3` — evidence: file content (entry count 1, `session_id: S3`).
- [ ] **A call matching nothing is visible and writes nothing.** On the two-entry fixture, `<bin> --config <cfg> task remove-metrics-session Alpha S3` exits **non-zero** with a message naming the id and stating that nothing was removed, and `Tasks/Alpha.md` is **byte-identical** (`sha256` before and after) — evidence: exit code non-zero, stderr match, and negative evidence (file hash equal). A silent no-op is the exact defect this verb exists to fix, so the absent-id path must fail rather than report success.
- [ ] **An id the verb cannot honour is refused with nothing written.** `<bin> --config <cfg> task remove-metrics-session Alpha ""`, `… Alpha "not-a-uuid"`, and `… Alpha "../escape"` each exit non-zero with a message naming the required shape (a well-formed UUID), and `Tasks/Alpha.md` is byte-identical after all three — evidence: three exit codes, stderr match, and negative evidence (file hash equal).
- [ ] **The refusal is untouched.** `<bin> --config <cfg> task set Alpha metrics_sessions 'x'`, `… task add Alpha metrics_sessions 'x'`, and `… task remove Alpha metrics_sessions 'x'` each still exit non-zero naming `metrics_sessions` and `append-metrics-session`, and `Tasks/Alpha.md` is byte-identical after all three — evidence: three exit codes, stderr match, and negative evidence (file hash equal). The existing AC4 spec in `integration/cli_test.go` is **unmodified** and passes.
- [ ] **The command is registered, its output contract holds, and the suite passes.** `make precommit` exits 0, and `make test` exits 0 apart from exactly one named pre-existing failure — `topic defer writes defer_date for a relative and an absolute date`, which is red only in the window where the local date leads the UTC date (see Constraints) and which this change must **not** touch; `grep -n 'createTaskRemoveMetricsSessionCommand' pkg/cli/cli.go` returns **≥2** lines (the `cmd.AddCommand` registration and the factory definition) — ⚠️ do **not** grep for the hyphenated verb literal: it appears exactly **once** in that file (the `Use:` field), because the constructor is camelCase, and the twin `append-metrics-session` behaves identically, so a `≥2` threshold on it is unachievable by a correct implementation; `go test ./integration/... -ginkgo.focus="remove-metrics-session"` exits 0; and `<bin> --config <cfg> task remove-metrics-session Alpha S1 --output json` emits an object whose `success` is `true` for a matching id and `false` for a non-matching one — evidence: exit codes, grep line counts, the focus-run, and both JSON objects. No other failure is acceptable, and a run in which the named flake is green is a pass.
- [ ] **The auditor's advice and its test share one predicate.** `grep -n 'never recommend linking a goal the alignment check will then score as an orphan' agents/task-auditor.md` returns ≥1 line, and that line lies **after** the `**Flag orphans as MAJOR**` bullet and **before** the `**Flag implementation-level tasks**` bullet in § Task-Goal Alignment (checked section-scoped, not file-wide) — evidence: file content (line number between the two bullet line numbers).
- [ ] **Post-Deploy (Rung-2): the auditor no longer emits the contradictory recommendation.** The `task-auditor` agent is loaded from the installed plugin, so this AC cannot pass before release. Two observations: `grep -n 'never recommend linking a goal the alignment check will then score as an orphan' ~/.claude/plugins/cache/vault-cli/vault-cli/<version>/agents/task-auditor.md` returns ≥1 line, **and** running `task-auditor` against a task that has no `goals:` link and whose work advances no candidate goal's criteria produces a report containing **no** line matching `Link \[\[.*\]\] in goals:` and **does** contain an orphan/theme-linkage verdict — evidence: the load-path grep (file content), plus negative evidence (the recommendation pattern returns 0 lines) and the verdict line.
  - `deploy_check:` `bash -lc 'b=$(vault-cli --version | grep -oE "[0-9]+\.[0-9]+\.[0-9]+" | head -1); p=$(for d in ~/.claude/plugins/cache/vault-cli/vault-cli/*/; do sed -n "s/.*\"version\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$d/.claude-plugin/plugin.json"; done | sort -V | tail -1); [ -n "$b" ] && [ "$b" = "$p" ] && echo "$b" || exit 1'`
  - `deploy_target:` `$(git describe --tags --abbrev=0 origin/master | sed 's/^v//')`
- [ ] **The removal path is documented where the repo records behaviour.** `docs/work-on-session-lifecycle.md` records that removal is its own verb and why the generic verbs cannot express a map entry, `CHANGELOG.md` carries a `## Unreleased` section holding a `feat:` bullet for the verb and a `fix:` bullet for the auditor — evidence: file content (`grep -n 'remove-metrics-session' docs/work-on-session-lifecycle.md` returns ≥1 line; the `## Unreleased` section exists above the newest `## vX.Y.Z` with both bullets).
- [ ] **Post-Deploy (Rung-2): both halves are installed and the verb works on the load path.** After merge and release, `make install` has been run and `claude plugin update vault-cli@vault-cli` has been run; the installed binary and the released plugin agree on the version; and running `vault-cli task remove-metrics-session "<a real task carrying two entries>" "<one of its ids>"` against a **copy** of a real vault task file removes only that entry, leaving the other intact. Evidence: the version string from `vault-cli --version`, the same string read from the load-path `plugin.json`, and the real-file before/after entry counts.
  - `deploy_check:` `bash -lc 'b=$(vault-cli --version | grep -oE "[0-9]+\.[0-9]+\.[0-9]+" | head -1); p=$(for d in ~/.claude/plugins/cache/vault-cli/vault-cli/*/; do sed -n "s/.*\"version\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$d/.claude-plugin/plugin.json"; done | sort -V | tail -1); [ -n "$b" ] && [ "$b" = "$p" ] && echo "$b" || exit 1'`
  - `deploy_target:` `$(git describe --tags --abbrev=0 origin/master | sed 's/^v//')`

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — format + generate + test + check, exits 0
- `make test` — full suite; the new verb's integration specs pass and the existing `AC4` spec is unmodified
- `go test ./integration/... -ginkgo.focus="remove-metrics-session"` — exits 0
- `go test ./pkg/domain/...` — the `RemoveMetricsSession` unit specs pass
- `grep -n 'remove-metrics-session' pkg/cli/cli.go docs/work-on-session-lifecycle.md CHANGELOG.md` — ≥1 line per file
- `grep -n 'never recommend linking a goal the alignment check will then score as an orphan' agents/task-auditor.md` — ≥1 line

### Operator-executable (runs on the host after merge, spec verification ladder)

- `go build -o /tmp/new-vault-cli .` — fresh binary from HEAD; replay ACs 1–6 against a temp vault with it
- Merge to `bborbe/vault-cli` master, then let `github-releaser-agent` cut the tag (`.maintainer.yaml` carries `release.autoRelease: true`) — do **not** hand-cut it
- Install **both halves** — the verb is Go, so the plugin update alone leaves it unreachable: `make install` (the binary must win on PATH over any shadowing copy — see the [[Install vault-cli]] runbook trap) **and** `claude plugin update vault-cli@vault-cli`; then confirm the released version in the load path
- Exercise AC 9 by running `task-auditor` against a task carrying a non-advancing goal link and reading the report
- Exercise the verb against a **copy** of a real task file carrying two `metrics_sessions` entries

## Desired Behavior

1. A vault-cli verb — `task remove-metrics-session <task-name> <session-id>` — removes **every** `metrics_sessions` entry on the named task whose `session_id` equals the supplied id, and preserves every other entry byte-identical, along with every other frontmatter key (known and unknown). The name is frozen: it is the counterpart of `append-metrics-session`, and the pair is the field's documented write surface. (fires AC 1, AC 3)
2. When the removal leaves no entries, the `metrics_sessions` key is **deleted** rather than written as an empty list, so a task that has had its last session removed is indistinguishable from one that never had an entry. (fires AC 2)
3. A call whose id matches no entry exits **non-zero** with a message naming the id and stating that nothing was removed, and writes nothing at all — the file is byte-identical. A silent success here would reproduce the exact defect class this verb exists to close (a write that reports success while recording nothing). (fires AC 4)
4. A session id the verb cannot honour — empty, not a well-formed UUID, or carrying a path separator — is refused **before** the task is read, with a message naming the required shape and nothing written. (fires AC 5)
5. The generic write verbs are untouched: `task set`, `task add` and `task remove` still refuse `metrics_sessions` before any mutation, with no `--force` bypass, and the allowlist `knownTaskListFields` is unmodified. (fires AC 6)
6. The verb resolves the task exactly as `task set` does — vault resolution plus first-success dispatch across the configured vaults — so it behaves identically under `--vault` and under all-vaults, and its output contract mirrors `append-metrics-session`: a plain confirmation line by default, and an `--output json` object whose `success` reflects the outcome. (fires AC 7 — the JSON half is asserted there; the vault-resolution half is carried by the mirror-the-append constraint rather than separately asserted, because it reaches runtime through the same dispatcher path every other `task` subcommand uses, and no AC for it would add signal.)
7. `agents/task-auditor.md` § Task-Goal Alignment carries one guard sentence, placed **after** the `Flag orphans as MAJOR` bullet and before the implementation-level bullet, so the rule that judges a goal link and the advice that recommends one use the same predicate: if the goal can be marked complete without this task, recommend theme-only linkage and say so. (fires AC 8, AC 9)
8. The removal path and the auditor change are recorded where the repo keeps its prose — `docs/work-on-session-lifecycle.md` § the write-verb section, and a `## Unreleased` CHANGELOG section carrying a `feat:` bullet for the verb and a `fix:` bullet for the auditor. (fires AC 10)

## Constraints

- **The refusal is a frozen invariant.** `metricsSessionsWriteRefusal` and `knownTaskListFields` are not modified by this change, and the integration suite's AC4 spec is not modified either. Any diff that touches them fails review.
- **The append path is unchanged.** `task append-metrics-session`, `AppendMetricsSessionOperation` and `domain.TaskFrontmatter.AppendMetricsSession` keep their behaviour and their accumulate-never-replace semantics.
- **The write goes through the existing domain and storage path**, so unknown frontmatter keys round-trip and prior entries survive byte-for-byte (the repo's map-based frontmatter invariant). The symbol names are frozen because the Verification rung and the generated prompt reference them: the domain method is `RemoveMetricsSession` on `TaskFrontmatter` (taking the session id, returning the number of entries removed), and the op is `RemoveMetricsSessionOperation` built by `NewRemoveMetricsSessionOperation`. A string-level rewrite of the `metrics_sessions:` block that bypasses the domain and `TaskStorage.WriteTask` would satisfy every AC while violating this constraint — which is why the domain unit specs are part of the Verification rung.
- **Command files never import `encoding/json`** — output goes through the `PrintJSON` helper, as in every other command in `pkg/cli/cli.go`.
- **The verb writes only when something changed.** When no entry matches, no `WriteTask` call happens at all, so the file is byte-identical by construction rather than by a rewrite that happens to be stable.
- **The verb never writes `claude_session_id`**, and it takes no clock: unlike the append, it stamps nothing, so its constructor takes only `TaskStorage`.
- **A counterfeiter mock is generated** for the new operation interface, following `mocks/append-metrics-session-operation.go`, and `make generate` regenerates it. ⚠️ `make generate` rewrites `mocks/mocks.go` as a bare `package mocks` line and **strips its four-line copyright header** — a pre-existing generator defect. The commit must exclude that file (explicit pathspec) rather than carry the deletion.
- **Existing tests pass.** The `InteractionCounter` specs, spec 038's dedup specs, and the whole `append-metrics-session` block are unmodified.
- **`make precommit` passes**, with a `## Unreleased` CHANGELOG section below the preamble block and above the newest `## vX.Y.Z`.
- ⚠️ **Known pre-existing failure, not this change's to fix.** `integration/cli_test.go`'s `topic defer writes defer_date for a relative and an absolute date` is red for the ~2 h each day when the local date leads the UTC date: it computes its expected value from `time.Now().UTC().AddDate(0,0,7)` while the CLI writes the local date. Measured 2026-09-27 00:48 CEST: UTC+7d = `2026-10-03`, local+7d = `2026-10-04`. CI runs UTC and never sees it. The fix is a design call (the test's expectation or the CLI's date basis) and is out of scope; a prompt must not "fix" it opportunistically.

## Assumptions

- The field's entries are maps (`session_id` + `started_at`), not scalars — which is why the generic verbs' scalar and comma-split shapes cannot carry them, and why a dedicated verb is the only viable shape.
- The map-based frontmatter invariant holds: the storage writer re-serializes the whole frontmatter in alphabetical key order on every write, so a fixture must be authored alphabetically or every byte comparison in the ACs is meaningless.
- The shared-session rule needs **per-entry** removal, not whole-field clearing: the id set is read from `claude_session_id` **and** every `metrics_sessions` id, so clearing the whole field would destroy legitimate runs' entries while a surviving entry would keep the collision alive. `ClearMetricsSessions` (which fires on task completion) is therefore not the tool.
- A session id normally appears at most once per task, but duplicates are reachable — the append accumulator deliberately never suppresses a repeat — so removal must handle N > 1 rather than assuming uniqueness.
- The `task-auditor` agent is loaded from the installed plugin, so its post-change behaviour is observable only after `claude plugin update vault-cli@vault-cli`; the source-level AC (AC 8) is checkable before release, the behavioural one (AC 9) is not.
- The vault's task files are the only storage: there is no index, cache, or derived artifact that also carries `metrics_sessions` and would need updating.

## Failure Modes

| Trigger | Expected behavior | Detection | Reversibility | Recovery |
|---------|-------------------|-----------|---------------|----------|
| The id matches no entry (typo, or a peer already cleared it) | Non-zero exit naming the id; nothing written; file byte-identical | The exit code and the message | Reversible | Re-read the task's `metrics_sessions` ids and re-run with the right one; if a peer already cleared it, no action |
| A malformed id reaches the verb | Refused before the task is read; nothing written | Non-zero exit plus output naming the required shape | Reversible | Pass a well-formed UUID |
| The task file is absent or unreadable | Non-zero exit, nothing written | The exit code and the wrapped `find task` error | Reversible | Fix the task name or vault selection; re-run |
| The last entry is removed | The key is deleted, not left empty | `grep -c 'metrics_sessions' <task>` returns 0 | Reversible (re-append via `append-metrics-session`) | None needed — the deletion is the contract |
| A future edit adds `metrics_sessions` to `knownTaskListFields` | The field becomes writable through `add` in a shape readers discard — the spec-053 defect returns | The AC4 spec fails in `make test`; AC6 fails | Reversible | Remove the allowlist entry; the refusal is the behaviour |
| A future edit exempts `remove` from the refusal | Same silent-divergence hole, reached through the generic verb | AC6 fails; the refusal's own test fails | Reversible | Restore the refusal; the dedicated verb is the supported path |
| Two processes mutate the same task file concurrently | Last write wins — the same read-modify-write race the append path has; a removal can be lost | The entry survives when it should have gone | Partial (a lost removal; no corruption) | Re-run the verb |
| The auditor sentence is later edited away | The contradictory recommendation returns silently | AC 9 fails on the next audit; the grep in AC 8 fails | Reversible | Restore the sentence between the two bullets |

No network I/O is involved — the verb reads and writes one local file — so external-system unavailability and rate limiting are not applicable. Removal is a single read-modify-write over one task file, linear in the number of entries.

## Security / Abuse Cases

- **Attacker-controlled input is the session-id argument.** The caller is an operator or an agent, and the value is validated as a well-formed UUID **before** the task is read, so a value carrying a newline, a `---` block break, or a path separator cannot be injected into the frontmatter block or the file layout. This mirrors `append-metrics-session`'s guard exactly, and it is the boundary that keeps a caller-supplied string from becoming YAML structure.
- **Trust boundary.** The verb writes the operator's vault with the operator's permissions. Its only mutation is removing list entries on one task, resolved by name through the existing find-by-name path — no path is constructed from input, no arbitrary file is written, and the write is confined to the resolved task file.
- **Nothing can hang or retry forever.** No network, no subprocess, no retry loop: one read-modify-write per invocation.
- **No secret material.** The verb reads and writes session ids and timestamps already present in the task file; nothing is logged beyond the id and the task name, and no new file is touched.

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | The removal verb: the domain method, the ops operation, the CLI command and its registration, the generated mock, and the integration + domain specs (two-entry removal, key deletion, duplicate id, absent id, malformed id). Also asserts the refusal is untouched — the same prompt must leave `metricsSessionsWriteRefusal` and `knownTaskListFields` byte-identical | 1, 2, 3, 4, 5, 6 | 1, 2, 3, 4, 5, 6, 7 | — |
| 2 | The auditor guard sentence in `agents/task-auditor.md` § Task-Goal Alignment | 7 | 8 | — |
| 3 | The prose: `docs/work-on-session-lifecycle.md` § the write-verb section and the `## Unreleased` CHANGELOG section | 8 | 10 | prompt 1 (the frozen verb name) |
| 4 | No code prompt — operator rung: release, install both halves, exercise the verb on a real task file and the auditor on a real task | — | 9, 11 | prompts 1–3 |

Rationale: prompt 1 is the entire capability and is self-contained in the domain/ops/CLI layers with its own tests; prompt 2 is the Direct-layer agent-definition edit, independent of the Go change; prompt 3 records the behaviour and depends only on the verb name being frozen; prompt 4 is the operator-executable rung. The two defects ride one spec because the driving task ships them together in one PR and both are `vault-cli` tool repairs found in one session — but they share no code, and prompt 2 can land or be reverted independently of prompt 1.

## Do-Nothing Option

The `metrics_sessions` field stays write-only in practice: entries can be appended but never removed, so the shared-session rule's clearing step has no programmatic path and every collision is cleared by hand-editing a field whose own contract forbids it. The next collision costs an operator the same manual repair, and the field's documented contract and its actual reachable operations stay in disagreement. Separately, `task-auditor` keeps spending two extra audit cycles plus an operator ruling on each task where its advice and its test disagree. Both costs are small per occurrence and unbounded in count, and both were measured once already on 2026-09-15.
