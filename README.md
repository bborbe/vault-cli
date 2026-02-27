# vault-cli

[![Go Reference](https://pkg.go.dev/badge/github.com/bborbe/vault-cli.svg)](https://pkg.go.dev/github.com/bborbe/vault-cli)
[![CI](https://github.com/bborbe/vault-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/bborbe/vault-cli/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bborbe/vault-cli)](https://goreportcard.com/report/github.com/bborbe/vault-cli)

Go CLI tool for managing Obsidian vault tasks, goals, and themes.

## Overview

Fast CRUD operations for Obsidian markdown files (tasks, goals, themes) without spawning full Claude Code sessions. Reduces TaskOrchestrator operation latency from 2-5 seconds to <100ms.

## Status

🚧 **Under Development** - Bootstrapped from go-skeleton, implementation in progress.

## Planned Features

- **Task operations**: complete, defer, update
- **Goal operations**: update, check progress
- **Theme operations**: read, list
- **Multi-vault support** via config file
- **Markdown storage** with Obsidian-compatible YAML frontmatter

## Installation

```bash
go install github.com/bborbe/vault-cli@latest
```

## Usage

```bash
# Complete a task
vault complete "Build vault-cli Go Tool"

# Defer a task
vault defer "Migrate TaskOrchestrator" +7d

# Update task progress
vault update "Build vault-cli Go Tool"
```

## Development

```bash
make test      # Run tests
make check     # Linting and checks
make precommit # Full development workflow
```

## License

BSD-2-Clause
