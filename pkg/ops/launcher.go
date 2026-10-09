// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// LauncherFactory builds the starter/resumer pair for a resolved launcher script.
// It is the injection seam that lets a task open on a different launcher than the
// vault's configured claude_script without the operation knowing how a launcher
// is constructed. A nil member means the resolved script could not be found.
type LauncherFactory func(script string) (ClaudeSessionStarter, ClaudeResumer)

// launcherValuePattern is the accepted shape of a launcher value: a plain path
// made of letters, digits, dot, underscore, dash and slash. The value comes from
// worker-writable frontmatter and is later executed, so anything else (spaces,
// shell metacharacters, command substitution) is refused. This is a shape check,
// not a check against a list of known launchers — operator decision 2026-10-09.
var launcherValuePattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// ResolveLauncher picks the launcher script for a task from its own value, else
// its goals' values, else the vault's configured claude_script.
//
// Precedence: a non-empty taskLauncher wins outright. Otherwise the distinct
// non-empty goalLaunchers are consulted: none means the vault script, exactly one
// means that one, and more than one is refused with an error naming every
// distinct value (picking one by frontmatter order would be arbitrary).
//
// A bare name (no slash) resolves next to the vault's own launcher, so vault
// script "claude" + launcher "cc-x" yields "cc-x" and vault script
// "/s/cc-private" + launcher "cc-private-claude" yields "/s/cc-private-claude".
// A value containing a slash is used as given.
func ResolveLauncher(
	ctx context.Context,
	vaultScript string,
	taskLauncher string,
	goalLaunchers []string,
) (string, error) {
	if trimmed := strings.TrimSpace(taskLauncher); trimmed != "" {
		return resolveLauncherValue(ctx, vaultScript, trimmed)
	}
	distinct, err := distinctLaunchers(ctx, goalLaunchers)
	if err != nil {
		return "", err
	}
	switch len(distinct) {
	case 0:
		return vaultScript, nil
	case 1:
		return resolveLauncherValue(ctx, vaultScript, distinct[0])
	default:
		return "", errors.Errorf(
			ctx,
			"conflicting launchers on goals: %s (set launcher on the task to choose one)",
			strings.Join(distinct, ", "),
		)
	}
}

// ResolveTaskLauncher resolves the launcher for a task, consulting its goals when
// the task carries no launcher of its own.
//
// A goal link that no longer resolves — or any FindGoalByName error — is skipped
// and reported as a warning naming the goal, so a stale link never blocks work-on.
// The task's own launcher short-circuits the goal lookups entirely.
func ResolveTaskLauncher(
	ctx context.Context,
	goalStorage storage.GoalStorage,
	vaultPath string,
	vaultScript string,
	task *domain.Task,
) (resolved string, warnings []string, err error) {
	if task == nil {
		return vaultScript, nil, nil
	}
	if trimmed := strings.TrimSpace(task.Launcher()); trimmed != "" {
		resolved, err := resolveLauncherValue(ctx, vaultScript, trimmed)
		return resolved, nil, err
	}
	if goalStorage == nil {
		return vaultScript, nil, nil
	}
	goalLaunchers, warnings := collectGoalLaunchers(ctx, goalStorage, vaultPath, task.Goals())
	resolved, err = ResolveLauncher(ctx, vaultScript, "", goalLaunchers)
	if err != nil {
		return "", warnings, err
	}
	return resolved, warnings, nil
}

// collectGoalLaunchers resolves each goal link to its Launcher() value. A goal
// that cannot be found is skipped and reported as a warning; empty names such as
// "[[]]" are ignored.
func collectGoalLaunchers(
	ctx context.Context,
	goalStorage storage.GoalStorage,
	vaultPath string,
	goalLinks []string,
) ([]string, []string) {
	var launchers []string
	var warnings []string
	for _, link := range goalLinks {
		name := stripWikilink(link)
		if name == "" {
			continue
		}
		goal, err := goalStorage.FindGoalByName(ctx, vaultPath, name)
		if err != nil || goal == nil {
			warnings = append(
				warnings,
				fmt.Sprintf("goal %q not found; launcher not inherited", name),
			)
			continue
		}
		launchers = append(launchers, goal.Launcher())
	}
	return launchers, warnings
}

// stripWikilink trims surrounding whitespace and the "[[" / "]]" markers from a
// goal link, returning the bare goal name ("" for "[[]]").
func stripWikilink(link string) string {
	trimmed := strings.TrimSpace(link)
	trimmed = strings.TrimPrefix(trimmed, "[[")
	trimmed = strings.TrimSuffix(trimmed, "]]")
	return strings.TrimSpace(trimmed)
}

// distinctLaunchers trims, validates and de-duplicates the goal launcher values,
// preserving first-seen order so the conflict error names each value once.
func distinctLaunchers(ctx context.Context, goalLaunchers []string) ([]string, error) {
	seen := make(map[string]bool, len(goalLaunchers))
	var distinct []string
	for _, raw := range goalLaunchers {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if err := validateLauncherValue(ctx, value); err != nil {
			return nil, err
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		distinct = append(distinct, value)
	}
	return distinct, nil
}

// resolveLauncherValue validates a single non-empty launcher value and expands a
// bare name relative to the vault's own launcher script.
func resolveLauncherValue(ctx context.Context, vaultScript string, value string) (string, error) {
	if err := validateLauncherValue(ctx, value); err != nil {
		return "", err
	}
	if strings.Contains(value, "/") {
		return value, nil
	}
	return filepath.Join(filepath.Dir(vaultScript), value), nil
}

// validateLauncherValue refuses a launcher value that is not a plain path. The
// shape check rejects shell metacharacters; the ".." segment check rejects
// traversal out of the launcher's directory.
func validateLauncherValue(ctx context.Context, value string) error {
	if !launcherValuePattern.MatchString(value) {
		return errors.Errorf(
			ctx,
			"invalid launcher %q: must match %s",
			value,
			launcherValuePattern.String(),
		)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return errors.Errorf(
				ctx,
				"invalid launcher %q: must not contain a .. path segment",
				value,
			)
		}
	}
	return nil
}
