// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
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
	) ([]DecisionListItem, error)
}

// NewDecisionListOperation creates a new decision list operation.
func NewDecisionListOperation(decisionStorage storage.DecisionStorage) DecisionListOperation {
	return &decisionListOperation{decisionStorage: decisionStorage}
}

type decisionListOperation struct {
	decisionStorage storage.DecisionStorage
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
) ([]DecisionListItem, error) {
	decisions, err := d.decisionStorage.ListDecisions(ctx, vaultPath)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "list decisions")
	}

	filtered := filterDecisions(decisions, showReviewed, showAll)

	sort.Slice(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
	})

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
	return items, nil
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
