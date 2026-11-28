// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bborbe/errors"
)

// managerResolverTimeout bounds a single manager resolver invocation. The
// script reads a live fleet; a wedged read must not hang an approval.
const managerResolverTimeout = 30 * time.Second

//counterfeiter:generate -o ../../mocks/manager-resolver.go --fake-name ManagerResolver . ManagerResolver

// ManagerResolver answers which live manager owns a subject title.
type ManagerResolver interface {
	// LiveManagerForGoal returns the live manager session name for a goal (or
	// a `managed_by` subject) title. found=false, err=nil means the subject
	// resolves no manager. err!=nil means the fleet read failed and the answer
	// is unknown — never treat it as "no manager".
	LiveManagerForGoal(
		ctx context.Context,
		subject string,
		vaultPath string,
	) (name string, found bool, err error)
}

// ManagerResolverCommandResult is exported so the external ops_test package can
// write the runner closure (pkg/ops tests are package ops_test).
type ManagerResolverCommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// NewScriptManagerResolver returns a resolver that runs the named script. An
// empty scriptPath is resolved lazily on the first LiveManagerForGoal call, so
// the constructor stays pure.
func NewScriptManagerResolver(scriptPath string) ManagerResolver {
	return &scriptManagerResolver{
		scriptPath: scriptPath,
		runCmd:     defaultManagerResolverRunner,
	}
}

// NewScriptManagerResolverWithRunner returns a resolver with an injectable
// command runner, for tests.
func NewScriptManagerResolverWithRunner(
	scriptPath string,
	runCmd func(ctx context.Context, args []string) (ManagerResolverCommandResult, error),
) ManagerResolver {
	return &scriptManagerResolver{
		scriptPath: scriptPath,
		runCmd:     runCmd,
	}
}

type scriptManagerResolver struct {
	scriptPath string
	runCmd     func(ctx context.Context, args []string) (ManagerResolverCommandResult, error)
}

// LiveManagerForGoal runs the resolver script and maps its answer onto the
// three-way result the approval gate needs.
func (r *scriptManagerResolver) LiveManagerForGoal(
	ctx context.Context,
	subject string,
	vaultPath string,
) (string, bool, error) {
	script, err := r.resolveScript(ctx)
	if err != nil {
		return "", false, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, managerResolverTimeout)
	defer cancel()

	result, runErr := r.runCmd(timeoutCtx, []string{
		script, "--goal", subject, "--vault", vaultPath, "--json",
	})
	if runErr != nil {
		return "", false, errors.Wrapf(ctx, runErr, "run manager resolver %q", script)
	}
	// CommandContext reports a timeout as a killed process, which the runner
	// surfaces as an exit status. The context is the authoritative signal: a
	// killed read is unknown, never "no manager".
	if timeoutCtx.Err() != nil {
		return "", false, errors.Wrapf(
			ctx,
			timeoutCtx.Err(),
			"manager resolver %q did not complete",
			script,
		)
	}

	stderr := strings.TrimSpace(string(result.Stderr))
	switch result.ExitCode {
	case 0:
		// Parsed below.
	case 2:
		return "", false, errors.Errorf(
			ctx,
			"manager resolver %q: fleet unreadable: %s",
			script,
			stderr,
		)
	default:
		return "", false, errors.Errorf(
			ctx,
			"manager resolver %q exited %d: %s",
			script,
			result.ExitCode,
			stderr,
		)
	}

	var payload struct {
		Manager *string `json:"manager"`
	}
	if err := json.Unmarshal(result.Stdout, &payload); err != nil {
		return "", false, errors.Wrapf(ctx, err, "parse manager resolver %q output", script)
	}
	if payload.Manager != nil && *payload.Manager != "" {
		return *payload.Manager, true, nil
	}
	return "", false, nil
}

// resolveScript returns the configured script, or searches the installed
// claude-supervisor plugin when none was configured.
func (r *scriptManagerResolver) resolveScript(ctx context.Context) (string, error) {
	if r.scriptPath != "" {
		return r.scriptPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(ctx, err, "get home directory")
	}

	cachePattern := filepath.Join(
		home, ".claude", "plugins", "cache", "*", "supervisor", "*", "scripts", "manager-for.py",
	)
	if newest := newestMatch(cachePattern); newest != "" {
		return newest, nil
	}

	fallback := filepath.Join(
		home, ".claude", "plugins", "marketplaces", "claude-supervisor", "scripts", "manager-for.py",
	)
	if _, statErr := os.Stat(fallback); statErr == nil {
		return fallback, nil
	}

	return "", errors.Errorf(
		ctx,
		"manager resolver script not found: searched %s and %s",
		cachePattern,
		fallback,
	)
}

// newestMatch returns the glob match whose parent directory was modified most
// recently, or "" when the pattern matches nothing.
func newestMatch(pattern string) string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return ""
	}
	newest := ""
	var newestTime time.Time
	for _, match := range matches {
		info, statErr := os.Stat(filepath.Dir(match))
		if statErr != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = match
			newestTime = info.ModTime()
		}
	}
	return newest
}

// defaultManagerResolverRunner runs the script under the supplied context and
// reports its own exit status. A spawn failure or a context error is returned
// as err; a non-zero script exit is reported through ExitCode with a nil error.
func defaultManagerResolverRunner(
	ctx context.Context,
	args []string,
) (ManagerResolverCommandResult, error) {
	cmd := exec.CommandContext(
		ctx,
		args[0],
		args[1:]...) //#nosec G204 -- args[0] is the resolver script path
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := ManagerResolverCommandResult{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if stderrors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}
