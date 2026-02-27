# vault-cli

[![Go Reference](https://pkg.go.dev/badge/github.com/bborbe/vault-cli.svg)](https://pkg.go.dev/github.com/bborbe/vault-cli)
[![CI](https://github.com/bborbe/vault-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/bborbe/vault-cli/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bborbe/vault-cli)](https://goreportcard.com/report/github.com/bborbe/vault-cli)

Go CLI tool for managing Obsidian vault tasks, goals, and themes.

## Overview

Fast CRUD operations for Obsidian markdown files (tasks, goals, themes) without spawning full Claude Code sessions. Reduces TaskOrchestrator operation latency from 2-5 seconds to <100ms.

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
  brogrammers:
    name: brogrammers
    path: ~/Documents/Obsidian/Brogrammers
    tasks_dir: "40 Tasks"
    daily_dir: "60 Periodic Notes/Daily"
```

## Usage

```bash
# Complete a task
vault-cli complete "Build vault-cli Go Tool"

# Complete a task in a specific vault
vault-cli --vault brogrammers complete "Some Task"

# Defer a task
vault-cli defer "Migrate TaskOrchestrator" +7d
vault-cli defer "Migrate TaskOrchestrator" monday
vault-cli defer "Migrate TaskOrchestrator" 2026-03-01

# Update task progress from checkboxes
vault-cli update "Build vault-cli Go Tool"

# Use custom config file
vault-cli --config /path/to/config.yaml complete "Some Task"
```

## Development

```bash
make test      # Run tests
make check     # Linting and checks
make precommit # Full development workflow
```

## License

BSD-2-Clause
