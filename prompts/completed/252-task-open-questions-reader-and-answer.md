---
status: completed
summary: Added a storage reader for a task's Open Questions section (exposed as the additive `open_questions` JSON field on `task show`) and a `vault-cli task answer` verb that records answers in place, with domain types, Counterfeiter mocks, unit/integration tests and docs.
execution_id: vault-cli-open-questions-exec-252-task-open-questions-reader-and-answer
dark-factory-version: v0.196.0
created: "2026-10-06T15:11:35Z"
queued: "2026-10-06T15:11:35Z"
started: "2026-10-06T15:13:06Z"
completed: "2026-10-06T15:26:06Z"
---

# Add a reader and writer for a task's Open Questions section

<summary>
- A task's Open Questions section becomes machine-readable: the CLI can list the questions a task is waiting on, and record the operator's answers back into that same section.
- Today those questions are prose only. Nothing can enumerate them, so a UI cannot ask for them and a worker cannot tell whether a task is still waiting on an answer.
- The vault-ui board's Approve modal needs exactly this: one labelled field per open question, and the answers written back when the operator confirms.
- Answers are recorded in place, keeping each question and its answer on one line, so the task file still reads as a document in Obsidian.
- Answering again replaces the previous answer instead of appending a second one, so the command is safe to re-run.
- A task with no Open Questions section is left untouched, so callers never have to special-case it.
- Existing behaviour is unchanged: `task approve` keeps its contract, and `task show` gains one additive JSON field.
</summary>

<objective>
Make a task's open questions addressable by the CLI, so a UI can render one input per question and write the answers back. Add a reader that exposes the section on `task show`, and a `task answer` verb that records answers into that section in place.
</objective>

<context>
Read `CLAUDE.md` for project conventions. Read `docs/development-patterns.md` § "Adding a New Command" — it is the Domain → Storage → Operation → CLI layering this change follows (note its step 2 still names a `markdownStorage` type that no longer exists; the real types are `TaskStorage` / `baseStorage` / `taskStorage`, as the requirements below name them) — and `docs/dod.md`, which the daemon uses as its validation prompt and which carries the integration-table, README and test-framework rules that requirements 6–8 satisfy. For requirement 5's output shape, follow `docs/output-formatting.md` rather than re-deriving it from the exemplar.

Read these files before writing — each demonstrates a pattern the new code follows:

- `pkg/cli/cli.go` — find `createTaskApproveCommand`. It is the exemplar for a task mutation verb: flag binding, the `getVaults` + `ops.NewVaultDispatcher().FirstSuccess` shape, JSON vs plain output, and the rule that a refusal exits non-zero. Its doc comment explains why it deliberately differs from `task set` on that exit path.
- `pkg/ops/task_approve.go` — the exemplar for an operation: the `TaskApproveOperation` interface with a doc comment stating the refusal contract, the `//counterfeiter:generate` annotation, `NewTaskApproveOperation`, and an `Execute` returning `MutationResult`.
- `pkg/ops/show.go` — `TaskDetail` is the struct `createTaskShowCommand` prints for `--output json` (via `PrintJSON(detail)` in `pkg/cli/cli.go`). That is where requirement 3's new field is declared and populated, **not** `pkg/cli/cli.go`.
- `pkg/storage/base.go` — find `parseCheckboxes`. It is the exemplar for reading a markdown body section: the section is located by heading, its lines are classified, and the result is a slice of `domain` values. Match its tolerance for the section being absent. Note there is a second, unrelated `parseCheckboxes` on `updateOperation` in `pkg/ops/update.go`; the reader belongs beside the **storage** one, and the ops layer must call the reader through the storage interface rather than parsing the section a second time.
- `pkg/storage/task.go` — find `ReadTask` and `WriteTask`. Together they are the round trip: `WriteTask` serializes `task.RawMap()` as frontmatter over `task.Content`, so a body edit is made by changing `Content` and writing the task back. Do not write the file any other way.
- `pkg/storage/storage.go` — the `TaskStorage` interface. Both consumers in requirement 3 and requirement 4 hold this interface, not a concrete type.
- `pkg/domain/task.go` — `Task.Content` is the full markdown, frontmatter block included. Use it to read the body.
</context>

