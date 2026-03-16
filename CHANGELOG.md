# Changelog

## v0.25.1

- docs: Rewrite README Usage section to document all commands (task, goal, theme, objective, vision, decision, search, config)

## v0.25.0

- feat: Add `vault-cli decision list` and `vault-cli decision ack` CLI commands wired into the multi-vault pattern

## v0.24.0

- feat: Add `DecisionAckOperation` that marks a decision as reviewed with today's date and optionally overrides its status field

## v0.23.0

- feat: Add `DecisionListOperation` with filter modes (unreviewed/reviewed/all), plain and JSON output, alphabetical sorting, and counterfeiter mock

## v0.22.0

- feat: Add `ListDecisions`, `FindDecisionByName`, and `WriteDecision` to `Storage` interface with recursive vault scanning, symlink path-traversal guard, ambiguous-match detection, and in-place frontmatter update

## v0.21.0

- feat: Add `Decision` domain struct with YAML frontmatter fields (`needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type`) and `DecisionID` type

## v0.20.1

- fix: Redirect warning messages from stdout to stderr in storage layer to avoid corrupting JSON output

## v0.20.0

- feat: Add `vault-cli task watch` streaming command that emits newline-delimited JSON events on stdout when task, goal, theme, or objective files change

## v0.19.0

- feat: Add `vault-cli task show <name>` command returning full task detail including content, metadata, and file modification time

## v0.18.0

- feat: Enrich task list JSON output with category, recurring, defer_date, planned_date, claude_session_id, and phase fields for external tool integration

## v0.17.1

- fix: Increase claude session timeout from 60s to 5m for longer-running tasks
- fix: Remove hardcoded `--max-turns 1` limit, allow unlimited turns by default
- feat: Add stderr progress message when starting Claude session

## v0.17.0

- feat: Add optional `claude_script` field to `Vault` config so each vault can specify a custom Claude wrapper script for sessions, defaulting to "claude"

## v0.16.0

- feat: Add Claude session management to `vault-cli task work-on` — starts or resumes a Claude coding session, with `--mode` flag (auto/interactive/headless) for TTY detection

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## v0.15.0

- feat: Add `vault-cli config current-user` subcommand that prints the current user from the config file

## v0.14.0

- feat: Add `vault-cli config list` command to list configured vaults with plain and JSON output formats

## v0.13.0

- feat: Add `--version` flag to `vault-cli` reporting the installed build version (git tag or "dev")

## v0.12.0

- feat: Add `RecurringInterval` type with `ParseRecurringInterval` supporting named aliases (`quarterly`, `yearly`) and numeric shorthand (`3d`, `2w`, `2m`, `1q`, `2y`) for recurring tasks

## v0.11.1

- fix: Change `DeferDate` and `PlannedDate` in Task domain model from `*time.Time` to `*libtime.Date` so YAML serialization produces date-only values (`2026-03-08`) instead of full timestamps

## v0.11.0

- feat: Make date argument optional in `vault-cli task defer`, defaulting to `+1d` when omitted

## v0.10.8

- go mod update

## v0.10.7

- Add recurring task support to complete command (reset checkboxes, bump defer_date, keep in_progress)

## v0.10.6

- Fix frontmatter serialization: exclude Name, Content, FilePath from YAML output via `yaml:"-"` tags

## v0.10.5

- Remove root-level command aliases (complete, defer, list, lint) — use `task` subcommand instead

## v0.10.4

- Add context-aware error wrapping with github.com/bborbe/errors

## v0.10.3

- Improve test coverage for pkg/storage

## v0.10.2

- Improve test coverage for pkg/ops (complete, update operations)

## v0.10.1

- Improve test coverage for pkg/ops (lint, validate operations)

## v0.10.0

- Add `vault-cli task validate <task-name>` command for single-task linting

## v0.9.0

- Add `vault-cli task get <name> <key>` to read frontmatter field values
- Add `vault-cli task set <name> <key> <value>` to write frontmatter field values
- Add `vault-cli task clear <name> <key>` to remove frontmatter field values
- Add Phase and ClaudeSessionID fields to Task domain type

## v0.8.0

- Add `--output plain|json` flag for all commands
- Add JSON output with vault field and warnings in response body

## v0.7.0

- Add `--status` filter flag for all list commands (task, goal, theme, objective, vision)

## v0.6.1

- Improve test coverage for pkg/ops, pkg/config, pkg/domain, pkg/storage

## v0.6.0

- Add `vault-cli task work-on <task-name>` command (sets in_progress + assigns current user)
- Add `current_user` field in config

## v0.5.0

- Add `--assignee` flag for all list commands

## v0.4.0

- Fix priority parsing to handle invalid string values gracefully (use -1 instead of skipping)

## v0.3.0

- Run all commands across all configured vaults by default
- Add `--vault` flag to restrict output to a single vault

## v0.2.0

- Add lint subcommand for goal, theme, objective, and vision entity types

## v0.1.0

- Add `vault-cli list` command with `--status` and `--all` flags
- Add `vault-cli lint` command with `--fix` flag
- Detect MISSING_FRONTMATTER, INVALID_PRIORITY, DUPLICATE_KEY, INVALID_STATUS
- Auto-fix INVALID_PRIORITY and DUPLICATE_KEY issues
