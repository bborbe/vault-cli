// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

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

	switch key {
	case "phase":
		return task.Phase, nil
	case "claude_session_id":
		return task.ClaudeSessionID, nil
	case "assignee":
		return task.Assignee, nil
	case "status":
		return string(task.Status), nil
	case "priority":
		return strconv.Itoa(int(task.Priority)), nil
	case "defer_date":
		if task.DeferDate != nil {
			return task.DeferDate.Format("2006-01-02"), nil
		}
		return "", nil
	default:
		return "", fmt.Errorf("unknown field: %s", key)
	}
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

	switch key {
	case "phase":
		task.Phase = value
	case "claude_session_id":
		task.ClaudeSessionID = value
	case "assignee":
		task.Assignee = value
	case "status":
		// Validate status value
		validStatuses := []domain.TaskStatus{
			domain.TaskStatusTodo,
			domain.TaskStatusInProgress,
			domain.TaskStatusBacklog,
			domain.TaskStatusCompleted,
			domain.TaskStatusHold,
			domain.TaskStatusAborted,
		}
		isValid := false
		for _, valid := range validStatuses {
			if value == string(valid) {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.Wrap(
				ctx,
				fmt.Errorf("invalid status value: %s", value),
				"expected one of: todo, in_progress, backlog, completed, hold, aborted",
			)
		}
		task.Status = domain.TaskStatus(value)
	case "priority":
		p, err := strconv.Atoi(value)
		if err != nil {
			return errors.Wrap(ctx, err, "invalid priority value (expected integer)")
		}
		task.Priority = domain.Priority(p)
	case "defer_date":
		if value == "" {
			task.DeferDate = nil
		} else {
			t, err := time.Parse("2006-01-02", value)
			if err != nil {
				return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
			}
			d := libtime.ToDate(t)
			task.DeferDate = d.Ptr()
		}
	default:
		return fmt.Errorf("unknown field: %s", key)
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

// Execute clears (zeros) the value of a frontmatter field on a task.
func (o *frontmatterClearOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key string,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	switch key {
	case "phase":
		task.Phase = ""
	case "claude_session_id":
		task.ClaudeSessionID = ""
	case "assignee":
		task.Assignee = ""
	case "status":
		task.Status = ""
	case "priority":
		task.Priority = 0
	case "defer_date":
		task.DeferDate = nil
	default:
		return fmt.Errorf("unknown field: %s", key)
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	return nil
}
