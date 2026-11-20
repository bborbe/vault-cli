---
status: completed
spec: [070-parent-closure-cascade-on-complete]
summary: 'goal complete now cascades to topics that name the goal through either the goal''s topics: list or the topic''s ## Goals section, with topic-scan failures reported as a refused <topics scan> parent'
execution_id: vault-cli-exec-289-goal-topic-cascade-goals-section
dark-factory-version: v0.198.17
created: "2026-11-20T03:50:00Z"
queued: "2026-11-20T02:46:47Z"
started: "2026-11-20T02:49:11Z"
completed: "2026-11-20T02:53:49Z"
---

# Resolve goal→topic membership from the topic's `## Goals` section

<summary>
- `goal complete` cascades to every topic that names the goal, whether through the goal's `topics:` frontmatter or through the topic's own `## Goals` section.
- A topic that merely mentions the goal outside `## Goals` is never treated as a member.
- A topic-scan failure never fails the goal close and is never silent.
- `topic complete` is unchanged — it already reads the correct section.
</summary>

<objective>
Fix the disagreement between the two membership readers. `vault-cli topic complete` decides membership from the topic page's `## Goals` section (`parseGoalSectionMembers`), while `vault-cli goal complete` cascades only to the topics named in the completed goal's own `topics:` frontmatter (`goal.Topics()`). A goal that is a member of a topic through `## Goals` alone therefore closes with exit 0 and leaves its topic `in_progress` forever, with no message.

Measured on v0.199.0: a scratch topic whose `## Goals` names a goal carrying no `topics:` frontmatter — `vault-cli goal complete <goal>` printed `✅ Goal completed: <goal>`, exited 0, and the topic stayed `status: in_progress`. The same pages with `topics: ['[[<topic>]]']` on the goal cascaded and the topic reached `status: completed`.
</objective>

<context>
Read these files fully before changing anything:

- `pkg/ops/goal_complete.go` — `cascadeTopics` (lines ~140-153) is the only seam this prompt changes. It currently passes `goal.Topics()` straight to `cascadeParents`.
- `pkg/ops/topic_complete.go` — `goalMembers` / `parseGoalSectionMembers` is the **correct** reader. Do not change how membership is parsed; reuse it.
- `pkg/ops/closure_gate.go` — `parseGoalSectionMembers(content string) []string` (bounded level-relative scan of the `Goals` heading, line ~470) and `normaliseGoalLink` (line ~522). Both are package-private and already available to `goal_complete.go`.
- `pkg/ops/parent_cascade.go` — `cascadeParents(ctx, names, attempt)` iterates names, normalises each, skips empties, and never returns an error; a refusal is recorded, not propagated. `ParentAttempt` is the wire shape.
- `pkg/storage/storage.go` — `TopicStorage` (lines ~105-108) currently has only `WriteTopic` and `FindTopicByName`.
- `pkg/storage/page.go` — `ListPages` (line 58) is the enumeration shape to mirror: `os.ReadDir`, skip directories and non-`.md` entries, warn-and-continue on an unreadable file, and return `nil, nil` when the directory does not exist.
- `pkg/storage/topic.go` — `topicStorage`, `readTopicFromPath`, `isTopicNameWithinDir`.
- `pkg/ops/parent_cascade_single_path_test.go` — asserts the cascade never writes a parent page directly. Do not weaken it.
- `pkg/ops/goal_complete_test.go` — existing operation wiring to extend, not rewrite.
- `integration/cli_test.go` — helpers `createTempVaultWithGoals` (line ~47) and `createTempVaultWithTopicPages(topicsDir string, topics, goals map[string]string)` (line ~188). The existing `Describe("vault-cli goal and topic closure gate", ...)` block shows the local-closure style.
- `specs/in-progress/070-parent-closure-cascade-on-complete.md` — the parent design. Its Assumption ("`topics:` … authoritative") and Desired Behavior #2 are what this prompt corrects; requirement 8 amends them.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80 / gocognit 20, nestif 4, dupl.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — prefix, one bullet per logical change, `## Unreleased` placement.
</context>

<requirements>
## 1. `TopicStorage.ListTopics`

Add to the interface in `pkg/storage/storage.go`:

```go
// ListTopics returns every topic page in the vault's configured topics
// directory. An unreadable or unparseable topic file is skipped with a warning,
// mirroring ListPages; a missing directory returns an empty list, and a
// directory-walk failure is returned.
ListTopics(ctx context.Context, vaultPath string) ([]*domain.Topic, error)
```

Implement it on `topicStorage` in `pkg/storage/topic.go`, mirroring `pageStorage.ListPages`:
`os.ReadDir(filepath.Join(vaultPath, t.config.TopicsDir))`; return `nil, nil` on `fs.ErrNotExist`;
skip directory entries and non-`.md` files; build each page with `t.readTopicFromPath(ctx, filePath, strings.TrimSuffix(entry.Name(), ".md"), vaultPath)`; on a per-file error log `slog.Warn("skipping unreadable topic", "file", filePath, "error", err)` and continue; honour `ctx.Done()` between entries.

