// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"encoding/json"
	"io"
	"os"

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
		ctx             context.Context
		err             error
		decisionAckOp   ops.DecisionAckOperation
		mockStorage     *mocks.Storage
		currentDateTime libtime.CurrentDateTime
		vaultPath       string
		vaultName       string
		decisionName    string
		statusOverride  string
		outputFormat    string
		decision        *domain.Decision
	)

	captureStdout := func(fn func()) []byte {
		r, w, pipeErr := os.Pipe()
		Expect(pipeErr).To(BeNil())
		orig := os.Stdout
		os.Stdout = w
		fn()
		w.Close()
		os.Stdout = orig
		data, readErr := io.ReadAll(r)
		Expect(readErr).To(BeNil())
		return data
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		currentDateTime = libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-16T12:00:00Z"))
		decisionAckOp = ops.NewDecisionAckOperation(mockStorage, currentDateTime)
		vaultPath = "/path/to/vault"
		vaultName = "test-vault"
		decisionName = "decisions/my-decision"
		statusOverride = ""
		outputFormat = "plain"

		decision = &domain.Decision{
			Name:        decisionName,
			NeedsReview: true,
			Reviewed:    false,
			Status:      "pending",
		}
		mockStorage.FindDecisionByNameReturns(decision, nil)
		mockStorage.WriteDecisionReturns(nil)
	})

	JustBeforeEach(func() {
		err = decisionAckOp.Execute(
			ctx,
			vaultPath,
			vaultName,
			decisionName,
			statusOverride,
			outputFormat,
		)
	})

	Context("successful ack", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("calls FindDecisionByName with correct args", func() {
			Expect(mockStorage.FindDecisionByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualName := mockStorage.FindDecisionByNameArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualName).To(Equal(decisionName))
		})

		It("sets Reviewed=true and ReviewedDate=today", func() {
			Expect(mockStorage.WriteDecisionCallCount()).To(Equal(1))
			_, written := mockStorage.WriteDecisionArgsForCall(0)
			Expect(written.Reviewed).To(BeTrue())
			Expect(written.ReviewedDate).To(Equal("2026-03-16"))
		})

		It("prints plain output", func() {
			var output []byte
			mockStorage.FindDecisionByNameReturns(decision, nil)
			mockStorage.WriteDecisionReturns(nil)
			output = captureStdout(func() {
				innerErr := decisionAckOp.Execute(
					ctx,
					vaultPath,
					vaultName,
					decisionName,
					statusOverride,
					"plain",
				)
				Expect(innerErr).To(BeNil())
			})
			Expect(string(output)).To(ContainSubstring("Acknowledged: " + decisionName))
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
			Expect(mockStorage.WriteDecisionCallCount()).To(Equal(1))
			_, written := mockStorage.WriteDecisionArgsForCall(0)
			Expect(written.Status).To(Equal("approved"))
		})
	})

	Context("JSON output format", func() {
		BeforeEach(func() {
			outputFormat = "json"
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("outputs valid JSON MutationResult", func() {
			var output []byte
			mockStorage.FindDecisionByNameReturns(decision, nil)
			mockStorage.WriteDecisionReturns(nil)
			output = captureStdout(func() {
				innerErr := decisionAckOp.Execute(
					ctx,
					vaultPath,
					vaultName,
					decisionName,
					statusOverride,
					"json",
				)
				Expect(innerErr).To(BeNil())
			})
			var result ops.MutationResult
			Expect(json.Unmarshal(output, &result)).To(Succeed())
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal(decisionName))
			Expect(result.Vault).To(Equal(vaultName))
		})
	})

	Context("FindDecisionByName error", func() {
		BeforeEach(func() {
			mockStorage.FindDecisionByNameReturns(nil, ErrTest)
		})

		It("propagates the error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("find decision"))
		})

		It("does not call WriteDecision", func() {
			Expect(mockStorage.WriteDecisionCallCount()).To(Equal(0))
		})
	})

	Context("WriteDecision error", func() {
		BeforeEach(func() {
			mockStorage.WriteDecisionReturns(ErrTest)
		})

		It("propagates the error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("write decision"))
		})
	})
})
