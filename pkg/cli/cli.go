// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

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

	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks in the vault",
	}

	taskCmd.AddCommand(createTaskListCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createLintCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createValidateCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createCompleteCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createDeferCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createUpdateCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createWorkOnCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createTaskGetCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createTaskSetCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createTaskClearCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createTaskShowCommand(ctx, &configLoader, &vaultName, &outputFormat))
	taskCmd.AddCommand(createTaskWatchCommand(ctx, &configLoader, &vaultName))
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
				return fmt.Errorf("get vaults: %w", err)
			}

			currentDateTime := libtime.NewCurrentDateTime()

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
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
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
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
		Use:   "defer <task-name> [date]",
		Short: "Defer a task to a specific date",
		Long: `Defer a task to a specific date.

If no date is provided, defaults to +1d (tomorrow).

Date formats:
  +Nd         - Relative days (e.g., +7d for 7 days from now)
  monday      - Next occurrence of weekday
  2024-12-31  - ISO date format (YYYY-MM-DD)`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskName := args[0]
			dateStr := "+1d"
			if len(args) > 1 {
				dateStr = args[1]
			}

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			currentDateTime := libtime.NewCurrentDateTime()

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
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
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				dailyStore := storage.NewDailyNoteStorage(storageConfig)
				deferOp := ops.NewDeferOperation(taskStore, dailyStore, currentDateTime)
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
				taskStore := storage.NewTaskStorage(storageConfig)
				goalStore := storage.NewGoalStorage(storageConfig)
				updateOp := ops.NewUpdateOperation(taskStore, goalStore)
				return updateOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat)
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				goalStore := storage.NewGoalStorage(storageConfig)
				updateOp := ops.NewUpdateOperation(taskStore, goalStore)
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
				return fmt.Errorf("get current user: %w", err)
			}

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return fmt.Errorf("get vaults: %w", err)
			}

			currentDateTime := libtime.NewCurrentDateTime()

			var lastErr error
			for _, vault := range vaults {
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
				err := workOnOp.Execute(
					ctx,
					vault.Path,
					taskName,
					currentUser,
					vault.Name,
					*outputFormat,
					isInteractive,
				)
				if err == nil {
					return nil
				}
				lastErr = err
			}

			return fmt.Errorf("task not found in any vault: %w", lastErr)
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
				pageStore := storage.NewPageStorage(storageConfig)
				listOp := ops.NewListOperation(pageStore)

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
				return fmt.Errorf("get vaults: %w", err)
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
				return fmt.Errorf("task not found: %s", taskName)
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
				pageStore := storage.NewPageStorage(storageConfig)
				listOp := ops.NewListOperation(pageStore)

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
				return fmt.Errorf("get vaults: %w", err)
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
				return fmt.Errorf("get vaults: %w", err)
			}

			currentDateTime := libtime.NewCurrentDateTime()

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				decisionStore := storage.NewDecisionStorage(storageConfig)
				ackOp := ops.NewDecisionAckOperation(decisionStore, currentDateTime)
				err := ackOp.Execute(
					ctx,
					vault.Path,
					vault.Name,
					decisionName,
					statusOverride,
					*outputFormat,
				)
				if err == nil {
					return nil
				}
				lastErr = err
			}
			return fmt.Errorf("decision not found in any vault: %w", lastErr)
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
				taskStore := storage.NewTaskStorage(storageConfig)
				getOp := ops.NewFrontmatterGetOperation(taskStore)
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
				taskStore := storage.NewTaskStorage(storageConfig)
				getOp := ops.NewFrontmatterGetOperation(taskStore)
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
				taskStore := storage.NewTaskStorage(storageConfig)
				setOp := ops.NewFrontmatterSetOperation(taskStore)
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
				taskStore := storage.NewTaskStorage(storageConfig)
				setOp := ops.NewFrontmatterSetOperation(taskStore)
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
				taskStore := storage.NewTaskStorage(storageConfig)
				clearOp := ops.NewFrontmatterClearOperation(taskStore)
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
				taskStore := storage.NewTaskStorage(storageConfig)
				clearOp := ops.NewFrontmatterClearOperation(taskStore)
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
				return fmt.Errorf("get vaults: %w", err)
			}

			// If only one vault, execute directly
			if len(vaults) == 1 {
				vault := vaults[0]
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				showOp := ops.NewShowOperation(taskStore)
				return showOp.Execute(ctx, vault.Path, vault.Name, taskName, *outputFormat)
			}

			// Multiple vaults: try each until successful
			var lastErr error
			for _, vault := range vaults {
				storageConfig := storage.NewConfigFromVault(vault)
				taskStore := storage.NewTaskStorage(storageConfig)
				showOp := ops.NewShowOperation(taskStore)
				if err := showOp.Execute(ctx, vault.Path, vault.Name, taskName, *outputFormat); err == nil {
					return nil
				}
				lastErr = err
			}

			return fmt.Errorf("task not found in any vault: %w", lastErr)
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
				return fmt.Errorf("get vaults: %w", err)
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
				return fmt.Errorf("get vaults: %w", err)
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
				return fmt.Errorf("get current user: %w", err)
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
