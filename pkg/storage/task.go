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
		return t.readTaskFromPath(ctx, filePath, taskID.String(), vaultPath)
	}
	tasksDir := filepath.Join(vaultPath, t.config.TasksDir)
	matchedPath, matchedName, err := t.findFileByName(ctx, tasksDir, taskID.String())
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("find task %s", taskID))
	}
	return t.readTaskFromPath(ctx, matchedPath, matchedName, vaultPath)
}

// WriteTask writes a task to a markdown file.
func (t *taskStorage) WriteTask(ctx context.Context, task *domain.Task) error {
	if task.TaskIdentifier() == "" {
		task.SetTaskIdentifier(uuid.New().String())
	}

	if isSymlink(task.FilePath) {
		return errors.Errorf(ctx, "refusing to write through symlink: %s", task.FilePath)
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
	return t.readTaskFromPath(ctx, matchedPath, matchedName, vaultPath)
}

// ListTasks returns all tasks from the vault, including subdirectories.
// An unreadable task file is skipped rather than failing the listing.
func (t *taskStorage) ListTasks(
	ctx context.Context,
	vaultPath string,
) ([]*domain.Task, error) {
	var tasks []*domain.Task
	if err := t.walkTaskFiles(
		ctx,
		vaultPath,
		func(task *domain.Task, name, _ string, err error) error {
			if err != nil {
				slog.Debug("skipping unreadable task", "file", name, "error", err)
				return nil
			}
			tasks = append(tasks, task)
			return nil
		},
	); err != nil {
		return nil, err
	}

	return tasks, nil
}

// ListTasksStrict returns all tasks from the vault, including subdirectories.
// Unlike ListTasks it does not skip an unreadable task file: the error is
// returned instead, naming the file, so a caller that must not report a
// silently smaller set fails loudly.
func (t *taskStorage) ListTasksStrict(
	ctx context.Context,
	vaultPath string,
) ([]*domain.Task, error) {
	var tasks []*domain.Task
	if err := t.walkTaskFiles(
		ctx,
		vaultPath,
		func(task *domain.Task, _, path string, err error) error {
			if err != nil {
				return errors.Wrapf(ctx, err, "read task file %s", path)
			}
			tasks = append(tasks, task)
			return nil
		},
	); err != nil {
		return nil, err
	}

	return tasks, nil
}

// walkTaskFiles walks the vault's configured tasks_dir recursively and calls
// onFile for every *.md file found, with the filename stem as name. A per-file
// read error is handed to onFile as err rather than handled here, so ListTasks
// and ListTasksStrict can differ only in what they do with it. A walk error
// (a missing or unreadable directory) is returned wrapped, naming the directory.
func (t *taskStorage) walkTaskFiles(
	ctx context.Context,
	vaultPath string,
	onFile func(task *domain.Task, name string, path string, err error) error,
) error {
	tasksDir := filepath.Join(vaultPath, t.config.TasksDir)

	err := filepath.WalkDir(tasksDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return errors.Wrapf(ctx, err, "walk tasks dir")
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		fileName := strings.TrimSuffix(d.Name(), ".md")
		task, readErr := t.readTaskFromPath(ctx, path, fileName, vaultPath)
		return onFile(task, fileName, path, readErr)
	})
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("walk tasks directory %s", tasksDir))
	}

	return nil
}
