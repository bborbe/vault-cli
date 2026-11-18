---
status: completed
spec: [069-task-approve-records-reapproved-at]
summary: '`task approve` now records `reapproved_at` on its re-approval branch, pinned by unit, integration, and end-to-end probe coverage'
execution_id: vault-cli-reapproved-at-exec-284-spec-069-task-approve-records-reapproved-at
dark-factory-version: v0.198.17
created: "2026-11-18T09:00:00Z"
queued: "2026-11-18T19:23:12Z"
started: "2026-11-18T19:26:05Z"
completed: "2026-11-18T19:33:38Z"
---

# `task approve` records `reapproved_at` on its re-approval branch (spec 069)

<summary>
- A re-approval becomes an observable event with a timestamp, not just a phase change.
- Any consumer reading a task row can answer "was this row re-approved, and when?" from the frontmatter alone.
- The first approval is unchanged: it still records the approver and the instant, and never writes the new key.
- The existing approval record is still preserved byte-for-byte on re-approval, so a first approval and a re-approval stay distinguishable.
- Each further re-approval refreshes the new timestamp, because it records the latest event rather than the first one.
- The re-approval branch stays presence-based, so a row carrying only part of the approval record still gets the new timestamp.
- A finished row is still refused and still writes nothing, including the new key.
- The command's built-in help names the new key and says it is written only on a re-approval, so a reader can discover it without a spec.
- Reading the value back works through the existing field reader — no new verb, no new flag, no new output path.
- One write per invocation, as today; the new key travels in that same write.
</summary>

<objective>
Make a re-approval observable and timestamped: when `vault-cli task approve` advances a row that already carries an approval record, it must record the instant of that re-approval in a new frontmatter key, `reapproved_at`, in the same single write that moves the phase to planning. Today the re-approval branch moves the phase and writes nothing else, so the event is invisible — a consumer that must react to "the operator cleared this wait just now" cannot distinguish a re-approval a minute ago from one three weeks ago, nor either of them from a row that was never re-approved. This is spec 069, one prompt covering Desired Behaviors 1–6 and Acceptance Criteria 1–8.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/dod.md` for the definition of done this change is graded against.

Read these files fully before changing anything:

- `pkg/ops/task_approve.go` — `(*taskApproveOperation).Execute`. The change lands in the `if hasApprovalRecord(task)` branch: after the terminal-status refusal and before `return o.write(ctx, task, vaultName)`. `now := o.currentDateTime.Now()` is captured before every guard (the zero-clock guard); `now.Time()` is the `time.Time` the first-approval path already writes as `approved_at`. `hasApprovalRecord` stays presence-based on `task.Keys()` — do not touch it.
- `pkg/ops/task_approve_test.go` — the Ginkgo/Gomega suite this change extends. Read the existing `It("records the injected instant as a bare time.Time")` and `It("serializes approved_at unquoted and round-trips it as a time.Time")` (the two shape tests the new key mirrors), the `DescribeTable("advances a todo task that already carries an approval record")` block (the preservation pin that must stay byte-identical), `It("refuses a zero clock with zero writes")`, and `DescribeTable("refuses a terminal row that is still at phase todo, with zero writes")`. Note how `seedTask` replaces the task the mock returns and how `pinned.SetNow` moves the injected clock.
- `pkg/cli/cli.go` — `createTaskApproveCommand`, its `Long` field. Only the second paragraph changes; the unbounded-approval paragraph and the operator-flag paragraph are untouched.
- `pkg/cli/task_approve_help_test.go` — the existing help-text pin from spec 067. It requires a clause containing `preserv` that names `approved_at` to appear at or after the phrase `records approved_by and approved_at in one write`. The new sentence must not break it.
- `integration/cli_test.go` — `Describe("task approve", ...)`. Read the helpers `runEntityCommand`, `readFile`, `valueOf`, `frontmatterOf`, `sha256OfFile`, and the fixtures `todoFrontmatter`, `approvedByOnlyFrontmatter`, `approvedAtOnlyFrontmatter`, `approvedBothFrontmatter`. Read `It("AC6: a task carrying any approval record is advanced, per fixture")` — the re-approval case this prompt extends — and `It("AC1: task approve writes phase/approved_by/approved_at in one write")`, whose `Expect(approvedKeys(after)).To(HaveLen(4))` must stay 4 because a first approval writes no new key.
- `CHANGELOG.md` lines 1–16 only — a `## Unreleased` section already exists and already carries the bullet for this change (`task approve` writes `reapproved_at` on its re-approval branch). Do not add a second one.
- `docs/task-writing.md` — read the approval-record paragraph in § Phase transitions and change nothing in it. It stays true: `approved_by` and `approved_at` are neither overwritten nor fabricated.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega suite shape, external test packages, Counterfeiter mocks.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the `## Unreleased` placement rule and the no-hand-bump rule for an auto-release repository.
</context>

