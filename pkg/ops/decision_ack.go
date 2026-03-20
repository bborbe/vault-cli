// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/decision-ack-operation.go --fake-name DecisionAckOperation . DecisionAckOperation
type DecisionAckOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		vaultName string,
		decisionName string,
		statusOverride string,
	) (MutationResult, error)
}

// NewDecisionAckOperation creates a new decision ack operation.
func NewDecisionAckOperation(
	decisionStorage storage.DecisionStorage,
	currentDateTime libtime.CurrentDateTime,
) DecisionAckOperation {
	return &decisionAckOperation{
		decisionStorage: decisionStorage,
		currentDateTime: currentDateTime,
	}
}

type decisionAckOperation struct {
	decisionStorage storage.DecisionStorage
	currentDateTime libtime.CurrentDateTime
}

// Execute finds a decision by name, marks it as reviewed, and optionally updates its status.
func (d *decisionAckOperation) Execute(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	decisionName string,
	statusOverride string,
) (MutationResult, error) {
	decision, err := d.decisionStorage.FindDecisionByName(ctx, vaultPath, decisionName)
	if err != nil {
		return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(
				ctx,
				err,
				"find decision",
			)
	}

	decision.Reviewed = true
	decision.ReviewedDate = d.currentDateTime.Now().Format("2006-01-02")

	if statusOverride != "" {
		decision.Status = statusOverride
	}

	if err := d.decisionStorage.WriteDecision(ctx, decision); err != nil {
		return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(
				ctx,
				err,
				"write decision",
			)
	}

	return MutationResult{
		Success: true,
		Name:    decision.Name,
		Vault:   vaultName,
	}, nil
}
