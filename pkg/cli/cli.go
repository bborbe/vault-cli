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

// getVaults returns the vaults to operate on.
// If vaultName is set, returns just that vault.
// If vaultName is empty, returns all configured vaults.
func getVaults(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) ([]*config.Vault, error) {
	if *vaultName != "" {
		vault, err := (*configLoader).GetVault(ctx, *vaultName)
		if err != nil {
			return nil, err
		}
		return []*config.Vault{vault}, nil
	}
	return (*configLoader).GetAllVaults(ctx)
}

// Run executes the CLI application.
func Run(ctx context.Context, args []string) error {
	var configLoader config.Loader
	var vaultName string
	var configPath string
	var outputFormat string

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
	rootCmd.PersistentFlags().
		StringVar(&outputFormat, "output", OutputFormatPlain, "Output format: plain or json")

	// Add root-level search command
	rootCmd.AddCommand(createSearchCommand(ctx, &configLoader, &vaultName, &outputFormat))

	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks in the vault",
	}

	taskCmd.AddCommand(createTaskListCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createLintCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createCompleteCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createDeferCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createUpdateCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createWorkOnCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createTaskGetCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createTaskSetCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createTaskClearCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(
		createGenericSearchCommand(
			ctx,
			&configLoader,
			&vaultName,
			"tasks",
			func(c *storage.Config) string { return c.TasksDir },
			&outputFormat,
		),
	)

	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(createGoalCommands(ctx, &configLoader, &vaultName, &outputFormat))
	rootCmd.AddCommand(createThemeCommands(ctx, &configLoader, &vaultName, &outputFormat))
	rootCmd.AddCommand(createObjectiveCommands(ctx, &configLoader, &vaultName, &outputFormat))
	rootCmd.AddCommand(createVisionCommands(ctx, &configLoader, &vaultName, &outputFormat))

	rootCmd.SetArgs(args)
	return rootCmd.ExecuteContext(ctx)
}

//nolint:dupl // Mutation commands have similar structure but different operations
func createCompleteCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "complete <task-name>",
		Short: "Mark a task as complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				completeOp := ops.NewCompleteOperation(store)
				return completeOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat)
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				completeOp := ops.NewCompleteOperation(store)
				if err := completeOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat); err == nil {
					return nil
				}
				lastErr = err
			}

			// Not found in any vault
			return fmt.Errorf("task not found in any vault: %w", lastErr)
		},
	}
}

//nolint:dupl // Mutation commands have similar structure but different operations
func createDeferCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
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

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				deferOp := ops.NewDeferOperation(store)
				return deferOp.Execute(
					ctx,
					vault.Path,
					taskName,
					dateStr,
					vault.Name,
					*outputFormat,
				)
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				deferOp := ops.NewDeferOperation(store)
				if err := deferOp.Execute(ctx, vault.Path, taskName, dateStr, vault.Name, *outputFormat); err == nil {
					return nil
				}
				lastErr = err
			}

			// Not found in any vault
			return fmt.Errorf("task not found in any vault: %w", lastErr)
		},
	}
}

//nolint:dupl // Mutation commands have similar structure but different operations
func createUpdateCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "update <task-name>",
		Short: "Update task progress from checkboxes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				updateOp := ops.NewUpdateOperation(store)
				return updateOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat)
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				updateOp := ops.NewUpdateOperation(store)
				if err := updateOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat); err == nil {
					return nil
				}
				lastErr = err
			}

			// Not found in any vault
			return fmt.Errorf("task not found in any vault: %w", lastErr)
		},
	}
}

//nolint:dupl // Mutation commands have similar structure but different operations
func createWorkOnCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "work-on <task-name>",
		Short: "Mark a task as in_progress and assign it to current user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]

			// Get current user from config
			currentUser, err := (*configLoader).GetCurrentUser(ctx)
			if err != nil {
				return fmt.Errorf("get current user: %w", err)
			}

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				workOnOp := ops.NewWorkOnOperation(store)
				return workOnOp.Execute(
					ctx,
					vault.Path,
					taskName,
					currentUser,
					vault.Name,
					*outputFormat,
				)
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				workOnOp := ops.NewWorkOnOperation(store)
				if err := workOnOp.Execute(ctx, vault.Path, taskName, currentUser, vault.Name, *outputFormat); err == nil {
					return nil
				}
				lastErr = err
			}

			// Not found in any vault
			return fmt.Errorf("task not found in any vault: %w", lastErr)
		},
	}
}

func createTaskListCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	var statusFilter string
	var showAll bool
	var assigneeFlag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks from the vault",
		Long: `List tasks from the vault, optionally filtered by status and assignee.

By default, shows only tasks with status "todo" or "in_progress".
Use --status to filter by specific status, or --all to show all tasks.
Use --assignee to filter by assignee.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			for _, vault := range vaults {
				if len(vaults) > 1 && *outputFormat == OutputFormatPlain {
					fmt.Printf("=== %s ===\n", vault.Name)
				}

				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				listOp := ops.NewListOperation(store)

				if err := listOp.Execute(ctx, vault.Path, vault.Name, storageConfig.TasksDir, statusFilter, showAll, assigneeFlag, *outputFormat); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&statusFilter, "status", "",
		"Filter by status (e.g. todo, in_progress, completed, done, deferred)")
	cmd.Flags().BoolVar(&showAll, "all", false,
		"Show all tasks regardless of status")
	cmd.Flags().StringVar(&assigneeFlag, "assignee", "", "Filter by assignee")

	return cmd
}

func createLintCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return createGenericLintCommand(
		ctx,
		configLoader,
		vaultName,
		"task",
		func(c *storage.Config) string { return c.TasksDir },
		outputFormat,
	)
}

// createGenericLintCommand creates a lint command for any page type.
func createGenericLintCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	pageType string,
	getDirFunc func(*storage.Config) string,
	outputFormat *string,
) *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "lint",
		Short: fmt.Sprintf("Detect and optionally fix frontmatter issues in %s files", pageType),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			for _, vault := range vaults {
				if len(vaults) > 1 && *outputFormat == OutputFormatPlain {
					fmt.Printf("=== %s ===\n", vault.Name)
				}

				storageConfig := storage.NewConfigFromVault(vault)
				lintOp := ops.NewLintOperation()
				if err := lintOp.Execute(ctx, vault.Path, getDirFunc(storageConfig), fix, *outputFormat); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Automatically fix fixable issues")

	return cmd
}

// createGenericListCommand creates a list command for any page type.
func createGenericListCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	pageType string,
	getDirFunc func(*storage.Config) string,
	outputFormat *string,
) *cobra.Command {
	var statusFilter string
	var showAll bool
	var assigneeFlag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List %s from the vault", pageType),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			for _, vault := range vaults {
				if len(vaults) > 1 && *outputFormat == OutputFormatPlain {
					fmt.Printf("=== %s ===\n", vault.Name)
				}

				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				listOp := ops.NewListOperation(store)

				if err := listOp.Execute(ctx, vault.Path, vault.Name, getDirFunc(storageConfig), statusFilter, showAll, assigneeFlag, *outputFormat); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(
		&statusFilter,
		"status",
		"",
		"Filter by status (e.g. todo, in_progress, completed, done, deferred)",
	)
	cmd.Flags().BoolVar(
		&showAll,
		"all",
		false,
		fmt.Sprintf("Show all %s regardless of status", pageType),
	)
	cmd.Flags().StringVar(
		&assigneeFlag,
		"assignee",
		"",
		"Filter by assignee",
	)

	return cmd
}

func createGoalCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goal",
		Short: "Manage goals in the vault",
	}
	cmd.AddCommand(
		createGenericListCommand(
			ctx,
			configLoader,
			vaultName,
			"goals",
			func(c *storage.Config) string { return c.GoalsDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericLintCommand(
			ctx,
			configLoader,
			vaultName,
			"goal",
			func(c *storage.Config) string { return c.GoalsDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericSearchCommand(
			ctx,
			configLoader,
			vaultName,
			"goals",
			func(c *storage.Config) string { return c.GoalsDir },
			outputFormat,
		),
	)
	return cmd
}

func createThemeCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "theme",
		Short: "Manage themes in the vault",
	}
	cmd.AddCommand(
		createGenericListCommand(
			ctx,
			configLoader,
			vaultName,
			"themes",
			func(c *storage.Config) string { return c.ThemesDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericLintCommand(
			ctx,
			configLoader,
			vaultName,
			"theme",
			func(c *storage.Config) string { return c.ThemesDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericSearchCommand(
			ctx,
			configLoader,
			vaultName,
			"themes",
			func(c *storage.Config) string { return c.ThemesDir },
			outputFormat,
		),
	)
	return cmd
}

func createObjectiveCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "objective",
		Short: "Manage objectives in the vault",
	}
	cmd.AddCommand(
		createGenericListCommand(
			ctx,
			configLoader,
			vaultName,
			"objectives",
			func(c *storage.Config) string { return c.ObjectivesDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericLintCommand(
			ctx,
			configLoader,
			vaultName,
			"objective",
			func(c *storage.Config) string { return c.ObjectivesDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericSearchCommand(
			ctx,
			configLoader,
			vaultName,
			"objectives",
			func(c *storage.Config) string { return c.ObjectivesDir },
			outputFormat,
		),
	)
	return cmd
}

func createVisionCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vision",
		Short: "Manage vision in the vault",
	}
	cmd.AddCommand(
		createGenericListCommand(
			ctx,
			configLoader,
			vaultName,
			"vision items",
			func(c *storage.Config) string { return c.VisionDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericLintCommand(
			ctx,
			configLoader,
			vaultName,
			"vision",
			func(c *storage.Config) string { return c.VisionDir },
			outputFormat,
		),
	)
	cmd.AddCommand(
		createGenericSearchCommand(
			ctx,
			configLoader,
			vaultName,
			"vision items",
			func(c *storage.Config) string { return c.VisionDir },
			outputFormat,
		),
	)
	return cmd
}

func createSearchCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	var topK int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the entire vault using semantic search",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			for _, vault := range vaults {
				if len(vaults) > 1 && *outputFormat == OutputFormatPlain {
					fmt.Printf("=== %s ===\n", vault.Name)
				}

				searchOp := ops.NewSearchOperation()
				if err := searchOp.Execute(ctx, vault.Path, "", query, topK, *outputFormat); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&topK, "top-k", 5, "Maximum number of results to return")

	return cmd
}

// createGenericSearchCommand creates a search command for any page type.
func createGenericSearchCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	pageType string,
	getDirFunc func(*storage.Config) string,
	outputFormat *string,
) *cobra.Command {
	var topK int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: fmt.Sprintf("Search %s using semantic search", pageType),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			for _, vault := range vaults {
				if len(vaults) > 1 && *outputFormat == OutputFormatPlain {
					fmt.Printf("=== %s ===\n", vault.Name)
				}

				storageConfig := storage.NewConfigFromVault(vault)
				scopeDir := getDirFunc(storageConfig)

				searchOp := ops.NewSearchOperation()
				if err := searchOp.Execute(ctx, vault.Path, scopeDir, query, topK, *outputFormat); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&topK, "top-k", 5, "Maximum number of results to return")

	return cmd
}

//nolint:dupl,gocognit,nestif // Mutation commands have similar structure but different operations
func createTaskGetCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "get <task-name> <key>",
		Short: "Get a frontmatter field value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			key := args[1]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				getOp := ops.NewFrontmatterGetOperation(store)
				value, err := getOp.Execute(ctx, vault.Path, taskName, key)
				if err != nil {
					if *outputFormat == OutputFormatJSON {
						result := map[string]any{
							"success": false,
							"error":   err.Error(),
						}
						return PrintJSON(result)
					}
					return err
				}

				if *outputFormat == OutputFormatJSON {
					result := map[string]any{
						"key":   key,
						"value": value,
						"name":  taskName,
					}
					return PrintJSON(result)
				}

				fmt.Println(value)
				return nil
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				getOp := ops.NewFrontmatterGetOperation(store)
				value, err := getOp.Execute(ctx, vault.Path, taskName, key)
				if err == nil {
					if *outputFormat == OutputFormatJSON {
						result := map[string]any{
							"key":   key,
							"value": value,
							"name":  taskName,
						}
						return PrintJSON(result)
					}
					fmt.Println(value)
					return nil
				}
				lastErr = err
			}

			// Not found in any vault
			if *outputFormat == OutputFormatJSON {
				result := map[string]any{
					"success": false,
					"error":   lastErr.Error(),
				}
				return PrintJSON(result)
			}
			return fmt.Errorf("task not found in any vault: %w", lastErr)
		},
	}
}

//nolint:dupl,gocognit,nestif // Mutation commands have similar structure but different operations
func createTaskSetCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "set <task-name> <key> <value>",
		Short: "Set a frontmatter field value",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			key := args[1]
			value := args[2]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				setOp := ops.NewFrontmatterSetOperation(store)
				if err := setOp.Execute(ctx, vault.Path, taskName, key, value); err != nil {
					if *outputFormat == OutputFormatJSON {
						result := map[string]any{
							"success": false,
							"error":   err.Error(),
						}
						return PrintJSON(result)
					}
					return err
				}

				if *outputFormat == OutputFormatJSON {
					result := map[string]any{
						"success": true,
						"key":     key,
						"value":   value,
						"name":    taskName,
					}
					return PrintJSON(result)
				}

				fmt.Printf("✅ Set %s=%s on: %s\n", key, value, taskName)
				return nil
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				setOp := ops.NewFrontmatterSetOperation(store)
				if err := setOp.Execute(ctx, vault.Path, taskName, key, value); err == nil {
					if *outputFormat == OutputFormatJSON {
						result := map[string]any{
							"success": true,
							"key":     key,
							"value":   value,
							"name":    taskName,
						}
						return PrintJSON(result)
					}
					fmt.Printf("✅ Set %s=%s on: %s\n", key, value, taskName)
					return nil
				}
				lastErr = err
			}

			// Not found in any vault
			if *outputFormat == OutputFormatJSON {
				result := map[string]any{
					"success": false,
					"error":   lastErr.Error(),
				}
				return PrintJSON(result)
			}
			return fmt.Errorf("task not found in any vault: %w", lastErr)
		},
	}
}

//nolint:dupl,gocognit,nestif // Mutation commands have similar structure but different operations
func createTaskClearCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <task-name> <key>",
		Short: "Clear a frontmatter field value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			key := args[1]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				clearOp := ops.NewFrontmatterClearOperation(store)
				if err := clearOp.Execute(ctx, vault.Path, taskName, key); err != nil {
					if *outputFormat == OutputFormatJSON {
						result := map[string]any{
							"success": false,
							"error":   err.Error(),
						}
						return PrintJSON(result)
					}
					return err
				}

				if *outputFormat == OutputFormatJSON {
					result := map[string]any{
						"success": true,
						"key":     key,
						"name":    taskName,
					}
					return PrintJSON(result)
				}

				fmt.Printf("✅ Cleared %s on: %s\n", key, taskName)
				return nil
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				store := storage.NewStorage(storageConfig)
				clearOp := ops.NewFrontmatterClearOperation(store)
				if err := clearOp.Execute(ctx, vault.Path, taskName, key); err == nil {
					if *outputFormat == OutputFormatJSON {
						result := map[string]any{
							"success": true,
							"key":     key,
							"name":    taskName,
						}
						return PrintJSON(result)
					}
					fmt.Printf("✅ Cleared %s on: %s\n", key, taskName)
					return nil
				}
				lastErr = err
			}

			// Not found in any vault
			if *outputFormat == OutputFormatJSON {
				result := map[string]any{
					"success": false,
					"error":   lastErr.Error(),
				}
				return PrintJSON(result)
			}
			return fmt.Errorf("task not found in any vault: %w", lastErr)
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