⚠️ There is **no** topic walker to reuse: `walkTaskFiles` is a `*taskStorage` method (`pkg/storage/task.go` line ~183) and is not reachable from `topicStorage`. Do not move it and do not add a second copy of it — mirror `ListPages` as described.

Add a `Describe("ListTopics", ...)` block to the `storage_test` package covering: every topic file returned; a non-`.md` file and a subdirectory ignored; an unparseable topic file skipped while the rest are returned; a missing topics directory returning an empty list with a nil error.

Then run `make generate` so `mocks/topic-storage.go` gains the method. Do not hand-edit anything under `mocks/`.

## 2. `cascadeTopics` resolves the union of both readers

Replace the body of `goalCompleteOperation.cascadeTopics` so it builds the name list before calling `cascadeParents`:

1. Start with `goal.Topics()` in document order.
2. Call `g.topicStorage.ListTopics(ctx, vaultPath)`.
3. For each returned topic, compute `parseGoalSectionMembers(string(topic.Content))` and keep the topic when that list contains a name equal to `normaliseGoalLink(goal.Name)`.
4. Append the kept topic names **sorted by name** for determinism, skipping any name already present in the list from step 1 after `normaliseGoalLink` comparison on both sides.
5. Pass the combined list to `cascadeParents`.

`goalCompleteOperation` gains a `topicStorage storage.TopicStorage` field. Update `NewGoalCompleteOperation` to take it and update **every** call site — `pkg/cli/cli.go` (lines ~301 and ~1583), `pkg/ops/parent_cascade_single_path_test.go` (line ~44) and `pkg/ops/goal_complete_test.go` (line ~63). The constructor signature change is not optional and the build must stay green at each step.

## 3. A topic-scan failure is recorded, never swallowed

`ListTopics` can fail on a directory-walk error. That failure must not fail the goal close (the goal is already written) and must not be silent. When it returns an error:

- still attempt the declared `topics:` names, then
- append exactly one entry to the returned slice:

```go
ParentAttempt{
    Name:      "<topics scan>",
    Result:    ParentResultRefused,
    OpenItems: []string{err.Error()},
}
```

The angle-bracket name is deliberate: it can never collide with a real page name, and it makes an incomplete cascade visible in both the text output (`parent <topics scan>: refused — open: …`) and the `parents` JSON array.

## 4. Unit tests — `Describe("goal to topic cascade membership", ...)` in package `ops_test`

Wire the operation with a **real** `ops.NewTopicCompleteOperation(mockTopicStorage, mockGoalStorage)` — a `mocks.TopicCompleteOperation` would leave `WriteTopicCallCount() == 0`. Each case stubs `mockTopicStorage.ListTopicsReturns`, and where the topic gate must pass also `mockTopicStorage.FindTopicByNameReturns(<topic>)` plus `mockGoalStorage.FindGoalByNameReturns(<completed member goal>)`, because the topic gate reads each member goal's status from disk.

Cases:

- `It("completes a topic that lists the goal in its Goals section")` — the goal carries no `topics:`; `ListTopicsReturns` one `domain.Topic` whose content holds `## Goals` with `- [[my-goal]]` and whose gate is closed. Assert `WriteTopicCallCount() == 1`, the written topic's `status` is `completed`, and `result.Parents[0] == {Name: "my-topic", Result: ParentResultCompleted}`.
- `It("does not treat a goal named outside the Goals section as a member")` — same topic but the goal name appears only in prose above an empty `## Goals`. Assert `WriteTopicCallCount() == 0` and `len(result.Parents) == 0`.
- `It("does not cascade when no topic lists the goal")` — `ListTopicsReturns` topics that do not name it. Assert `WriteTopicCallCount() == 0` and no parent entry.
- `It("attempts declared topics before discovered ones, without duplicates")` — the goal carries `topics: ['[[declared-topic]]']` and `ListTopicsReturns` both `declared-topic` (which also lists the goal under `## Goals`) and `discovered-topic`. Assert exactly two entries, `Parents[0].Name == "declared-topic"`, `Parents[1].Name == "discovered-topic"`.
- `It("records a topic-scan failure and still completes the goal")` — `ListTopicsReturns(nil, ErrTest)`. Assert `err` is nil, `result.Success` is true, `WriteGoalCallCount() == 1`, the last parent entry is named `<topics scan>` with `Result == ParentResultRefused` and an `OpenItems[0]` containing `test error`.

## 5. Integration tests — `Describe("vault-cli goal to topic cascade membership", ...)` in `integration/cli_test.go`

Build vaults with `createTempVaultWithTopicPages("Topics", topics, goals)`. Fixtures:

- member goal, no `topics:`: `"---\nstatus: in_progress\n---\n# Success Criteria\n- [x] done\n# Definition of Done\n- [x] done\n"`
- topic whose `## Goals` lists the goal, gate closed: `"---\nstatus: in_progress\n---\n## Goals\n- [[my-goal]]\n# Success Criteria\n- [x] done\n# Completion Gate\n- [x] done\n"`
- control topic: same shape, `## Goals` empty, the goal named only in its body prose.

