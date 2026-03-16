// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

type taskStorage struct {
	*baseStorage
}

// ReadTask reads a task from a markdown file.
func (t *taskStorage) ReadTask(
	ctx context.Context,
	vaultPath string,
	taskID domain.TaskID,
) (*domain.Task, error) {
	filePath := filepath.Join(vaultPath, t.config.TasksDir, taskID.String()+".md")
	return t.readTaskFromPath(ctx, filePath, taskID.String())
}

// WriteTask writes a task to a markdown file.
func (t *taskStorage) WriteTask(ctx context.Context, task *domain.Task) error {
	content, err := t.serializeWithFrontmatter(task, task.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(task.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", task.FilePath))
	}

	return nil
}

// FindTaskByName searches for a task by name in the vault.
func (t *taskStorage) FindTaskByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Task, error) {
	tasksDir := filepath.Join(vaultPath, t.config.TasksDir)
	matchedPath, matchedName, err := t.findFileByName(tasksDir, name)
	if err != nil {
		return nil, err
	}
	return t.readTaskFromPath(ctx, matchedPath, matchedName)
}

// ListTasks returns all tasks from the vault.
func (t *taskStorage) ListTasks(
	ctx context.Context,
	vaultPath string,
) ([]*domain.Task, error) {
	tasksDir := filepath.Join(vaultPath, t.config.TasksDir)

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read tasks directory %s", tasksDir))
	}

	tasks := make([]*domain.Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fileName := strings.TrimSuffix(entry.Name(), ".md")
		filePath := filepath.Join(tasksDir, entry.Name())

		task, err := t.readTaskFromPath(ctx, filePath, fileName)
		if err != nil {
			// Log error but continue with other tasks
			slog.Debug("skipping unreadable task", "file", fileName, "error", err)
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}
