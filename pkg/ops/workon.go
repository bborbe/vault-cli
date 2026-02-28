// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/workon-operation.go --fake-name WorkOnOperation . WorkOnOperation
type WorkOnOperation interface {
	Execute(ctx context.Context, vaultPath string, taskName string, assignee string) error
}

// NewWorkOnOperation creates a new work-on operation.
func NewWorkOnOperation(
	storage storage.Storage,
) WorkOnOperation {
	return &workOnOperation{
		storage: storage,
	}
}

type workOnOperation struct {
	storage storage.Storage
}

// Execute marks a task as in_progress and assigns it to the given user.
func (w *workOnOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	assignee string,
) error {
	// Find and read the task
	task, err := w.storage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}

	// Update task status to in_progress and set assignee
	task.Status = domain.TaskStatusInProgress
	task.Assignee = assignee

	// Write updated task
	if err := w.storage.WriteTask(ctx, task); err != nil {
		return fmt.Errorf("write task: %w", err)
	}

	fmt.Printf("✅ Now working on: %s (assigned to %s)\n", task.Name, assignee)
	return nil
}
