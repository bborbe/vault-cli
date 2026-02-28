// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// Run executes the CLI application.
func Run(ctx context.Context, args []string) error {
	var configLoader config.Loader
	var vaultName string
	var configPath string

	rootCmd := &cobra.Command{
		Use:          "vault-cli",
		Short:        "Obsidian vault task management CLI",
		Long:         "Fast CRUD operations for Obsidian markdown files (tasks, goals, themes).",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			configLoader = config.NewLoader(configPath)
			return nil
		},
	}

	rootCmd.PersistentFlags().
		StringVar(&vaultName, "vault", "", "Vault name (uses default if not specified)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path")

	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks in the vault",
	}

	taskCmd.AddCommand(createListCommand(ctx, &configLoader, &vaultName))
	taskCmd.AddCommand(createLintCommand(ctx, &configLoader, &vaultName))
	taskCmd.AddCommand(createCompleteCommand(ctx, &configLoader, &vaultName))
	taskCmd.AddCommand(createDeferCommand(ctx, &configLoader, &vaultName))
	taskCmd.AddCommand(createUpdateCommand(ctx, &configLoader, &vaultName))

	rootCmd.AddCommand(taskCmd)

	rootCmd.SetArgs(args)
	return rootCmd.ExecuteContext(ctx)
}

func createCompleteCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "complete <task-name>",
		Short: "Mark a task as complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			vault, err := (*configLoader).GetVault(ctx, *vaultName)
			if err != nil {
				return fmt.Errorf("get vault: %w", err)
			}

			storageConfig := storage.NewConfigFromVault(vault)
			store := storage.NewStorage(storageConfig)
			completeOp := ops.NewCompleteOperation(store)
			return completeOp.Execute(ctx, vault.Path, taskName)
		},
	}
}

func createDeferCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "defer <task-name> <date>",
		Short: "Defer a task to a specific date",
		Long: `Defer a task to a specific date.

Date formats:
  +Nd         - Relative days (e.g., +7d for 7 days from now)
  monday      - Next occurrence of weekday
  2024-12-31  - ISO date format (YYYY-MM-DD)`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			dateStr := args[1]

			vault, err := (*configLoader).GetVault(ctx, *vaultName)
			if err != nil {
				return fmt.Errorf("get vault: %w", err)
			}

			storageConfig := storage.NewConfigFromVault(vault)
			store := storage.NewStorage(storageConfig)
			deferOp := ops.NewDeferOperation(store)
			return deferOp.Execute(ctx, vault.Path, taskName, dateStr)
		},
	}
}

func createUpdateCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "update <task-name>",
		Short: "Update task progress from checkboxes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			vault, err := (*configLoader).GetVault(ctx, *vaultName)
			if err != nil {
				return fmt.Errorf("get vault: %w", err)
			}

			storageConfig := storage.NewConfigFromVault(vault)
			store := storage.NewStorage(storageConfig)
			updateOp := ops.NewUpdateOperation(store)
			return updateOp.Execute(ctx, vault.Path, taskName)
		},
	}
}

func createListCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) *cobra.Command {
	var statusFlag []string
	var showAll bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks from the vault",
		Long: `List tasks from the vault, optionally filtered by status.

By default, shows only tasks with status "todo" or "in_progress".
Use --status to filter by specific statuses, or --all to show all tasks.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vault, err := (*configLoader).GetVault(ctx, *vaultName)
			if err != nil {
				return fmt.Errorf("get vault: %w", err)
			}

			storageConfig := storage.NewConfigFromVault(vault)
			store := storage.NewStorage(storageConfig)
			listOp := ops.NewListOperation(store)

			// Parse status filter
			var statusFilter []domain.TaskStatus
			for _, s := range statusFlag {
				statusFilter = append(statusFilter, domain.TaskStatus(s))
			}

			return listOp.Execute(ctx, vault.Path, statusFilter, showAll)
		},
	}

	cmd.Flags().StringSliceVar(&statusFlag, "status", nil,
		"Filter by status (can be repeated): todo, in_progress, done, deferred")
	cmd.Flags().BoolVar(&showAll, "all", false,
		"Show all tasks regardless of status")

	return cmd
}

func createLintCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Detect and optionally fix frontmatter issues in task files",
		Long: `Detect and optionally fix common frontmatter issues in task files.

Issues detected:
  - MISSING_FRONTMATTER: file has no frontmatter block
  - INVALID_PRIORITY: priority field is string instead of int
  - DUPLICATE_KEY: duplicate YAML key in frontmatter
  - INVALID_STATUS: status value not in allowed set

Use --fix to automatically fix INVALID_PRIORITY and DUPLICATE_KEY issues.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vault, err := (*configLoader).GetVault(ctx, *vaultName)
			if err != nil {
				return fmt.Errorf("get vault: %w", err)
			}

			storageConfig := storage.NewConfigFromVault(vault)
			lintOp := ops.NewLintOperation()
			return lintOp.Execute(ctx, vault.Path, storageConfig.TasksDir, fix)
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Automatically fix fixable issues")

	return cmd
}

// Execute is the main entry point for the CLI.
func Execute() {
	ctx := context.Background()
	if err := Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
