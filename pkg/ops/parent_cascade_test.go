// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("Parent cascade", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("attempts",
		func(
			names []string,
			attempt func(ctx context.Context, name string) (ops.MutationResult, error),
			expected []ops.ParentAttempt,
		) {
			recorded := make([]string, 0, len(names))
			got := ops.CascadeParents(
				ctx,
				names,
				func(ctx context.Context, name string) (ops.MutationResult, error) {
					recorded = append(recorded, name)
					return attempt(ctx, name)
				},
			)

			Expect(got).To(HaveLen(len(expected)))
			Expect(got).To(Equal(expected))

			// The callback must receive the normalised name, in document order.
			recordedExpected := make([]string, 0, len(expected))
			for _, entry := range expected {
				recordedExpected = append(recordedExpected, entry.Name)
			}
			Expect(recorded).To(Equal(recordedExpected))
		},
		Entry(
			"empty list attempts nothing",
			nil,
			func(ctx context.Context, name string) (ops.MutationResult, error) {
				return ops.MutationResult{Success: true, Name: name}, nil
			},
			[]ops.ParentAttempt{},
		),
		Entry(
			"successful parent reports completed",
			[]string{"Goal A"},
			func(ctx context.Context, name string) (ops.MutationResult, error) {
				return ops.MutationResult{Success: true, Name: "Goal A"}, nil
			},
			[]ops.ParentAttempt{
				{Name: "Goal A", Result: ops.ParentResultCompleted},
			},
		),
		Entry(
			"refused parent carries the operation's open items",
			[]string{"Goal A"},
			func(ctx context.Context, name string) (ops.MutationResult, error) {
				return ops.MutationResult{
					Success:   false,
					OpenItems: []string{"box one", "box two"},
				}, ErrTest
			},
			[]ops.ParentAttempt{
				{
					Name:      "Goal A",
					Result:    ops.ParentResultRefused,
					OpenItems: []string{"box one", "box two"},
				},
			},
		),
		Entry(
			"refused parent falls back to the error text",
			[]string{"Goal A"},
			func(ctx context.Context, name string) (ops.MutationResult, error) {
				return ops.MutationResult{Success: false}, ErrTest
			},
			[]ops.ParentAttempt{
				{
					Name:      "Goal A",
					Result:    ops.ParentResultRefused,
					OpenItems: []string{"test error"},
				},
			},
		),
		Entry(
			"refused parent falls back to the result error field",
			[]string{"Goal A"},
			func(ctx context.Context, name string) (ops.MutationResult, error) {
				return ops.MutationResult{
					Success: false,
					Error:   "cannot complete goal: 1 open",
				}, nil
			},
			[]ops.ParentAttempt{
				{
					Name:      "Goal A",
					Result:    ops.ParentResultRefused,
					OpenItems: []string{"cannot complete goal: 1 open"},
				},
			},
		),
		Entry(
			"wikilink and alias normalise to the bare name",
			[]string{"[[Goal A]]", "[[Goal B|label]]", "[[Goal C#Heading]]"},
			func(ctx context.Context, name string) (ops.MutationResult, error) {
				return ops.MutationResult{Success: true, Name: name}, nil
			},
			[]ops.ParentAttempt{
				{Name: "Goal A", Result: ops.ParentResultCompleted},
				{Name: "Goal B", Result: ops.ParentResultCompleted},
				{Name: "Goal C", Result: ops.ParentResultCompleted},
			},
		),
		Entry(
			"empty normalised name is skipped",
			[]string{"[[]]"},
			func(ctx context.Context, name string) (ops.MutationResult, error) {
				return ops.MutationResult{Success: true, Name: name}, nil
			},
			[]ops.ParentAttempt{},
		),
		Entry(
			"document order is preserved",
			[]string{"Goal B", "Goal A"},
			func(ctx context.Context, name string) (ops.MutationResult, error) {
				return ops.MutationResult{Success: true, Name: name}, nil
			},
			[]ops.ParentAttempt{
				{Name: "Goal B", Result: ops.ParentResultCompleted},
				{Name: "Goal A", Result: ops.ParentResultCompleted},
			},
		),
	)
})
