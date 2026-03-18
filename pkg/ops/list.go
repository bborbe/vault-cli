// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/list-operation.go --fake-name ListOperation . ListOperation
type ListOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		vaultName string,
		pagesDir string,
		statusFilters []string,
		showAll bool,
		assigneeFilter string,
		goalFilter string,
		outputFormat string,
	) error
}

// NewListOperation creates a new list operation.
func NewListOperation(
	pageStorage storage.PageStorage,
) ListOperation {
	return &listOperation{
		pageStorage: pageStorage,
	}
}

type listOperation struct {
	pageStorage storage.PageStorage
}

// TaskListItem represents a task in list output.
type TaskListItem struct {
	Name            string `json:"name"`
	Status          string `json:"status"`
	Assignee        string `json:"assignee,omitempty"`
	Priority        int    `json:"priority,omitempty"`
	Vault           string `json:"vault"`
	Category        string `json:"category,omitempty"`
	Recurring       string `json:"recurring,omitempty"`
	DeferDate       string `json:"defer_date,omitempty"`
	PlannedDate     string `json:"planned_date,omitempty"`
	DueDate         string `json:"due_date,omitempty"`
	ClaudeSessionID string `json:"claude_session_id,omitempty"`
	Phase           string `json:"phase,omitempty"`
	ModifiedDate    string `json:"modified_date,omitempty"`
	CompletedDate   string `json:"completed_date,omitempty"`
}

// Execute lists tasks from the vault, optionally filtered by status, assignee, and goal.
func (l *listOperation) Execute(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	pagesDir string,
	statusFilters []string,
	showAll bool,
	assigneeFilter string,
	goalFilter string,
	outputFormat string,
) error {
	// Read all tasks/pages from the specified directory
	tasks, err := l.pageStorage.ListPages(ctx, vaultPath, pagesDir)
	if err != nil {
		return errors.Wrap(ctx, err, "list pages")
	}

	// Filter tasks by status, assignee, and goal
	filteredTasks := filterTasks(tasks, statusFilters, showAll, assigneeFilter, goalFilter)

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

	// Output tasks based on format
	if outputFormat == "json" {
		items := make([]TaskListItem, len(filteredTasks))
		for i, task := range filteredTasks {
			items[i] = TaskListItem{
				Name:            task.Name,
				Status:          string(task.Status),
				Assignee:        task.Assignee,
				Priority:        int(task.Priority),
				Vault:           vaultName,
				Category:        task.PageType,
				Recurring:       task.Recurring,
				ClaudeSessionID: task.ClaudeSessionID,
				Phase:           task.Phase,
			}
			items[i].DeferDate = formatDateOrDateTime(task.DeferDate)
			items[i].PlannedDate = formatDateOrDateTime(task.PlannedDate)
			items[i].DueDate = formatDateOrDateTime(task.DueDate)
			if task.ModifiedDate != nil {
				items[i].ModifiedDate = task.ModifiedDate.UTC().Format("2006-01-02T15:04:05Z")
			}
			items[i].CompletedDate = task.CompletedDate
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	// Plain output
	for _, task := range filteredTasks {
		fmt.Printf("[%s] %s\n", task.Status, task.Name)
	}

	return nil
}

// filterTasks filters tasks by status, assignee, and goal.
func filterTasks(
	tasks []*domain.Task,
	statusFilters []string,
	showAll bool,
	assigneeFilter string,
	goalFilter string,
) []*domain.Task {
	filteredTasks := make([]*domain.Task, 0, len(tasks))
	for _, task := range tasks {
		if !shouldIncludeTask(task, statusFilters, showAll, assigneeFilter, goalFilter) {
			continue
		}
		filteredTasks = append(filteredTasks, task)
	}
	return filteredTasks
}

// shouldIncludeTask determines if a task should be included based on filters.
func shouldIncludeTask(
	task *domain.Task,
	statusFilters []string,
	showAll bool,
	assigneeFilter string,
	goalFilter string,
) bool {
	// Filter by assignee if specified
	if assigneeFilter != "" && task.Assignee != assigneeFilter {
		return false
	}

	// Filter by goal if specified (exact, case-sensitive match)
	if goalFilter != "" && !taskHasGoal(task.Goals, goalFilter) {
		return false
	}

	// Skip status filtering if showAll is true
	if showAll {
		return true
	}

	// Apply status filter
	return matchesStatusFilter(task.Status, statusFilters)
}

// taskHasGoal returns true if the goals list contains the given goal name.
func taskHasGoal(goals []string, goal string) bool {
	for _, g := range goals {
		if g == goal {
			return true
		}
	}
	return false
}

// matchesStatusFilter checks if task status matches any of the filters.
func matchesStatusFilter(status domain.TaskStatus, filters []string) bool {
	if len(filters) > 0 {
		for _, f := range filters {
			if strings.EqualFold(string(status), f) {
				return true
			}
		}
		return false
	}
	// Default: show only todo and in_progress
	return status == domain.TaskStatusTodo || status == domain.TaskStatusInProgress
}

// statusPriority returns a numeric priority for sorting task statuses.
func statusPriority(status domain.TaskStatus) int {
	switch status {
	case domain.TaskStatusInProgress:
		return 1
	case domain.TaskStatusTodo:
		return 2
	case domain.TaskStatusHold:
		return 3
	case domain.TaskStatusBacklog:
		return 4
	case domain.TaskStatusCompleted:
		return 5
	case domain.TaskStatusAborted:
		return 6
	default:
		return 99
	}
}