<requirements>
1. **Add the domain types.** In `pkg/domain`, add:
   - `OpenQuestion` carrying a 1-based `Index` and the question `Text`.
   - `OpenAnswer` carrying an `Index` and the `Answer` text.

   Index is the item's position in the section, counted from 1, and is what a caller passes back to answer it.

2. **Add the section reader to the storage interface.** In `pkg/storage/storage.go`, add to the `TaskStorage` interface:
   `ReadOpenQuestions(ctx context.Context, vaultPath string, taskName string) ([]domain.OpenQuestion, error)`
   Implement it in `pkg/storage/task.go` by delegating to a new unexported helper on `baseStorage` in `pkg/storage/base.go`, alongside `parseCheckboxes`. The interface method is what `pkg/ops` calls — an unexported `baseStorage` method alone is unreachable from the ops layer, which holds the interface, and the enabled `unused` linter rejects it.

   Parsing rules:
   - The section heading is any heading level (`#` through `######`) whose text is exactly `Open Questions`.
   - Its items are the list items at the section's top level — `- `, `* `, or `N. ` — up to the next heading of the same or higher level.
   - An item's `Text` is the line with its leading list marker stripped and the remainder trimmed. Remember the marker and its indentation for requirement 4.
   - A missing section yields an empty slice and no error. A blank list item is skipped rather than yielding an empty question.
   - If a task carries more than one such heading, use the first and ignore the rest.

3. **Expose the questions on `task show`.** In `pkg/ops/show.go`, add `OpenQuestions []domain.OpenQuestion` with json tag `open_questions` to `TaskDetail`, and populate it in `showOperation.Execute` from the section reader, in section order. Initialize it to a non-nil empty slice so `--output json` emits `[]`, not `null`, when the task has no open questions. `createTaskShowCommand` needs no change — it already prints the whole `TaskDetail`. The plain (non-JSON) output is unchanged.

4. **Add the answer operation.** Create `pkg/ops/task_answer.go` following `pkg/ops/task_approve.go`. Define a `TaskAnswerOperation` interface with a doc comment stating the refusal contract, add the `//counterfeiter:generate` annotation, and provide `NewTaskAnswerOperation`. `Execute` takes the vault path, task name, vault name, and a `[]domain.OpenAnswer`. It returns `MutationResult`.

   Behaviour:
   - Read the task, locate the section, and rewrite each named item's line as `<original marker and indentation><question text> → **<answer>**`. Preserve the marker exactly as it was — a `1. ` item stays a numbered item, a `- ` item stays a bullet — so the file's own list style is untouched.
   - **Answering again replaces the previous answer.** If the item's text already ends with ` → **…**`, replace that whole suffix rather than adding a second one, so re-running is idempotent.
   - **Refuse, writing nothing, when** an index names no item in the section, or when answers are supplied for a task that has no such section. The error text names the offending index and the task.
   - **Leave every other line untouched** — the frontmatter, the other sections, the heading itself, and the items the caller did not answer. The only lines that change are the answered ones.
   - Use `github.com/bborbe/errors` (`errors.Errorf` / `errors.Wrap`) for every error — `github.com/pkg/errors` is depguard-denied in this repo.

5. **Add the CLI verb.** In `pkg/cli/cli.go`, add `vault-cli task answer <task-name>` with a repeatable `--answer <index>=<text>` flag, following `createTaskApproveCommand`'s structure. Require at least one `--answer`; a missing or malformed `--answer` (no `=`, a non-numeric index, or an empty text) is a usage error naming the offending value. Print a JSON result under `--output json` and a plain confirmation otherwise. Match `task approve`: a refusal prints the JSON error object and then returns the error, so it exits non-zero in both output modes.

