// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/list-operation.go --fake-name ListOperation . ListOperation
type ListOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		statusFilter []domain.TaskStatus,
		showAll bool,
	) error
}

// NewListOperation creates a new list operation.
func NewListOperation(
	storage storage.Storage,
) ListOperation {
	return &listOperation{
		storage: storage,
	}
}

type listOperation struct {
	storage storage.Storage
}

// Execute lists tasks from the vault, optionally filtered by status.
func (l *listOperation) Execute(
	ctx context.Context,
	vaultPath string,
	statusFilter []domain.TaskStatus,
	showAll bool,
) error {
	// Read all tasks
	tasks, err := l.storage.ListTasks(ctx, vaultPath)
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	// Determine which statuses to show
	var statusesToShow map[domain.TaskStatus]bool
	if !showAll {
		if len(statusFilter) > 0 {
			statusesToShow = make(map[domain.TaskStatus]bool)
			for _, status := range statusFilter {
				statusesToShow[status] = true
			}
		} else {
			// Default: show only todo and in_progress
			statusesToShow = map[domain.TaskStatus]bool{
				domain.TaskStatusTodo:       true,
				domain.TaskStatusInProgress: true,
			}
		}
	}

	// Filter tasks by status
	var filteredTasks []*domain.Task
	for _, task := range tasks {
		if showAll || statusesToShow[task.Status] {
			filteredTasks = append(filteredTasks, task)
		}
	}

	// Sort tasks: in_progress first, then todo, then alphabetically within each group
	sort.Slice(filteredTasks, func(i, j int) bool {
		taskI := filteredTasks[i]
		taskJ := filteredTasks[j]

		// Sort by status priority
		if taskI.Status != taskJ.Status {
			return statusPriority(taskI.Status) < statusPriority(taskJ.Status)
		}

		// Within same status, sort alphabetically by name
		return strings.ToLower(taskI.Name) < strings.ToLower(taskJ.Name)
	})

	// Output tasks
	for _, task := range filteredTasks {
		fmt.Printf("[%s] %s\n", task.Status, task.Name)
	}

	return nil
}

// statusPriority returns a numeric priority for sorting task statuses.
func statusPriority(status domain.TaskStatus) int {
	switch status {
	case domain.TaskStatusInProgress:
		return 1
	case domain.TaskStatusTodo:
		return 2
	case domain.TaskStatusDeferred:
		return 3
	case domain.TaskStatusDone:
		return 4
	default:
		return 99
	}
}
