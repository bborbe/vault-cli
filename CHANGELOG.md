# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
