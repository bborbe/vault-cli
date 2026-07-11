// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/bborbe/errors"
	"gopkg.in/yaml.v3"
)

// Config represents the vault-cli configuration.
type Config struct {
	CurrentUser  string           `yaml:"current_user"`
	DefaultVault string           `yaml:"default_vault"`
	Vaults       map[string]Vault `yaml:"vaults"`
}

// Vault represents a single vault configuration.
type Vault struct {
	Path              string   `yaml:"path"                           json:"path"`
	Name              string   `yaml:"name"                           json:"name"`
	TasksDir          string   `yaml:"tasks_dir,omitempty"            json:"tasks_dir,omitempty"`
	GoalsDir          string   `yaml:"goals_dir,omitempty"            json:"goals_dir,omitempty"`
	ThemesDir         string   `yaml:"themes_dir,omitempty"           json:"themes_dir,omitempty"`
	ObjectivesDir     string   `yaml:"objectives_dir,omitempty"       json:"objectives_dir,omitempty"`
	VisionDir         string   `yaml:"vision_dir,omitempty"           json:"vision_dir,omitempty"`
	DailyDir          string   `yaml:"daily_dir,omitempty"            json:"daily_dir,omitempty"`
	KnowledgeDir      string   `yaml:"knowledge_dir,omitempty"        json:"knowledge_dir,omitempty"`
	ClaudeScript      string   `yaml:"claude_script,omitempty"        json:"claude_script,omitempty"`
	SessionProjectDir string   `yaml:"session_project_dir,omitempty"  json:"session_project_dir,omitempty"`
	WorkOnCommand     string   `yaml:"work_on_command,omitempty"      json:"work_on_command,omitempty"`
	WorkOnGoalCommand string   `yaml:"work_on_goal_command,omitempty" json:"work_on_goal_command,omitempty"`
	TaskTemplate      string   `yaml:"task_template,omitempty"        json:"task_template,omitempty"`
	GoalTemplate      string   `yaml:"goal_template,omitempty"        json:"goal_template,omitempty"`
	ThemeTemplate     string   `yaml:"theme_template,omitempty"       json:"theme_template,omitempty"`
	ObjectiveTemplate string   `yaml:"objective_template,omitempty"   json:"objective_template,omitempty"`
	VisionTemplate    string   `yaml:"vision_template,omitempty"      json:"vision_template,omitempty"`
	Excludes          []string `yaml:"excludes,omitempty"             json:"excludes,omitempty"`
}

// GetTasksDir returns the tasks directory, defaulting to "Tasks" if not set.
func (v *Vault) GetTasksDir() string {
	if v.TasksDir != "" {
		return v.TasksDir
	}
	return "Tasks"
}

// GetGoalsDir returns the goals directory, defaulting to "Goals" if not set.
func (v *Vault) GetGoalsDir() string {
	if v.GoalsDir != "" {
		return v.GoalsDir
	}
	return "Goals"
}

// GetThemesDir returns the themes directory, defaulting to "21 Themes" if not set.
func (v *Vault) GetThemesDir() string {
	if v.ThemesDir != "" {
		return v.ThemesDir
	}
	return "21 Themes"
}

// GetObjectivesDir returns the objectives directory, defaulting to "22 Objectives" if not set.
func (v *Vault) GetObjectivesDir() string {
	if v.ObjectivesDir != "" {
		return v.ObjectivesDir
	}
	return "22 Objectives"
}

// GetVisionDir returns the vision directory, defaulting to "20 Vision" if not set.
func (v *Vault) GetVisionDir() string {
	if v.VisionDir != "" {
		return v.VisionDir
	}
	return "20 Vision"
}

// GetDailyDir returns the daily notes directory, defaulting to "Daily Notes" if not set.
func (v *Vault) GetDailyDir() string {
	if v.DailyDir != "" {
		return v.DailyDir
	}
	return "Daily Notes"
}

// GetKnowledgeDir returns the knowledge base directory, defaulting to "50 Knowledge Base" if not set.
func (v *Vault) GetKnowledgeDir() string {
	if v.KnowledgeDir != "" {
		return v.KnowledgeDir
	}
	return "50 Knowledge Base"
}

// GetExcludes returns the list of excluded directory prefixes.
func (v *Vault) GetExcludes() []string {
	return v.Excludes
}

