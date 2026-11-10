---
status: completed
summary: 'Exempted operator-flagged rows (flag: true with flag_set_by: operator) from the task approve admission cap, added guard tests, docs, and an Unreleased changelog entry'
execution_id: vault-cli-admission-exemption-exec-281-admission-cap-exempt-flagged-rows
dark-factory-version: v0.198.17
created: "2026-11-10T00:00:00Z"
queued: "2026-11-10T11:59:01Z"
started: "2026-11-10T12:01:24Z"
completed: "2026-11-10T12:05:48Z"
---

# Exempt operator-flagged rows from the approved-row admission cap

<summary>
- `task approve` no longer refuses a row the operator flagged when the vault's approved-row count is at or above the admission cap.
- The exemption keys on the flag's provenance, not on the flag alone: a row must carry `flag: true` **and** `flag_set_by: operator`, so an agent-set or unattributed flag still refuses at the cap.
- The cap itself is unchanged: an unflagged row, and a row flagged by anything other than the operator, is still refused at 110.
- An exempt row still counts toward the 110 once approved — the exemption is from the refusal, not from the count.
- The guide states the exemption in the section that defines the flag, so the two statements cannot contradict each other.
- The changelog records the change under `## Unreleased`; no version string is bumped and no tag is created.
</summary>

<objective>
Exempt rows the operator has flagged from the approved-row admission cap in `vault-cli task approve`, so the operator can always approve a row they deliberately picked, while the cap keeps refusing ordinary intake at or above 110.
</objective>

<context>
Read `CLAUDE.md` first, then `docs/dod.md`.

Read these files before editing:

- `pkg/ops/task_approve.go` — `DefaultMaxApprovedNotStarted` (line 30) and its doc comment (lines 18–29), the `TaskApproveOperation` interface doc (lines 33–71), `Execute` (line 101) with the `refuseAdmissionOverCap` call at lines 188–194, and `refuseAdmissionOverCap` itself (lines 243–293).
- `pkg/domain/task_frontmatter.go` line 79 — `Flag()` reads the `flag` key through `GetBool`, which accepts a bool and the strings `true` / `yes` case-insensitively.
- `pkg/domain/frontmatter_map.go` line 39 — `GetString` is the accessor for `flag_set_by`. `pkg/ops/list.go:121` already reads it that way; follow that idiom rather than adding an accessor.
- `pkg/ops/task_approve_test.go` lines 26–53 — the suite setup and the `seedTask(fields map[string]any)` helper that replaces the task `FindTaskByName` returns.
- `pkg/ops/task_approve_test.go` lines 377–526 — the `Describe("admission cap")` block and its helpers `newCapOp(limit)`, `approvedRow(name)`, `approvedRows(n)`.
- `docs/task-writing.md` lines 162–181 — `### The approval flag (flag)`. **This is the section that defines the field and it is load-bearing for this change**: line 166 says the fleet ranks on `flag: true` **plus `flag_set_by: operator`**; line 168 says an agent-set flag is not an approval; line 181 says an absent `flag_set_by` on a flagged row is never to be read as operator-set.
- `docs/task-writing.md` lines 505–521 — `### Phase transitions`: the `todo` table row (509), the `planning` row (510), and the paragraph beginning `**The approval is \`vault-cli task approve\`.**` (519).
- `CHANGELOG.md` lines 1–16 — the preamble ends at the SemVer bullet block; the first section is `## v0.190.0` and there is NO `## Unreleased` today.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the unreleased-section placement rule, the frozen-preamble rule, the required conventional prefixes, and the rule that an auto-release repository's feature branch adds bullets under the unreleased section and does NOT bump version strings or tag.
- `docs/releasing-vault-cli.md` lines 86–95 and 186–192 — this project's own authoritative statement of the same rule: a feature branch adds `## Unreleased` bullets and never renames the section, bumps the four version strings, or creates a tag.
</context>

