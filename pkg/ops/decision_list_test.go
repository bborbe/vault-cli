// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("DecisionListOperation", func() {
	var ctx context.Context
	var err error
	var items []ops.DecisionListItem
	var decisionListOp ops.DecisionListOperation
	var mockDecisionStorage *mocks.DecisionStorage
	var vaultPath string
	var vaultName string
	var showReviewed bool
	var showAll bool
	var decisions []*domain.Decision

	BeforeEach(func() {
		ctx = context.Background()
		mockDecisionStorage = &mocks.DecisionStorage{}
		decisionListOp = ops.NewDecisionListOperation(mockDecisionStorage)
		vaultPath = "/path/to/vault"
		vaultName = "test-vault"
		showReviewed = false
		showAll = false

		decisions = []*domain.Decision{
			{
				Name:     "decisions/alpha",
				Reviewed: false,
				Status:   "pending",
			},
			{
				Name:     "decisions/beta",
				Reviewed: true,
				Status:   "approved",
			},
			{
				Name:     "decisions/gamma",
				Reviewed: false,
			},
		}
		mockDecisionStorage.ListDecisionsReturns(decisions, nil)
	})

	JustBeforeEach(func() {
		items, err = decisionListOp.Execute(
			ctx,
			vaultPath,
			vaultName,
			showReviewed,
			showAll,
		)
	})

	Context("default filter (unreviewed only)", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("calls ListDecisions with correct args", func() {
			Expect(mockDecisionStorage.ListDecisionsCallCount()).To(Equal(1))
			actualCtx, actualVaultPath := mockDecisionStorage.ListDecisionsArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
		})

		It("returns only unreviewed decisions", func() {
			Expect(items).To(HaveLen(2))
			for _, item := range items {
				Expect(item.Reviewed).To(BeFalse())
			}
		})
	})

	Context("with showReviewed=true", func() {
		BeforeEach(func() {
			showReviewed = true
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("returns only reviewed decisions", func() {
			Expect(items).To(HaveLen(1))
			Expect(items[0].Name).To(Equal("decisions/beta"))
			Expect(items[0].Reviewed).To(BeTrue())
		})
	})

	Context("with showAll=true", func() {
		BeforeEach(func() {
			showAll = true
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("returns all decisions", func() {
			Expect(items).To(HaveLen(3))
		})
	})

	Context("empty vault", func() {
		BeforeEach(func() {
			mockDecisionStorage.ListDecisionsReturns([]*domain.Decision{}, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("returns empty slice not nil", func() {
			Expect(items).NotTo(BeNil())
			Expect(items).To(HaveLen(0))
		})
	})

	Context("storage error", func() {
		BeforeEach(func() {
			mockDecisionStorage.ListDecisionsReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})

	Context("result includes vault name", func() {
		BeforeEach(func() {
			showAll = true
		})

		It("sets vault name on all items", func() {
			Expect(err).To(BeNil())
			for _, item := range items {
				Expect(item.Vault).To(Equal(vaultName))
			}
		})
	})

	Context("sorting", func() {
		BeforeEach(func() {
			unsorted := []*domain.Decision{
				{Name: "decisions/zebra", Reviewed: false},
				{Name: "decisions/apple", Reviewed: false},
				{Name: "decisions/mango", Reviewed: false},
			}
			mockDecisionStorage.ListDecisionsReturns(unsorted, nil)
			showAll = true
		})

		It("sorts decisions alphabetically by name", func() {
			Expect(err).To(BeNil())
			Expect(items).To(HaveLen(3))
			Expect(items[0].Name).To(Equal("decisions/apple"))
			Expect(items[1].Name).To(Equal("decisions/mango"))
			Expect(items[2].Name).To(Equal("decisions/zebra"))
		})
	})
})
