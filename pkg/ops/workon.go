// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/workon-operation.go --fake-name WorkOnOperation . WorkOnOperation
type WorkOnOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		assignee string,
		vaultName string,
		outputFormat string,
	) error
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
	vaultName string,
	outputFormat string,
) error {
	// Find and read the task
	task, err := w.storage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		if outputFormat == "json" {
			result := MutationResult{
				Success: false,
				Error:   err.Error(),
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		}
		return errors.Wrap(ctx, err, "find task")
	}

	// Update task status to in_progress and set assignee
	task.Status = domain.TaskStatusInProgress
	task.Assignee = assignee

	// Write updated task
	if err := w.storage.WriteTask(ctx, task); err != nil {
		if outputFormat == "json" {
			result := MutationResult{
				Success: false,
				Error:   err.Error(),
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		}
		return errors.Wrap(ctx, err, "write task")
	}

	if outputFormat == "json" {
		result := MutationResult{
			Success: true,
			Name:    task.Name,
			Vault:   vaultName,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("✅ Now working on: %s (assigned to %s)\n", task.Name, assignee)
	return nil
}
