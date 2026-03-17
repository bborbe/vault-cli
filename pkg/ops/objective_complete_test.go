// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"

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
		op                   ops.ObjectiveCompleteOperation
		mockObjectiveStorage *mocks.ObjectiveStorage
		currentDateTime      libtime.CurrentDateTime
		vaultPath            string
		objectiveName        string
		vaultName            string
		outputFormat         string
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
		outputFormat = "plain"

		objective = &domain.Objective{
			Name:   objectiveName,
			Status: domain.ObjectiveStatusActive,
		}
		mockObjectiveStorage.FindObjectiveByNameReturns(objective, nil)
		mockObjectiveStorage.WriteObjectiveReturns(nil)
	})

	JustBeforeEach(func() {
		err = op.Execute(ctx, vaultPath, objectiveName, vaultName, outputFormat)
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
			objective.Status = domain.ObjectiveStatusCompleted
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

	Context("success plain mode", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("writes objective with completed status", func() {
			Expect(mockObjectiveStorage.WriteObjectiveCallCount()).To(Equal(1))
			_, writtenObjective := mockObjectiveStorage.WriteObjectiveArgsForCall(0)
			Expect(writtenObjective.Status).To(Equal(domain.ObjectiveStatusCompleted))
			Expect(writtenObjective.Completed).NotTo(BeNil())
		})
	})

	Context("success JSON mode", func() {
		var (
			pipeReader *os.File
			pipeWriter *os.File
			origStdout *os.File
			output     string
		)

		BeforeEach(func() {
			outputFormat = "json"
			pipeReader, pipeWriter, _ = os.Pipe()
			origStdout = os.Stdout
			os.Stdout = pipeWriter
		})

		JustBeforeEach(func() {
			pipeWriter.Close()
			os.Stdout = origStdout
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(pipeReader)
			output = buf.String()
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("outputs valid JSON with success true", func() {
			var result ops.ObjectiveCompleteResult
			Expect(json.Unmarshal([]byte(output), &result)).To(Succeed())
			Expect(result.Success).To(BeTrue())
			Expect(result.Status).To(Equal("completed"))
			Expect(result.Completed).To(Equal("2026-03-17"))
			Expect(result.Name).To(Equal(objectiveName))
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
