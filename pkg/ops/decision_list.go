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

//counterfeiter:generate -o ../../mocks/decision-list-operation.go --fake-name DecisionListOperation . DecisionListOperation
type DecisionListOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		vaultName string,
		showReviewed bool,
		showAll bool,
		outputFormat string,
	) error
}

// NewDecisionListOperation creates a new decision list operation.
func NewDecisionListOperation(storage storage.Storage) DecisionListOperation {
	return &decisionListOperation{storage: storage}
}

type decisionListOperation struct {
	storage storage.Storage
}

// DecisionListItem represents a decision in list output.
type DecisionListItem struct {
	Name         string `json:"name"`
	Reviewed     bool   `json:"reviewed"`
	ReviewedDate string `json:"reviewed_date,omitempty"`
	Status       string `json:"status,omitempty"`
	Type         string `json:"type,omitempty"`
	PageType     string `json:"page_type,omitempty"`
	Vault        string `json:"vault"`
}

// Execute lists decisions from the vault filtered by review status.
func (d *decisionListOperation) Execute(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	showReviewed bool,
	showAll bool,
	outputFormat string,
) error {
	decisions, err := d.storage.ListDecisions(ctx, vaultPath)
	if err != nil {
		return errors.Wrap(ctx, err, "list decisions")
	}

	filtered := filterDecisions(decisions, showReviewed, showAll)

	sort.Slice(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
	})

	if outputFormat == "json" {
		items := make([]DecisionListItem, 0, len(filtered))
		for _, dec := range filtered {
			items = append(items, DecisionListItem{
				Name:         dec.Name,
				Reviewed:     dec.Reviewed,
				ReviewedDate: dec.ReviewedDate,
				Status:       dec.Status,
				Type:         dec.Type,
				PageType:     dec.PageType,
				Vault:        vaultName,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	for _, dec := range filtered {
		reviewStatus := "unreviewed"
		if dec.Reviewed {
			reviewStatus = "reviewed"
		}
		fmt.Printf("[%s] %s\n", reviewStatus, dec.Name)
	}

	return nil
}

func filterDecisions(
	decisions []*domain.Decision,
	showReviewed bool,
	showAll bool,
) []*domain.Decision {
	result := make([]*domain.Decision, 0, len(decisions))
	for _, dec := range decisions {
		if showAll {
			result = append(result, dec)
			continue
		}
		if showReviewed && dec.Reviewed {
			result = append(result, dec)
			continue
		}
		if !showReviewed && !dec.Reviewed {
			result = append(result, dec)
		}
	}
	return result
}