<requirements>
1. **Exempt operator-flagged rows in `Execute`.** Find this exact block in `pkg/ops/task_approve.go` (around lines 188–194):

   ```go
   	// The admission cap: refuse a new approval when the vault already holds at
   	// or above the cap of approved rows still on the board. This sits after
   	// every guard that writes nothing and after the already-approved advance
   	// path has returned, so an already-approved row is never gated.
   	if err := o.refuseAdmissionOverCap(ctx, vaultPath, vaultName, taskName); err != nil {
   		return MutationResult{Success: false, Error: err.Error()}, err
   	}
   ```

   Replace it with:

   ```go
   	// The admission cap: refuse a new approval when the vault already holds at
   	// or above the cap of approved rows still on the board. This sits after
   	// every guard that writes nothing and after the already-approved advance
   	// path has returned, so an already-approved row is never gated.
   	//
   	// A row the operator flagged is exempt. The cap bounds the approved queue
   	// nobody has looked at; a row the operator deliberately picked is the
   	// opposite of that. The test is `flag_set_by: operator`, not `flag` alone:
   	// docs/task-writing.md § The approval flag makes an agent-set flag, an
   	// unattributed one and a pre-field one explicitly not operator-set, and
   	// exempting those would hand any agent session a cap bypass. The row still
   	// counts toward the cap once approved — the exemption is from the refusal,
   	// not from the count.
   	if !operatorFlagged(task) {
   		if err := o.refuseAdmissionOverCap(ctx, vaultPath, vaultName, taskName); err != nil {
   			return MutationResult{Success: false, Error: err.Error()}, err
   		}
   	}
   ```

   Then add this helper beside `hasApprovalRecord` (around line 307), so the predicate has one definition and a name:

   ```go
   // operatorFlagged reports whether the operator deliberately flagged this row.
   //
   // The flag alone is not the test. docs/task-writing.md § The approval flag
   // makes an agent-set `flag: true`, an unattributed one (`flag_set_by:
   // unknown`, the `task set` default) and a pre-field one (no `flag_set_by`
   // at all) explicitly not operator-set, so gating on `Flag()` alone would let
   // any agent session flag a row and approve it past the admission cap — the
   // bypass `flag_set_by` exists to close. The fleet's own open leg reads the
   // same pair (docs/task-writing.md line 166).
   func operatorFlagged(task *domain.Task) bool {
   	return task.Flag() && task.GetString("flag_set_by") == "operator"
   }
   ```

   Do not touch `refuseAdmissionOverCap`'s body: the count keeps counting every non-terminal row carrying `approved_at`, flagged or not.

2. **State the exemption in the interface doc.** In the `TaskApproveOperation` interface's `Execute` comment, the paragraph beginning `// A new approval is also refused, with nothing written, when the vault's` (around lines 56–61) ends with `// already-approved advance path above, which writes no approval.` Append this multi-line block to that paragraph:

   ```
   // A row the operator flagged (`flag: true` with `flag_set_by: operator`) is
   // exempt from that refusal: the cap bounds the approved queue nobody has
   // looked at, and a row the operator deliberately picked is the opposite of
   // that. An agent-set or unattributed flag is not an exemption.
   ```

3. **State the exemption on the constant.** In the doc comment above `DefaultMaxApprovedNotStarted` (lines 18–29), append these two lines as the last lines of that comment, immediately above the `const` on line 30:

   ```
   // Rows the operator flagged (`flag: true` with `flag_set_by: operator`) are
   // exempt from the refusal: the flag is the operator's deliberate pick, not
   // unattended backlog. An agent-set or unattributed flag is not an exemption.
   ```

