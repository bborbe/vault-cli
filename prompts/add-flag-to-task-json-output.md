---
status: draft
created: "2026-09-02T23:10:00+02:00"
spec:
  - 043-flag-field
---

<summary>
- JSON output from `task show` and `task list` currently omits the `flag` field entirely — spec 043 added the typed accessor but not the JSON read surface.
- Consumers that read task state from JSON — vault-ui's `_parse_task` reads `data.get("flag")` — therefore always see the flag as absent, so a flagged task renders unflagged on the board.
- This prompt adds the `flag` value to both task JSON output shapes (full detail and list row), populated from the existing typed accessor.
- Round-trip: after this change, `set flag true` then `task show --output json` / `task list --output json` contains `"flag": true`.
</summary>

<objective>
Task JSON output (`show` and `list`) reports the boolean `flag` value from frontmatter, so downstream consumers (vault-ui) can render and sort flagged cards. `make precommit` passes and the JSON output carries `"flag": true` / `"flag": false` / absent for un-flagged tasks.
</objective>

<context>
- Read `pkg/ops/show.go` (TaskDetail struct, ~lines 42-62, and Execute which populates it) and `pkg/ops/list.go` (TaskListItem struct, ~lines 46-62).
- The typed accessor already exists: `pkg/domain/task_frontmatter.go:74` `func (f TaskFrontmatter) Flag() bool { return f.GetBool("flag") }`.
- Existing JSON structs use `json:"field,omitempty"` for optional fields. Decide the serialization shape for a boolean with omitempty semantics: a `bool` with `omitempty` omits `false` — which is acceptable (un-flagged = absent) and matches how the frontmatter round-trips (no `flag:` key for un-flagged tasks). Use `Flag bool \`json:"flag,omitempty"\`` and populate from `task.Flag()`.
- Follow the repo's error-handling (bborbe/errors with ctx wrapping), no fmt.Errorf, no encoding/json imports in command files (PrintJSON helper), counterfeiter mocks, libtime for dates.
- Coding plugin docs: `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`, `go-doc-best-practices.md`, `changelog-guide.md`.
</context>

<requirements>
1. `pkg/ops/show.go` — add `Flag bool \`json:"flag,omitempty"\`` to the `TaskDetail` struct and populate it in `Execute` from `task.Flag()`.
2. `pkg/ops/list.go` — add `Flag bool \`json:"flag,omitempty"\`` to the `TaskListItem` struct and populate it in the list operation from the task's `Flag()`.
3. Tests: extend the existing show/list tests (find them by grepping for `TaskDetail` / `TaskListItem` in pkg/ops/*_test.go) with a case asserting that a task with `flag: true` in frontmatter yields `"flag": true` in the JSON output, and that a task without the key yields no `flag` field (omitempty).
4. Create a `## Unreleased` section directly above `## v0.119.0` if one does not already exist, and add a `feat:` bullet under it describing that task JSON output now includes the flag field.
</requirements>

<constraints>
- Use the existing `task.Flag()` accessor — do not re-implement coercion.
- `omitempty` on the bool: un-flagged tasks omit the field (matches the frontmatter round-trip invariant from spec 043).
- Do not change any other field in the structs, and do not reorder existing fields.
- Do not touch the vault-ui repo or any other consumer — this is vault-cli output only.
- No version bump, no tag, no commit of unrelated files.
</constraints>

<verification>
- `make precommit` exits 0.
- `grep -n 'json:"flag,omitempty"' pkg/ops/show.go pkg/ops/list.go` returns 2 lines.
- Manual: on a scratch task with `flag: true`, `vault-cli task show "T" --output json` contains `"flag": true`; after `clear flag`, the JSON output has no `flag` key.
</verification>
