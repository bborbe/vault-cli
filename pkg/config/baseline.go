// Copyright (c) 2026 Benjamin Borbe All rights reserved.
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

// ValidateBaselinePath validates a vault-relative baseline path. It rejects an
// empty path, an absolute path, and any path whose cleaned form escapes the vault
// root. The same rule guards the write (`config set-baseline`) and the read
// (`rollup weekly`): a path accepted here is a path both sides can resolve
// against the vault root, and a path rejected here is rejected by both.
func ValidateBaselinePath(ctx context.Context, baselinePath string) error {
	if baselinePath == "" {
		return errors.Errorf(ctx, "baseline path is empty")
	}
	if filepath.IsAbs(baselinePath) {
		return errors.Errorf(
			ctx,
			"baseline path must be vault-relative, got absolute path %s",
			baselinePath,
		)
	}
	cleaned := filepath.Clean(baselinePath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return errors.Errorf(ctx, "baseline path escapes the vault root: %s", baselinePath)
	}
	return nil
}

//counterfeiter:generate -o ../../mocks/baseline-writer.go --fake-name BaselineWriter . BaselineWriter

// BaselineWriter persists a vault's baseline file path into the vault-cli config
// file. It is the only writer of the config file; `rollup weekly` never writes.
type BaselineWriter interface {
	// SetBaseline writes baselinePath onto the named vault's config entry under the
	// key "baseline". The config file is read and validated before anything is
	// written: a missing or unparseable config file, a vault the config does not
	// name, or a rejected path leaves the file exactly as it was.
	SetBaseline(ctx context.Context, vaultName string, baselinePath string) error
}

// NewBaselineWriter creates a BaselineWriter for the config file at configPath.
// An empty configPath resolves the config file by the loader's own rule.
func NewBaselineWriter(configPath string) BaselineWriter {
	return &baselineWriter{configPath: configPath}
}

type baselineWriter struct {
	configPath string
}

// SetBaseline reads the config file, sets the named vault's baseline key and
// writes the config file back atomically. Nothing is written on any failure.
func (b *baselineWriter) SetBaseline(
	ctx context.Context,
	vaultName string,
	baselinePath string,
) error {
	if err := ValidateBaselinePath(ctx, baselinePath); err != nil {
		return err
	}

	configPath, err := resolveConfigPath(ctx, b.configPath)
	if err != nil {
		return errors.Wrap(ctx, err, "resolve config path")
	}

	cfg, err := readConfigFile(ctx, configPath)
	if err != nil {
		return err
	}

	matchedKey, err := findVaultKey(ctx, cfg, vaultName)
	if err != nil {
		return err
	}

	vault := cfg.Vaults[matchedKey]
	vault.Baseline = baselinePath
	cfg.Vaults[matchedKey] = vault

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(ctx, err, "marshal config")
	}

	return writeFileAtomic(ctx, configPath, out)
}

// readConfigFile reads and parses the config file at configPath. A missing or
// unparseable file is an error: this verb requires a config file to write into
// and never creates one.
func readConfigFile(ctx context.Context, configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath) //#nosec G304 -- user-controlled config path
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "read config file %s", configPath)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, errors.Wrapf(ctx, err, "parse config file %s", configPath)
	}
	return &cfg, nil
}

// findVaultKey returns the vault map key matching vaultName case-insensitively,
// preserving the file's own key spelling. A vault absent from the config is an
// error.
func findVaultKey(ctx context.Context, cfg *Config, vaultName string) (string, error) {
	var matchedKey string
	for key := range cfg.Vaults {
		if strings.EqualFold(key, vaultName) {
			matchedKey = key
			break
		}
	}
	if matchedKey == "" {
		return "", errors.Errorf(ctx, "vault not found in config: %s", vaultName)
	}
	return matchedKey, nil
}

// writeFileAtomic writes data to path atomically: it writes a temp file in path's
// own directory and renames it over path, so a reader observes either the previous
// content or the new content and never a truncated or partially merged file. On
// any failure path is untouched and the temp file is removed.
func writeFileAtomic(ctx context.Context, path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return errors.Wrapf(ctx, err, "create temp file in %s", dir)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return errors.Wrapf(ctx, err, "write temp file %s", tmp.Name())
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return errors.Wrapf(ctx, err, "close temp file %s", tmp.Name())
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return errors.Wrapf(ctx, err, "rename temp file to %s", path)
	}

	return nil
}
