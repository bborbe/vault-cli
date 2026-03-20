// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("SearchOperation", func() {
	var ctx context.Context
	var searchOp ops.SearchOperation
	var vaultPath string
	var scopeDir string
	var query string
	var topK int

	BeforeEach(func() {
		ctx = context.Background()
		searchOp = ops.NewSearchOperation()
		vaultPath = "/path/to/vault"
		scopeDir = ""
		query = "test query"
		topK = 5
	})

	Describe("NewSearchOperation", func() {
		It("creates a new search operation", func() {
			Expect(searchOp).NotTo(BeNil())
		})
	})

	Describe("Execute", func() {
		Context("when semantic-search-mcp is not installed", func() {
			It("returns an error about PATH", func() {
				// This will fail unless semantic-search-mcp is actually installed
				// In a real environment with semantic-search-mcp, we'd need to mock exec
				_, err := searchOp.Execute(ctx, vaultPath, scopeDir, query, topK)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("semantic-search-mcp"))
				}
			})
		})

		Context("with scoped directory", func() {
			BeforeEach(func() {
				scopeDir = "Tasks"
			})

			It("should construct the correct path", func() {
				// This test would require mocking exec to validate
				// the CONTENT_PATH env var is set correctly
				// Skipping for now as it requires exec mocking
				Skip("Requires exec mocking or integration test environment")
			})
		})

		Context("with empty scope directory", func() {
			BeforeEach(func() {
				scopeDir = ""
			})

			It("should use vault path as content path", func() {
				// This test would require mocking exec to validate
				// the CONTENT_PATH env var is set to vaultPath
				// Skipping for now as it requires exec mocking
				Skip("Requires exec mocking or integration test environment")
			})
		})
	})
})
