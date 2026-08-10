// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bborbe/errors"
)

//counterfeiter:generate -o ../../mocks/search-operation.go --fake-name SearchOperation . SearchOperation
type SearchOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		scopeDir string,
		query string,
		topK int,
	) ([]string, error)
}

// NewSearchOperation creates a new search operation.
func NewSearchOperation() SearchOperation {
	return &searchOperation{}
}

type searchOperation struct{}

// Execute performs semantic search using semantic-search-mcp.
func (s *searchOperation) Execute(
	ctx context.Context,
	vaultPath string,
	scopeDir string,
	query string,
	topK int,
) ([]string, error) {
	// Determine the content path
	contentPath := vaultPath
	if scopeDir != "" {
		contentPath = filepath.Join(vaultPath, scopeDir)
	}

	// Check if semantic-search-mcp is available
	if _, err := exec.LookPath("semantic-search-mcp"); err != nil {
		return nil, errors.Wrap(ctx, err, "semantic-search-mcp not found on PATH")
	}

	// Build command
	cmd := exec.CommandContext(
		ctx,
		"semantic-search-mcp",
		"search",
		query,
	) // #nosec G204 -- args are passed via Cmd.Args, never through a shell; query is a plain string, not a shell expression
	cmd.Env = append(os.Environ(), fmt.Sprintf("CONTENT_PATH=%s", contentPath))

	// Add top-k parameter if specified
	if topK > 0 {
		cmd.Args = append(cmd.Args, "--limit", strconv.Itoa(topK))
	}

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Return without logging: the wrapped error already carries the command
		// output, and the caller logs it. Doing both duplicates every failure.
		return nil, errors.Wrap(
			ctx,
			err,
			fmt.Sprintf("semantic-search-mcp failed\nOutput: %s", string(output)),
		)
	}
	slog.Info(
		"semantic-search-mcp completed",
		"vault",
		vaultPath,
		"results",
		len(strings.Split(strings.TrimSpace(string(output)), "\n")),
	)

	result := strings.TrimSpace(string(output))
	if result == "" {
		return []string{}, nil
	}

	return strings.Split(result, "\n"), nil
}
