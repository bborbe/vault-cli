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

	"github.com/bborbe/vault-cli/pkg/domain"
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
	Flag            bool     `json:"flag,omitempty"`
	FlagSetBy       string   `json:"flag_set_by,omitempty"`
	FlagSetAt       string   `json:"flag_set_at,omitempty"`
	// OpenQuestions lists the task's Open Questions section items in section
	// order. It is always non-nil so `--output json` emits `[]`, not `null`, for
	// a task with no such section.
	OpenQuestions []domain.OpenQuestion `json:"open_questions"`
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
		Status: string(task.Status()),
		Phase: func() string {
			if task.Phase() != nil {
				return task.Phase().String()
			}
			return ""
		}(),
		Assignee:        task.Assignee(),
		Priority:        int(task.Priority()),
		Category:        task.PageType(),
		Recurring:       task.Recurring(),
		ClaudeSessionID: task.ClaudeSessionID(),
		Goals:           task.Goals(),
		Content:         string(task.Content),
		FilePath:        task.FilePath,
		Vault:           vaultName,
		Flag:            task.Flag(),
		FlagSetBy:       task.GetString("flag_set_by"),
		FlagSetAt:       formatFlagSetAt(task.FrontmatterMap),
	}

	// Parse the section from the content already in hand. Going back through the
	// storage interface would call FindTaskByName a second time — a full vault
	// walk plus a frontmatter parse on the CLI's most-used read path — and would
	// let the questions describe a different file version than Content above if
	// the file were written between the two reads. ParseOpenQuestions always
	// returns a non-nil slice, which is the shape the JSON contract promises.
	items := storage.ParseOpenQuestions(ctx, string(task.Content))
	openQuestions := make([]domain.OpenQuestion, 0, len(items))
	for _, item := range items {
		openQuestions = append(
			openQuestions,
			domain.OpenQuestion{Index: item.Index, Text: item.Text},
		)
	}
	detail.OpenQuestions = openQuestions

	if d := task.DeferDate(); d != nil {
		detail.DeferDate = d.String()
	}
	if d := task.PlannedDate(); d != nil {
		detail.PlannedDate = d.String()
	}
	if d := task.DueDate(); d != nil {
		detail.DueDate = d.String()
	}
	if d := task.CompletedDate(); d != nil {
		detail.CompletedDate = d.String()
	}

	// Extract description from body content
	if matches := showFrontmatterRegex.FindStringSubmatch(string(task.Content)); len(matches) >= 2 {
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
