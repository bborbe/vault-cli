// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/frontmatter-get-operation.go --fake-name FrontmatterGetOperation . FrontmatterGetOperation
type FrontmatterGetOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, key string) (string, error)
}

// NewFrontmatterGetOperation creates a new frontmatter get operation.
func NewFrontmatterGetOperation(taskStorage storage.TaskStorage) FrontmatterGetOperation {
	return &frontmatterGetOperation{
		taskStorage: taskStorage,
	}
}

type frontmatterGetOperation struct {
	taskStorage storage.TaskStorage
}

// Execute retrieves the value of a frontmatter field from a task.
func (o *frontmatterGetOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key string,
) (string, error) {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return "", errors.Wrap(ctx, err, "find task")
	}

	return task.GetField(key), nil
}

//counterfeiter:generate -o ../../mocks/frontmatter-set-operation.go --fake-name FrontmatterSetOperation . FrontmatterSetOperation
type FrontmatterSetOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, key, value, reason, gateSuccessor string) error
}

// NewFrontmatterSetOperation creates a new frontmatter set operation.
func NewFrontmatterSetOperation(taskStorage storage.TaskStorage) FrontmatterSetOperation {
	return &frontmatterSetOperation{
		taskStorage: taskStorage,
	}
}

type frontmatterSetOperation struct {
	taskStorage storage.TaskStorage
}

// Execute sets the value of a frontmatter field on a task.
func (o *frontmatterSetOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key, value, reason, gateSuccessor string,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	// One-step close-out: when this invocation sets a close-out status, persist
	// reason and successor first so both land in a single WriteTask. Fields are
	// written only when provided; non-close-out targets never receive them.
	if err := writeTaskCloseOutFieldsIfCloseOut(ctx, task, value, reason, gateSuccessor); err != nil {
		return err
	}

	if err := task.SetField(ctx, key, value); err != nil {
		if key == "status" && strings.Contains(err.Error(), "missing close-out field(s)") {
			err = errors.Errorf(ctx,
				"%s\nTry: vault-cli task set \"%s\" status %s --reason \"<text>\" --gate-successor \"<successor|none>\"",
				err.Error(), taskName, value)
		}
		return errors.Wrap(ctx, err, "set field")
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	return nil
}

// writeTaskCloseOutFieldsIfCloseOut writes the one-step close-out fields when
// the target status is a close-out (aborted or completed), returning the first
// write error. Fields are written only when provided; non-close-out targets
// never receive them.
func writeTaskCloseOutFieldsIfCloseOut(
	ctx context.Context,
	task *domain.Task,
	value, reason, gateSuccessor string,
) error {
	target, ok := domain.NormalizeTaskStatus(value)
	if !ok || (target != domain.TaskStatusAborted && target != domain.TaskStatusCompleted) {
		return nil
	}
	if reason != "" {
		if err := task.SetField(ctx, "aborted_reason", reason); err != nil {
			return errors.Wrap(ctx, err, "set aborted_reason")
		}
	}
	if gateSuccessor != "" {
		if err := task.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
			return errors.Wrap(ctx, err, "set gate_successor")
		}
	}
	return nil
}

//counterfeiter:generate -o ../../mocks/frontmatter-clear-operation.go --fake-name FrontmatterClearOperation . FrontmatterClearOperation
type FrontmatterClearOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, key string) error
}

// NewFrontmatterClearOperation creates a new frontmatter clear operation.
func NewFrontmatterClearOperation(taskStorage storage.TaskStorage) FrontmatterClearOperation {
	return &frontmatterClearOperation{
		taskStorage: taskStorage,
	}
}

type frontmatterClearOperation struct {
	taskStorage storage.TaskStorage
}

// Execute clears (removes) the value of a frontmatter field on a task.
func (o *frontmatterClearOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key string,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	task.ClearField(key)

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	return nil
}
