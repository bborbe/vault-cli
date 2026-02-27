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
	storage := storage.NewStorage()
	configLoader := config.NewLoader("")

	var vaultName string
	var configPath string

	rootCmd := &cobra.Command{
		Use:   "vault-cli",
		Short: "Obsidian vault task management CLI",
		Long:  "Fast CRUD operations for Obsidian markdown files (tasks, goals, themes).",
	}

	rootCmd.PersistentFlags().
		StringVar(&vaultName, "vault", "", "Vault name (uses default if not specified)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path")

	// Complete command
	completeCmd := &cobra.Command{
		Use:   "complete <task-name>",
		Short: "Mark a task as complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			vaultPath, err := configLoader.GetVaultPath(ctx, vaultName)
			if err != nil {
				return fmt.Errorf("get vault path: %w", err)
			}

			completeOp := ops.NewCompleteOperation(storage)
			return completeOp.Execute(ctx, vaultPath, taskName)
		},
	}

	// Defer command
	deferCmd := &cobra.Command{
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

			vaultPath, err := configLoader.GetVaultPath(ctx, vaultName)
			if err != nil {
				return fmt.Errorf("get vault path: %w", err)
			}

			deferOp := ops.NewDeferOperation(storage)
			return deferOp.Execute(ctx, vaultPath, taskName, dateStr)
		},
	}

	// Update command
	updateCmd := &cobra.Command{
		Use:   "update <task-name>",
		Short: "Update task progress from checkboxes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			vaultPath, err := configLoader.GetVaultPath(ctx, vaultName)
			if err != nil {
				return fmt.Errorf("get vault path: %w", err)
			}

			updateOp := ops.NewUpdateOperation(storage)
			return updateOp.Execute(ctx, vaultPath, taskName)
		},
	}

	rootCmd.AddCommand(completeCmd)
	rootCmd.AddCommand(deferCmd)
	rootCmd.AddCommand(updateCmd)

	rootCmd.SetArgs(args)
	return rootCmd.ExecuteContext(ctx)
}

// Execute is the main entry point for the CLI.
func Execute() {
	ctx := context.Background()
	if err := Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
