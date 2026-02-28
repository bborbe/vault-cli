# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## v0.7.0

### Added
- `--status` filter flag for all list commands (task, goal, theme, objective, vision)
- Case-insensitive status matching (e.g. `--status in_progress`)
- Works in combination with existing `--vault` and `--assignee` filters

## v0.6.1

### Added
- Test coverage for pkg/ops, pkg/config, pkg/domain, pkg/storage
- Tests for workon operation, priority parsing, config loading, and storage round-trips

## v0.6.0

### Added
- `vault-cli task work-on <task-name>` command: sets status to in_progress and assigns to current user
- `current_user` field in config (~/.vault-cli/config.yaml) to identify the active user

## v0.5.0

### Added
- `--assignee` flag for all list commands (task, goal, theme, objective, vision)
- Filter tasks/goals by assignee frontmatter field

## v0.4.0

### Changed
- Priority field parsing is now resilient: invalid string values (e.g. "medium", "high") use -1 instead of skipping the file
- Eliminated INVALID_PRIORITY warnings during list operations

## v0.3.0

### Changed
- All commands (list, lint, search) now run across all configured vaults by default
- `--vault` flag becomes a filter to restrict output to a single vault
- Output prefixed with `=== vault-name ===` header when multiple vaults are shown

## v0.2.0

### Added
- `lint` subcommand for goal, theme, objective, and vision entity types
- Generic lint command supporting all vault entity types with `--fix` flag

## v0.1.0

### Added
- Initial project structure from go-skeleton
- Go module github.com/bborbe/vault-cli
- `vault-cli list` command to list tasks from vault
- Support for filtering tasks by status with `--status` flag
- Support for showing all tasks with `--all` flag
- Default behavior shows only todo and in_progress tasks
- `vault-cli lint` command to detect and fix common frontmatter issues in task files
  - Detects: MISSING_FRONTMATTER, INVALID_PRIORITY, DUPLICATE_KEY, INVALID_STATUS
  - Auto-fixes: INVALID_PRIORITY (string to int conversion), DUPLICATE_KEY (keeps first occurrence)
  - `--fix` flag to automatically fix fixable issues
- Fix INVALID_STATUS allowed values to match vault schema (todo, in_progress, backlog, completed, hold, aborted)