<requirements>
The defect: the `hasApprovalRecord` branch in `Execute` advances a row's phase and preserves the approval record exactly — which is correct — but records nothing about the act itself, so a re-approval is only visible as a phase string and carries no time. Add one key, written only on that branch, in the branch's existing single write.

1. **Write `reapproved_at` on the re-approval branch** in `pkg/ops/task_approve.go`, inside `(*taskApproveOperation).Execute`, in the `if hasApprovalRecord(task)` branch, after the terminal-status refusal and after `task.SetPhase(domain.TaskPhasePlanning.Ptr())`. Reuse the `now` captured before the guards — do not read the clock a second time and do not move or weaken the zero-clock guard.

   Replace this block (the two-line `SetPhase` + `write` adjacency occurs only in this branch):

   ```go
   		task.SetPhase(domain.TaskPhasePlanning.Ptr())
   		return o.write(ctx, task, vaultName)
   	}
   ```

   with:

   ```go
   		task.SetPhase(domain.TaskPhasePlanning.Ptr())
   		// The re-approval is an event with a time of its own: record it in a
   		// separate key, beside the preserved approval record, in the same single
   		// write. A first approval never writes this key, so a row that carries it
   		// was re-approved and the value is when.
   		task.Set("reapproved_at", now.Time())
   		return o.write(ctx, task, vaultName)
   	}
   ```

   Then extend the branch's existing explanatory comment (the paragraph beginning "A todo row that already carries an approval record has already been approved") with one sentence saying the branch records the re-approval's instant in `reapproved_at` while leaving the approval record itself untouched. The comment currently says "no approval is written"; it must not be left reading as if nothing at all is written, or a later reader will delete the new line as a contradiction. Leave the `TaskApproveOperation` interface doc comment and `hasApprovalRecord`'s doc comment unchanged — both speak about the approval record, which is still neither rewritten nor fabricated.

2. **Pin the new key with unit tests** in `pkg/ops/task_approve_test.go`, added after the existing `DescribeTable("advances a todo task that already carries an approval record", ...)` block. Do not modify that table, or any other existing test, in this file. Add these specs, reusing the file's `seedTask` and `pinned` helpers:

   - `It("records reapproved_at as a bare time.Time on the re-approval branch")` — seed `{"status": "next", "phase": "todo", "approved_by": "someone", "approved_at": "2026-01-01T00:00:00Z"}`, execute with `approvedBy "operator"`, then assert on `WriteTaskArgsForCall(0)`: `WriteTaskCallCount()` is 1, `written.Phase()` equals `domain.TaskPhasePlanning.Ptr()`, `written.Get("reapproved_at").(time.Time)` succeeds (`Expect(ok).To(BeTrue())`) and `raw.Equal(pinned.Now().Time())` is true, and `written.Get("approved_by")` / `written.Get("approved_at")` still equal the seeded values. The type assertion is the assertion — a string value must fail this test.
   - `It("serializes reapproved_at unquoted and round-trips it as a time.Time")` — mirror `It("serializes approved_at unquoted and round-trips it as a time.Time")`: `yaml.Marshal(written.RawMap())` must contain `reapproved_at: 2026-03-03T12:00:00Z`, must not contain the quoted form, and unmarshalling the output into `map[string]any` must yield a `time.Time` under `reapproved_at` equal to the pinned instant. This is the serialization boundary — the file on disk is what a consumer reads.
   - `It("writes no reapproved_at on a first approval")` — seed `{"status": "next", "phase": "todo", "assignee": "someone"}`, execute, assert `written.Get("reapproved_at")` is `BeNil()` while `written.Get("approved_at")` is not nil.
   - `DescribeTable("writes reapproved_at for any presence-only approval record", ...)` — one `Entry` per shape, each asserting `written.Get("reapproved_at").(time.Time)` succeeds and the seeded `approved_by` / `approved_at` values are unchanged — snapshot `seededBy := fields["approved_by"]` and `seededAt := fields["approved_at"]` **before** `Execute` (as the existing table's comment at lines 239-242 requires, because `domain.NewTask` stores the caller's map by reference, so reading `fields[...]` after the call compares the written map against itself) and compare with the same nil-aware `expectSameValue` helper shape the existing table uses for absent keys: `Entry("only approved_by", ...)`, `Entry("only approved_at", ...)`, `Entry("an empty approved_by value counts as present", map[string]any{"approved_by": ""})`, `Entry("a present-but-null approved_by", map[string]any{"approved_by": nil})`, `Entry("an unparseable approved_at", map[string]any{"approved_at": "not-a-date"})`. This is what separates a presence-based predicate from a value-based one — a `task.Get("approved_at") != nil` predicate passes the first-approval test and fails every entry here.
   - `It("refreshes reapproved_at on a second re-approval")` — seed the both-keys row, execute (assert the written value equals the pinned instant), then call `seedTask(...)` again with a **fresh map literal** of the same fields **before** the second `Execute` — never reuse the first map variable, because `domain.NewTask` stores the caller's map by reference (the existing preservation table's comment at lines 239-242 warns about this), so a reused map would already read `phase: planning` and the second call would be refused by the inbox guard. Then `pinned.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:05Z"))`, execute again, and assert `WriteTaskCallCount()` is 2, the second written task's `reapproved_at` equals the new pinned instant, and it does not equal the first one.
   - `It("refuses a zero clock on the re-approval branch with zero writes")` — seed `{"status": "next", "phase": "todo", "approved_by": "someone"}`, build the operation with a zero `libtime.CurrentDateTime` exactly as `It("refuses a zero clock with zero writes")` does, assert the error contains `approved_at` and `WriteTaskCallCount()` is 0. This pins DB5 on the branch that now writes a timestamp.

