// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("ClaudeSessionStarter detachment (integration)", func() {
	// The non-interactive branch waits for the child's headless turn to finish, but
	// the wait is a bound, never a kill: the child runs in its own process group and
	// must survive the parent giving up on it. Cancelling the context is the cheapest
	// way to make the parent stop waiting while the child is still mid-turn.
	It("child outlives a cancelled parent wait", func() {
		dir, err := os.MkdirTemp("", "vault-claude-detach-*")
		Expect(err).To(BeNil())
		defer os.RemoveAll(dir)
		sentinel := filepath.Join(dir, "sentinel.txt")
		script := filepath.Join(dir, "worker.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\nsleep 6\ntouch "+sentinel+"\n"), 0755)).To(Succeed())

		starter := ops.NewClaudeSessionStarter(script)
		Expect(starter).NotTo(BeNil())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			defer GinkgoRecover()
			time.Sleep(500 * time.Millisecond)
			cancel()
		}()

		// The parent stops waiting well before the child's 6s turn ends, so it must
		// report an error and persist no session id.
		err = starter.StartSession(ctx, "123e4567-e89b-12d3-a456-426614174000", "prompt", dir, "worker", false)
		Expect(err).To(HaveOccurred())
		Expect(os.Stat(sentinel)).Error().To(HaveOccurred())

		// The sentinel appears only because the detached child survived the parent's
		// context cancellation and ran its turn to completion.
		Eventually(func() bool {
			_, statErr := os.Stat(sentinel)
			return statErr == nil
		}, "20s", "200ms").Should(BeTrue())
	})
})
