// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var version = "dev"

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
	var verbose bool

	rootCmd := &cobra.Command{
		Use:          "vault-cli",
		Short:        "Obsidian vault task management CLI",
		Long:         "Fast CRUD operations for Obsidian markdown files (tasks, goals, themes).",
		Version:      version,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level := slog.LevelWarn
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(
				slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})),
			)
			configLoader = config.NewLoader(configPath)
			return nil
		},
	}

	rootCmd.PersistentFlags().
		StringVar(&vaultName, "vault", "", "Vault name (uses default if not specified)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path")
	rootCmd.PersistentFlags().
		StringVar(&outputFormat, "output", OutputFormatPlain, "Output format: plain or json")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")

	// Add root-level search command
	rootCmd.AddCommand(createSearchCommand(ctx, &configLoader, &vaultName, &outputFormat))

	rootCmd.AddCommand(createTaskCommands(ctx, &configLoader, &vaultName, &outputFormat))

	rootCmd.AddCommand(createGoalCommands(ctx, &configLoader, &vaultName, &outputFormat))
	rootCmd.AddCommand(createThemeCommands(ctx, &configLoader, &vaultName, &outputFormat))
	rootCmd.AddCommand(createObjectiveCommands(ctx, &configLoader, &vaultName, &outputFormat))
	rootCmd.AddCommand(createVisionCommands(ctx, &configLoader, &vaultName, &outputFormat))
	rootCmd.AddCommand(createDecisionCommands(ctx, &configLoader, &vaultName, &outputFormat))

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
	}
	configCmd.AddCommand(createConfigListCommand(ctx, &configLoader, &vaultName, &outputFormat))
	configCmd.AddCommand(createConfigCurrentUserCommand(ctx, &configLoader))
	rootCmd.AddCommand(configCmd)

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
				return errors.Wrap(ctx, err, "get vaults")
			}

			currentDateTime := libtime.NewCurrentDateTime()

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				goalStore := storage.NewGoalStorage(storageConfig)
				dailyStore := storage.NewDailyNoteStorage(storageConfig)
				completeOp := ops.NewCompleteOperation(
					taskStore,
					goalStore,
					dailyStore,
					currentDateTime,
				)
				return completeOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat)
			})
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
		Use:   "defer <task-name> [date]",
		Short: "Defer a task to a specific date",
		Long: `Defer a task to a specific date.

If no date is provided, defaults to +1d (tomorrow).

Date formats:
  +Nd                        - Relative days (e.g., +7d for 7 days from now)
  monday                     - Next occurrence of weekday
  2024-12-31                 - ISO date format (YYYY-MM-DD)
  2026-03-19T16:00:00+01:00  - Full datetime with timezone`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			dateStr := "+1d"
			if len(args) > 1 {
				dateStr = args[1]
			}

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			currentDateTime := libtime.NewCurrentDateTime()

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				dailyStore := storage.NewDailyNoteStorage(storageConfig)
				deferOp := ops.NewDeferOperation(taskStore, dailyStore, currentDateTime)
				return deferOp.Execute(
					ctx,
					vault.Path,
					taskName,
					dateStr,
					vault.Name,
					*outputFormat,
				)
			})
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
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				goalStore := storage.NewGoalStorage(storageConfig)
				updateOp := ops.NewUpdateOperation(taskStore, goalStore)
				return updateOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat)
			})
		},
	}
}

func createWorkOnCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	var mode string

	cmd := &cobra.Command{
		Use:   "work-on <task-name>",
		Short: "Mark a task as in_progress and assign it to current user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]

			isInteractive, err := resolveSessionMode(mode)
			if err != nil {
				return err
			}

			currentUser, err := (*configLoader).GetCurrentUser(ctx)
			if err != nil {
				return errors.Wrap(ctx, err, "get current user")
			}

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			currentDateTime := libtime.NewCurrentDateTime()

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				starter := ops.NewClaudeSessionStarter(vault.GetClaudeScript())
				resumer := ops.NewClaudeResumer(vault.GetClaudeScript())
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				dailyStore := storage.NewDailyNoteStorage(storageConfig)
				workOnOp := ops.NewWorkOnOperation(
					taskStore,
					dailyStore,
					currentDateTime,
					starter,
					resumer,
				)
				return workOnOp.Execute(
					ctx,
					vault.Path,
					taskName,
					currentUser,
					vault.Name,
					*outputFormat,
					isInteractive,
				)
			})
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "auto", "Session mode: auto, interactive, or headless")
	return cmd
}

