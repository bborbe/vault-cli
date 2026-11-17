// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import (
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// spliceTestPage builds a page whose ordering key is name plus ".md".
func spliceTestPage(name string) *Entry {
	return &Entry{Page: domain.NewPage(
		map[string]any{"title": name},
		domain.FileMetadata{Name: name},
		domain.Content("# "+name),
	)}
}

// spliceTestNames returns the ordering key of every page.
func spliceTestNames(pages []*Entry) []string {
	names := make([]string, 0, len(pages))
	for _, page := range pages {
		names = append(names, page.Page.Name+".md")
	}
	return names
}

var _ = ginkgo.Describe("splicePage", func() {
	ginkgo.It("inserts into an empty slice", func() {
		Expect(spliceTestNames(splicePage(nil, "a.md", spliceTestPage("a")))).
			To(Equal([]string{"a.md"}))
	})

	ginkgo.It("inserts first", func() {
		pages := []*Entry{spliceTestPage("b"), spliceTestPage("c")}
		Expect(spliceTestNames(splicePage(pages, "a.md", spliceTestPage("a")))).
			To(Equal([]string{"a.md", "b.md", "c.md"}))
	})

	ginkgo.It("inserts last", func() {
		pages := []*Entry{spliceTestPage("a"), spliceTestPage("b")}
		Expect(spliceTestNames(splicePage(pages, "c.md", spliceTestPage("c")))).
			To(Equal([]string{"a.md", "b.md", "c.md"}))
	})

	ginkgo.It("replaces in place without mutating the input", func() {
		original := spliceTestPage("b")
		pages := []*Entry{spliceTestPage("a"), original, spliceTestPage("c")}
		replacement := spliceTestPage("b")

		spliced := splicePage(pages, "b.md", replacement)

		Expect(spliceTestNames(spliced)).To(Equal([]string{"a.md", "b.md", "c.md"}))
		Expect(spliced[1]).To(BeIdenticalTo(replacement))
		Expect(pages[1]).To(BeIdenticalTo(original))
	})

	ginkgo.It("removes a present page without mutating the input", func() {
		pages := []*Entry{spliceTestPage("a"), spliceTestPage("b"), spliceTestPage("c")}

		spliced := splicePage(pages, "b.md", nil)

		Expect(spliceTestNames(spliced)).To(Equal([]string{"a.md", "c.md"}))
		Expect(spliceTestNames(pages)).To(Equal([]string{"a.md", "b.md", "c.md"}))
	})

	ginkgo.It("returns the pages unchanged when removing an absent one", func() {
		first := spliceTestPage("a")
		second := spliceTestPage("b")
		pages := []*Entry{first, second}

		spliced := splicePage(pages, "z.md", nil)

		Expect(spliceTestNames(spliced)).To(Equal([]string{"a.md", "b.md"}))
		Expect(spliced[0]).To(BeIdenticalTo(first))
		Expect(spliced[1]).To(BeIdenticalTo(second))
	})
})
