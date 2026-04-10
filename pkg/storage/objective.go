// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage //nolint:dupl // Objective and Vision storage have similar structure but operate on different entity types

import (
	"context"
	"os"
	"path/filepath"
	"time"

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
		return nil, errors.Wrapf(ctx, err, "read file %s", filePath)
	}

	var modTime *time.Time
	if info, err := os.Stat(filePath); err == nil {
		t := info.ModTime().UTC()
		modTime = &t
	}

	data, err := o.parseToFrontmatterMap(ctx, content)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	meta := domain.FileMetadata{Name: name, FilePath: filePath, ModifiedDate: modTime}
	return domain.NewObjective(data, meta, domain.Content(content)), nil
}

// WriteObjective writes an objective to a markdown file.
func (o *objectiveStorage) WriteObjective(ctx context.Context, objective *domain.Objective) error {
	content, err := o.serializeMapAsFrontmatter(ctx, objective.RawMap(), string(objective.Content))
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(objective.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrapf(ctx, err, "write file %s", objective.FilePath)
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