// resolveSessionMode converts a mode string to an isInteractive bool.
func resolveSessionMode(mode string) (bool, error) {
	switch mode {
	case "interactive":
		return true, nil
	case "headless":
		return false, nil
	case "auto":
		fd := int(os.Stdin.Fd()) //#nosec G115 -- fd value fits in int on all supported platforms
		return term.IsTerminal(fd), nil
	default:
		return false, fmt.Errorf(
			"invalid --mode value: %s (must be auto, interactive, or headless)",
			mode,
		)
	}
}

func createTaskListCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	var statusFilters []string
	var showAll bool
	var assigneeFlag string
	var goalFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks from the vault",
		Long: `List tasks from the vault, optionally filtered by status, assignee, and goal.

By default, shows only tasks with status "todo" or "in_progress".
Use --status to filter by specific status, or --all to show all tasks.
Use --assignee to filter by assignee.
Use --goal to filter by goal name.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			for _, vault := range vaults {
				if len(vaults) > 1 && *outputFormat == OutputFormatPlain {
					fmt.Printf("=== %s ===\n", vault.Name)
				}

				storageConfig := storage.NewConfigFromVault(vault)
				pageStore := storage.NewPageStorage(storageConfig)
				listOp := ops.NewListOperation(pageStore)

				if err := listOp.Execute(ctx, vault.Path, vault.Name, storageConfig.TasksDir, statusFilters, showAll, assigneeFlag, goalFilter, *outputFormat); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&statusFilters, "status", nil,
		"Filter by status (e.g. --status=in_progress --status=completed). Cobra StringSliceVar natively supports both repeated flags and comma-separated values.")
	cmd.Flags().BoolVar(&showAll, "all", false,
		"Show all tasks regardless of status")
	cmd.Flags().StringVar(&assigneeFlag, "assignee", "", "Filter by assignee")
	cmd.Flags().StringVar(&goalFilter, "goal", "", "Filter by goal name (exact match)")

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

func createValidateCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <task-name>",
		Short: "Validate a single task by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			// Track if task was found in any vault
			var foundInVault *config.Vault
			var taskFilePath string

			// Search for the task across vaults
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)

				task, err := taskStore.FindTaskByName(ctx, vault.Path, taskName)
				if err == nil {
					foundInVault = vault
					taskFilePath = task.FilePath
					break
				}
			}

			// Task not found in any vault
			if foundInVault == nil {
				if *outputFormat == OutputFormatJSON {
					result := map[string]any{
						"success": false,
						"error":   "task not found",
					}
					return PrintJSON(result)
				}
				return errors.Errorf(ctx, "task not found: %s", taskName)
			}

			// Validate the task file
			lintOp := ops.NewLintOperation()
			return lintOp.ExecuteFile(ctx, taskFilePath, taskName, foundInVault.Name, *outputFormat)
		},
	}
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
				return errors.Wrap(ctx, err, "get vaults")
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
	var statusFilters []string
	var showAll bool
	var assigneeFlag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List %s from the vault", pageType),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			for _, vault := range vaults {
				if len(vaults) > 1 && *outputFormat == OutputFormatPlain {
					fmt.Printf("=== %s ===\n", vault.Name)
				}

				storageConfig := storage.NewConfigFromVault(vault)
				pageStore := storage.NewPageStorage(storageConfig)
				listOp := ops.NewListOperation(pageStore)

				if err := listOp.Execute(ctx, vault.Path, vault.Name, getDirFunc(storageConfig), statusFilters, showAll, assigneeFlag, "", *outputFormat); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(
		&statusFilters,
		"status",
		nil,
		"Filter by status (e.g. --status=in_progress --status=completed). Cobra StringSliceVar natively supports both repeated flags and comma-separated values.",
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

//nolint:dupl // Entity commands have similar structure but operate on different types
func createEntityGetCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	entityType string,
	newGetOp func(cfg *storage.Config) ops.EntityGetOperation,
) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name> <key>",
		Short: fmt.Sprintf("Get a frontmatter field value from a %s", entityType),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityName := args[0]
			key := args[1]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				getOp := newGetOp(storageConfig)
				value, err := getOp.Execute(ctx, vault.Path, entityName, key)
				if err != nil {
					return err
				}
				if *outputFormat == OutputFormatJSON {
					return PrintJSON(map[string]any{
						"key":   key,
						"value": value,
						"name":  entityName,
					})
				}
				fmt.Println(value)
				return nil
			})
			if err != nil {
				if *outputFormat == OutputFormatJSON {
					return PrintJSON(map[string]any{
						"success": false,
						"error":   err.Error(),
					})
				}
				return err
			}
			return nil
		},
	}
}

//nolint:dupl // Entity commands have similar structure but operate on different types
func createEntitySetCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	entityType string,
	newSetOp func(cfg *storage.Config) ops.EntitySetOperation,
) *cobra.Command {
	return &cobra.Command{
		Use:   "set <name> <key> <value>",
		Short: fmt.Sprintf("Set a frontmatter field value on a %s", entityType),
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityName := args[0]
			key := args[1]
			value := args[2]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				setOp := newSetOp(storageConfig)
				if err := setOp.Execute(ctx, vault.Path, entityName, key, value); err != nil {
					return err
				}
				if *outputFormat == OutputFormatJSON {
					return PrintJSON(map[string]any{
						"success": true,
						"key":     key,
						"value":   value,
						"name":    entityName,
					})
				}
				fmt.Printf("✅ Set %s=%s on: %s\n", key, value, entityName)
				return nil
			})
			if err != nil {
				if *outputFormat == OutputFormatJSON {
					return PrintJSON(map[string]any{
						"success": false,
						"error":   err.Error(),
					})
				}
				return err
			}
			return nil
		},
	}
}

//nolint:dupl // Entity commands have similar structure but operate on different types
func createEntityClearCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	entityType string,
	newClearOp func(cfg *storage.Config) ops.EntityClearOperation,
) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <name> <key>",
		Short: fmt.Sprintf("Clear a frontmatter field value on a %s", entityType),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityName := args[0]
			key := args[1]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				clearOp := newClearOp(storageConfig)
				if err := clearOp.Execute(ctx, vault.Path, entityName, key); err != nil {
					return err
				}
				if *outputFormat == OutputFormatJSON {
					return PrintJSON(map[string]any{
						"success": true,
						"key":     key,
						"name":    entityName,
					})
				}
				fmt.Printf("✅ Cleared %s on: %s\n", key, entityName)
				return nil
			})
			if err != nil {
				if *outputFormat == OutputFormatJSON {
					return PrintJSON(map[string]any{
						"success": false,
						"error":   err.Error(),
					})
				}
				return err
			}
			return nil
		},
	}
}

func createEntityShowCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	entityType string,
	newShowOp func(cfg *storage.Config) ops.EntityShowOperation,
) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: fmt.Sprintf("Show full detail for a single %s", entityType),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityName := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				showOp := newShowOp(storageConfig)
				return showOp.Execute(ctx, vault.Path, vault.Name, entityName, *outputFormat)
			})
		},
	}
}

//nolint:dupl // Entity list commands have similar structure but operate on different types
func createEntityListAddCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	entityType string,
	newAddOp func(cfg *storage.Config) ops.EntityListAddOperation,
) *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <field> <value>",
		Short: fmt.Sprintf("Add a value to a list field on a %s", entityType),
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityName := args[0]
			field := args[1]
			value := args[2]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				addOp := newAddOp(storageConfig)
				return addOp.Execute(ctx, vault.Path, entityName, field, value)
			})
			if err != nil {
				if *outputFormat == OutputFormatJSON {
					return PrintJSON(map[string]any{
						"success": false,
						"error":   err.Error(),
					})
				}
				return err
			}
			if *outputFormat == OutputFormatJSON {
				return PrintJSON(map[string]any{
					"success": true,
					"field":   field,
					"value":   value,
					"name":    entityName,
				})
			}
			fmt.Printf("✅ Added %s to %s on: %s\n", value, field, entityName)
			return nil
		},
	}
}

//nolint:dupl // Entity list commands have similar structure but operate on different types
func createEntityListRemoveCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
	entityType string,
	newRemoveOp func(cfg *storage.Config) ops.EntityListRemoveOperation,
) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name> <field> <value>",
		Short: fmt.Sprintf("Remove a value from a list field on a %s", entityType),
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			entityName := args[0]
			field := args[1]
			value := args[2]

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				removeOp := newRemoveOp(storageConfig)
				return removeOp.Execute(ctx, vault.Path, entityName, field, value)
			})
			if err != nil {
				if *outputFormat == OutputFormatJSON {
					return PrintJSON(map[string]any{
						"success": false,
						"error":   err.Error(),
					})
				}
				return err
			}
			if *outputFormat == OutputFormatJSON {
				return PrintJSON(map[string]any{
					"success": true,
					"field":   field,
					"value":   value,
					"name":    entityName,
				})
			}
			fmt.Printf("✅ Removed %s from %s on: %s\n", value, field, entityName)
			return nil
		},
	}
}

func createTaskCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks in the vault",
	}
	cmd.AddCommand(createTaskListCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createLintCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createValidateCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createCompleteCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createDeferCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createUpdateCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createWorkOnCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createTaskGetCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createTaskSetCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createTaskClearCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createTaskShowCommand(ctx, configLoader, vaultName, outputFormat))
	cmd.AddCommand(createEntityListAddCommand(ctx, configLoader, vaultName, outputFormat, "task",
		func(cfg *storage.Config) ops.EntityListAddOperation {
			return ops.NewTaskListAddOperation(storage.NewTaskStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityListRemoveCommand(ctx, configLoader, vaultName, outputFormat, "task",
		func(cfg *storage.Config) ops.EntityListRemoveOperation {
			return ops.NewTaskListRemoveOperation(storage.NewTaskStorage(cfg))
		},
	))
	cmd.AddCommand(createTaskWatchCommand(ctx, configLoader, vaultName))
	cmd.AddCommand(
		createGenericSearchCommand(
			ctx,
			configLoader,
			vaultName,
			"tasks",
			func(c *storage.Config) string { return c.TasksDir },
			outputFormat,
		),
	)
	return cmd
}

//nolint:dupl // Command groups are structurally similar but manage distinct entity types
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
	cmd.AddCommand(createEntityGetCommand(ctx, configLoader, vaultName, outputFormat, "goal",
		func(cfg *storage.Config) ops.EntityGetOperation {
			return ops.NewGoalGetOperation(storage.NewGoalStorage(cfg))
		},
	))
	cmd.AddCommand(createEntitySetCommand(ctx, configLoader, vaultName, outputFormat, "goal",
		func(cfg *storage.Config) ops.EntitySetOperation {
			return ops.NewGoalSetOperation(storage.NewGoalStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityClearCommand(ctx, configLoader, vaultName, outputFormat, "goal",
		func(cfg *storage.Config) ops.EntityClearOperation {
			return ops.NewGoalClearOperation(storage.NewGoalStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityShowCommand(ctx, configLoader, vaultName, outputFormat, "goal",
		func(cfg *storage.Config) ops.EntityShowOperation {
			return ops.NewGoalShowOperation(storage.NewGoalStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityListAddCommand(ctx, configLoader, vaultName, outputFormat, "goal",
		func(cfg *storage.Config) ops.EntityListAddOperation {
			return ops.NewGoalListAddOperation(storage.NewGoalStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityListRemoveCommand(ctx, configLoader, vaultName, outputFormat, "goal",
		func(cfg *storage.Config) ops.EntityListRemoveOperation {
			return ops.NewGoalListRemoveOperation(storage.NewGoalStorage(cfg))
		},
	))
	cmd.AddCommand(createGoalCompleteCommand(ctx, configLoader, vaultName, outputFormat))
	return cmd
}

func createGoalCompleteCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "complete <goal-name>",
		Short: "Mark a goal as complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			goalName := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			currentDateTime := libtime.NewCurrentDateTime()

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				goalStore := storage.NewGoalStorage(storageConfig)
				taskStore := storage.NewTaskStorage(storageConfig)
				completeOp := ops.NewGoalCompleteOperation(goalStore, taskStore, currentDateTime)
				return completeOp.Execute(
					ctx,
					vault.Path,
					goalName,
					vault.Name,
					*outputFormat,
					force,
				)
			})
		},
	}

	cmd.Flags().
		BoolVar(&force, "force", false, "Complete even if open tasks are linked to this goal")
	return cmd
}

//nolint:dupl // Command groups are structurally similar but manage distinct entity types
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
	cmd.AddCommand(createEntityGetCommand(ctx, configLoader, vaultName, outputFormat, "theme",
		func(cfg *storage.Config) ops.EntityGetOperation {
			return ops.NewThemeGetOperation(storage.NewThemeStorage(cfg))
		},
	))
	cmd.AddCommand(createEntitySetCommand(ctx, configLoader, vaultName, outputFormat, "theme",
		func(cfg *storage.Config) ops.EntitySetOperation {
			return ops.NewThemeSetOperation(storage.NewThemeStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityClearCommand(ctx, configLoader, vaultName, outputFormat, "theme",
		func(cfg *storage.Config) ops.EntityClearOperation {
			return ops.NewThemeClearOperation(storage.NewThemeStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityShowCommand(ctx, configLoader, vaultName, outputFormat, "theme",
		func(cfg *storage.Config) ops.EntityShowOperation {
			return ops.NewThemeShowOperation(storage.NewThemeStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityListAddCommand(ctx, configLoader, vaultName, outputFormat, "theme",
		func(cfg *storage.Config) ops.EntityListAddOperation {
			return ops.NewThemeListAddOperation(storage.NewThemeStorage(cfg))
		},
	))
	cmd.AddCommand(
		createEntityListRemoveCommand(ctx, configLoader, vaultName, outputFormat, "theme",
			func(cfg *storage.Config) ops.EntityListRemoveOperation {
				return ops.NewThemeListRemoveOperation(storage.NewThemeStorage(cfg))
			},
		),
	)
	return cmd
}

//nolint:dupl // Command groups are structurally similar but manage distinct entity types
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
	cmd.AddCommand(createEntityGetCommand(ctx, configLoader, vaultName, outputFormat, "objective",
		func(cfg *storage.Config) ops.EntityGetOperation {
			return ops.NewObjectiveGetOperation(storage.NewObjectiveStorage(cfg))
		},
	))
	cmd.AddCommand(createEntitySetCommand(ctx, configLoader, vaultName, outputFormat, "objective",
		func(cfg *storage.Config) ops.EntitySetOperation {
			return ops.NewObjectiveSetOperation(storage.NewObjectiveStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityClearCommand(ctx, configLoader, vaultName, outputFormat, "objective",
		func(cfg *storage.Config) ops.EntityClearOperation {
			return ops.NewObjectiveClearOperation(storage.NewObjectiveStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityShowCommand(ctx, configLoader, vaultName, outputFormat, "objective",
		func(cfg *storage.Config) ops.EntityShowOperation {
			return ops.NewObjectiveShowOperation(storage.NewObjectiveStorage(cfg))
		},
	))
	cmd.AddCommand(
		createEntityListAddCommand(ctx, configLoader, vaultName, outputFormat, "objective",
			func(cfg *storage.Config) ops.EntityListAddOperation {
				return ops.NewObjectiveListAddOperation(storage.NewObjectiveStorage(cfg))
			},
		),
	)
	cmd.AddCommand(
		createEntityListRemoveCommand(ctx, configLoader, vaultName, outputFormat, "objective",
			func(cfg *storage.Config) ops.EntityListRemoveOperation {
				return ops.NewObjectiveListRemoveOperation(storage.NewObjectiveStorage(cfg))
			},
		),
	)
	cmd.AddCommand(createObjectiveCompleteCommand(ctx, configLoader, vaultName, outputFormat))
	return cmd
}

func createObjectiveCompleteCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "complete <objective-name>",
		Short: "Mark an objective as complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			objectiveName := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			currentDateTime := libtime.NewCurrentDateTime()

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				objectiveStore := storage.NewObjectiveStorage(storageConfig)
				completeOp := ops.NewObjectiveCompleteOperation(objectiveStore, currentDateTime)
				return completeOp.Execute(ctx, vault.Path, objectiveName, vault.Name, *outputFormat)
			})
		},
	}
}

//nolint:dupl // Command groups are structurally similar but manage distinct entity types
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
	cmd.AddCommand(createEntityGetCommand(ctx, configLoader, vaultName, outputFormat, "vision",
		func(cfg *storage.Config) ops.EntityGetOperation {
			return ops.NewVisionGetOperation(storage.NewVisionStorage(cfg))
		},
	))
	cmd.AddCommand(createEntitySetCommand(ctx, configLoader, vaultName, outputFormat, "vision",
		func(cfg *storage.Config) ops.EntitySetOperation {
			return ops.NewVisionSetOperation(storage.NewVisionStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityClearCommand(ctx, configLoader, vaultName, outputFormat, "vision",
		func(cfg *storage.Config) ops.EntityClearOperation {
			return ops.NewVisionClearOperation(storage.NewVisionStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityShowCommand(ctx, configLoader, vaultName, outputFormat, "vision",
		func(cfg *storage.Config) ops.EntityShowOperation {
			return ops.NewVisionShowOperation(storage.NewVisionStorage(cfg))
		},
	))
	cmd.AddCommand(createEntityListAddCommand(ctx, configLoader, vaultName, outputFormat, "vision",
		func(cfg *storage.Config) ops.EntityListAddOperation {
			return ops.NewVisionListAddOperation(storage.NewVisionStorage(cfg))
		},
	))
	cmd.AddCommand(
		createEntityListRemoveCommand(ctx, configLoader, vaultName, outputFormat, "vision",
			func(cfg *storage.Config) ops.EntityListRemoveOperation {
				return ops.NewVisionListRemoveOperation(storage.NewVisionStorage(cfg))
			},
		),
	)
	return cmd
}

func createDecisionCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	decisionCmd := &cobra.Command{
		Use:   "decision",
		Short: "Manage decisions in the vault",
	}
	decisionCmd.AddCommand(createDecisionListCommand(ctx, configLoader, vaultName, outputFormat))
	decisionCmd.AddCommand(createDecisionAckCommand(ctx, configLoader, vaultName, outputFormat))
	return decisionCmd
}

func createDecisionListCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	var showReviewed bool
	var showAll bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List decisions pending review",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				decisionStore := storage.NewDecisionStorage(storageConfig)
				listOp := ops.NewDecisionListOperation(decisionStore)
				if err := listOp.Execute(ctx, vault.Path, vault.Name, showReviewed, showAll, *outputFormat); err != nil {
					slog.Warn("vault error", "vault", vault.Name, "error", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&showReviewed, "reviewed", false, "Show only reviewed decisions")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all decisions (reviewed and unreviewed)")
	return cmd
}

func createDecisionAckCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	var statusOverride string

	cmd := &cobra.Command{
		Use:   "ack <decision-name>",
		Short: "Acknowledge a decision (mark as reviewed)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			decisionName := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			currentDateTime := libtime.NewCurrentDateTime()

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				decisionStore := storage.NewDecisionStorage(storageConfig)
				ackOp := ops.NewDecisionAckOperation(decisionStore, currentDateTime)
				return ackOp.Execute(
					ctx,
					vault.Path,
					vault.Name,
					decisionName,
					statusOverride,
					*outputFormat,
				)
			})
		},
	}

	cmd.Flags().StringVar(&statusOverride, "status", "", "Override the decision's status field")
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
				return errors.Wrap(ctx, err, "get vaults")
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
				return errors.Wrap(ctx, err, "get vaults")
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
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				getOp := ops.NewFrontmatterGetOperation(taskStore)
				value, err := getOp.Execute(ctx, vault.Path, taskName, key)
				if err != nil {
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
			})
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
			return nil
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
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				setOp := ops.NewFrontmatterSetOperation(taskStore)
				if err := setOp.Execute(ctx, vault.Path, taskName, key, value); err != nil {
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
			})
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
			return nil
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
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				clearOp := ops.NewFrontmatterClearOperation(taskStore)
				if err := clearOp.Execute(ctx, vault.Path, taskName, key); err != nil {
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
			})
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
			return nil
		},
	}
}

func createTaskShowCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-name>",
		Short: "Show full detail for a single task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			dispatcher := ops.NewVaultDispatcher()
			return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				showOp := ops.NewShowOperation(taskStore)
				return showOp.Execute(ctx, vault.Path, vault.Name, taskName, *outputFormat)
			})
		},
	}
}

func createTaskWatchCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch task folders for changes (streaming JSON output)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			targets := make([]ops.WatchTarget, 0, len(vaults))
			for _, vault := range vaults {
				targets = append(targets, ops.WatchTarget{
					VaultPath: vault.Path,
					VaultName: vault.Name,
					WatchDirs: []string{
						vault.GetTasksDir(),
						vault.GetGoalsDir(),
						vault.GetThemesDir(),
						vault.GetObjectivesDir(),
					},
				})
			}

			watchOp := ops.NewWatchOperation()
			return watchOp.Execute(ctx, targets)
		},
	}
}

func createConfigListCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured vaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			if *outputFormat == OutputFormatJSON {
				return PrintJSON(vaults)
			}

			for _, vault := range vaults {
				fmt.Printf("%s\t%s\n", vault.Name, vault.Path)
			}
			return nil
		},
	}
}

func createConfigCurrentUserCommand(
	ctx context.Context,
	configLoader *config.Loader,
) *cobra.Command {
	return &cobra.Command{
		Use:   "current-user",
		Short: "Print the current user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			user, err := (*configLoader).GetCurrentUser(ctx)
			if err != nil {
				return errors.Wrap(ctx, err, "get current user")
			}
			fmt.Println(user)
			return nil
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
