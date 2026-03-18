// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

type goalStorage struct {
	*baseStorage
}

// ReadGoal reads a goal from a markdown file.
func (g *goalStorage) ReadGoal(
	ctx context.Context,
	vaultPath string,
	goalID domain.GoalID,
) (*domain.Goal, error) {
	filePath := filepath.Join(vaultPath, g.config.GoalsDir, goalID.String()+".md")
	return g.readGoalFromPath(ctx, filePath, goalID.String())
}

func (g *goalStorage) readGoalFromPath(
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Goal, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	goal := &domain.Goal{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if info, err := os.Stat(filePath); err == nil {
		t := info.ModTime().UTC()
		goal.ModifiedDate = &t
	}

	if err := g.parseFrontmatter(ctx, content, goal); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	// Parse checkbox items from content
	goal.Tasks = g.parseCheckboxes(string(content))

	return goal, nil
}

// WriteGoal writes a goal to a markdown file.
func (g *goalStorage) WriteGoal(ctx context.Context, goal *domain.Goal) error {
	content, err := g.serializeWithFrontmatter(ctx, goal, goal.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(goal.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", goal.FilePath))
	}

	return nil
}

// FindGoalByName searches for a goal by name in the vault.
func (g *goalStorage) FindGoalByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Goal, error) {
	goalsDir := filepath.Join(vaultPath, g.config.GoalsDir)
	matchedPath, matchedName, err := g.findFileByName(ctx, goalsDir, name)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "find goal file")
	}
	return g.readGoalFromPath(ctx, matchedPath, matchedName)
}
