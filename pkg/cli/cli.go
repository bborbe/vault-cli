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
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// Run executes the CLI application.
func Run(ctx context.Context, args []string) error {
	var configLoader config.Loader
	var vaultName string
	var configPath string

	rootCmd := &cobra.Command{
		Use:   "vault-cli",
		Short: "Obsidian vault task management CLI",
		Long:  "Fast CRUD operations for Obsidian markdown files (tasks, goals, themes).",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			configLoader = config.NewLoader(configPath)
			return nil
		},
	}

	rootCmd.PersistentFlags().
		StringVar(&vaultName, "vault", "", "Vault name (uses default if not specified)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path")

	rootCmd.AddCommand(createCompleteCommand(ctx, &configLoader, &vaultName))
	rootCmd.AddCommand(createDeferCommand(ctx, &configLoader, &vaultName))
	rootCmd.AddCommand(createUpdateCommand(ctx, &configLoader, &vaultName))

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

// Execute is the main entry point for the CLI.
func Execute() {
	ctx := context.Background()
	if err := Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
