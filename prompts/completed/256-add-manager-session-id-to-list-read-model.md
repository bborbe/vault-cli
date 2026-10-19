---
status: completed
summary: Added manager_session_id to the shared list read model, its tests, integration coverage, docs, and changelog.
execution_id: vault-cli-manager-session-id-exec-256-add-manager-session-id-to-list-read-model
dark-factory-version: v0.198.7
created: "2026-10-19T16:21:34Z"
queued: "2026-10-19T16:21:34Z"
started: "2026-10-19T16:23:42Z"
completed: "2026-10-19T16:26:56Z"
---

# Expose manager_session_id in the task list read model

<summary>
- Every list read model (task, goal, topic, theme, objective, vision) carries the manager session id recorded on a page
- Consumers can classify a page against its manager session without re-reading frontmatter
- Pages without the field are unchanged — the value is omitted from the JSON output
- Existing filters, ordering, and every other field are untouched
- Covered by tests for the set, absent, and JSON-omitted cases

</summary>

<objective>
Every list command (`task list`, `goal list`, `topic list`, `theme list`, `objective list`, `vision list`) must carry the manager session id recorded on a page, so the vault UI can classify a goal or topic against the manager session that armed it instead of only the page's own session. Today claude-supervisor writes the field on goal and topic pages but no list command surfaces it, so the UI cannot see it.

</objective>

<context>
Read `CLAUDE.md` for project conventions.
Read `pkg/ops/list.go` — the `TaskListItem` struct and the `Execute` loop that builds each item.
Read `pkg/domain/frontmatter_map.go` — `GetString`, the accessor the same loop already uses for `flag_set_by`.
Read `pkg/ops/list_test.go` — the existing list test style (Ginkgo v2 + Gomega).
`pkg/cli/cli.go`'s generic list command (`createGenericListCommand`, used by `goal`/`topic`/`theme`/`objective`/`vision` list) shares `ListOperation.Execute`, which is why one `TaskListItem` change reaches every list command.
The key `manager_session_id` is written by claude-supervisor (a separate repo): its manager-resolve step writes it with `vault-cli goal set "<subject>" manager_session_id "$CLAUDE_CODE_SESSION_ID"` (the `topic` verb on a topic page), so the field lives on goal and topic pages, never on tasks. Nothing in this repo references the key; the name above is verified and is the contract.

</context>

<requirements>
1. In `pkg/ops/list.go`, add `ManagerSessionID string` with the struct tag `json:"manager_session_id,omitempty"` to `TaskListItem`, immediately after `ClaudeSessionID`.
2. In the `Execute` loop that builds `items[i]`, populate it with `ManagerSessionID: task.GetString("manager_session_id")`, mirroring the existing `FlagSetBy: task.GetString("flag_set_by")` line.
3. In `pkg/ops/list_test.go`, add coverage that:
   - a page whose frontmatter carries `manager_session_id` returns that exact value in the list item;
   - a page without the key returns an empty string;
   - JSON serialization omits the key when empty and includes it when set.
4. In `integration/cli_test.go`, add an assertion that `goal list --output json` emits `manager_session_id` for a fixture goal whose frontmatter carries it (use `createTempVaultWithGoals`, line 47); add the same assertion for `topic list --output json` (use `createTempVaultWithTopics`, line 149, plus manually written topic files) — the producer writes the key on goal and topic pages, and the wire key is what the downstream consumer reads.
5. Document the new output field in `docs/goal-writing.md` (its frontmatter field table and its "Where it surfaces" line, ~line 142) — the producer writes the key on goal pages, so that is the primary consumer-facing location. Add a matching note to the shared list-output documentation rather than the `blocked_by` dependency subsection. State that this change adds no setter or CLI flag of its own; the field is written by the producer (claude-supervisor) through the existing generic `vault-cli goal set "<subject>" manager_session_id "$CLAUDE_CODE_SESSION_ID"` / `topic set` verbs, so do not describe the field as unsettable.
6. Add a `## Unreleased` entry to `CHANGELOG.md` describing the new field. Place the section below the preamble block and above the newest `## vX.Y.Z` section.
7. Before finishing, re-run `<verification>` and confirm it passes; walk each requirement above against the change.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Do not change any existing field, filter, sort, or output shape
- Read-only exposure: no setter, no CLI flag, no write path
- Scope is the list read model only: `TaskListItem` and `ListOperation.Execute` are shared by `task list`, `goal list`, `topic list`, `theme list`, `objective list`, and `vision list`, so the field surfaces in all six automatically — do not add a page-type condition to limit it to tasks. `pkg/ops/show.go`'s `TaskDetail` (`task show --output json`) is deliberately NOT changed — the downstream consumer reads the list model. If `task show` should carry the field too, that is a separate prompt.

</constraints>

<verification>
make precommit
</verification>
