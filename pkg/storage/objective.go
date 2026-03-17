// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage //nolint:dupl // Objective and Vision storage have similar structure but operate on different entity types

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

type objectiveStorage struct {
	*baseStorage
}

// ReadObjective reads an objective from a markdown file.
func (o *objectiveStorage) ReadObjective(
	ctx context.Context,
	vaultPath string,
	objectiveID domain.ObjectiveID,
) (*domain.Objective, error) {
	filePath := filepath.Join(vaultPath, o.config.ObjectivesDir, objectiveID.String()+".md")
	return o.readObjectiveFromPath(ctx, filePath, objectiveID.String())
}

func (o *objectiveStorage) readObjectiveFromPath(
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Objective, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	objective := &domain.Objective{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := o.parseFrontmatter(ctx, content, objective); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	return objective, nil
}

// WriteObjective writes an objective to a markdown file.
func (o *objectiveStorage) WriteObjective(ctx context.Context, objective *domain.Objective) error {
	content, err := o.serializeWithFrontmatter(ctx, objective, objective.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(objective.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", objective.FilePath))
	}

	return nil
}

// FindObjectiveByName searches for an objective by name in the vault.
func (o *objectiveStorage) FindObjectiveByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Objective, error) {
	objectivesDir := filepath.Join(vaultPath, o.config.ObjectivesDir)
	matchedPath, matchedName, err := o.findFileByName(ctx, objectivesDir, name)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "find objective file")
	}
	return o.readObjectiveFromPath(ctx, matchedPath, matchedName)
}
