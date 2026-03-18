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

type visionStorage struct {
	*baseStorage
}

// ReadVision reads a vision from a markdown file.
func (v *visionStorage) ReadVision(
	ctx context.Context,
	vaultPath string,
	visionID domain.VisionID,
) (*domain.Vision, error) {
	filePath := filepath.Join(vaultPath, v.config.VisionDir, visionID.String()+".md")
	return v.readVisionFromPath(ctx, filePath, visionID.String())
}

func (v *visionStorage) readVisionFromPath(
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Vision, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	vision := &domain.Vision{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if info, err := os.Stat(filePath); err == nil {
		t := info.ModTime().UTC()
		vision.ModifiedDate = &t
	}

	if err := v.parseFrontmatter(ctx, content, vision); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	return vision, nil
}

// WriteVision writes a vision to a markdown file.
func (v *visionStorage) WriteVision(ctx context.Context, vision *domain.Vision) error {
	content, err := v.serializeWithFrontmatter(ctx, vision, vision.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(vision.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", vision.FilePath))
	}

	return nil
}

// FindVisionByName searches for a vision by name in the vault.
func (v *visionStorage) FindVisionByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Vision, error) {
	visionDir := filepath.Join(vaultPath, v.config.VisionDir)
	matchedPath, matchedName, err := v.findFileByName(ctx, visionDir, name)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "find vision file")
	}
	return v.readVisionFromPath(ctx, matchedPath, matchedName)
}