3. **Pin the end-to-end path** in `integration/cli_test.go`, inside `Describe("task approve", ...)`, after `It("AC6: a task carrying any approval record is advanced, per fixture")`. Two specs, each creating its own vault:

   - `It("AC1/AC2/AC4: a re-approval writes a timestamp-typed reapproved_at, preserves approved_at, and the field reader returns it")` — fixture `approvedBothFrontmatter` via `createTempVault` (no owner is resolved on this branch, so no `current_user` is needed). Run `runEntityCommand("task", "approve", "Alpha")`, expect exit 0. From `readFile(taskFile)`: `strings.Trim(valueOf(after, "reapproved_at"), "\"")` must be non-empty and must parse with `time.Parse(time.RFC3339, ...)`; `valueOf(after, "approved_at")` must equal `2026-01-01T00:00:00Z`; `valueOf(after, "approved_by")` must equal `someone`; `valueOf(after, "phase")` must equal `planning`. Then run `runEntityCommand("task", "get", "Alpha", "reapproved_at")`, expect exit 0, and expect `strings.TrimSpace(string(session.Out.Contents()))` to be non-empty. Assert only that the reader returns a non-empty value — do not assert an RFC3339 shape from `task get`, because the generic reader renders a timestamp-typed value with Go's `%v` form.
   - `It("AC3: a first approval writes no reapproved_at")` — fixture `todoFrontmatter` via `createTempVaultWithCurrentUser` (the first-approval path resolves an owner, so an empty `assignee` needs a configured `current_user`). Run `task approve Alpha`, expect exit 0, and assert `readFile(taskFile)` contains no `reapproved_at` line (`Expect(after).NotTo(ContainSubstring("reapproved_at"))`) while `valueOf(after, "approved_at")` is non-empty.

4. **Name the key in the command help.** In `pkg/cli/cli.go`, `createTaskApproveCommand`'s `Long`, extend the second paragraph (the one that begins "An existing approved_at is preserved on re-approval") with a sentence that names `reapproved_at`, says it is written only on a re-approval, and says each re-approval refreshes it. Replace:

   ```go
   			"fabricates a missing one, and never refreshes approved_at.\n\n" +
   ```

   with:

   ```go
   			"fabricates a missing one, and never refreshes approved_at. The act of " +
   			"re-approval is recorded in its own key: reapproved_at is written only on a " +
   			"re-approval and refreshed by each one, so a reader can tell a row " +
   			"re-approved a minute ago from one that was never re-approved.\n\n" +
   ```

   Do not reword the rest of the paragraph or the other two paragraphs. The printed help line must contain both `reapproved_at` and `re-approval` on one line — cobra prints `Long` verbatim, so the concatenated sentence satisfies that. Keep the existing spec-067 assertion passing: the phrase `records approved_by and approved_at in one write` must still be followed, at or after it, by a clause containing `preserv` that names `approved_at`.