// GetSessionProjectDir returns the session project directory override, or empty string if not set.
func (v *Vault) GetSessionProjectDir() string {
	return v.SessionProjectDir
}

// GetWorkOnCommand returns the Claude slash command for starting work-on sessions,
// defaulting to /vault-cli:work-on-task if not configured.
func (v *Vault) GetWorkOnCommand() string {
	if v.WorkOnCommand != "" {
		return v.WorkOnCommand
	}
	return "/vault-cli:work-on-task"
}

// GetWorkOnGoalCommand returns the Claude slash command for starting goal work-on
// sessions, defaulting to /vault-cli:work-on-goal if not configured.
func (v *Vault) GetWorkOnGoalCommand() string {
	if v.WorkOnGoalCommand != "" {
		return v.WorkOnGoalCommand
	}
	return "/vault-cli:work-on-goal"
}

// GetTaskTemplate returns the resolved absolute path to the task template, or empty string if not set.
func (v *Vault) GetTaskTemplate() string {
	return v.TaskTemplate
}

// GetGoalTemplate returns the resolved absolute path to the goal template, or empty string if not set.
func (v *Vault) GetGoalTemplate() string {
	return v.GoalTemplate
}

// GetThemeTemplate returns the resolved absolute path to the theme template, or empty string if not set.
func (v *Vault) GetThemeTemplate() string {
	return v.ThemeTemplate
}

// GetObjectiveTemplate returns the resolved absolute path to the objective template, or empty string if not set.
func (v *Vault) GetObjectiveTemplate() string {
	return v.ObjectiveTemplate
}

// GetVisionTemplate returns the resolved absolute path to the vision template, or empty string if not set.
func (v *Vault) GetVisionTemplate() string {
	return v.VisionTemplate
}

// GetClaudeScript returns the claude script to use for sessions, defaulting to "claude" if not set.
func (v *Vault) GetClaudeScript() string {
	if v.ClaudeScript != "" {
		return v.ClaudeScript
	}
	return "claude"
}

// FindConfigDir returns the config directory for the given tool, applying
// XDG-first priority. If ~/.config/<toolName>/ exists it is returned.
// Otherwise, if the legacy ~/.<toolName>/ directory exists, it is returned.
// When neither exists, the XDG path ~/.config/<toolName>/ is returned as the
// default for new installs. FindConfigDir never creates directories and never
// writes to the filesystem; it only checks directory existence via os.Stat.
func FindConfigDir(ctx context.Context, toolName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(ctx, err, "get home directory")
	}
	xdgDir := filepath.Join(homeDir, ".config", toolName)
	if info, statErr := os.Stat(xdgDir); statErr == nil && info.IsDir() {
		return xdgDir, nil
	}
	legacyDir := filepath.Join(homeDir, "."+toolName)
	if info, statErr := os.Stat(legacyDir); statErr == nil && info.IsDir() {
		return legacyDir, nil
	}
	return xdgDir, nil
}

//counterfeiter:generate -o ../../mocks/config-loader.go --fake-name Loader . Loader
type Loader interface {
	Load(ctx context.Context) (*Config, error)
	GetVaultPath(ctx context.Context, vaultName string) (string, error)
	GetVault(ctx context.Context, vaultName string) (*Vault, error)
	GetAllVaults(ctx context.Context) ([]*Vault, error)
	GetCurrentUser(ctx context.Context) (string, error)
}

// NewLoader creates a new config loader.
func NewLoader(configPath string) Loader {
	return &configLoader{
		configPath: configPath,
	}
}

type configLoader struct {
	configPath string
}

// Load reads the configuration from file or returns default config.
func (c *configLoader) Load(ctx context.Context) (*Config, error) {
	// If config path is empty, use default location
	configPath := c.configPath
	if configPath == "" {
		dir, err := FindConfigDir(ctx, "vault-cli")
		if err != nil {
			return nil, errors.Wrap(ctx, err, "find config dir")
		}
		configPath = filepath.Join(dir, "config.yaml")
	}

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return c.getDefaultConfig(ctx)
	}

	// Read config file
	data, err := os.ReadFile(configPath) //#nosec G304 -- user-controlled config path
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "read config file %s", configPath)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(ctx, err, "parse config yaml")
	}

	config.DefaultVault = strings.ToLower(config.DefaultVault)
	normalized := make(map[string]Vault, len(config.Vaults))
	for key, vault := range config.Vaults {
		vault.Name = strings.ToLower(vault.Name)
		normalized[strings.ToLower(key)] = vault
	}
	config.Vaults = normalized

	return &config, nil
}

