// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/objective-complete-operation.go --fake-name ObjectiveCompleteOperation . ObjectiveCompleteOperation
type ObjectiveCompleteOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		objectiveName string,
		vaultName string,
	) (MutationResult, error)
}

// NewObjectiveCompleteOperation creates a new objective complete operation.
func NewObjectiveCompleteOperation(
	objectiveStorage storage.ObjectiveStorage,
	currentDateTime libtime.CurrentDateTime,
) ObjectiveCompleteOperation {
	return &objectiveCompleteOperation{
		objectiveStorage: objectiveStorage,
		currentDateTime:  currentDateTime,
	}
}

type objectiveCompleteOperation struct {
	objectiveStorage storage.ObjectiveStorage
	currentDateTime  libtime.CurrentDateTime
}

// Execute marks an objective as completed.
func (o *objectiveCompleteOperation) Execute(
	ctx context.Context,
	vaultPath string,
	objectiveName string,
	vaultName string,
) (MutationResult, error) {
	objective, err := o.objectiveStorage.FindObjectiveByName(ctx, vaultPath, objectiveName)
	if err != nil {
		return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(
				ctx,
				err,
				"find objective",
			)
	}

	if objective.Status() == domain.ObjectiveStatusCompleted {
		msg := fmt.Sprintf("objective %q is already completed", objectiveName)
		return MutationResult{Success: false, Error: msg}, fmt.Errorf("%s", msg) //nolint:goerr113
	}

	_ = objective.SetStatus(domain.ObjectiveStatusCompleted)
	objective.SetCompleted(libtime.ToDate(o.currentDateTime.Now().Time()).Ptr())

	if err := o.objectiveStorage.WriteObjective(ctx, objective); err != nil {
		return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(
				ctx,
				err,
				"write objective",
			)
	}

	return MutationResult{Success: true, Name: objective.Name, Vault: vaultName}, nil
}