5. **Leave the changelog alone.** `CHANGELOG.md` already has exactly one `## Unreleased` section carrying the bullet for this change. Do not add a second bullet, do not move the section, and do not touch any `## vX.Y.Z` section, version string or tag.

6. **Do not change anything else.** Specifically:
   - `pkg/ops/task_approve.go` — no new operation, no new interface method, no new constructor parameter, no change to the first-approval path, no change to `hasApprovalRecord`, no change to `resolveOwner`, no change to `write`. `task approve` performs exactly one `WriteTask` call per invocation, as today.
   - `docs/task-writing.md` — no change. Its paragraph stays true and editing it is out of scope.
   - `README.md`, `specs/`, `prompts/` — no change.
   - No new CLI verb, subcommand or flag, so no integration registration-table entry is needed.
   - `pkg/ops/workon.go`'s `advancePhaseIfEntering` operator-flag path is deliberately out of scope: it advances a `todo` row to planning with no `approved_*` write because the flag itself is the approval record there (`docs/task-writing.md` § The approval flag), so it writes no `reapproved_at` either. Do not add one.
   - No version string in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` is bumped and no tag is created.

7. **Self-check before finishing.** Re-run the `<verification>` block and confirm every step passes, then walk Acceptance Criteria 1–8 of the spec against the change one by one. For each, name the artefact that proves it: AC1 → the unit type assertion plus the probe's timestamp grep; AC2 → the probe's `task get`; AC3 → the first-approval unit test and integration spec; AC4 → the preserved-value assertions; AC5 → the refresh unit test; AC6 → the existing terminal-row `DescribeTable` (zero writes) plus the probe's digest check; AC7 → the help grep; AC8 → `make precommit`.
</requirements>

<constraints>
- Copied from the spec: `approved_by` and `approved_at` preservation on re-approval is a frozen contract. This change adds a key; it removes and reinterprets nothing.
- Copied from the spec: the key is named exactly `reapproved_at` and holds a timestamp value, matching `approved_at`'s type and formatting.
- Copied from the spec: `hasApprovalRecord` stays presence-based (`task.Keys()`), not value-based. A row whose `approved_at:` is present but blank — or which carries only `approved_by` — still takes the re-approval branch and still receives `reapproved_at`.
- Copied from the spec: the change stays inside the existing re-approval branch and its single write path. No new operation, no new interface method.
- Copied from the spec: `task approve` performs exactly one storage write per invocation; the new key travels in that same write.
- Copied from the spec: `task get "<task>" reapproved_at` reads the value back through the existing generic field reader — no new subcommand, no new output path.
- Copied from the spec: every existing test in `pkg/ops/task_approve_test.go` continues to pass unchanged. Do not edit the preservation `DescribeTable`, the terminal-row `DescribeTable`, or the zero-clock test.
- Copied from the spec: `docs/task-writing.md` is not edited.
- Copied from the spec: do not change `task approve`'s `phase: todo` entry contract, do not give re-approval its own verb, and do not consume the new key anywhere in this repository.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass, including the whole `pkg/ops` approve suite and the whole `integration` suite.
</constraints>

<verification>
Run `make precommit` — must exit 0. It includes `test`, plus `check`, which shells out to `trivy`, `osv-scanner`, `vulncheck` and an `addlicense` network fetch; if `check` goes red for an environment reason unrelated to this change, run `make test lint check-changelog` instead and say which step failed and why — `check-changelog` is change-related and must not be dropped with the environment-dependent steps.

```bash
go test ./pkg/ops/... ./pkg/cli/... ./integration/...
```

Then the behavioural probe. Build the binary first — never the installed `vault-cli`, which is a different version:

```bash
go build -o /tmp/vc-reapproved-bin . && echo built
```

```bash
rm -rf /tmp/vc-reapproved-probe && mkdir -p "/tmp/vc-reapproved-probe/25 Tasks"
cat > /tmp/vc-reapproved-probe/config.yaml <<'EOF'
current_user: bborbe
default_vault: probe
vaults:
    probe:
        path: /tmp/vc-reapproved-probe
        name: probe
        tasks_dir: 25 Tasks
        goals_dir: 24 Goals
        topics_dir: 23 Topics
