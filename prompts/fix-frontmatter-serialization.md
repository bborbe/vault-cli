<objective>
Fix frontmatter corruption: WriteTask/WriteGoal/WriteTheme serialize metadata fields (Name, Content, FilePath) into YAML frontmatter. These fields must be excluded from serialization.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read pkg/domain/task.go, pkg/domain/goal.go, pkg/domain/theme.go — domain structs.
Read pkg/storage/markdown.go — serializeWithFrontmatter method.
</context>

<requirements>
1. In `pkg/domain/task.go`, add `yaml:"-"` tag to the three metadata fields on the Task struct:
   - `Name string` → `Name string \`yaml:"-"\``
   - `Content string` → `Content string \`yaml:"-"\``
   - `FilePath string` → `FilePath string \`yaml:"-"\``

2. In `pkg/domain/goal.go`, add `yaml:"-"` tag to the equivalent metadata fields on the Goal struct (Name, Content, FilePath, Tasks).

3. In `pkg/domain/theme.go`, add `yaml:"-"` tag to the equivalent metadata fields on the Theme struct (Name, Content, FilePath).

4. Add tests in `pkg/storage/markdown_test.go` that verify serialization safety:

   a. **Round-trip test**: Create a task with frontmatter fields (status, priority) AND metadata fields (Name, Content, FilePath). Write it using WriteTask. Read the raw file bytes. Verify the frontmatter does NOT contain `name:`, `content:`, or `filepath:` keys. Verify frontmatter contains only expected YAML fields.

   b. **Content not embedded test**: Create a task where Content contains a full markdown file with its own frontmatter block (--- delimiters). Write it using WriteTask. Read the raw file bytes. Verify the written file has exactly one frontmatter block (two `---` lines), not nested/embedded frontmatter. This is the exact corruption that occurred in production.

   c. **Goal and Theme round-trip**: Same as (a) but for WriteGoal and WriteTheme — verify their metadata fields are also excluded.

   Follow existing test patterns in the file.
</requirements>

<constraints>
- Do NOT change the parseFrontmatter or serializeWithFrontmatter logic
- Do NOT change any function signatures
- Do NOT modify existing passing tests
- The fix is purely adding yaml:"-" struct tags
</constraints>

<verification>
Run: `make test`
Confirm: all tests pass, including the new round-trip test.
</verification>

<success_criteria>
After WriteTask, the markdown file frontmatter contains only YAML-tagged fields (status, page_type, goals, priority, assignee, defer_date, tags, phase, claude_session_id) — never name, content, or filepath.
</success_criteria>
