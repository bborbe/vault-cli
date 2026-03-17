// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
)

// Config holds the configuration for storage paths.
type Config struct {
	TasksDir      string
	GoalsDir      string
	ThemesDir     string
	ObjectivesDir string
	VisionDir     string
	DailyDir      string
	Excludes      []string
}

// NewConfigFromVault creates a Config from a Vault.
func NewConfigFromVault(vault *config.Vault) *Config {
	return &Config{
		TasksDir:      vault.GetTasksDir(),
		GoalsDir:      vault.GetGoalsDir(),
		ThemesDir:     vault.GetThemesDir(),
		ObjectivesDir: vault.GetObjectivesDir(),
		VisionDir:     vault.GetVisionDir(),
		DailyDir:      vault.GetDailyDir(),
		Excludes:      vault.GetExcludes(),
	}
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig() *Config {
	return &Config{
		TasksDir:      "Tasks",
		GoalsDir:      "Goals",
		ThemesDir:     "21 Themes",
		ObjectivesDir: "22 Objectives",
		VisionDir:     "20 Vision",
		DailyDir:      "Daily Notes",
	}
}

//counterfeiter:generate -o ../../mocks/task-storage.go --fake-name TaskStorage . TaskStorage
type TaskStorage interface {
	WriteTask(ctx context.Context, task *domain.Task) error
	FindTaskByName(ctx context.Context, vaultPath string, name string) (*domain.Task, error)
	ListTasks(ctx context.Context, vaultPath string) ([]*domain.Task, error)
}

//counterfeiter:generate -o ../../mocks/goal-storage.go --fake-name GoalStorage . GoalStorage
type GoalStorage interface {
	WriteGoal(ctx context.Context, goal *domain.Goal) error
	FindGoalByName(ctx context.Context, vaultPath string, name string) (*domain.Goal, error)
}

//counterfeiter:generate -o ../../mocks/theme-storage.go --fake-name ThemeStorage . ThemeStorage
type ThemeStorage interface {
	WriteTheme(ctx context.Context, theme *domain.Theme) error
	FindThemeByName(ctx context.Context, vaultPath string, name string) (*domain.Theme, error)
}

//counterfeiter:generate -o ../../mocks/objective-storage.go --fake-name ObjectiveStorage . ObjectiveStorage
type ObjectiveStorage interface {
	WriteObjective(ctx context.Context, objective *domain.Objective) error
	FindObjectiveByName(
		ctx context.Context,
		vaultPath string,
		name string,
	) (*domain.Objective, error)
}

//counterfeiter:generate -o ../../mocks/vision-storage.go --fake-name VisionStorage . VisionStorage
type VisionStorage interface {
	WriteVision(ctx context.Context, vision *domain.Vision) error
	FindVisionByName(ctx context.Context, vaultPath string, name string) (*domain.Vision, error)
}

//counterfeiter:generate -o ../../mocks/daily-note-storage.go --fake-name DailyNoteStorage . DailyNoteStorage
type DailyNoteStorage interface {
	ReadDailyNote(ctx context.Context, vaultPath string, date string) (string, error)
	WriteDailyNote(ctx context.Context, vaultPath string, date string, content string) error
}

//counterfeiter:generate -o ../../mocks/page-storage.go --fake-name PageStorage . PageStorage
type PageStorage interface {
	ListPages(ctx context.Context, vaultPath string, pagesDir string) ([]*domain.Task, error)
}

//counterfeiter:generate -o ../../mocks/decision-storage.go --fake-name DecisionStorage . DecisionStorage
type DecisionStorage interface {
	ListDecisions(ctx context.Context, vaultPath string) ([]*domain.Decision, error)
	FindDecisionByName(ctx context.Context, vaultPath string, name string) (*domain.Decision, error)
	WriteDecision(ctx context.Context, decision *domain.Decision) error
}

//counterfeiter:generate -o ../../mocks/storage.go --fake-name Storage . Storage
type Storage interface {
	TaskStorage
	GoalStorage
	ThemeStorage
	ObjectiveStorage
	VisionStorage
	DailyNoteStorage
	PageStorage
	DecisionStorage
	// Legacy methods — used by storage tests, not by ops.
	// Keep on composed interface for backward compat; not on narrow interfaces.
	ReadTask(ctx context.Context, vaultPath string, taskID domain.TaskID) (*domain.Task, error)
	ListTasks(ctx context.Context, vaultPath string) ([]*domain.Task, error)
	ReadGoal(ctx context.Context, vaultPath string, goalID domain.GoalID) (*domain.Goal, error)
	ReadTheme(ctx context.Context, vaultPath string, themeID domain.ThemeID) (*domain.Theme, error)
	ReadObjective(
		ctx context.Context,
		vaultPath string,
		objectiveID domain.ObjectiveID,
	) (*domain.Objective, error)
	ReadVision(
		ctx context.Context,
		vaultPath string,
		visionID domain.VisionID,
	) (*domain.Vision, error)
}

// NewStorage creates a new markdown storage instance with custom configuration.
func NewStorage(storageConfig *Config) Storage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	base := &baseStorage{config: storageConfig}
	return &markdownStorage{
		taskStorage:      &taskStorage{baseStorage: base},
		goalStorage:      &goalStorage{baseStorage: base},
		dailyNoteStorage: &dailyNoteStorage{baseStorage: base},
		pageStorage:      &pageStorage{baseStorage: base},
		decisionStorage:  &decisionStorage{baseStorage: base},
		themeStorage:     &themeStorage{baseStorage: base},
		objectiveStorage: &objectiveStorage{baseStorage: base},
		visionStorage:    &visionStorage{baseStorage: base},
	}
}

type markdownStorage struct {
	*taskStorage
	*goalStorage
	*dailyNoteStorage
	*pageStorage
	*decisionStorage
	*themeStorage
	*objectiveStorage
	*visionStorage
}

// NewTaskStorage creates a storage for task operations only.
func NewTaskStorage(storageConfig *Config) TaskStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &taskStorage{baseStorage: &baseStorage{config: storageConfig}}
}

// NewGoalStorage creates a storage for goal operations only.
func NewGoalStorage(storageConfig *Config) GoalStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &goalStorage{baseStorage: &baseStorage{config: storageConfig}}
}

// NewThemeStorage creates a storage for theme operations only.
func NewThemeStorage(storageConfig *Config) ThemeStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &themeStorage{baseStorage: &baseStorage{config: storageConfig}}
}

// NewObjectiveStorage creates a storage for objective operations only.
func NewObjectiveStorage(storageConfig *Config) ObjectiveStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &objectiveStorage{baseStorage: &baseStorage{config: storageConfig}}
}

// NewVisionStorage creates a storage for vision operations only.
func NewVisionStorage(storageConfig *Config) VisionStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &visionStorage{baseStorage: &baseStorage{config: storageConfig}}
}

// NewDailyNoteStorage creates a storage for daily note operations only.
func NewDailyNoteStorage(storageConfig *Config) DailyNoteStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &dailyNoteStorage{baseStorage: &baseStorage{config: storageConfig}}
}

// NewPageStorage creates a storage for page operations only.
func NewPageStorage(storageConfig *Config) PageStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &pageStorage{baseStorage: &baseStorage{config: storageConfig}}
}

// NewDecisionStorage creates a storage for decision operations only.
func NewDecisionStorage(storageConfig *Config) DecisionStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &decisionStorage{baseStorage: &baseStorage{config: storageConfig}}
}