4. **Add the tests.** Inside the existing `Describe("admission cap", func() {` block in `pkg/ops/task_approve_test.go`, add four `It`s and one `DescribeTable` beside the existing `"approves when the count is one below the cap"`:

   ```go
   		It("approves an operator-flagged row at the cap", func() {
   			capOp := newCapOp(5)
   			seedTask(map[string]any{
   				"status": "next", "phase": "todo", "assignee": "someone",
   				"flag": true, "flag_set_by": "operator",
   			})
   			mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

   			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
   			Expect(execErr).To(BeNil())
   			Expect(result.Success).To(BeTrue())
   			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
   			_, written := mockStorage.WriteTaskArgsForCall(0)
   			Expect(written.Get("approved_at")).NotTo(BeNil())
   		})

   		It("approves an operator-flagged row above the cap", func() {
   			capOp := newCapOp(5)
   			seedTask(map[string]any{
   				"status": "next", "phase": "todo", "assignee": "someone",
   				"flag": "true", "flag_set_by": "operator",
   			})
   			mockStorage.ListTasksStrictReturns(approvedRows(12), nil)

   			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
   			Expect(execErr).To(BeNil())
   			Expect(result.Success).To(BeTrue())
   			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
   		})

   		It("approves an operator-flagged row whose flag is the hand-quoted string \"yes\"", func() {
   			capOp := newCapOp(5)
   			seedTask(map[string]any{
   				"status": "next", "phase": "todo", "assignee": "someone",
   				"flag": "yes", "flag_set_by": "operator",
   			})
   			mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

   			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
   			Expect(execErr).To(BeNil())
   			Expect(result.Success).To(BeTrue())
   			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
   		})

   		It("still refuses an unflagged row at the cap", func() {
   			capOp := newCapOp(5)
   			seedTask(map[string]any{
   				"status": "next", "phase": "todo", "assignee": "someone", "flag": false,
   			})
   			mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

   			_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
   			Expect(execErr).To(HaveOccurred())
   			Expect(execErr.Error()).To(ContainSubstring("admission cap of 5"))
   			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
   		})

   		DescribeTable("a flag without operator provenance is not an exemption",
   			func(fields map[string]any) {
   				capOp := newCapOp(5)
   				seedTask(fields)
   				mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

   				_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
   				Expect(execErr).To(HaveOccurred())
   				Expect(execErr.Error()).To(ContainSubstring("admission cap of 5"))
   				Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
   			},
   			Entry("agent actor", map[string]any{
   				"status": "next", "phase": "todo", "assignee": "someone",
   				"flag": true, "flag_set_by": "agent-x",
   			}),
   			Entry("unattributed (task set without --by)", map[string]any{
   				"status": "next", "phase": "todo", "assignee": "someone",
   				"flag": true, "flag_set_by": "unknown",
   			}),
   			Entry("pre-field, no provenance", map[string]any{
   				"status": "next", "phase": "todo", "assignee": "someone",
   				"flag": true,
   			}),
   		)

   		It("counts an approved operator-flagged row toward the cap", func() {
   			capOp := newCapOp(5)
   			rows := append(approvedRows(4), domain.NewTask(
   				map[string]any{
   					"status": "in_progress", "phase": "planning",
   					"approved_at": "2026-01-01T00:00:00Z",
   					"flag": true, "flag_set_by": "operator",
   				},
   				domain.FileMetadata{Name: "Flagged Approved"}, domain.Content(""),
   			))
   			mockStorage.ListTasksStrictReturns(rows, nil)

   			_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
   			Expect(execErr).To(HaveOccurred())
   			Expect(execErr.Error()).To(ContainSubstring("admission cap of 5"))
   		})
   ```

   The `DescribeTable` is the security guard: it proves a flag attributed to an agent, to no actor, or to the pre-field state is not an exemption. `flag_set_by` is self-declared, so an agent that passes `--by operator` is outside this test's scope — the field records who claimed the write, not who ran it. Do not delete or weaken the existing `"refuses at the cap, naming the count, the cap and the vault, with no write"` test.

5. **Amend the guide, including the section that defines the flag.** In `docs/task-writing.md`:

   - Line 164 (first paragraph of `### The approval flag (flag)`), after the sentence ending `because the operator's flag stands in for the approval they would otherwise type.`, insert:

     ```
      It is **also an exemption from the approved-row admission cap** in `vault-cli task approve` (§ Phase transitions): an operator-set flag (`flag_set_by: operator`, written by `vault-cli task set <name> flag true --by operator`; the `task set` default `--by` is `unknown`, which is not an exemption) lets that row be approved past the cap, while the cap still bounds every unflagged row and every row whose flag an agent set.
     ```

   - Line 509 (the `todo` table row) ends `... It also refuses, writing nothing, when the vault's approved-row count is at or above the admission cap |`. Insert the following **inside the cell, before the trailing ` |`** — not at the end of the line — and keep the leading period so the two sentences do not run together:

     ```
     . A row the operator flagged (`flag: true` with `flag_set_by: operator`) is exempt from that refusal.
     ```

   - Line 519 (the paragraph beginning `**The approval is \`vault-cli task approve\`.**`) contains `That cap does **not** gate the already-approved advance path: an approved row is still advanced to \`planning\` when the count is at the cap, because that path writes no approval.` Insert immediately after that sentence:

     ```
      A row the operator flagged is exempt from the cap as well: the cap bounds the approved queue nobody has looked at, and a row the operator deliberately picked is the opposite of that, so it is always approvable — though once approved it still counts toward the cap. The exemption requires `flag_set_by: operator`; an agent-set or unattributed flag is not one.
     ```

   Replace only those sentences. Leave every other sentence, table row and section byte-identical.

