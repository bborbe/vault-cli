// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
	case "planned_date":
		if task.PlannedDate != nil {
			return task.PlannedDate.Format("2006-01-02"), nil
		}
		return "", nil
	case "recurring":
		return task.Recurring, nil
	case "last_completed":
		return task.LastCompleted, nil
	case "page_type":
		return task.PageType, nil
	case "goals":
		return strings.Join(task.Goals, ","), nil
	case "tags":
		return strings.Join(task.Tags, ","), nil
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
		status, err := parseTaskStatus(ctx, value)
		if err != nil {
			return err
		}
		task.Status = status
	case "priority":
		p, err := strconv.Atoi(value)
		if err != nil {
			return errors.Wrap(ctx, err, "invalid priority value (expected integer)")
		}
		task.Priority = domain.Priority(p)
	case "defer_date":
		d, err := parseDatePtr(ctx, value)
		if err != nil {
			return err
		}
		task.DeferDate = d
	case "planned_date":
		d, err := parseDatePtr(ctx, value)
		if err != nil {
			return err
		}
		task.PlannedDate = d
	case "recurring":
		task.Recurring = value
	case "last_completed":
		task.LastCompleted = value
	case "page_type":
		task.PageType = value
	case "goals":
		task.Goals = parseStringSlice(value)
	case "tags":
		task.Tags = parseStringSlice(value)
	default:
		return fmt.Errorf("unknown field: %s", key)
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	return nil
}

func parseTaskStatus(ctx context.Context, value string) (domain.TaskStatus, error) {
	validStatuses := []domain.TaskStatus{
		domain.TaskStatusTodo,
		domain.TaskStatusInProgress,
		domain.TaskStatusBacklog,
		domain.TaskStatusCompleted,
		domain.TaskStatusHold,
		domain.TaskStatusAborted,
	}
	for _, valid := range validStatuses {
		if value == string(valid) {
			return domain.TaskStatus(value), nil
		}
	}
	return "", errors.Wrap(
		ctx,
		fmt.Errorf("invalid status value: %s", value),
		"expected one of: todo, in_progress, backlog, completed, hold, aborted",
	)
}

func parseDatePtr(ctx context.Context, value string) (*libtime.Date, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
	}
	d := libtime.ToDate(t)
	return d.Ptr(), nil
}

func parseStringSlice(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
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
	case "planned_date":
		task.PlannedDate = nil
	case "recurring":
		task.Recurring = ""
	case "last_completed":
		task.LastCompleted = ""
	case "page_type":
		task.PageType = ""
	case "goals":
		task.Goals = nil
	case "tags":
		task.Tags = nil
	default:
		return fmt.Errorf("unknown field: %s", key)
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	return nil
}
