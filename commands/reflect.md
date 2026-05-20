---
description: Extract high-significance learnings from this conversation and document them in the vault's Knowledge Base — works in any vault via vault-cli config
allowed-tools:
  - Read
  - Write
  - Edit
  - Grep
  - Glob
  - Bash
  - AskUserQuestion
  - mcp__semantic-search__search_related
  - mcp__semantic-search__check_duplicates
---

Extract high-significance learnings from the current conversation and document them in the active vault's Knowledge Base directory.

This command **must stay inline** — it analyzes the parent conversation; a sub-agent cannot see the conversation context.

## Runtime context

```bash
vault-cli config list --output json
```

Match cwd against each `path`. Use the matched vault's:
- `knowledge_dir` — Knowledge Base folder (e.g. `50 Knowledge Base`, `50 Knowledge`); falls back to default `50 Knowledge Base` if absent

If cwd is not inside any vault path, ask the user which vault to write to (AskUserQuestion with vault names from the config).

## Runtime detection

```
SEMANTIC_SEARCH_AVAILABLE = `mcp__semantic-search__search_related` present in session
DUPLICATE_CHECK_AVAILABLE = `mcp__semantic-search__check_duplicates` present in session
```

Both are optional — gracefully fall back to `Glob` / `Grep` when absent.

## Step 1: Analyze conversation for learnings

Scan the full conversation context. Look for:

- **Discoveries**: "I found that...", "Turns out...", "The issue was..."
- **Solutions**: Bug fixes, workarounds, successful approaches
- **Patterns**: Code patterns, API usage, tool configurations
- **Gotchas**: Things that didn't work, common mistakes, edge cases
- **New capabilities**: Tools created, features added, workflows established
- **Corrections**: Wrong assumptions corrected, better approaches found

### Significance filter

Each potential learning must pass all three:

| Question | Must be YES |
|---|---|
| **Reusable?** | Applies to 2+ future contexts? |
| **Non-obvious?** | Would forget in 6 months? |
| **Not in code?** | Insight beyond what's readable in source? |

**Skip (low significance):** API query parameters, script path changes, config tweaks, simple refactors, bug fixes for specific edge cases, implementation details visible in code.

**Keep (high significance):** Patterns combining multiple systems, architectural decisions with tradeoffs, non-obvious gotchas that caused wasted time, new capabilities/tools, workflow improvements affecting daily work.

Extract **0-3 high-significance learnings**. For each:

- **Title**: 3-7 words, descriptive
- **Category**: `code-pattern` | `tool-usage` | `workflow` | `gotcha` | `capability` | `configuration`
- **Summary**: 1-2 sentences
- **Context**: Why it matters, when it applies

## Step 2: Present learnings + ask user

**No learnings passed the filter:**

```
📚 No significant learnings detected.

This conversation contained:
- <reason 1>
- <reason 2>

Nothing to document — vault stays lean ✨
```

STOP.

**1-3 learnings found:**

```
📚 High-significance learnings (N found):

1. <Title> (<category>)
   <Summary>

2. ...

Which learnings should we document? (comma-separated numbers, "all", or "none"):
```

AskUserQuestion with options derived from the list. Wait for selection.

## Step 3: For each selected learning, find a documentation home

For each selected learning, process one at a time.

**Find candidates:**

If `SEMANTIC_SEARCH_AVAILABLE`:
- `mcp__semantic-search__search_related(query="<title> <key terms>", top_k=5)`
- Filter to results under the vault's `<knowledge_dir>/` path

Else (fallback):
- `Glob: <knowledge_dir>/*<keyword>*.md`
- `Grep` for the strongest keyword in `<knowledge_dir>/`

**Decide action:**

| Top-match score | Action |
|---|---|
| ≥ 0.6 (or strong title/content overlap in fallback) | **ENHANCE** an existing page |
| 0.4–0.6 (or partial match) | **REVIEW** — show options |
| < 0.4 (or no usable match) | **CREATE** a new page |

Present:

```
📝 Learning: <title>

Recommended: <ENHANCE | CREATE | REVIEW>

[ENHANCE]   Add to: [[<existing page>]]  (similarity: 0.XX)
[CREATE]    Create new: <knowledge_dir>/<suggested name>.md
[REVIEW]    Options:
              1. [[<page A>]]  (0.XX)
              2. [[<page B>]]  (0.XX)
              3. Create new

Proceed? (y / n / skip)
```

Wait for user confirmation.

## Step 4: Apply documentation

**ENHANCE (add to existing page):**

1. Read the page
2. Find or create a `## Additional Insights` section
3. Append as a subsection:

   ```markdown
   ### <Learning Title>

   <Summary>

   **Context**: <when it applies>

   **Details**:
   - <key point 1>
   - <key point 2>
   ```

**CREATE (new page):**

1. Pick a filename (PascalCase or natural language matching existing KB pages)
2. If `DUPLICATE_CHECK_AVAILABLE`:
   - `mcp__semantic-search__check_duplicates(file_path="<knowledge_dir>/<filename>.md")`
   - If duplicate detected → ask user whether to enhance the duplicate instead
3. Create the file at `<knowledge_dir>/<filename>.md`:

   ```markdown
   ---
   tags:
     - knowledge
     - <category>
   created: <YYYY-MM-DD>
   ---
   Tags: [[Knowledge Base]]

   ---

   <Summary paragraph>

   ## Context

   <when and why this matters>

   ## Details

   - <key point 1>
   - <key point 2>
   - <code examples if applicable>

   ## Related

   - [[<related page 1>]]
   - [[<related page 2>]]
   ```

## Step 5: Report

```
✅ Reflection complete

Documented N learnings:
- [[<page 1>]] (enhanced)
- [[<page 2>]] (created)

Skipped: M learnings
```

## Guidelines

- Be selective — not everything is worth documenting
- Always check existing pages before creating new ones
- Keep KB pages scannable, not verbose
- Link generously to related pages
- Match existing KB organization patterns in the active vault
- Skip the obvious — basic operations, one-time fixes

## Notes

- No hardcoded `50 Knowledge Base/` — the vault's `knowledge_dir` config drives the folder
- Works in any vault registered with `vault-cli config` that has `knowledge_dir` set (or falls back to the documented default)
- Multiple Atlassian / semantic-search MCPs supported via the same unified detection pattern
