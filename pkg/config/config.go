// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the vault-cli configuration.
type Config struct {
	DefaultVault string           `yaml:"default_vault"`
	Vaults       map[string]Vault `yaml:"vaults"`
}

// Vault represents a single vault configuration.
type Vault struct {
	Path          string `yaml:"path"`
	Name          string `yaml:"name"`
	TasksDir      string `yaml:"tasks_dir,omitempty"`
	GoalsDir      string `yaml:"goals_dir,omitempty"`
	ThemesDir     string `yaml:"themes_dir,omitempty"`
	ObjectivesDir string `yaml:"objectives_dir,omitempty"`
	VisionDir     string `yaml:"vision_dir,omitempty"`
	DailyDir      string `yaml:"daily_dir,omitempty"`
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

//counterfeiter:generate -o ../../mocks/config-loader.go --fake-name Loader . Loader
type Loader interface {
	Load(ctx context.Context) (*Config, error)
	GetVaultPath(ctx context.Context, vaultName string) (string, error)
	GetVault(ctx context.Context, vaultName string) (*Vault, error)
	GetAllVaults(ctx context.Context) ([]*Vault, error)
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
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, ".vault-cli", "config.yaml")
	}

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return c.getDefaultConfig()
	}

	// Read config file
	data, err := os.ReadFile(configPath) //#nosec G304 -- user-controlled config path
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", configPath, err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	return &config, nil
}

// GetVault returns the vault configuration for a given vault name or the default vault.
func (c *configLoader) GetVault(ctx context.Context, vaultName string) (*Vault, error) {
	config, err := c.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// If no vault name specified, use default
	if vaultName == "" {
		vaultName = config.DefaultVault
	}

	// Look up vault
	vault, ok := config.Vaults[vaultName]
	if !ok {
		return nil, fmt.Errorf("vault not found: %s", vaultName)
	}

	// Expand home directory if path starts with ~
	if len(vault.Path) > 0 && vault.Path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		vault.Path = filepath.Join(homeDir, vault.Path[1:])
	}

	return &vault, nil
}

// GetAllVaults returns all configured vaults with expanded paths.
func (c *configLoader) GetAllVaults(ctx context.Context) ([]*Vault, error) {
	config, err := c.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	vaults := make([]*Vault, 0, len(config.Vaults))
	for _, vault := range config.Vaults {
		v := vault // Create a copy to avoid pointer issues
		// Expand home directory if path starts with ~
		if len(v.Path) > 0 && v.Path[0] == '~' {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("get home directory: %w", err)
			}
			v.Path = filepath.Join(homeDir, v.Path[1:])
		}
		vaults = append(vaults, &v)
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

// getDefaultConfig returns a default configuration.
func (c *configLoader) getDefaultConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
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
