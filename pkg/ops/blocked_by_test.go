// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("IsBlocked", func() {
	page := func(name string, status any) *domain.Page {
		data := map[string]any{}
		if status != nil {
			data["status"] = status
		}
		return domain.NewPage(data, domain.FileMetadata{Name: name}, domain.Content(""))
	}

	DescribeTable("resolves blockers against the same page set",
		func(blockedBy []string, entities []*domain.Page, expected bool) {
			Expect(ops.IsBlocked(blockedBy, entities)).To(Equal(expected))
		},
		Entry("empty list is unblocked", nil, []*domain.Page{page("A", "completed")}, false),
		Entry("single completed blocker is unblocked", []string{"A"}, []*domain.Page{page("A", "completed")}, false),
		Entry("all completed blockers are unblocked", []string{"A", "B"}, []*domain.Page{page("A", "completed"), page("B", "completed")}, false),
		Entry("one of two blockers not completed is blocked", []string{"A", "B"}, []*domain.Page{page("A", "completed"), page("B", "in_progress")}, true),
		Entry("blocker with status next is blocked", []string{"A"}, []*domain.Page{page("A", "next")}, true),
		Entry("blocker with status in_progress is blocked", []string{"A"}, []*domain.Page{page("A", "in_progress")}, true),
		Entry("blocker with status hold is blocked", []string{"A"}, []*domain.Page{page("A", "hold")}, true),
		Entry("blocker with status backlog is blocked", []string{"A"}, []*domain.Page{page("A", "backlog")}, true),
		Entry("blocker with status aborted is blocked", []string{"A"}, []*domain.Page{page("A", "aborted")}, true),
		Entry("blocker with status done alias is unblocked", []string{"A"}, []*domain.Page{page("A", "done")}, false),
		Entry("missing blocker file is blocked", []string{"Ghost"}, []*domain.Page{page("A", "completed")}, true),
		Entry("blocker without a status key is blocked", []string{"A"}, []*domain.Page{page("A", nil)}, true),
		Entry("blocker with an unparseable status is blocked", []string{"A"}, []*domain.Page{page("A", "banana")}, true),
		Entry("wikilink blocker name resolves to the plain-named file", []string{"[[A]]"}, []*domain.Page{page("A", "completed")}, false),
		Entry("wikilink blocker with a case mismatch resolves", []string{"[[alpha]]"}, []*domain.Page{page("Alpha", "completed")}, false),
		Entry("plain blocker name with a case mismatch resolves", []string{"ALPHA"}, []*domain.Page{page("Alpha", "completed")}, false),
		Entry("substring name does not match a longer file name", []string{"Alpha"}, []*domain.Page{page("Alpha Two", "completed")}, true),
		Entry("a completed entity blocked by itself is unblocked", []string{"A"}, []*domain.Page{page("A", "completed")}, false),
		Entry("a blocked blocker is not followed transitively", []string{"B"}, []*domain.Page{
			domain.NewPage(
				map[string]any{"status": "in_progress", "blocked_by": []any{"C"}},
				domain.FileMetadata{Name: "B"},
				domain.Content(""),
			),
			page("C", "completed"),
		}, true),
	)

	It("a cycle terminates and leaves both blocked", func() {
		a := domain.NewPage(
			map[string]any{"status": "in_progress", "blocked_by": []any{"B"}},
			domain.FileMetadata{Name: "A"},
			domain.Content(""),
		)
		b := domain.NewPage(
			map[string]any{"status": "in_progress", "blocked_by": []any{"A"}},
			domain.FileMetadata{Name: "B"},
			domain.Content(""),
		)
		entities := []*domain.Page{a, b}
		// Resolution is a single status read per blocker — a blocker's own blocked_by
		// is never followed, so the cycle terminates and both stay blocked.
		Expect(ops.IsBlocked(a.BlockedBy(), entities)).To(BeTrue())
		Expect(ops.IsBlocked(b.BlockedBy(), entities)).To(BeTrue())
	})

	It("resolves wikilinks and case-insensitive names against the same page set", func() {
		blocker := domain.NewPage(
			map[string]any{"status": "completed"},
			domain.FileMetadata{Name: "Blocker"},
			domain.Content(""),
		)
		dependent := domain.NewPage(
			map[string]any{"status": "todo", "blocked_by": []any{"[[blocker]]", "BLOCKER"}},
			domain.FileMetadata{Name: "Dependent"},
			domain.Content(""),
		)
		orphan := domain.NewPage(
			map[string]any{"status": "todo", "blocked_by": []any{"[[Ghost]]"}},
			domain.FileMetadata{Name: "Orphan"},
			domain.Content(""),
		)
		entities := []*domain.Page{dependent, orphan, blocker}
		Expect(ops.IsBlocked(dependent.BlockedBy(), entities)).To(BeFalse())
		Expect(ops.IsBlocked(orphan.BlockedBy(), entities)).To(BeTrue())
	})
})
