---
status: completed
spec: ["001"]
summary: Added ListDecisions, FindDecisionByName, and WriteDecision to Storage interface with recursive vault scanning, symlink guard, ambiguous-match detection, and regenerated mocks
container: vault-cli-052-spec-001-decision-storage
dark-factory-version: v0.54.0
created: "2026-03-16T00:00:00Z"
queued: "2026-03-16T10:36:41Z"
started: "2026-03-16T10:40:31Z"
completed: "2026-03-16T10:45:26Z"
branch: dark-factory/decision-list-ack
---

<summary>
- The Storage interface gains three new methods for decisions: list, find-by-name, and write
- Listing scans the entire vault recursively (not a fixed subdirectory), finding all markdown files with needs_review: true frontmatter
- Symlinks pointing outside the vault root are not followed (path traversal protection)
- Files with malformed frontmatter are skipped with a stderr warning — scanning continues
- FindDecisionByName uses exact-match-first then partial-match, and errors on ambiguous partial matches
- WriteDecision updates frontmatter in-place, preserving the markdown body content unchanged
- Unit tests cover the recursive scan, symlink guard, malformed-frontmatter skip, exact/partial/ambiguous name matching, and write-preserves-body
</summary>

<objective>
Add decision storage methods (`ListDecisions`, `FindDecisionByName`, `WriteDecision`) to the `Storage` interface and implement them on `markdownStorage`, enabling the ops layer to list and mutate decisions across the entire vault tree.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/storage/markdown.go` — understand `Storage` interface, `markdownStorage` struct, `parseFrontmatter`, `serializeWithFrontmatter`, and `findFileByName`.
Read `pkg/domain/decision.go` — the Decision struct added in prompt 1 (fields: NeedsReview, Reviewed, ReviewedDate, Status, Type, PageType; metadata: Name, Content, FilePath).
Read `pkg/storage/markdown_test.go` — understand the test style used for existing storage methods.
Read `docs/development-patterns.md` — "Naming" section for decision type naming.
</context>

<requirements>
1. Add three methods to the `Storage` interface in `pkg/storage/markdown.go` (inside the existing `//counterfeiter:generate` block):

```go
// Decision operations
ListDecisions(ctx context.Context, vaultPath string) ([]*domain.Decision, error)
FindDecisionByName(ctx context.Context, vaultPath string, name string) (*domain.Decision, error)
WriteDecision(ctx context.Context, decision *domain.Decision) error
```

2. Implement `ListDecisions` on `markdownStorage`:
   - Use `filepath.WalkDir(vaultPath, ...)` to scan the entire vault recursively
   - For each `.md` file encountered:
     - Resolve `filepath.EvalSymlinks` on the file path; if the resolved path does not have `vaultPath` as a prefix, log a warning to stderr and skip (path traversal guard)
     - Read the file and attempt to parse frontmatter into a `domain.Decision`
     - If frontmatter parsing fails (no frontmatter, malformed YAML): log warning `"Warning: failed to parse decision frontmatter %s: %v\n"` to stderr and continue
     - If `NeedsReview` is false after parsing, skip the file
     - Set `decision.Name` to the relative path from `vaultPath` without the `.md` extension (use `strings.TrimPrefix` + `strings.TrimSuffix`)
     - Set `decision.FilePath` to the absolute path
     - Set `decision.Content` to the full file content
   - Return the collected decisions (empty slice, not nil, when none found)
   - Errors during `WalkDir` itself (e.g., vault path doesn't exist): return the error wrapped with `errors.Wrap`

3. Implement `FindDecisionByName` on `markdownStorage` — NOTE: this deliberately differs from `findFileByName` which returns the first partial match. Decision names span multiple directories, so ambiguous matches must be caught:
   - Call `ListDecisions` to get all decisions
   - Exact match: find the decision whose `Name` equals `name` exactly (after `filepath.ToSlash` normalization on both sides) — return it immediately if found
   - Partial match: collect all decisions where `strings.Contains(strings.ToLower(decision.Name), strings.ToLower(name))`
   - If 0 partial matches: return error `fmt.Errorf("decision not found: %s", name)`
   - If 1 partial match: return it
   - If 2+ partial matches: return error `fmt.Errorf("ambiguous match: %q matches %d decisions: %s", name, len(matches), joined-names)` where joined-names is the matched names joined with `", "`
   - The `name` argument must NOT be treated as a file path — if `name` contains `..` path components, return error `fmt.Errorf("invalid decision name: %s", name)` immediately (path traversal guard)

4. Implement `WriteDecision` on `markdownStorage` — follow the same pattern as `WriteTask` (~line 135) and `WriteGoal` (~line 274):
   - Call `serializeWithFrontmatter(decision, decision.Content)` — returns `(string, error)`
   - If error: return `errors.Wrap(ctx, err, "serialize frontmatter")`
   - Call `os.WriteFile(decision.FilePath, []byte(content), 0600)`
   - If error: return `errors.Wrap(ctx, err, fmt.Sprintf("write file %s", decision.FilePath))`

5. Add a helper `readDecisionFromPath(ctx, filePath, name string) (*domain.Decision, error)` on `markdownStorage`:
   - Reads file, parses frontmatter, sets Name/Content/FilePath, returns decision
   - Used internally by `ListDecisions`

6. Run `go generate ./...` to regenerate the counterfeiter mock for `Storage` (the new methods must appear in `mocks/storage.go`).

7. Add tests in `pkg/storage/markdown_test.go` (or a new `pkg/storage/decision_test.go`):
   - `ListDecisions` returns only files with `needs_review: true`
   - `ListDecisions` skips files with no frontmatter (warning, no error)
   - `ListDecisions` returns empty slice when no decisions exist
   - `FindDecisionByName` exact match
   - `FindDecisionByName` partial match (single result)
   - `FindDecisionByName` ambiguous partial match returns error
   - `FindDecisionByName` not-found returns error
   - `FindDecisionByName` name containing `..` returns error
   - `WriteDecision` preserves markdown body content (only frontmatter changes)
</requirements>

<constraints>
- Decision is a separate domain type — do NOT reuse or extend Task storage methods
- Scans entire vault recursively — there is NO per-directory config for decisions
- Do NOT follow symlinks that resolve outside vaultPath (check with `filepath.EvalSymlinks` + `strings.HasPrefix`)
- `ListDecisions` must skip (not fail) on individual file errors — only `WalkDir` errors on the vault root itself cause a return error
- `FindDecisionByName` must reject names with `..` before any file system access
- `WriteDecision` must preserve markdown body — use `serializeWithFrontmatter` which already extracts the body
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- License header required (copy from markdown.go)
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
