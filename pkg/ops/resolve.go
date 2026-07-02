// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// ResolveOperation resolves a name to a task or goal for a single vault.
//
//counterfeiter:generate -o ../../mocks/resolve-operation.go --fake-name ResolveOperation . ResolveOperation
type ResolveOperation interface {
	Execute(ctx context.Context, vaultPath string, name string) (domain.ResolveResult, error)
}

// NewResolveOperation creates a new resolve operation.
func NewResolveOperation(
	taskStorage storage.TaskStorage,
	goalStorage storage.GoalStorage,
) ResolveOperation {
	return &resolveOperation{
		taskStorage: taskStorage,
		goalStorage: goalStorage,
	}
}

type resolveOperation struct {
	taskStorage storage.TaskStorage
	goalStorage storage.GoalStorage
}

// Execute resolves a name to a task or goal, probing task storage first then goal storage.
// A name matching both a task and a goal resolves to "task" (task-first priority).
// When the name matches neither, it returns a not-found result with no error.
func (o *resolveOperation) Execute(
	ctx context.Context,
	vaultPath string,
	name string,
) (domain.ResolveResult, error) {
	// Task-first: a name matching both a task and a goal resolves to "task".
	if _, err := o.taskStorage.FindTaskByName(ctx, vaultPath, name); err == nil {
		return domain.ResolveResult{Type: "task", Name: name, Found: true}, nil
	}
	if _, err := o.goalStorage.FindGoalByName(ctx, vaultPath, name); err == nil {
		return domain.ResolveResult{Type: "goal", Name: name, Found: true}, nil
	}
	return domain.ResolveResult{Type: "", Name: name, Found: false}, nil
}
