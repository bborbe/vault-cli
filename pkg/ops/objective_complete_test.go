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

var _ = Describe("ObjectiveCompleteOperation", func() {
	var (
		ctx                  context.Context
		err                  error
		result               ops.MutationResult
		op                   ops.ObjectiveCompleteOperation
		mockObjectiveStorage *mocks.ObjectiveStorage
		currentDateTime      libtime.CurrentDateTime
		vaultPath            string
		objectiveName        string
		vaultName            string
		objective            *domain.Objective
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockObjectiveStorage = &mocks.ObjectiveStorage{}
		currentDateTime = libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-17T12:00:00Z"))
		op = ops.NewObjectiveCompleteOperation(mockObjectiveStorage, currentDateTime)
		vaultPath = "/path/to/vault"
		objectiveName = "my-objective"
		vaultName = "test-vault"

		objective = domain.NewObjective(
			map[string]any{"status": "active"},
			domain.FileMetadata{Name: objectiveName},
			domain.Content(""),
		)
		mockObjectiveStorage.FindObjectiveByNameReturns(objective, nil)
		mockObjectiveStorage.WriteObjectiveReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = op.Execute(ctx, vaultPath, objectiveName, vaultName)
	})

	Context("objective not found", func() {
		BeforeEach(func() {
			mockObjectiveStorage.FindObjectiveByNameReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("does not write objective", func() {
			Expect(mockObjectiveStorage.WriteObjectiveCallCount()).To(Equal(0))
		})
	})

	Context("objective already completed", func() {
		BeforeEach(func() {
			_ = objective.SetStatus(domain.ObjectiveStatusCompleted)
			mockObjectiveStorage.FindObjectiveByNameReturns(objective, nil)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("already completed"))
		})

		It("does not write objective", func() {
			Expect(mockObjectiveStorage.WriteObjectiveCallCount()).To(Equal(0))
		})
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("writes objective with completed status", func() {
			Expect(mockObjectiveStorage.WriteObjectiveCallCount()).To(Equal(1))
			_, writtenObjective := mockObjectiveStorage.WriteObjectiveArgsForCall(0)
			Expect(writtenObjective.Status()).To(Equal(domain.ObjectiveStatusCompleted))
			Expect(writtenObjective.Completed()).NotTo(BeNil())
		})

		It("returns result with success true", func() {
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal(objectiveName))
			Expect(result.Vault).To(Equal(vaultName))
		})
	})

	Context("WriteObjective error", func() {
		BeforeEach(func() {
			mockObjectiveStorage.WriteObjectiveReturns(ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})
})
