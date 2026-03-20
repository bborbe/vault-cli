// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/storage"
)

// ShowOperation returns full detail for a single task.
//
//counterfeiter:generate -o ../../mocks/show-operation.go --fake-name ShowOperation . ShowOperation
type ShowOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		vaultName string,
		taskName string,
	) (TaskDetail, error)
}

// NewShowOperation creates a new show operation.
func NewShowOperation(taskStorage storage.TaskStorage) ShowOperation {
	return &showOperation{
		taskStorage: taskStorage,
	}
}

type showOperation struct {
	taskStorage storage.TaskStorage
}

// TaskDetail contains full task information for JSON output.
type TaskDetail struct {
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	Phase           string   `json:"phase,omitempty"`
	Assignee        string   `json:"assignee,omitempty"`
	Priority        int      `json:"priority,omitempty"`
	Category        string   `json:"category,omitempty"`
	Recurring       string   `json:"recurring,omitempty"`
	DeferDate       string   `json:"defer_date,omitempty"`
	PlannedDate     string   `json:"planned_date,omitempty"`
	DueDate         string   `json:"due_date,omitempty"`
	ClaudeSessionID string   `json:"claude_session_id,omitempty"`
	Goals           []string `json:"goals,omitempty"`
	Description     string   `json:"description,omitempty"`
	Content         string   `json:"content"`
	ModifiedDate    string   `json:"modified_date,omitempty"`
	CompletedDate   string   `json:"completed_date,omitempty"`
	FilePath        string   `json:"file_path"`
	Vault           string   `json:"vault"`
}

var (
	showFrontmatterRegex = regexp.MustCompile(`(?s)^---\n.*?\n---\n(.*)$`)
	markdownStripRegex   = regexp.MustCompile(`[#*_\[\]` + "`" + `]`)
)

// Execute finds a task by name and returns its full detail.
func (o *showOperation) Execute(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	taskName string,
) (TaskDetail, error) {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return TaskDetail{}, errors.Wrap(ctx, err, "find task")
	}

	detail := TaskDetail{
		Name:   task.Name,
		Status: string(task.Status),
		Phase: func() string {
			if task.Phase != nil {
				return task.Phase.String()
			}
			return ""
		}(),
		Assignee:        task.Assignee,
		Priority:        int(task.Priority),
		Category:        task.PageType,
		Recurring:       task.Recurring,
		ClaudeSessionID: task.ClaudeSessionID,
		Goals:           task.Goals,
		Content:         task.Content,
		FilePath:        task.FilePath,
		Vault:           vaultName,
	}

	detail.DeferDate = formatDateOrDateTime(task.DeferDate)
	detail.PlannedDate = formatDateOrDateTime(task.PlannedDate)
	detail.DueDate = formatDateOrDateTime(task.DueDate)
	detail.CompletedDate = task.CompletedDate

	// Extract description from body content
	if matches := showFrontmatterRegex.FindStringSubmatch(task.Content); len(matches) >= 2 {
		body := strings.TrimSpace(matches[1])
		stripped := markdownStripRegex.ReplaceAllString(body, "")
		stripped = strings.Join(strings.Fields(stripped), " ")
		if len(stripped) > 200 {
			stripped = stripped[:200]
		}
		detail.Description = stripped
	}

	// Get file modification time
	if info, statErr := os.Stat(task.FilePath); statErr == nil {
		detail.ModifiedDate = info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
	}

	return detail, nil
}
