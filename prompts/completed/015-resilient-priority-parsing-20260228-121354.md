<objective>
Make priority field parsing resilient: if priority cannot be parsed as int (e.g. "medium", "high"), use -1 to represent invalid priority instead of skipping the file with a warning.
This eliminates noisy warnings during list/lint for files with string priority values.
</objective>

<context>
Go CLI project at ~/Documents/workspaces/vault-cli.
Currently, pages with INVALID_PRIORITY cause a Warning log and the file is skipped entirely from list output.
The priority field is non-critical — tasks/goals should still be listed even with an invalid priority.
Read CLAUDE.md for project conventions.
</context>

<requirements>
1. Find where pages are parsed from YAML frontmatter (likely in `./pkg/storage/` or `./pkg/domain/`)
2. Change priority parsing to be non-fatal:
   - If priority field is missing → use 0 (existing default behavior)
   - If priority field is a valid int → use that value
   - If priority field is any other type (string like "medium", "high", etc.) → use -1, no warning
3. Remove the Warning log for INVALID_PRIORITY during page reads — lint is the right place to report it
4. Pages with invalid priority must still appear in list output
5. The `lint` command should still detect and report INVALID_PRIORITY (no change to lint logic)
</requirements>

<implementation>
The YAML parsing likely uses strict unmarshaling. Switch to a two-pass approach or use a custom unmarshaler for the priority field:

Option: Use `yaml.Node` or a raw map to extract priority, then attempt int conversion, fallback to -1.

Example approach for the page struct:
```go
type PageFrontmatter struct {
    Status   string `yaml:"status"`
    Priority int    `yaml:"priority"` // keep for valid int case
    // ...
}

// After unmarshal, if priority failed, re-parse with interface{} and check type
```

Or use a custom type:
```go
type Priority int

func (p *Priority) UnmarshalYAML(value *yaml.Node) error {
    var i int
    if err := value.Decode(&i); err == nil {
        *p = Priority(i)
        return nil
    }
    *p = Priority(-1) // invalid/unparseable
    return nil
}
```
</implementation>

<verification>
Run: `make test`
Manually test:
- `vault-cli task list --vault brogrammers` → no INVALID_PRIORITY warnings, all tasks listed
- `vault-cli task lint --vault brogrammers` → still reports INVALID_PRIORITY issues
- `vault-cli task list --vault personal` → still works correctly
</verification>

<success_criteria>
- `make test` passes
- `vault-cli task list --vault brogrammers` shows tasks without any INVALID_PRIORITY warnings
- `vault-cli task lint --vault brogrammers` still detects and reports INVALID_PRIORITY
- Files with invalid priority appear in list output with priority -1 internally
</success_criteria>
