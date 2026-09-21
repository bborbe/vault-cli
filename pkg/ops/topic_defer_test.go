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

var _ = Describe("TopicDeferOperation", func() {
	var (
		ctx              context.Context
		err              error
		result           ops.MutationResult
		deferOp          ops.TopicDeferOperation
		mockTopicStorage *mocks.TopicStorage
		vaultPath        string
		topicName        string
		dateStr          string
		vaultName        string
		topic            *domain.Topic
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTopicStorage = &mocks.TopicStorage{}
		currentDateTime := libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-25T12:00:00Z"))
		deferOp = ops.NewTopicDeferOperation(mockTopicStorage, currentDateTime)
		vaultPath = "/path/to/vault"
		topicName = "my-topic"
		dateStr = "+7d"
		vaultName = "test-vault"

		topic = domain.NewTopic(
			map[string]any{"status": "in_progress"},
			domain.FileMetadata{Name: topicName, FilePath: "/vault/Topics/my-topic.md"},
			domain.Content(""),
		)
		mockTopicStorage.FindTopicByNameReturns(topic, nil)
		mockTopicStorage.WriteTopicReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = deferOp.Execute(ctx, vaultPath, topicName, dateStr, vaultName)
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
				Expect(result.Name).To(Equal(topicName))
				Expect(result.Vault).To(Equal(vaultName))
				Expect(result.Message).To(Equal("2026-04-01"))
			})

			It("sets defer_date to 7 days from now", func() {
				Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
				_, written := mockTopicStorage.WriteTopicArgsForCall(0)
				Expect(written.DeferDate()).NotTo(BeNil())
				expected := libtimetest.ParseDateTime("2026-03-25T12:00:00Z").
					Time().
					AddDate(0, 0, 7).
					Truncate(24 * time.Hour)
				Expect(written.DeferDate().Time()).To(Equal(expected))
			})
		})

		Context("with weekday name monday", func() {
			BeforeEach(func() {
				dateStr = "monday"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets defer_date to the next Monday", func() {
				Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
				_, written := mockTopicStorage.WriteTopicArgsForCall(0)
				Expect(written.DeferDate()).NotTo(BeNil())
				Expect(written.DeferDate().Time().Weekday()).To(Equal(time.Monday))
				Expect(
					written.DeferDate().Time().After(
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

			It("sets defer_date to the specified date", func() {
				Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
				_, written := mockTopicStorage.WriteTopicArgsForCall(0)
				Expect(written.DeferDate()).NotTo(BeNil())
				Expect(written.DeferDate().Time()).To(Equal(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)))
			})

			It("returns the formatted date in Message", func() {
				Expect(result.Message).To(Equal("2026-12-31"))
			})
		})

		Context("with RFC3339 datetime", func() {
			BeforeEach(func() {
				dateStr = "2026-12-31T16:00:00+01:00"
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("writes the topic", func() {
				Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
			})
		})
	})

	Context("past date validation", func() {
		Context("when deferring to a past date", func() {
			BeforeEach(func() {
				dateStr = "2025-01-01"
			})

			It("returns an error naming the refusal", func() {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("cannot defer to past date: 2025-01-01"))
			})

			It("returns a failed result", func() {
				Expect(result.Success).To(BeFalse())
				Expect(result.Error).To(ContainSubstring("cannot defer to past date: 2025-01-01"))
			})

			It("does not read the topic", func() {
				Expect(mockTopicStorage.FindTopicByNameCallCount()).To(Equal(0))
			})

			It("does not write the topic", func() {
				Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(0))
			})
		})

		Context("when deferring to today", func() {
			BeforeEach(func() {
				dateStr = "2026-03-25"
			})

			It("succeeds without error", func() {
				Expect(err).To(BeNil())
			})

			It("writes the topic", func() {
				Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
			})
		})
	})

	Context("invalid date format", func() {
		BeforeEach(func() {
			dateStr = "invalid"
		})

		It("returns an error", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid date format"))
		})

		It("returns a failed result", func() {
			Expect(result.Success).To(BeFalse())
			Expect(result.Error).To(ContainSubstring("invalid date format"))
		})

		It("does not read the topic", func() {
			Expect(mockTopicStorage.FindTopicByNameCallCount()).To(Equal(0))
		})

		It("does not write the topic", func() {
			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(0))
		})
	})

	Context("topic not found", func() {
		BeforeEach(func() {
			mockTopicStorage.FindTopicByNameReturns(nil, ErrTest)
		})

		It("returns an error wrapped with find topic", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("find topic"))
		})

		It("returns a failed result", func() {
			Expect(result.Success).To(BeFalse())
		})

		It("does not write the topic", func() {
			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(0))
		})
	})

	Context("write topic fails", func() {
		BeforeEach(func() {
			mockTopicStorage.WriteTopicReturns(ErrTest)
		})

		It("returns an error wrapped with write topic", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("write topic"))
		})

		It("returns a failed result", func() {
			Expect(result.Success).To(BeFalse())
		})
	})

	Context("does not update daily notes", func() {
		It("calls FindTopicByName once with the vault path and topic name", func() {
			Expect(mockTopicStorage.FindTopicByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualTopicName := mockTopicStorage.FindTopicByNameArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualTopicName).To(Equal(topicName))
		})
	})
})