EOF
cat > "/tmp/vc-reapproved-probe/25 Tasks/Probe Re-Approval.md" <<'EOF'
---
page_type: task
status: next
phase: todo
assignee: bborbe
---
EOF
F="/tmp/vc-reapproved-probe/25 Tasks/Probe Re-Approval.md"
VC="/tmp/vc-reapproved-bin --config /tmp/vc-reapproved-probe/config.yaml"
fail() { echo "FAIL $1"; exit 1; }

# AC3 — the first approval writes no reapproved_at
$VC task approve "Probe Re-Approval" >/dev/null
[ "$(grep -c '^reapproved_at:' "$F")" = 0 ] || fail "AC3: first approval wrote reapproved_at"
[ "$(grep -c '^approved_at:' "$F")" = 1 ]   || fail "AC3: approved_at missing"
BEFORE_AT=$(grep -m1 '^approved_at:' "$F")

# AC1 + AC2 + AC4 — revert, re-approve, the key appears and approved_at did not move
$VC task set "Probe Re-Approval" phase todo >/dev/null
sleep 1
$VC task approve "Probe Re-Approval" >/dev/null
[ "$(grep -cE '^reapproved_at: [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}' "$F")" = 1 ] \
  || fail "AC1: reapproved_at missing or not a timestamp"
[ -n "$($VC task get 'Probe Re-Approval' reapproved_at)" ] || fail "AC2: reader returned empty"
[ "$(grep -m1 '^approved_at:' "$F")" = "$BEFORE_AT" ] || fail "AC4: approved_at moved"

# AC5 — a second re-approval refreshes, and the new instant is later
FIRST=$(grep -m1 '^reapproved_at:' "$F")
$VC task set "Probe Re-Approval" phase todo >/dev/null
sleep 1
$VC task approve "Probe Re-Approval" >/dev/null
SECOND=$(grep -m1 '^reapproved_at:' "$F")
[ "$FIRST" != "$SECOND" ] || fail "AC5: reapproved_at was preserved, not refreshed"
[ "$SECOND" \> "$FIRST" ] || fail "AC5: the refreshed instant is not later"

# AC6 — a terminal row is refused and nothing is written
cat > "/tmp/vc-reapproved-probe/25 Tasks/Probe Terminal.md" <<'EOF'
---
page_type: task
status: completed
phase: todo
approved_by: operator
approved_at: 2026-01-01T00:00:00Z
assignee: bborbe
---
EOF
T="/tmp/vc-reapproved-probe/25 Tasks/Probe Terminal.md"
sha() { { sha256sum "$1" 2>/dev/null || shasum -a 256 "$1"; } | awk '{print $1}'; }
BEFORE_SUM=$(sha "$T")
[ -n "$BEFORE_SUM" ] || fail "AC6: no sha256 tool available (need sha256sum or shasum)"
$VC task approve "Probe Terminal" >/dev/null 2>&1 && fail "AC6: a terminal row was approved"
[ "$(sha "$T")" = "$BEFORE_SUM" ] || fail "AC6: the file changed"

# AC7 — the help names the key and its condition
$VC task approve --help | grep -qE 'reapproved_at.*re-approval|re-approval.*reapproved_at' \
  || fail "AC7: help does not name reapproved_at as a re-approval key"

echo "probe: all assertions passed"
```

Notes on the probe: run it with `bash`. The `sha()` helper already accepts either `sha256sum` or `shasum`, and refuses loudly when neither exists — the assertion is the digest equality before and after, not the tool. The `sleep 1` calls are load-bearing: the key holds an RFC3339 instant with second precision, so two re-approvals in the same second legitimately write the same value.

Finally, the durable file-level checks:

```bash
go run . task approve --help | grep -qE 'reapproved_at.*re-approval|re-approval.*reapproved_at'
grep -c 'reapproved_at' pkg/ops/task_approve.go
grep -c 'reapproved_at' pkg/ops/task_approve_test.go
grep -c 'reapproved_at' pkg/cli/cli.go
grep -c '^## Unreleased' CHANGELOG.md
awk '/^## /{sec=$0} /reapproved_at/ {print "sits under: " sec}' CHANGELOG.md
! grep -q 'reapproved_at' docs/task-writing.md
```

Expected: the help grep exits 0; the three source counts are at least 1; the `## Unreleased` count is exactly 1; the `awk` prints `sits under: ## Unreleased`; the `docs/task-writing.md` absence check exits 0. Do not use `git diff` or `git status` here — the daemon may run this container with `.git` masked, and those commands fail with `fatal: not a git repository`.
</verification>
