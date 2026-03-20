// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("DecisionAckOperation", func() {
	var (
		ctx                 context.Context
		err                 error
		result              ops.MutationResult
		decisionAckOp       ops.DecisionAckOperation
		mockDecisionStorage *mocks.DecisionStorage
		currentDateTime     libtime.CurrentDateTime
		vaultPath           string
		vaultName           string
		decisionName        string
		statusOverride      string
		decision            *domain.Decision
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockDecisionStorage = &mocks.DecisionStorage{}
		currentDateTime = libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-16T12:00:00Z"))
		decisionAckOp = ops.NewDecisionAckOperation(mockDecisionStorage, currentDateTime)
		vaultPath = "/path/to/vault"
		vaultName = "test-vault"
		decisionName = "decisions/my-decision"
		statusOverride = ""

		decision = &domain.Decision{
			Name:        decisionName,
			NeedsReview: true,
			Reviewed:    false,
			Status:      "pending",
		}
		mockDecisionStorage.FindDecisionByNameReturns(decision, nil)
		mockDecisionStorage.WriteDecisionReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = decisionAckOp.Execute(
			ctx,
			vaultPath,
			vaultName,
			decisionName,
			statusOverride,
		)
	})

	Context("successful ack", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("calls FindDecisionByName with correct args", func() {
			Expect(mockDecisionStorage.FindDecisionByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualName := mockDecisionStorage.FindDecisionByNameArgsForCall(
				0,
			)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualName).To(Equal(decisionName))
		})

		It("sets Reviewed=true and ReviewedDate=today", func() {
			Expect(mockDecisionStorage.WriteDecisionCallCount()).To(Equal(1))
			_, written := mockDecisionStorage.WriteDecisionArgsForCall(0)
			Expect(written.Reviewed).To(BeTrue())
			Expect(written.ReviewedDate).To(Equal("2026-03-16"))
		})

		It("returns result with correct name and vault", func() {
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal(decisionName))
			Expect(result.Vault).To(Equal(vaultName))
		})
	})

	Context("with statusOverride", func() {
		BeforeEach(func() {
			statusOverride = "approved"
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("sets decision.Status to override value", func() {
			Expect(mockDecisionStorage.WriteDecisionCallCount()).To(Equal(1))
			_, written := mockDecisionStorage.WriteDecisionArgsForCall(0)
			Expect(written.Status).To(Equal("approved"))
		})
	})

	Context("JSON output format", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("returns result with success true", func() {
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal(decisionName))
			Expect(result.Vault).To(Equal(vaultName))
		})
	})

	Context("FindDecisionByName error", func() {
		BeforeEach(func() {
			mockDecisionStorage.FindDecisionByNameReturns(nil, ErrTest)
		})

		It("propagates the error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("find decision"))
		})

		It("does not call WriteDecision", func() {
			Expect(mockDecisionStorage.WriteDecisionCallCount()).To(Equal(0))
		})
	})

	Context("WriteDecision error", func() {
		BeforeEach(func() {
			mockDecisionStorage.WriteDecisionReturns(ErrTest)
		})

		It("propagates the error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("write decision"))
		})
	})
})
