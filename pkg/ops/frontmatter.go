// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"time"

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
	Execute(ctx context.Context, vaultPath, taskName, key, value string) error
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
	vaultPath, taskName, key, value string,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	if err := task.SetField(ctx, key, value); err != nil {
		return errors.Wrap(ctx, err, "set field")
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
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

// formatDateOrDateTime serializes a DateOrDateTime to YYYY-MM-DD for date-only values
// (midnight UTC) and RFC3339 for values with a time component. Returns empty string for nil.
func formatDateOrDateTime(d *domain.DateOrDateTime) string {
	if d == nil {
		return ""
	}
	t := d.Time().UTC()
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return d.Time().Format(time.DateOnly)
	}
	return d.Time().Format(time.RFC3339)
}