// expandVaultPaths expands home directory references and resolves template paths in a vault copy.
// It does not mutate the input vault.
func expandVaultPaths(ctx context.Context, vault *Vault) (*Vault, error) {
	result := *vault

	// Expand home directory if path starts with ~
	if len(result.Path) > 0 && result.Path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(ctx, err, "get home directory")
		}
		result.Path = filepath.Join(homeDir, result.Path[1:])
	}

	if len(result.SessionProjectDir) > 0 && result.SessionProjectDir[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(ctx, err, "get home directory")
		}
		result.SessionProjectDir = filepath.Join(homeDir, result.SessionProjectDir[1:])
	}

	templateFields := []*string{
		&result.TaskTemplate,
		&result.GoalTemplate,
		&result.ThemeTemplate,
		&result.ObjectiveTemplate,
		&result.VisionTemplate,
	}
	for _, f := range templateFields {
		resolved, err := resolveTemplatePath(ctx, *f, result.Path)
		if err != nil {
			return nil, errors.Wrap(ctx, err, "resolve template path")
		}
		*f = resolved
	}

	return &result, nil
}

// GetVault returns the vault configuration for a given vault name or the default vault.
func (c *configLoader) GetVault(ctx context.Context, vaultName string) (*Vault, error) {
	config, err := c.Load(ctx)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "load config")
	}

	// If no vault name specified, use default
	if vaultName == "" {
		vaultName = config.DefaultVault
	}
	vaultName = strings.ToLower(vaultName)

	// Look up vault
	vault, ok := config.Vaults[vaultName]
	if !ok {
		return nil, errors.Errorf(ctx, "vault not found: %s", vaultName)
	}

	return expandVaultPaths(ctx, &vault)
}

// GetAllVaults returns all configured vaults with expanded paths.
func (c *configLoader) GetAllVaults(ctx context.Context) ([]*Vault, error) {
	config, err := c.Load(ctx)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "load config")
	}

	vaults := make([]*Vault, 0, len(config.Vaults))
	for _, vault := range config.Vaults {
		v := vault // Create a copy to avoid pointer issues
		expanded, err := expandVaultPaths(ctx, &v)
		if err != nil {
			return nil, errors.Wrap(ctx, err, "expand vault paths")
		}
		vaults = append(vaults, expanded)
	}

	return vaults, nil
}

// GetVaultPath returns the path for a given vault name or the default vault.
func (c *configLoader) GetVaultPath(ctx context.Context, vaultName string) (string, error) {
	vault, err := c.GetVault(ctx, vaultName)
	if err != nil {
		return "", err
	}
	return vault.Path, nil
}

// GetCurrentUser returns the current user from config.
func (c *configLoader) GetCurrentUser(ctx context.Context) (string, error) {
	config, err := c.Load(ctx)
	if err != nil {
		return "", errors.Wrap(ctx, err, "load config")
	}
	if config.CurrentUser == "" {
		return "", errors.Errorf(ctx, "current_user not configured")
	}
	return config.CurrentUser, nil
}

// resolveTemplatePath resolves a template field value to an absolute path.
// Returns empty string if value is empty.
// Expands a leading ~ to the user home directory.
// Joins a relative path against vaultPath (already an absolute path).
// Returns an absolute path unchanged.
func resolveTemplatePath(ctx context.Context, value, vaultPath string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) > 0 && value[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", errors.Wrap(ctx, err, "get home directory")
		}
		return filepath.Join(homeDir, value[1:]), nil
	}
	if filepath.IsAbs(value) {
		return value, nil
	}
	return filepath.Join(vaultPath, value), nil
}

// getDefaultConfig returns a default configuration.
func (c *configLoader) getDefaultConfig(ctx context.Context) (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(ctx, err, "get home directory")
	}

	return &Config{
		DefaultVault: "main",
		Vaults: map[string]Vault{
			"main": {
				Name: "main",
				Path: filepath.Join(homeDir, "Documents", "vault"),
			},
		},
	}, nil
}
