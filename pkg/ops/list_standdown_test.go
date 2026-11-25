// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

// The ordering cases carry time.Time values, not strings: that is the shape a
// YAML-parsed vault page has, and a string-only fixture cannot catch a
// GetString+time.Parse accessor mistake — which makes every stopped row
// invisible while the suite stays green.
var _ = Describe("task list omits a task stopped after it was flagged", func() {
	stopAt := time.Date(2026, 11, 24, 18, 5, 33, 0, time.UTC)
	flagBefore := time.Date(2026, 11, 24, 17, 10, 0, 0, time.UTC)
	flagAfter := time.Date(2026, 11, 24, 19, 36, 58, 408271000, time.UTC)

	cases := []struct {
		name        string
		frontmatter map[string]any
		listed      bool
	}{
		{"stopped-after-flag", map[string]any{"status": "in_progress", "stopped": "true", "stop_date": stopAt, "flag_set_at": flagBefore}, false},
		{"flag-after-stop", map[string]any{"status": "in_progress", "stopped": "true", "stop_date": stopAt, "flag_set_at": flagAfter}, true},
		{"no-stop-date", map[string]any{"status": "in_progress", "stopped": "true", "flag_set_at": flagAfter}, false},
		{"unparseable-stop-date", map[string]any{"status": "in_progress", "stopped": "true", "stop_date": "not-a-date", "flag_set_at": flagAfter}, false},
		{"unparseable-flag-instant", map[string]any{"status": "in_progress", "stopped": "true", "stop_date": stopAt, "flag_set_at": "not-a-time"}, false},
		{"not-stopped", map[string]any{"status": "in_progress", "flag_set_at": flagAfter}, true},
		{"stopped-false", map[string]any{"status": "in_progress", "stopped": false, "flag_set_at": flagAfter}, true},
		{"stopped-bool-true", map[string]any{"status": "in_progress", "stopped": true, "stop_date": stopAt, "flag_set_at": flagBefore}, false},
	}

	It("omits a task stopped after it was flagged", func() {
		ctx := context.Background()
		for _, c := range cases {
			pageStorage := &mocks.PageStorage{}
			page := domain.NewPage(
				c.frontmatter,
				domain.FileMetadata{Name: "T"},
				domain.Content(""),
			)
			pageStorage.ListPagesReturns([]*domain.Page{page}, nil)

			items, err := ops.NewListOperation(pageStorage).Execute(
				ctx, "/vault", "vault", "25 Tasks", nil, false, "", "",
			)
			Expect(err).NotTo(HaveOccurred())

			listed := len(items) == 1
			GinkgoWriter.Printf("case=%s listed=%t\n", c.name, listed)
			Expect(listed).To(Equal(c.listed), "case=%s", c.name)
		}
	})
})
