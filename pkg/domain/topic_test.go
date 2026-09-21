// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("Topic", func() {
	It("wires frontmatter, file metadata and content through the constructor", func() {
		topic := domain.NewTopic(
			map[string]any{"status": "in_progress"},
			domain.FileMetadata{
				Name:     "Attention Routing",
				FilePath: "/vault/23 Topics/Attention Routing.md",
			},
			domain.Content("---\nstatus: in_progress\n---\nbody\n"),
		)
		Expect(topic).NotTo(BeNil())
		Expect(topic.Name).To(Equal("Attention Routing"))
		Expect(topic.FilePath).To(Equal("/vault/23 Topics/Attention Routing.md"))
		Expect(topic.Content).To(Equal(domain.Content("---\nstatus: in_progress\n---\nbody\n")))
		Expect(topic.GetField("status")).To(Equal("in_progress"))
	})

	It("accepts a nil frontmatter map without panicking", func() {
		topic := domain.NewTopic(nil, domain.FileMetadata{}, domain.Content(""))
		Expect(topic).NotTo(BeNil())
		Expect(topic.Keys()).To(BeEmpty())
		Expect(topic.GetField("anything")).To(Equal(""))
	})

	It("preserves an unknown key across an unrelated write", func() {
		ctx := context.Background()
		topic := domain.NewTopic(
			map[string]any{"status": "in_progress", "attention_routing_hint": "keep me"},
			domain.FileMetadata{},
			domain.Content(""),
		)
		Expect(topic.GetField("attention_routing_hint")).To(Equal("keep me"))
		Expect(topic.SetField(ctx, "assignee", "alice")).To(Succeed())
		Expect(topic.RawMap()).To(HaveKey("attention_routing_hint"))
		Expect(topic.GetField("attention_routing_hint")).To(Equal("keep me"))
	})

	It("freezes the two topic status literals", func() {
		Expect(domain.TopicStatusInProgress).To(Equal("in_progress"))
		Expect(domain.TopicStatusCompleted).To(Equal("completed"))
	})

	It("promotes Keys from the embedded FrontmatterMap", func() {
		topic := domain.NewTopic(
			map[string]any{"a": 1, "b": 2},
			domain.FileMetadata{},
			domain.Content(""),
		)
		Expect(topic.Keys()).To(ConsistOf("a", "b"))
	})
})