6. **Record it in the changelog.** `CHANGELOG.md` has no `## Unreleased` section today. Insert it immediately after the preamble's SemVer bullet block and immediately before `## v0.190.0`, in this shape:

   ```
   ## Unreleased

   - feat: `task approve` exempts a row the operator flagged (`flag: true` with `flag_set_by: operator`) from the approved-row admission cap, so the operator can always approve a row they deliberately picked; the cap itself is unchanged, an unflagged row and a row whose flag an agent set are still refused at 110, and an approved flagged row still counts toward the cap.

   ```

   Exactly one `## Unreleased` heading may exist after this edit.
</requirements>

<constraints>
- No new CLI verb, subcommand, flag or config key. `pkg/cli/cli.go` is untouched — the constructor call stays `ops.NewTaskApproveOperation(taskStore, currentDateTime, 0)`.
- Do not change the cap's value (`110`) or its count definition (non-terminal rows carrying `approved_at`). Do not exclude flagged rows from the count.
- **Do not exempt a bare `flag: true`.** The gate must read `flag_set_by: operator` as well; an agent-set, unattributed (`unknown`) or pre-field flag stays refused at the cap.
- Do not touch the already-approved advance path (`hasApprovalRecord`) or `resolveOwner`.
- Do not add a `flag` accessor to `domain`; use the existing `Flag()` and `GetString("flag_set_by")`.
- Do not edit `docs/goal-writing.md`: line 160 states the cap is task-only, which remains true, and `goal approve` has no cap to exempt from.
- Do not bump `.claude-plugin/plugin.json`, either `.claude-plugin/marketplace.json` version, or add a versioned changelog section. All four stay at `0.190.0`.
- Do not create or move a git tag.
- Do not modify any scenario file.
- Do not commit — dark-factory handles git.
</constraints>

<verification>
Run `make precommit` once — it must exit 0 (format, generate, test, lint, vet, vulncheck, osv-scanner, trivy, addlicense, changelog structure).

Then confirm the edit surface. Read the counts, do not rely on the exit code — `grep -c` exits 1 on zero matches, so a broken expectation and a passing one can look alike:

```
grep -c 'func operatorFlagged' pkg/ops/task_approve.go      # expect 1
grep -c 'flag_set_by' pkg/ops/task_approve.go               # expect 5 (const doc, interface doc, Execute comment, helper doc, helper body)
grep -c 'approves an operator-flagged row at the cap' pkg/ops/task_approve_test.go       # expect 1
grep -c 'a flag without operator provenance is not an exemption' pkg/ops/task_approve_test.go  # expect 1
grep -c 'still refuses an unflagged row at the cap' pkg/ops/task_approve_test.go         # expect 1
grep -c 'also an exemption from the approved-row admission cap' docs/task-writing.md     # expect 1
grep -c 'is exempt from that refusal' docs/task-writing.md                               # expect 1
grep -c '^## Unreleased' CHANGELOG.md                       # expect 1
grep -m1 '^## v' CHANGELOG.md                               # still ## v0.190.0
jq -r .version .claude-plugin/plugin.json                   # still 0.190.0
jq -r '.metadata.version' .claude-plugin/marketplace.json   # still 0.190.0
jq -r '.plugins[0].version' .claude-plugin/marketplace.json # still 0.190.0
```

`go test ./pkg/ops/...` must be green, including every new spec (four `It`s plus the three-entry `DescribeTable`).
</verification>
