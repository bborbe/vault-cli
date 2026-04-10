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
	"github.com/google/uuid"

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
	if _, err := os.Stat(filePath); err == nil {
		return t.readTaskFromPath(ctx, filePath, taskID.String())
	}
	tasksDir := filepath.Join(vaultPath, t.config.TasksDir)
	matchedPath, matchedName, err := t.findFileByName(ctx, tasksDir, taskID.String())
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("find task %s", taskID))
	}
	return t.readTaskFromPath(ctx, matchedPath, matchedName)
}

// WriteTask writes a task to a markdown file.
func (t *taskStorage) WriteTask(ctx context.Context, task *domain.Task) error {
	if task.TaskIdentifier() == "" {
		task.SetTaskIdentifier(uuid.New().String())
	}

	content, err := t.serializeMapAsFrontmatter(ctx, task.RawMap(), string(task.Content))
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(task.FilePath, []byte(content), 0600); err != nil { //#nosec G306 -- task files require 0600
		return errors.Wrapf(ctx, err, "write file %s", task.FilePath)
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
	matchedPath, matchedName, err := t.findFileByName(ctx, tasksDir, name)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "find task file")
	}
	return t.readTaskFromPath(ctx, matchedPath, matchedName)
}

// ListTasks returns all tasks from the vault, including subdirectories.
func (t *taskStorage) ListTasks(
	ctx context.Context,
	vaultPath string,
) ([]*domain.Task, error) {
	tasksDir := filepath.Join(vaultPath, t.config.TasksDir)

	var tasks []*domain.Task
	err := filepath.WalkDir(tasksDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		fileName := strings.TrimSuffix(d.Name(), ".md")
		task, err := t.readTaskFromPath(ctx, path, fileName)
		if err != nil {
			slog.Debug("skipping unreadable task", "file", fileName, "error", err)
			return nil
		}
		tasks = append(tasks, task)
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("walk tasks directory %s", tasksDir))
	}

	return tasks, nil
}