Cases:

1. `It("goal complete closes a topic that names the goal only in its Goals section")` — run `goal complete my-goal`; `gexec.Exit(0)`; stdout contains `parent my-topic: completed`; the topic file reads `status: completed`.
2. `It("goal complete leaves a topic that names the goal outside Goals untouched")` — the control topic is the only other topic. Run `goal complete my-goal`; exit 0; stdout does not contain `parent zz-control`; the control topic still reads `status: in_progress`.
3. `It("goal complete with no member topic prints no parent line")` — `Topics` holds only the control topic. Exit 0, stdout does not match `(?m)^parent `.

The `<topics scan>` failure path is covered by the unit test only: a container run is root, so `chmod` cannot make the topics directory unreadable. State that in the completion report rather than adding a case that always skips.

## 6. Docs

In `docs/goal-writing.md`, extend the existing `### Topics (`topics`)` section with one short paragraph: membership in a topic is granted **either** by the goal's `topics:` list **or** by appearing under that topic page's `## Goals` section, and `goal complete` cascades on both. State that the two are equivalent for closure and that `topics:` is the cheaper read.

## 7. CHANGELOG

Add one bullet under `## Unreleased` (create the section below the preamble if absent, above the top versioned heading):

```
- fix: `goal complete` cascades to a topic that lists the goal under its `## Goals` section even when the goal carries no `topics:` frontmatter — the two membership readers now agree, and a topic-scan failure is reported as a refused parent instead of being silent
```

## 8. Amend spec 070

In `specs/in-progress/070-parent-closure-cascade-on-complete.md`, correct the two places that now contradict the shipped behaviour — text only, no status change:

- **Desired Behavior #2** — say the goal's parent set is every topic that names it, whether by the goal's `topics:` or by the topic's `## Goals` section.
- **Assumptions** — replace the "`topics:` is authoritative" claim with the union statement.

Do not regenerate prompts, do not change the spec's `status`, and do not touch any other section.
</requirements>

<constraints>
- **`topic complete` is not changed.** It is the correct reader; only the cascade adapts to it.
- **`parseGoalSectionMembers` stays the single Goals-section parser.** Do not write a second one in `pkg/ops` or `pkg/storage`.
- **Do not weaken `pkg/ops/parent_cascade_single_path_test.go`.** The cascade still writes a topic only through `topicCompleteOperation`.
- **Do not change the `parents` JSON wire shape.** A completed entry still omits `open_items`.
- **Do not delete or relax existing tests.** Every current integration case must keep passing unchanged.
- **Do not hand-bump version manifests or tag.** The releaser owns the tag after merge.
- Do NOT commit — dark-factory handles git.
- `make precommit` must exit 0; `make generate` output must be committed with the change.
</constraints>

<verification>
Run, in order, and report each exit code:

```
make generate
go test -mod=mod ./pkg/storage -run TestSuite -ginkgo.focus='ListTopics' -ginkgo.v
go test -mod=mod ./pkg/ops -run TestSuite -ginkgo.focus='goal to topic cascade membership' -ginkgo.v
go test -mod=mod ./integration -run TestIntegration -ginkgo.focus='goal to topic cascade membership' -ginkgo.v
make test
make precommit
```

The `-ginkgo.v` output must contain every `It` name from requirements 1, 4 and 5 — a focus that matches nothing exits 0 while running nothing, so confirm the names appear.

Structural greps (each count exact):

```
grep -c 'ListTopics' pkg/storage/storage.go        # 2 (doc comment + signature)
grep -c 'func (t \*topicStorage) ListTopics' pkg/storage/topic.go   # 1
grep -c 'topicStorage' pkg/ops/goal_complete.go    # >= 2
grep -c 'WriteTopic(' pkg/ops/goal_complete.go     # 0
grep -n '^## Unreleased' CHANGELOG.md              # 1 line, above the top versioned heading
```

If `make precommit` exits non-zero: fix, re-run only the failing target until it passes, then run `make precommit` once more. Report a non-zero exit as `"status":"failed"` — never rationalise it as success.
</verification>

<notes>
- Ordering rationale: declared `topics:` first keeps the existing observable order stable for every goal that already cascaded correctly; discovered topics are appended, so no existing behaviour changes for them.
- Deduplication is by `normaliseGoalLink` on both sides, so `[[My Topic]]`, `My Topic` and `[[My Topic|label]]` compare equal.
- The `<topics scan>` entry is the only new shape on the wire. It cannot be mistaken for a page because a page name may not contain `<` or `>`.
- `goalCompleteOperation` will hold a second `storage.TopicStorage` alongside the one inside `topicCompleteOperation`. That redundancy is accepted: routing the read through `topicCompleteOperation` would put a listing concern on the closure operation.
</notes>
