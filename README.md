# vault-cli

[![Go Reference](https://pkg.go.dev/badge/github.com/bborbe/vault-cli.svg)](https://pkg.go.dev/github.com/bborbe/vault-cli)
[![CI](https://github.com/bborbe/vault-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/bborbe/vault-cli/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bborbe/vault-cli)](https://goreportcard.com/report/github.com/bborbe/vault-cli)

Go CLI tool for managing Obsidian vault tasks, goals, themes, objectives, visions, and decisions.

## Overview

Fast CRUD operations for Obsidian markdown files (tasks, goals, themes, objectives, visions, decisions) without spawning full Claude Code sessions.

## Installation

```bash
go install github.com/bborbe/vault-cli@latest
```

## Configuration

Create `~/.vault-cli/config.yaml`:

```yaml
default_vault: personal
vaults:
  personal:
    name: personal
    path: ~/Documents/Obsidian/Personal
    tasks_dir: "24 Tasks"
    goals_dir: "23 Goals"
    daily_dir: "60 Periodic Notes/Daily"
    excludes:
      - "90 Templates"
      - ".obsidian"
  brogrammers:
    name: brogrammers
    path: ~/Documents/Obsidian/Brogrammers
    tasks_dir: "40 Tasks"
    daily_dir: "60 Periodic Notes/Daily"
```

## Usage

### Global Flags

```bash
--vault <name>       # Use a specific vault (default vault if omitted)
--output plain|json  # Output format (default: plain)
--config <path>      # Custom config file path
```

### task

```bash
vault-cli task list                                # List active tasks (todo + in_progress)
vault-cli task list --status deferred              # Filter by status
vault-cli task list --all                          # Show all tasks
vault-cli task list --assignee alice               # Filter by assignee

vault-cli task show "Build vault-cli Go Tool"      # Show full task detail
vault-cli task complete "Build vault-cli Go Tool"  # Mark task as complete
vault-cli task defer "Migrate TaskOrchestrator" +7d     # Defer by relative days
vault-cli task defer "Migrate TaskOrchestrator" monday  # Defer to next weekday
vault-cli task defer "Migrate TaskOrchestrator" 2026-03-01  # Defer to ISO date
vault-cli task update "Build vault-cli Go Tool"    # Update progress from checkboxes
vault-cli task work-on "Build vault-cli Go Tool"   # Mark in_progress and assign to current user

vault-cli task get "Build vault-cli Go Tool" status         # Get a frontmatter field
vault-cli task set "Build vault-cli Go Tool" status done    # Set a frontmatter field
vault-cli task clear "Build vault-cli Go Tool" assignee     # Clear a frontmatter field

vault-cli task lint                                # Detect frontmatter issues
vault-cli task lint --fix                          # Auto-fix frontmatter issues
vault-cli task validate "Build vault-cli Go Tool"  # Validate a single task

vault-cli task watch                               # Stream file-change events as JSON
vault-cli task search "improve CLI performance"    # Semantic search in tasks
```

### goal

```bash
vault-cli goal list                        # List goals
vault-cli goal lint                        # Detect frontmatter issues
vault-cli goal search "team productivity"  # Semantic search in goals
```

### theme

```bash
vault-cli theme list                       # List themes
vault-cli theme lint                       # Detect frontmatter issues
vault-cli theme search "engineering culture"  # Semantic search in themes
```

### objective

```bash
vault-cli objective list                   # List objectives
vault-cli objective lint                   # Detect frontmatter issues
vault-cli objective search "Q2 goals"      # Semantic search in objectives
```

### vision

```bash
vault-cli vision list                      # List vision items
vault-cli vision lint                      # Detect frontmatter issues
vault-cli vision search "long-term growth" # Semantic search in vision
```

### decision

```bash
vault-cli decision list                    # List decisions pending review
vault-cli decision list --reviewed         # Show only reviewed decisions
vault-cli decision list --all              # Show all decisions
vault-cli decision ack "Use PostgreSQL"    # Acknowledge (mark as reviewed)
vault-cli decision ack "Use PostgreSQL" --status accepted  # Ack with status override
```

### search

```bash
vault-cli search "improve performance"          # Search entire vault semantically
vault-cli search "improve performance" --top-k 10  # Return more results
```

### config

```bash
vault-cli config list          # List configured vaults
vault-cli config current-user  # Print the current user
```

## Claude Code Plugin

vault-cli includes a Claude Code plugin for task management commands.

```bash
claude plugin marketplace add bborbe/vault-cli
claude plugin install vault-cli
```

| Command | Description |
|---------|-------------|
| `/vault-cli:complete-task` | Mark task as complete (normal or recurring) |
| `/vault-cli:defer-task` | Defer task to specific date |

## Shell Completion

```bash
# Zsh
source <(vault-cli completion zsh)

# Bash
source <(vault-cli completion bash)

# Fish
vault-cli completion fish | source
```

## Development

```bash
make test      # Run tests
make check     # Linting and checks
make precommit # Full development workflow
```

## License

BSD-2-Clause
