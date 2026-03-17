// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
		outputFormat string,
	) error
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

// ObjectiveCompleteResult represents the JSON result of an objective complete operation.
type ObjectiveCompleteResult struct {
	Success   bool   `json:"success"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	Completed string `json:"completed,omitempty"`
	Vault     string `json:"vault,omitempty"`
	Error     string `json:"error,omitempty"`
}

func outputObjectiveCompleteError(outputFormat string, msg string) {
	if outputFormat == "json" {
		result := ObjectiveCompleteResult{Success: false, Error: msg}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	}
}

// Execute marks an objective as completed.
func (o *objectiveCompleteOperation) Execute(
	ctx context.Context,
	vaultPath string,
	objectiveName string,
	vaultName string,
	outputFormat string,
) error {
	objective, err := o.objectiveStorage.FindObjectiveByName(ctx, vaultPath, objectiveName)
	if err != nil {
		outputObjectiveCompleteError(outputFormat, err.Error())
		return errors.Wrap(ctx, err, "find objective")
	}

	if objective.Status == domain.ObjectiveStatusCompleted {
		outputObjectiveCompleteError(outputFormat,
			fmt.Sprintf("objective %q is already completed", objectiveName))
		return fmt.Errorf("objective %q is already completed", objectiveName) //nolint:goerr113
	}

	objective.Status = domain.ObjectiveStatusCompleted
	objective.Completed = libtime.ToDate(o.currentDateTime.Now().Time()).Ptr()

	if err := o.objectiveStorage.WriteObjective(ctx, objective); err != nil {
		outputObjectiveCompleteError(outputFormat, err.Error())
		return errors.Wrap(ctx, err, "write objective")
	}

	if outputFormat == "json" {
		result := ObjectiveCompleteResult{
			Success:   true,
			Name:      objective.Name,
			Status:    string(objective.Status),
			Completed: objective.Completed.Format("2006-01-02"),
			Vault:     vaultName,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("✅ Objective completed: %s\n", objective.Name)
	return nil
}
