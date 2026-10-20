// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"context"
	"regexp"

	"github.com/bborbe/errors"
	"gopkg.in/yaml.v3"
)

// a2aClusterName is the RFC 1123 label shape a Kubernetes cluster identifier
// takes: lowercase alphanumerics and interior hyphens, starting and ending with
// an alphanumeric. It constrains the SHAPE only. Which cluster names exist is
// the consumer's knowledge, so an unknown-but-well-formed name is accepted here
// and refused there.
var a2aClusterName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// a2aClusterNameMaxLength is the RFC 1123 label limit.
const a2aClusterNameMaxLength = 63

// ValidateA2ADefaultCluster validates a vault's default A2A cluster name. The
// value is a cluster NAME such as `nukedev`, never a spawn target such as
// `k8s:nukedev` — the consumer forms the target from the name, so a colon is
// refused rather than passed through. The rule is the input boundary for the
// write path: every value accepted here is safe to store and to re-emit in
// `config list --output json`, and every value rejected here leaves the config
// file untouched.
func ValidateA2ADefaultCluster(ctx context.Context, cluster string) error {
	if cluster == "" {
		return errors.Errorf(ctx, "a2a default cluster is empty")
	}
	if len(cluster) > a2aClusterNameMaxLength {
		return errors.Errorf(
			ctx,
			"a2a default cluster exceeds %d characters: %s",
			a2aClusterNameMaxLength,
			cluster,
		)
	}
	if !a2aClusterName.MatchString(cluster) {
		return errors.Errorf(
			ctx,
			"a2a default cluster must be an RFC 1123 label (lowercase alphanumerics and interior hyphens), got %q",
			cluster,
		)
	}
	return nil
}

// A2ADefaultClusterWriter persists a vault's default A2A cluster name into the
// vault-cli config file. It is the only writer of that key.
type A2ADefaultClusterWriter interface {
	// SetA2ADefaultCluster writes cluster onto the named vault's config entry
	// under the key "a2a_default_cluster". The config file is read and validated
	// before anything is written: a missing or unparseable config file, a vault
	// the config does not name, or a rejected cluster name leaves the file
	// exactly as it was.
	SetA2ADefaultCluster(ctx context.Context, vaultName string, cluster string) error
}

// NewA2ADefaultClusterWriter creates an A2ADefaultClusterWriter for the config
// file at configPath. An empty configPath resolves the config file by the
// loader's own rule.
func NewA2ADefaultClusterWriter(configPath string) A2ADefaultClusterWriter {
	return &a2aDefaultClusterWriter{configPath: configPath}
}

type a2aDefaultClusterWriter struct {
	configPath string
}

// SetA2ADefaultCluster reads the config file, sets the named vault's
// a2a_default_cluster key and writes the config file back atomically. Nothing is
// written on any failure.
func (w *a2aDefaultClusterWriter) SetA2ADefaultCluster(
	ctx context.Context,
	vaultName string,
	cluster string,
) error {
	if err := ValidateA2ADefaultCluster(ctx, cluster); err != nil {
		return err
	}

	configPath, err := resolveConfigPath(ctx, w.configPath)
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
	vault.A2ADefaultCluster = cluster
	cfg.Vaults[matchedKey] = vault

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(ctx, err, "marshal config")
	}

	return writeFileAtomic(ctx, configPath, out)
}
