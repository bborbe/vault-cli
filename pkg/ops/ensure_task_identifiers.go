// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"log/slog"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/storage"
)

// BackfillResult holds the outcome of an EnsureAllTaskIdentifiers run.
type BackfillResult struct {
	// ModifiedFiles is the list of absolute file paths that were written during backfill.
	ModifiedFiles []string
	// SkippedFiles is the count of files skipped due to errors.
	SkippedFiles int
}

//counterfeiter:generate -o ../../mocks/ensure-all-task-identifiers-operation.go --fake-name EnsureAllTaskIdentifiersOperation . EnsureAllTaskIdentifiersOperation
type EnsureAllTaskIdentifiersOperation interface {
	Execute(ctx context.Context, vaultPath string) (BackfillResult, error)
}

// NewEnsureAllTaskIdentifiersOperation creates a new backfill operation.
func NewEnsureAllTaskIdentifiersOperation(
	taskStorage storage.TaskStorage,
) EnsureAllTaskIdentifiersOperation {
	return &ensureAllTaskIdentifiersOperation{
		taskStorage: taskStorage,
	}
}

type ensureAllTaskIdentifiersOperation struct {
	taskStorage storage.TaskStorage
}

// Execute walks all tasks in vaultPath and writes back any task missing task_identifier.
// Tasks that already have task_identifier are skipped. Unparseable files are skipped
// with a warning. Returns the list of file paths that were modified.
func (e *ensureAllTaskIdentifiersOperation) Execute(
	ctx context.Context,
	vaultPath string,
) (BackfillResult, error) {
	tasks, err := e.taskStorage.ListTasks(ctx, vaultPath)
	if err != nil {
		return BackfillResult{}, errors.Wrap(ctx, err, "list tasks")
	}

	var result BackfillResult
	for _, task := range tasks {
		if task.TaskIdentifier != "" {
			continue // Already has an identifier, skip
		}

		// WriteTask auto-generates the UUID when TaskIdentifier is empty.
		if writeErr := e.taskStorage.WriteTask(ctx, task); writeErr != nil {
			slog.Warn("backfill: skipping task write error",
				"file", task.FilePath,
				"error", writeErr,
			)
			result.SkippedFiles++
			continue
		}

		result.ModifiedFiles = append(result.ModifiedFiles, task.FilePath)
	}

	return result, nil
}
