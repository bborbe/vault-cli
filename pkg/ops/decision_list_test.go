// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"encoding/json"
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("DecisionListOperation", func() {
	var ctx context.Context
	var err error
	var decisionListOp ops.DecisionListOperation
	var mockDecisionStorage *mocks.DecisionStorage
	var vaultPath string
	var vaultName string
	var showReviewed bool
	var showAll bool
	var outputFormat string
	var decisions []*domain.Decision

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
		mockDecisionStorage = &mocks.DecisionStorage{}
		decisionListOp = ops.NewDecisionListOperation(mockDecisionStorage)
		vaultPath = "/path/to/vault"
		vaultName = "test-vault"
		showReviewed = false
		showAll = false
		outputFormat = "plain"

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
		err = decisionListOp.Execute(
			ctx,
			vaultPath,
			vaultName,
			showReviewed,
			showAll,
			outputFormat,
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
	})

	Context("with showReviewed=true", func() {
		BeforeEach(func() {
			showReviewed = true
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("with showAll=true", func() {
		BeforeEach(func() {
			showAll = true
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("empty vault", func() {
		BeforeEach(func() {
			mockDecisionStorage.ListDecisionsReturns([]*domain.Decision{}, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
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

	Context("plain output format", func() {
		var output []byte

		BeforeEach(func() {
			outputFormat = "plain"
		})

		JustBeforeEach(func() {
			mockDecisionStorage.ListDecisionsReturns(decisions, nil)
			output = captureStdout(func() {
				err = decisionListOp.Execute(ctx, vaultPath, vaultName, false, false, "plain")
			})
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("outputs only unreviewed decisions", func() {
			Expect(string(output)).To(ContainSubstring("[unreviewed] decisions/alpha\n"))
			Expect(string(output)).To(ContainSubstring("[unreviewed] decisions/gamma\n"))
			Expect(string(output)).NotTo(ContainSubstring("decisions/beta"))
		})

		It("uses reviewed status label for reviewed decisions", func() {
			mockDecisionStorage.ListDecisionsReturns(decisions, nil)
			reviewedOutput := captureStdout(func() {
				err = decisionListOp.Execute(ctx, vaultPath, vaultName, true, false, "plain")
			})
			Expect(err).To(BeNil())
			Expect(string(reviewedOutput)).To(ContainSubstring("[reviewed] decisions/beta\n"))
		})
	})

	Context("JSON output format", func() {
		var output []byte

		JustBeforeEach(func() {
			mockDecisionStorage.ListDecisionsReturns(decisions, nil)
			output = captureStdout(func() {
				err = decisionListOp.Execute(ctx, vaultPath, vaultName, false, true, "json")
			})
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("produces valid JSON array", func() {
			var items []ops.DecisionListItem
			Expect(json.Unmarshal(output, &items)).To(Succeed())
			Expect(items).To(HaveLen(3))
		})

		It("includes vault name", func() {
			var items []ops.DecisionListItem
			Expect(json.Unmarshal(output, &items)).To(Succeed())
			for _, item := range items {
				Expect(item.Vault).To(Equal(vaultName))
			}
		})
	})

	Context("JSON output with empty vault", func() {
		var output []byte

		JustBeforeEach(func() {
			mockDecisionStorage.ListDecisionsReturns([]*domain.Decision{}, nil)
			output = captureStdout(func() {
				err = decisionListOp.Execute(ctx, vaultPath, vaultName, false, true, "json")
			})
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("produces empty JSON array not null", func() {
			var items []ops.DecisionListItem
			Expect(json.Unmarshal(output, &items)).To(Succeed())
			Expect(items).NotTo(BeNil())
			Expect(items).To(HaveLen(0))
		})
	})

	Context("sorting", func() {
		var output []byte

		BeforeEach(func() {
			unsorted := []*domain.Decision{
				{Name: "decisions/zebra", Reviewed: false},
				{Name: "decisions/apple", Reviewed: false},
				{Name: "decisions/mango", Reviewed: false},
			}
			mockDecisionStorage.ListDecisionsReturns(unsorted, nil)
		})

		JustBeforeEach(func() {
			output = captureStdout(func() {
				err = decisionListOp.Execute(ctx, vaultPath, vaultName, false, true, "plain")
			})
		})

		It("sorts decisions alphabetically by name", func() {
			Expect(err).To(BeNil())
			appleIdx := indexOfStr(string(output), "decisions/apple")
			mangoIdx := indexOfStr(string(output), "decisions/mango")
			zebraIdx := indexOfStr(string(output), "decisions/zebra")
			Expect(appleIdx).To(BeNumerically("<", mangoIdx))
			Expect(mangoIdx).To(BeNumerically("<", zebraIdx))
		})
	})
})

func indexOfStr(s, substr string) int {
	for i := range s {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
