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
	Path string `yaml:"path"`
	Name string `yaml:"name"`
}

//counterfeiter:generate -o ../../mocks/config-loader.go --fake-name Loader . Loader
type Loader interface {
	Load(ctx context.Context) (*Config, error)
	GetVaultPath(ctx context.Context, vaultName string) (string, error)
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
	data, err := os.ReadFile(configPath) //nolint:gosec // User-controlled config path
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

// GetVaultPath returns the path for a given vault name or the default vault.
func (c *configLoader) GetVaultPath(ctx context.Context, vaultName string) (string, error) {
	config, err := c.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	// If no vault name specified, use default
	if vaultName == "" {
		vaultName = config.DefaultVault
	}

	// Look up vault path
	vault, ok := config.Vaults[vaultName]
	if !ok {
		return "", fmt.Errorf("vault not found: %s", vaultName)
	}

	// Expand home directory if path starts with ~
	path := vault.Path
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[1:])
	}

	return path, nil
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
