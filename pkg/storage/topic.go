// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

type topicStorage struct {
	*baseStorage
}

// isTopicNameWithinDir reports whether name resolves to a markdown path inside dir.
//
// filepath.Join cleans its result, so a name carrying a parent-directory segment
// is resolved before this check rather than after it: "../x" joins to a path in
// dir's parent and is therefore not within dir. A name carrying a separator but no
// climb ("sub/x") stays inside and is allowed.
func isTopicNameWithinDir(dir string, name string) bool {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(name, "[["), "]]")
	candidate := filepath.Join(dir, trimmed+".md")
	cleanedDir := filepath.Clean(dir)
	return candidate == cleanedDir || strings.HasPrefix(candidate, cleanedDir+string(os.PathSeparator))
}

// FindTopicByName searches for a topic by name in the vault's configured topics directory.
//
// A name that would resolve outside that directory is refused before any file is
// read, and the refusal is an ErrNotFound-class error so the multi-vault dispatcher
// treats it as a miss in this vault rather than a hard failure. This check is local
// to the topic path on purpose: baseStorage.findFileByName is left byte-identical so
// the goal and task families keep their current behaviour.
func (t *topicStorage) FindTopicByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Topic, error) {
	topicsDir := filepath.Join(vaultPath, t.config.TopicsDir)
	if !isTopicNameWithinDir(topicsDir, name) {
		return nil, errors.Wrapf(
			ctx,
			ErrNotFound,
			"topic name %q does not resolve inside %s",
			name,
			topicsDir,
		)
	}
	matchedPath, matchedName, err := t.findFileByName(ctx, topicsDir, name)
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "find topic file in %s", topicsDir)
	}
	return t.readTopicFromPath(ctx, matchedPath, matchedName, vaultPath)
}

func (t *topicStorage) readTopicFromPath(
	ctx context.Context,
	filePath string,
	name string,
	vaultPath string,
) (*domain.Topic, error) {
	data, meta, content, err := t.readEntityComponentsFromPath(ctx, filePath, name, vaultPath)
	if err != nil {
		return nil, err
	}
	return domain.NewTopic(data, meta, content), nil
}

// WriteTopic writes a topic to a markdown file.
//
// The parent directory is created when it is missing, so a write into a
// configured-but-not-yet-populated topics directory succeeds instead of failing on
// a path that does not exist. The configured directory is the only one created —
// nothing writes to the default topics directory on this path.
func (t *topicStorage) WriteTopic(ctx context.Context, topic *domain.Topic) error {
	if isSymlink(topic.FilePath) {
		return errors.Errorf(ctx, "refusing to write through symlink: %s", topic.FilePath)
	}
	content, err := t.serializeMapAsFrontmatter(ctx, topic.RawMap(), string(topic.Content))
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}
	dir := filepath.Dir(topic.FilePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return errors.Wrapf(ctx, err, "create directory %s", dir)
	}
	if err := os.WriteFile(topic.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrapf(ctx, err, "write file %s", topic.FilePath)
	}
	return nil
}
