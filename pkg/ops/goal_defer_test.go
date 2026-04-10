// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"time"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("GoalDeferOperation", func() {
	var (
		ctx             context.Context
		err             error
		result          ops.MutationResult
		deferOp         ops.GoalDeferOperation
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		goalName        string
		dateStr         string
		vaultName       string
		goal            *domain.Goal
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		currentDateTime := libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-25T12:00:00Z"))
		deferOp = ops.NewGoalDeferOperation(mockGoalStorage, currentDateTime)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"
		dateStr = "+7d"
		vaultName = "test-vault"

		goal = domain.NewGoal(
			map[string]any{"status": "active"},
			domain.FileMetadata{Name: goalName},
			domain.Content(""),
		)
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = deferOp.Execute(ctx, vaultPath, goalName, dateStr, vaultName)
	})

	Context("success", func() {
		Context("with relative date +7d", func() {
			BeforeEach(func() {
				dateStr = "+7d"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("returns success result", func() {
				Expect(result.Success).To(BeTrue())
				Expect(result.Name).To(Equal(goalName))
				Expect(result.Vault).To(Equal(vaultName))
				Expect(result.Message).To(Equal("2026-04-01"))
			})

			It("sets defer_date to 7 days from now", func() {
				Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
				_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
				Expect(writtenGoal.DeferDate()).NotTo(BeNil())
				expected := libtimetest.ParseDateTime("2026-03-25T12:00:00Z").
					Time().
					AddDate(0, 0, 7).
					Truncate(24 * time.Hour)
				actual := writtenGoal.DeferDate().Time()
				Expect(actual).To(Equal(expected))
			})
		})

		Context("with weekday name monday", func() {
			BeforeEach(func() {
				dateStr = "monday"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets defer_date to next Monday", func() {
				Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
				_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
				Expect(writtenGoal.DeferDate()).NotTo(BeNil())
				Expect(writtenGoal.DeferDate().Time().Weekday()).To(Equal(time.Monday))
				Expect(
					writtenGoal.DeferDate().Time().After(
						libtimetest.ParseDateTime("2026-03-25T12:00:00Z").Time(),
					),
				).To(BeTrue())
			})
		})

		Context("with ISO date 2026-12-31", func() {
			BeforeEach(func() {
				dateStr = "2026-12-31"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets defer_date to specified date", func() {
				Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
				_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
				Expect(writtenGoal.DeferDate()).NotTo(BeNil())
				expected := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
				actual := writtenGoal.DeferDate().Time()
				Expect(actual).To(Equal(expected))
			})

			It("returns formatted date in Message", func() {
				Expect(result.Message).To(Equal("2026-12-31"))
			})
		})
	})

	Context("past date validation", func() {
		Context("when deferring to a past date", func() {
			BeforeEach(func() {
				dateStr = "2025-01-01"
			})

			It("returns error", func() {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("cannot defer to past date"))
			})

			It("returns failed result", func() {
				Expect(result.Success).To(BeFalse())
				Expect(result.Error).To(ContainSubstring("cannot defer to past date"))
			})

			It("does not call WriteGoal", func() {
				Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
			})
		})

		Context("when deferring to today", func() {
			BeforeEach(func() {
				dateStr = "2026-03-25"
			})

			It("succeeds without error", func() {
				Expect(err).To(BeNil())
			})

			It("writes goal", func() {
				Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
			})
		})
	})

	Context("invalid date format", func() {
		BeforeEach(func() {
			dateStr = "invalid"
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid date format"))
		})

		It("returns failed result", func() {
			Expect(result.Success).To(BeFalse())
			Expect(result.Error).To(ContainSubstring("invalid date format"))
		})

		It("does not call FindGoalByName", func() {
			Expect(mockGoalStorage.FindGoalByNameCallCount()).To(Equal(0))
		})

		It("does not call WriteGoal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("goal not found", func() {
		BeforeEach(func() {
			mockGoalStorage.FindGoalByNameReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("returns failed result", func() {
			Expect(result.Success).To(BeFalse())
		})

		It("does not call WriteGoal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("write goal fails", func() {
		BeforeEach(func() {
			mockGoalStorage.WriteGoalReturns(ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("returns failed result", func() {
			Expect(result.Success).To(BeFalse())
		})
	})

	Context("does not update daily notes", func() {
		It("calls FindGoalByName once", func() {
			Expect(mockGoalStorage.FindGoalByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualGoalName := mockGoalStorage.FindGoalByNameArgsForCall(
				0,
			)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualGoalName).To(Equal(goalName))
		})

		It("calls WriteGoal once", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
		})
	})
})
