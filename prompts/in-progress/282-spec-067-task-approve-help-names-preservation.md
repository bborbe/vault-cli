---
status: failed
spec: [067-bug-task-approve-help-claims-fresh-approved-at]
created: "2026-11-16T17:58:02Z"
queued: "2026-11-16T18:57:49Z"
completed: "2026-11-16T19:29:10Z"
lastFailReason: 'setup workflow: git fetch origin: fetch from origin: : context deadline exceeded'
---

# `task approve --help` states that an existing `approved_at` is preserved (spec 067)

<summary>
- The approve command's built-in help now says that a task which already carries an approval timestamp keeps it.
- A reader can no longer infer that re-approving refreshes the timestamp.
- The preservation sentence follows the existing "records both keys in one write" sentence, so that sentence is qualified instead of standing alone as the whole description of re-approval.
- A new test pins the wording, so deleting the preservation sentence fails the suite.
- The test reads the built command's help text, not the source literal, so a reformatted string cannot make it pass vacuously.
- The approval behaviour is unchanged: the timestamp still does not move on re-approval, and no missing key is invented.
- The existing behaviour-level test that pins the preservation is untouched and still passes.
- The changelog gains an unreleased bullet; no version string and no tag is touched.
</summary>

<objective>
Correct the misleading `vault-cli task approve` help text so it names that an existing `approved_at` is preserved on re-approval, and pin that wording with a test in `pkg/cli/` so it cannot silently regress. The behaviour is already correct and already pinned — only the user-visible message and its missing test change. This is spec 067, one prompt covering Acceptance Criteria 1–5.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/dod.md` for the definition of done this change is graded against.

Read these files fully before changing anything:

- `pkg/cli/cli.go` — `createTaskApproveCommand` (starts around line 2751). The `Long` field at lines 2763–2771 is the whole edit surface in this file. Its three paragraphs are: the approval write, the unbounded-approval note, and the operator-flag note. Only the first paragraph changes.
- `pkg/ops/task_approve_test.go` lines 236–285 — `DescribeTable("advances a todo task that already carries an approval record", ...)`. Read it, change nothing in it. It snapshots `approved_by`/`approved_at` before `Execute` and asserts the written values equal the seeded ones; that is the behaviour pin this prompt must leave byte-identical.
- `docs/task-writing.md` — the paragraph beginning `**The approval is \`vault-cli task approve\` — or an operator-set flag.**` in § Phase transitions (line 519). It states the preservation contract in full; the help text is being brought into line with it. Read it, change nothing in it.
- `CHANGELOG.md` lines 1–14 only — the frozen preamble ends at the SemVer bullet block; the first section is `## v0.194.0` and there is NO `## Unreleased` section today.
- `pkg/cli/cli_suite_test.go` and `pkg/cli/output_test.go` — the external-test-package (`package cli_test`) Ginkgo style and the BSD license header every new `.go` file carries.
- `scripts/check-changelog.sh` — the structural rule precommit enforces: `# Changelog` → preamble → `## Unreleased` → `## vX.Y.Z`.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega suite files and external test packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the unreleased-section placement rule, the frozen-preamble rule, and the rule that an auto-release repository adds bullets under the unreleased section without bumping version strings or tagging.
</context>

<requirements>
The defect: the help text says the command "records approved_by and approved_at in one write", which is literally true for a first approval and false as a description of re-approval — a row that already carries the record keeps it. `docs/task-writing.md` documents that preservation as deliberate, so the text is the bug and the behaviour is not.

