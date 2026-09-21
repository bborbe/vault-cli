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

var _ = Describe("TopicCompleteOperation", func() {
	var (
		ctx              context.Context
		err              error
		result           ops.MutationResult
		op               ops.TopicCompleteOperation
		mockTopicStorage *mocks.TopicStorage
		vaultPath        string
		topicName        string
		vaultName        string
		topic            *domain.Topic
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTopicStorage = &mocks.TopicStorage{}
		op = ops.NewTopicCompleteOperation(mockTopicStorage)
		vaultPath = "/path/to/vault"
		topicName = "my-topic"
		vaultName = "test-vault"

		topic = domain.NewTopic(
			map[string]any{"status": "in_progress"},
			domain.FileMetadata{Name: topicName, FilePath: "/vault/Topics/my-topic.md"},
			domain.Content("---\nstatus: in_progress\n---\n"),
		)
		mockTopicStorage.FindTopicByNameReturns(topic, nil)
		mockTopicStorage.WriteTopicReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = op.Execute(ctx, vaultPath, topicName, vaultName)
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("returns success result naming the topic and vault", func() {
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal(topicName))
			Expect(result.Vault).To(Equal(vaultName))
		})

		It("writes the completed status", func() {
			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("status")).To(Equal(domain.TopicStatusCompleted))
		})

		It("passes the vault path and topic name to storage", func() {
			Expect(mockTopicStorage.FindTopicByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualTopicName := mockTopicStorage.FindTopicByNameArgsForCall(0)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualTopicName).To(Equal(topicName))
		})

		It("writes no completed date field", func() {
			Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("completed")).To(Equal(""))
			Expect(written.GetField("completed_date")).To(Equal(""))
		})
	})

	Context("when the topic carries no status key", func() {
		BeforeEach(func() {
			topic = domain.NewTopic(
				map[string]any{},
				domain.FileMetadata{Name: topicName, FilePath: "/vault/Topics/my-topic.md"},
				domain.Content(""),
			)
			mockTopicStorage.FindTopicByNameReturns(topic, nil)
		})

		It("transitions normally", func() {
			Expect(err).To(BeNil())
			Expect(result.Success).To(BeTrue())
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("status")).To(Equal(domain.TopicStatusCompleted))
		})
	})

	Context("when the topic carries a non-canonical prior status", func() {
		BeforeEach(func() {
			topic = domain.NewTopic(
				map[string]any{"status": "whatever"},
				domain.FileMetadata{Name: topicName, FilePath: "/vault/Topics/my-topic.md"},
				domain.Content(""),
			)
			mockTopicStorage.FindTopicByNameReturns(topic, nil)
		})

		It("still completes the topic", func() {
			Expect(err).To(BeNil())
			Expect(result.Success).To(BeTrue())
			_, written := mockTopicStorage.WriteTopicArgsForCall(0)
			Expect(written.GetField("status")).To(Equal(domain.TopicStatusCompleted))
		})
	})

	Context("topic already completed", func() {
		BeforeEach(func() {
			topic = domain.NewTopic(
				map[string]any{"status": domain.TopicStatusCompleted},
				domain.FileMetadata{Name: topicName, FilePath: "/vault/Topics/my-topic.md"},
				domain.Content(""),
			)
			mockTopicStorage.FindTopicByNameReturns(topic, nil)
		})

		It("returns an error naming the refusal", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("topic \"my-topic\" is already completed"))
		})

		It("returns a failed result", func() {
			Expect(result.Success).To(BeFalse())
			Expect(result.Error).To(ContainSubstring("already completed"))
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
})
