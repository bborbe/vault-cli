---
status: completed
summary: Added support for [/] in-progress checkbox state in daily notes and storage layer
container: vault-cli-034-fix-daily-note-in-progress-checkbox
dark-factory-version: v0.14.5
created: "2026-03-03T22:40:04Z"
queued: "2026-03-03T22:40:04Z"
started: "2026-03-03T22:40:04Z"
completed: "2026-03-03T22:44:14Z"
---
<objective>
Fix daily note checkbox regex to handle `[/]` (in-progress) state in addition to `[ ]` and `[x]`. This affects complete, defer, and storage layer.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding-guidelines/go-testing-guide.md for testing patterns.
Read pkg/ops/complete.go — updateDailyNote method uses checkboxRegex.
Read pkg/ops/defer.go — removeFromDailyNote method uses checkboxRegex.
Read pkg/storage/markdown.go — global checkboxRegex and parseCheckboxes.
</context>

<requirements>
1. Update the global `checkboxRegex` in `pkg/storage/markdown.go`:
   - Current: `^(\s*)- \[([ x])\] (.+)$`
   - New: `^(\s*)- \[([ x/])\] (.+)$`
   - This adds `/` as a valid checkbox state character

2. Update `pkg/storage/markdown.go` `parseCheckboxes`:
   - Current: `matches[2] == "x"` for Checked
   - Keep: `matches[2] == "x"` still means Checked=true
   - Add: `matches[2] == "/"` → a new InProgress field, or treat as Checked=false (since it's not complete)
   - Actually: keep simple. `[/]` is neither checked nor unchecked. For now, `Checked: matches[2] == "x"` is fine. The regex just needs to MATCH the line so it can be found/replaced.

3. Update `pkg/ops/complete.go` `updateDailyNote`:
   - The local `checkboxRegex` must also match `[/]`
   - Change: `^(\s*)- \[([ x])\] (.+)$` → `^(\s*)- \[([ x/])\] (.+)$`
   - When `checked == true`: replace `- [ ]` OR `- [/]` with `- [x]`
   - Add: `lines[i] = strings.Replace(line, "- [/]", "- [x]", 1)` as fallback if `- [ ]` wasn't found

4. Update `pkg/ops/defer.go` `removeFromDailyNote`:
   - The local `checkboxRegex` must also match `[/]`
   - Change: `^(\s*)- \[([ x])\] (.+)$` → `^(\s*)- \[([ x/])\] (.+)$`
   - No other logic change needed — it already removes the matching line

5. Add `domain.CheckboxItem` field if not present:
   - Check if `InProgress bool` field exists on CheckboxItem
   - If not, add: `InProgress bool` — set to true when `matches[2] == "/"`

6. Update/add tests:
   - In `pkg/ops/complete_test.go`: add test where daily note has `- [/] [[my-task]]` → complete changes it to `- [x] [[my-task]]`
   - In `pkg/ops/defer_test.go`: add test where daily note has `- [/] [[my-task]]` → defer removes the line
   - In `pkg/storage/markdown_test.go`: add test that parseCheckboxes correctly parses `- [/] In progress item`
</requirements>

<constraints>
- Do NOT change any behavior for `[ ]` and `[x]` — only ADD support for `[/]`
- Do NOT modify the complete or defer business logic beyond checkbox matching
- Use Ginkgo v2 + Gomega, follow existing test patterns
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- `- [/] [[task]]` is found when completing → becomes `- [x] [[task]]`
- `- [/] [[task]]` is found when deferring → line removed
- `- [ ] [[task]]` still works as before
- `- [x] [[task]]` still works as before
- parseCheckboxes returns items for all three states
</verification>
