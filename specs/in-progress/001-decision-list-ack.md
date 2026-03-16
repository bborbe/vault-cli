---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-03-16T10:19:12Z"
prompted: "2026-03-16T10:23:15Z"
verifying: "2026-03-16T11:01:23Z"
branch: dark-factory/decision-list-ack
---

## Summary

- Add `vault-cli decision list` command to show Obsidian pages that need boss-mode review
- Add `vault-cli decision ack` command to mark a decision as reviewed (sets `reviewed: true` + date)
- New `Decision` domain type — separate from Task, scans entire vault recursively
- Output supports plain text and JSON, consistent with existing list/show commands

## Problem

The Obsidian vault has a "10 Decisions" dashboard showing pages needing review. This is only visible inside Obsidian via Dataview queries. There is no CLI way to list pending decisions or acknowledge them, which blocks automation and dark-factory workflows from acting on review items.

## Goal

After this work, a user (or autonomous agent) can list all unreviewed decisions across the vault and acknowledge them from the command line. The vault's markdown files are updated in-place with `reviewed: true` and `reviewed_date`, matching what a human would do manually in Obsidian.

## Non-goals

- Will NOT replicate the full Dataview query logic (status-based filtering beyond needs_review)
- Will NOT add batch-ack (one decision at a time)
- Will NOT modify the Obsidian dashboard itself
- Will NOT add notification/webhook on ack

## Desired Behavior

1. `vault-cli decision list` scans the entire vault recursively, finds all markdown files with `needs_review: true` frontmatter, and displays unreviewed ones by default
2. `--reviewed` flag switches to showing only already-reviewed decisions; `--all` shows both
3. Plain output shows `[status] relative/path/from/vault/root` per line; JSON output returns structured array
4. `vault-cli decision ack <name>` finds a decision by name (exact match first, then partial), sets `reviewed: true` and `reviewed_date: <today>`, writes back preserving existing content
5. `--status STATUS` flag on ack optionally overrides the decision's `status` field
6. Both commands support `--vault NAME` for multi-vault setups, defaulting to the configured default vault

## Assumptions

- A "decision" is any markdown file with `needs_review: true` in its YAML frontmatter
- Frontmatter follows standard YAML format between `---` delimiters
- The vault directory tree may contain non-markdown files and subdirectories (both are handled gracefully)
- Date format is ISO `YYYY-MM-DD` (e.g., `2026-03-16`)

## Constraints

- Decision is a new domain type, not an extension of Task — different semantics
- Scans entire vault (no per-directory config) — any file with `needs_review: true` is a decision
- Existing tests must continue to pass — no changes to Task, Goal, or other domain types
- Storage writes must preserve markdown body content (only frontmatter changes on ack)

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| No files with `needs_review: true` found | Empty list, no error | N/A |
| Name matches zero decisions | Error: "decision not found: <name>" | User retries with correct name |
| Name matches multiple decisions (partial) | Error: "ambiguous match: <name> matches N decisions" with list | User provides more specific name |
| Frontmatter parse error on a file | Skip file, log warning, continue scanning | Fix malformed frontmatter |
| Vault path doesn't exist | Skip vault, log warning, continue with remaining vaults | Fix config |
| File write fails (permissions) | Return error, no partial write | Fix permissions |
| Symlink pointing outside vault | Do not follow symlinks outside vault root | Remove symlink |

## Security

- Recursive vault scan must not follow symlinks outside the vault root (path traversal)
- Name matching for ack must not allow path components (e.g., `../../etc/passwd`) — match against known decision names only
- Frontmatter parsing must handle malformed YAML without panicking

## Acceptance Criteria

- [ ] `vault-cli decision list` returns all unreviewed decisions from entire vault
- [ ] `vault-cli decision list --reviewed` returns only reviewed decisions
- [ ] `vault-cli decision list --all` returns all decisions with needs_review
- [ ] `vault-cli decision list --output json` returns valid JSON array
- [ ] `vault-cli decision ack "Some Decision"` sets reviewed=true and reviewed_date=today
- [ ] `vault-cli decision ack "name" --status accepted` also sets status field
- [ ] Ack preserves existing markdown body content unchanged
- [ ] Works across all configured vaults (same as task list)
- [ ] Ambiguous partial name match returns descriptive error
- [ ] All new behaviors have automated tests
- [ ] `make precommit` passes

## Verification

```
cd ~/Documents/workspaces/vault-cli && make precommit
```

Expected: all tests pass, linting clean, no regressions.

Manual smoke test:
```
vault-cli decision list
vault-cli decision list --output json
vault-cli decision ack "Some Page Name"
vault-cli decision list --reviewed
```

## Do-Nothing Option

Without this, decisions can only be reviewed inside Obsidian. Dark-factory agents cannot list or acknowledge review items programmatically. The current manual process works but doesn't scale to automated workflows.

## Implementation Sequence (5 prompts)

1. Domain: `Decision` struct
2. Storage: list, find, write decisions with recursive vault scanning
3. Ops: list operation with filter/sort/output
4. Ops: ack operation with date injection
5. CLI: wire `decision list` and `decision ack` subcommands

## Frozen Constraints

- Decision struct fields: `NeedsReview`, `Reviewed`, `ReviewedDate`, `Status`, `Type`, `PageType`, `Name`, `Content`, `FilePath`
- No config changes — scans entire vault recursively
- Name display: relative path from vault root without `.md` extension
- See [docs/development-patterns.md](docs/development-patterns.md) for project conventions