1. **Rewrite the `Long` field of `createTaskApproveCommand` in `pkg/cli/cli.go`.** Replace this block exactly (lines 2763–2771):

   ```go
   		Long: "Approve a task in the operator's inbox: it moves a todo task to planning and " +
   			"records approved_by and approved_at in one write.\n\n" +
   			"Approval is unbounded: it never refuses on a count of approved rows. What opens " +
   			"a task is governed by flag provenance, not by a count — /supervisor:open --flagged " +
   			"opens only rows whose flag_set_by is operator or legacy, and an agent cannot cause " +
   			"an open by approving a row.\n\n" +
   			"An operator-set flag (flag: true, flag_set_by operator or legacy) is also an " +
   			"approval: `task work-on` moves such a todo task to planning without writing " +
   			"approved_by / approved_at.",
   ```

   with:

   ```go
   		Long: "Approve a task in the operator's inbox: it moves a todo task to planning and " +
   			"records approved_by and approved_at in one write.\n\n" +
   			"An existing approved_at is preserved on re-approval: a row that already carries " +
   			"approved_by or approved_at is already approved, so the command does not " +
   			"re-approve it. It completes the transition by moving the phase to planning and " +
   			"leaves the record exactly as it is — it never overwrites either key, never " +
   			"fabricates a missing one, and never refreshes approved_at.\n\n" +
   			"Approval is unbounded: it never refuses on a count of approved rows. What opens " +
   			"a task is governed by flag provenance, not by a count — /supervisor:open --flagged " +
   			"opens only rows whose flag_set_by is operator or legacy, and an agent cannot cause " +
   			"an open by approving a row.\n\n" +
   			"An operator-set flag (flag: true, flag_set_by operator or legacy) is also an " +
   			"approval: `task work-on` moves such a todo task to planning without writing " +
   			"approved_by / approved_at.",
   ```

   The preservation sentence must contain the substring `preserv` (so `grep -i preserv` finds it) and must name `approved_at`. It must appear at or after the "records approved_by and approved_at in one write" sentence — that ordering is what keeps the original sentence from standing alone as the description of re-approval. Do not reword the unbounded-approval paragraph or the operator-flag paragraph; they are unchanged, including the em dashes and the backticks.

2. **Add the pinning test.** Create `pkg/cli/task_approve_help_test.go` with exactly this content:

   ```go
   // Copyright (c) 2026 Benjamin Borbe All rights reserved.
   // Use of this source code is governed by a BSD-style
   // license that can be found in the LICENSE file.

   package cli_test

   import (
   	"context"
   	"strings"

   	. "github.com/onsi/ginkgo/v2"
   	. "github.com/onsi/gomega"
   	"github.com/spf13/cobra"

   	"github.com/bborbe/vault-cli/pkg/cli"
   )

   // approvalLong returns the constructed `task approve` command's Long help
   // text. It reads the built command tree rather than the source literal: Long
   // is assembled from adjacent string literals, so a source grep is not the
   // same assertion as the text a reader sees from `--help`.
   func approvalLong() string {
   	var approveCmd *cobra.Command
   	for _, top := range cli.NewRootCommand(context.Background()).Commands() {
   		if top.Name() != "task" {
   			continue
   		}
   		for _, sub := range top.Commands() {
   			if sub.Name() == "approve" {
   				approveCmd = sub
   			}
   		}
   	}
   	Expect(approveCmd).NotTo(BeNil(), "the root command must expose `task approve`")
   	return approveCmd.Long
   }

   var _ = Describe("task approve help text", func() {
   	It("names that an existing approved_at is preserved", func() {
   		long := approvalLong()
   		Expect(long).To(ContainSubstring("approved_at"))
   		Expect(strings.ToLower(long)).To(ContainSubstring("preserv"))
   	})

   	It("does not describe re-approval with the fresh-timestamp phrase alone", func() {
   		long := approvalLong()
   		const phrase = "records approved_by and approved_at in one write"
   		idx := strings.Index(long, phrase)
   		if idx < 0 {
   			// Removing the phrase is an acceptable fix: nothing is left to
   			// mislead a reader about a fresh timestamp.
   			return
   		}
   		rest := long[idx:]
   		preservIdx := strings.Index(strings.ToLower(rest), "preserv")
   		Expect(preservIdx).To(BeNumerically(">=", 0),
   			"a preservation clause must appear at or after the write phrase")
   		Expect(rest[preservIdx:]).To(ContainSubstring("approved_at"),
   			"the preservation clause must name approved_at")
   	})
   })
   ```

   Why this shape and not `! grep -q`: asserting bare absence of the old phrase would fail a correct qualify-in-place fix, which is the fix this prompt makes. The second spec must therefore accept either outcome — phrase removed, or phrase followed by a preservation clause naming `approved_at`. Do not weaken it to a bare `ContainSubstring("preserv")` check on the whole string: that would pass even if the preservation clause sat above the phrase and the misleading sentence still ended the description of re-approval.

