---
status: completed
spec: [001-decision-list-ack]
summary: Created Decision domain struct in pkg/domain/decision.go with YAML frontmatter fields and DecisionID type, plus unit tests verifying YAML round-tripping and omitempty behavior.
container: vault-cli-051-spec-001-decision-domain
dark-factory-version: v0.54.0
created: "2026-03-16T00:00:00Z"
queued: "2026-03-16T10:36:41Z"
started: "2026-03-16T10:36:43Z"
completed: "2026-03-16T10:40:26Z"
branch: dark-factory/decision-list-ack
---

<summary>
- A new Decision domain type is introduced — separate from Task, Goal, and other existing types
- The struct models markdown files with needs_review frontmatter, covering review state and status fields
- Metadata fields (name as relative vault path, content, file path) are tagged to be excluded from YAML serialization
- The domain file follows the same pattern as pkg/domain/goal.go and pkg/domain/task.go
- Unit tests verify YAML round-tripping for all frontmatter fields
</summary>

<objective>
Create the `Decision` domain struct in `pkg/domain/decision.go` with YAML frontmatter fields and metadata fields, establishing the data model that storage, ops, and CLI layers will build on.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/domain/goal.go` — follow the same pattern: frontmatter fields with yaml tags, metadata fields tagged `yaml:"-"`, a typed ID.
Read `pkg/domain/task.go` — note the separation between frontmatter fields and metadata fields.
Read `docs/development-patterns.md` — section "Adding a New Command" and "Naming".
</context>

<requirements>
1. Create `pkg/domain/decision.go` with a `Decision` struct:

```go
// Decision represents a markdown file in the vault that has needs_review frontmatter.
type Decision struct {
    // Frontmatter fields
    NeedsReview  bool   `yaml:"needs_review"`
    Reviewed     bool   `yaml:"reviewed,omitempty"`
    ReviewedDate string `yaml:"reviewed_date,omitempty"`
    Status       string `yaml:"status,omitempty"`
    Type         string `yaml:"type,omitempty"`
    PageType     string `yaml:"page_type,omitempty"`

    // Metadata — excluded from YAML serialization
    Name     string `yaml:"-"` // Relative path from vault root without .md extension
    Content  string `yaml:"-"` // Full markdown content including frontmatter
    FilePath string `yaml:"-"` // Absolute path to file
}
```

2. Create a `DecisionID` type (string alias) with a `String()` method, following the `GoalID` pattern in `pkg/domain/goal.go`.

3. Create `pkg/domain/decision_test.go` in the external test package `domain_test`:
   - Test that a Decision with all fields marshals to YAML and back correctly
   - Test that `reviewed`, `reviewed_date`, `status`, `type`, and `page_type` fields use `omitempty` (i.e., a Decision with only `needs_review: true` marshals without the omitted fields)
   - Test that metadata fields (`Name`, `Content`, `FilePath`) are NOT included in YAML output
</requirements>

<constraints>
- Decision is a new domain type — do NOT modify Task, Goal, Theme, or any other existing domain types
- `ReviewedDate` is a plain string (`YYYY-MM-DD`) — not a `*libtime.Date` — because decisions are not guaranteed to use the same date library as tasks
- `Name` stores the relative path from vault root without `.md` extension (e.g., `"10 Decisions/Some Page Name"`) — not just the bare filename
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- License header required: `// Copyright (c) 2025 Benjamin Borbe All rights reserved.` (copy from goal.go)
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