6. **Register the command.** Add it to the `task` command alongside the existing verbs so `vault-cli task answer --help` resolves, and add `Entry("task answer", "task", "answer"),` to the task-subcommand entries of the command-registration DescribeTable in `integration/cli_test.go`. `docs/dod.md` requires that table entry for every new subcommand.

7. **Tests — cover each boundary the new code crosses.** Ginkgo v2 / Gomega with Counterfeiter mocks, per `docs/dod.md`. Reader table test in `pkg/storage/base_test.go`; operation tests in `pkg/ops/task_answer_test.go`; the CLI-driving test as a `Describe("task answer", …)` block in `integration/cli_test.go`, beside the existing `Describe("task approve", …)`.
   - A table test for the section reader over: a section with several `- ` bullets; a section using `N. ` numbered items; a section under a `##` heading; a section whose last item is followed by another heading; a task with no such section; a section with a blank item; and an item that already carries an ` → **…**` answer.
   - Operation tests asserting the refusal cases write nothing (compare the file bytes before and after), and that a successful answer changes only the answered line — the marker, indentation and surrounding lines are byte-identical.
   - A test that answers the same question twice and asserts the file holds exactly one answer. This is the idempotence contract in requirement 4, and it is the case a shape-only test would miss.
   - A test driving the **CLI verb** itself, not just the operation, so the flag parsing and the command's registration are exercised on the real path.
   - A JSON contract test for the new field: run `task show --output json` for a task with no Open Questions section and assert `open_questions` is present as `[]`, not `null`; and for a task with questions, assert the `{"index","text"}` objects appear in section order. This is the serialization contract the UI consumes, and the `null`-vs-`[]` zero value is exactly what a shape-only test misses.

8. **Update the docs.**
   - `CHANGELOG.md`: add one bullet under `## Unreleased` describing the new verb and the new `task show` field. The file has no `## Unreleased` section today, so create it per `docs/dod.md` § Documentation (below the preamble, above the newest `## vX.Y.Z`); if one already exists, append inside it — never create a second one.
   - `README.md`: add a `vault-cli task answer "<task>" --answer 1="<text>"` line to the task-verbs block.
   - `docs/task-writing.md`: add `# Open Questions` to the section list in § "Required sections", **marked optional/recommended** the way `# Out of Scope` and `# Related` are — a task may legitimately lack the section — and record the `- <question> → **<answer>**` encoding as the convention. This is a file format other consumers read, so it belongs in the doc rather than only in this prompt.

9. **Self-check before finishing.** Re-run `<verification>` and confirm it passes. Then walk requirements 1–8 against the change one at a time and confirm each is met; report any you could not satisfy rather than silently dropping it.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT change the behaviour or the signature of `task approve`. It is a separate verb with its own contract, and existing callers depend on it.
- Do NOT change the plain (non-JSON) output of `task show`; the new field is additive and JSON-only.
- Do NOT bump the version strings in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag — the release agent owns both after merge.
- Do NOT edit any `CHANGELOG.md` section other than adding the one bullet in requirement 8.
- Do NOT add an Open Questions section to a task that lacks one, and do NOT reorder or rewrite existing questions.
- Existing tests must still pass.
- Use repo-relative paths only.
</constraints>

<verification>
Run each; record the output verbatim in the report.

- `go build ./...` → exit 0
- `go run . task answer --help` → exit 0, and the output names the `--answer` flag
- `grep -c 'func NewTaskAnswerOperation' pkg/ops/task_answer.go` → exactly 1
- `grep -c 'open_questions' pkg/ops/show.go` → exactly 1
- `grep -c 'ReadOpenQuestions' pkg/storage/storage.go` → exactly 1
- `grep -n 'Entry("task answer"' integration/cli_test.go` → exactly 1 (scoped to the Entry line — the requirement 7 CLI test adds a second `task answer` literal to this file, so a bare count would read 2)
- `make precommit` → exit 0

If `make precommit` fails, STOP and report `"status":"failed"` with the exact failing command and its output. Do not attempt a partial fix of unrelated pre-existing failures — report them and stop.
</verification>