3. **Add the changelog bullet.** `CHANGELOG.md` has no `## Unreleased` section today. Insert one immediately after the SemVer bullet block (`* PATCH version when…`) and immediately before `## v0.194.0`:

   ```
   ## Unreleased

   - fix: `task approve --help` now states that an existing `approved_at` is preserved on re-approval, so a reader cannot infer a fresh timestamp from the help text; the approval behaviour is unchanged

   ```

   Exactly one bullet, `fix:` prefix, and exactly one `## Unreleased` heading in the file. Do not touch `## v0.194.0` or any released section.

4. **Do not change anything else.** Specifically:
   - `pkg/ops/task_approve.go` — no change at all. The preservation branch is the documented contract.
   - `pkg/ops/task_approve_test.go` — no change at all; the `DescribeTable` at line 236 stays byte-identical.
   - `docs/task-writing.md` — no change; it already describes both branches.
   - `specs/` and `prompts/` — no change; historical records.
   - No new CLI verb, subcommand, or flag, so no `integration/cli_test.go` registration-table entry is needed.
   - No version string in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` is bumped, and no tag is created: this repository's release automation owns both after merge.
   - `pkg/ops/` stays output-free; the help string lives in `pkg/cli/`.

<!-- Open question (non-blocking, decided against): should the help text also point readers at docs/task-writing.md for the full contract? Not added — the spec's Acceptance Criteria ask only that the preservation be named, and a path in --help output goes stale silently. Revisit only if a reader reports the same misreading again. -->
</requirements>

<constraints>
- Copied from the spec: do not change the preservation behaviour. `pkg/ops/task_approve.go` is out of scope; only the help string changes.
- Copied from the spec: do not edit or delete the existing preservation test. `pkg/ops/task_approve_test.go:236` already pins the contract; leave it byte-identical.
- Copied from the spec: do not edit `docs/task-writing.md`. It is correct as written.
- Copied from the spec: do not rewrite released `CHANGELOG.md` sections, `specs/` or `prompts/`.
- Copied from the spec: no hand-bump of version strings or tags. Add a `## Unreleased` bullet and let the releaser run.
- Copied from the spec: `pkg/ops/` stays output-free; the help string lives in `pkg/cli/`.
- The new test must be a structural assertion, not a bare absence assertion — a correct qualify-in-place fix must pass it (see requirement 2).
- Existing tests must still pass, including the whole `pkg/ops` approve suite, unchanged.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make precommit` — must exit 0. It runs format, generate, lint, the full test suite, `check-changelog` and `addlicense`, so a misordered changelog section or a missing license header fails here.

Then:

```
go test ./pkg/cli/...          # the new help-text spec passes
go test ./pkg/ops/...          # the preservation table still passes, unchanged
go run . task approve --help | grep -i preserv
go run . task approve --help | grep -c approved_at
awk '/^## /{sec=$0} /task approve --help/ {print sec}' CHANGELOG.md
grep -c 'advances a todo task that already carries an approval record' pkg/ops/task_approve_test.go
grep -c '^## Unreleased' CHANGELOG.md
```

Expected: the `grep -i preserv` line prints the preservation sentence naming `approved_at` (pre-fix it exits 1 with no output); the `approved_at` count is at least 1; the `awk` prints `## Unreleased`; the `DescribeTable` count is 1, proving the behaviour pin is still in place; the Unreleased heading count is 1.

Confirm the write path and its test are untouched. `.git` is masked in this container (the daemon runs with `--set hideGit=true`), so `git diff` and `git status` fail with `fatal: not a git repository` — use content checks instead:

```
grep -cE 'already carries an approval record|hasApprovalRecord' pkg/ops/task_approve.go
grep -c 'advances a todo task that already carries an approval record' pkg/ops/task_approve_test.go
grep -c 'A row that already carries' docs/task-writing.md
```

Expected: at least 1, exactly 1, at least 1 (verified 2026-11-16: 4, 1, 1). Then read the `hasApprovalRecord` branch in `pkg/ops/task_approve.go` and confirm it returns before any write — that inspection is the AC4 "no behavioural change" evidence this container can produce.
</verification>
