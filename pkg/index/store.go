// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file adds the on-disk page index store: a metadata-only cache of the
// page index outside every vault. It is a cache only — a store that is
// missing, empty, damaged, unreadable or written by a different writer is
// discarded, never served, and never stops the process from starting.

package index

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/bborbe/boltkv"
	"github.com/bborbe/errors"
	"github.com/bborbe/kv"
	bolt "go.etcd.io/bbolt"
)

// StoreFormatVersion is the on-disk layout version. Version 2 records carry no
// page body. A store written under a different version is discarded, never
// converted.
const StoreFormatVersion = 2

// storeOpenTimeout bounds the wait for the store file's lock. The zero value of
// bolt's Timeout waits forever, which would let a second process on the same
// cache path hang startup.
const storeOpenTimeout = 5 * time.Second

// storeMetaBucketName holds the writer identity. It carries no NUL-separated
// key prefix, so it can never collide with an entry bucket.
const storeMetaBucketName = "vault-ui-page-index-meta"

// storeBucketPrefix starts every entry bucket name. The NUL bytes around the
// key parts cannot appear in a path, so two keys cannot produce one bucket.
const storeBucketPrefix = "vault-ui-page-index\x00"

// storeIdentityKey is the meta-bucket key holding the JSON writer identity.
const storeIdentityKey = "identity"

// vaultCLIModulePath is the dependency whose version identifies the parser that
// produced a stored page.
const vaultCLIModulePath = "github.com/bborbe/vault-cli"

// unknownParserVersion is reported when the parser version cannot be read.
const unknownParserVersion = "(unknown)"

// WriterIdentity identifies the process that wrote the store: the store-format
// version plus the vault-cli parser version this process was built against.
type WriterIdentity struct {
	StoreFormat   int
	ParserVersion string
}

//counterfeiter:generate -o ../../mocks/index-store.go --fake-name IndexStore . Store

// Store persists the parsed page index on local disk. It is a cache: a Load
// that cannot supply entries never fails the caller, it reports them absent.
type Store interface {
	// Load returns the key's stored entries. ok is false when there is nothing
	// usable to load for the key, which is the full-parse path. err is non-nil
	// only when ctx was cancelled.
	Load(ctx context.Context, key Key) (entries []StoredEntry, ok bool, err error)
	// Write applies one publication's delta for the key in a single
	// transaction: puts replaces or inserts entries keyed by Filename, deletes
	// removes them.
	Write(ctx context.Context, key Key, puts []StoredEntry, deletes []string) error
	// Path returns the store file path, for logging and warnings.
	Path() string
	// Close releases the store file.
	Close() error
}

// DefaultStorePath returns <user cache directory>/<app>/page-index.bolt. It is
// a pure path helper and creates nothing.
//
// It has no context parameter by contract, so the single stdlib lookup's error
// is returned unwrapped.
func DefaultStorePath(app string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, app, "page-index.bolt"), nil
}

// CurrentWriterIdentity returns this process's writer identity.
func CurrentWriterIdentity(ctx context.Context) WriterIdentity {
	return WriterIdentity{
		StoreFormat:   StoreFormatVersion,
		ParserVersion: vaultCLIParserVersion(),
	}
}

// vaultCLIParserVersion reads the vault-cli version this binary was built
// against: the dependency entry when vault-cli is a dependency, the main
// module when vault-cli is itself main, else a marker.
func vaultCLIParserVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return unknownParserVersion
	}
	for _, dep := range info.Deps {
		if dep.Path == vaultCLIModulePath {
			return dep.Version
		}
	}
	if info.Main.Path == vaultCLIModulePath && info.Main.Version != "" {
		return info.Main.Version
	}
	return unknownParserVersion
}

// NewBoltStore opens the embedded-Bolt store at path under the given writer
// identity. It never returns nil and never fails.
func NewBoltStore(
	ctx context.Context,
	path string,
	identity WriterIdentity,
	warnf func(format string, args ...any),
	vlogf func(format string, args ...any),
) Store {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return &discardedStore{path: path, reason: "create store directory", warnf: warnf}
	}

	db, err := boltkv.OpenFile(ctx, path, func(opts *bolt.Options) {
		opts.Timeout = storeOpenTimeout
	})
	if err != nil {
		return &discardedStore{path: path, reason: "open store", warnf: warnf}
	}

	store := &boltStore{
		db:       db,
		path:     path,
		identity: identity,
		warnf:    warnf,
		vlogf:    vlogf,
	}
	if err := store.reconcile(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			slog.Debug("close unusable page index store", "path", path, "error", closeErr)
		}
		return &discardedStore{path: path, reason: "record writer identity", warnf: warnf}
	}

	keys, entries := store.count(ctx)
	store.empty.Store(keys == 0)
	vlogf("page index store %s: %d keys, %d entries", path, keys, entries)
	return store
}

// boltStore is the live embedded-Bolt store.
type boltStore struct {
	db       boltkv.DB
	path     string
	identity WriterIdentity
	// empty is true while the store holds no entry bucket, so a Load can answer
	// without a transaction.
	empty atomic.Bool
	warnf func(format string, args ...any)
	vlogf func(format string, args ...any)
}

// reconcile reads the stored writer identity and either keeps the store or
// discards its entries, then records the current identity. It runs in one
// transaction. Format is checked before parser version, so a store differing
// in both reports a format mismatch.
func (s *boltStore) reconcile(ctx context.Context) error {
	return s.db.Update(ctx, func(ctx context.Context, tx kv.Tx) error {
		meta, err := tx.CreateBucketIfNotExists(ctx, kv.NewBucketName(storeMetaBucketName))
		if err != nil {
			return errors.Wrap(ctx, err, "create store meta bucket")
		}
		item, err := meta.Get(ctx, []byte(storeIdentityKey))
		if err != nil {
			return errors.Wrap(ctx, err, "read store writer identity")
		}
		var raw []byte
		if err := item.Value(func(value []byte) error {
			raw = append([]byte(nil), value...)
			return nil
		}); err != nil {
			return errors.Wrap(ctx, err, "read store writer identity")
		}

		reason := s.discardReason(raw)
		if reason == "" {
			return nil
		}
		s.warnf("page index store discarded (%s): %s", reason, s.path)
		if err := deleteEntryBuckets(ctx, tx); err != nil {
			return err
		}
		encoded, err := json.Marshal(s.identity)
		if err != nil {
			return errors.Wrap(ctx, err, "marshal store writer identity")
		}
		if err := meta.Put(ctx, []byte(storeIdentityKey), encoded); err != nil {
			return errors.Wrap(ctx, err, "write store writer identity")
		}
		return nil
	})
}

// discardReason returns why a stored identity cannot be used, or "" when it
// matches this process's identity.
func (s *boltStore) discardReason(raw []byte) string {
	if len(raw) == 0 {
		return "store file missing or empty"
	}
	var existing WriterIdentity
	if err := json.Unmarshal(raw, &existing); err != nil {
		return "damaged"
	}
	switch {
	case existing.StoreFormat != s.identity.StoreFormat:
		return fmt.Sprintf(
			"store format mismatch (stored %d, current %d)",
			existing.StoreFormat,
			s.identity.StoreFormat,
		)
	case existing.ParserVersion != s.identity.ParserVersion:
		return "parser version mismatch"
	}
	return ""
}

// deleteEntryBuckets removes every bucket except the meta bucket.
func deleteEntryBuckets(ctx context.Context, tx kv.Tx) error {
	names, err := tx.ListBucketNames(ctx)
	if err != nil {
		return errors.Wrap(ctx, err, "list store buckets")
	}
	for _, name := range names {
		if name.String() == storeMetaBucketName {
			continue
		}
		if err := tx.DeleteBucket(ctx, name); err != nil {
			return errors.Wrapf(ctx, err, "delete store bucket %s", name.String())
		}
	}
	return nil
}

// count reports the number of entry buckets and the total number of stored
// entries. It runs once at startup.
func (s *boltStore) count(ctx context.Context) (int, int) {
	keys, entries := 0, 0
	err := s.db.View(ctx, func(ctx context.Context, tx kv.Tx) error {
		names, err := tx.ListBucketNames(ctx)
		if err != nil {
			return err
		}
		for _, name := range names {
			if name.String() == storeMetaBucketName {
				continue
			}
			bucket, err := tx.Bucket(ctx, name)
			if err != nil {
				return err
			}
			keys++
			entries += bucketEntryCount(bucket)
		}
		return nil
	})
	if err != nil {
		return 0, 0
	}
	return keys, entries
}

// bucketEntryCount counts the items in one bucket by iterating it.
func bucketEntryCount(bucket kv.Bucket) int {
	count := 0
	iterator := bucket.Iterator()
	defer iterator.Close()
	for iterator.Rewind(); iterator.Valid(); iterator.Next() {
		count++
	}
	return count
}

// Load returns the key's stored entries. Any failure to supply them is a
// discard: it warns once and reports the key absent, so the caller full-parses.
func (s *boltStore) Load(ctx context.Context, key Key) ([]StoredEntry, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, errors.Wrap(ctx, err, "load page index store")
	}
	if s.empty.Load() {
		return nil, false, nil
	}

	var entries []StoredEntry
	err := s.db.View(ctx, func(ctx context.Context, tx kv.Tx) error {
		bucket, err := tx.Bucket(ctx, storeBucketName(key))
		if err != nil {
			return err
		}
		return kv.ForEach(ctx, bucket, func(item kv.Item) error {
			return item.Value(func(value []byte) error {
				entry, err := decodeRecord(ctx, value)
				if err != nil {
					return err
				}
				// Append rather than preallocate from a stored count, so a
				// crafted count cannot drive an unbounded allocation.
				entries = append(entries, entry)
				return nil
			})
		})
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, errors.Wrap(ctx, ctxErr, "load page index store")
		}
		if errors.Is(err, kv.BucketNotFoundError) {
			s.warnf("page index store discarded (no stored entries for key): %s", s.path)
			return nil, false, nil
		}
		s.warnf("page index store discarded (damaged): %s", s.path)
		return nil, false, nil
	}
	return entries, true, nil
}

// Write applies one publication's delta in a single transaction. A failed write
// leaves the previous content intact; the caller owns the failure warning.
func (s *boltStore) Write(
	ctx context.Context,
	key Key,
	puts []StoredEntry,
	deletes []string,
) error {
	err := s.db.Update(ctx, func(ctx context.Context, tx kv.Tx) error {
		if err := s.writeIdentity(ctx, tx); err != nil {
			return err
		}
		bucket, err := tx.CreateBucketIfNotExists(ctx, storeBucketName(key))
		if err != nil {
			return errors.Wrap(ctx, err, "create store entry bucket")
		}
		return applyDelta(ctx, bucket, puts, deletes)
	})
	if err != nil {
		return errors.Wrap(ctx, err, "write page index store")
	}
	s.empty.Store(false)
	return nil
}

// writeIdentity records this process's writer identity in the meta bucket.
func (s *boltStore) writeIdentity(ctx context.Context, tx kv.Tx) error {
	meta, err := tx.CreateBucketIfNotExists(ctx, kv.NewBucketName(storeMetaBucketName))
	if err != nil {
		return errors.Wrap(ctx, err, "create store meta bucket")
	}
	encoded, err := json.Marshal(s.identity)
	if err != nil {
		return errors.Wrap(ctx, err, "marshal store writer identity")
	}
	if err := meta.Put(ctx, []byte(storeIdentityKey), encoded); err != nil {
		return errors.Wrap(ctx, err, "write store writer identity")
	}
	return nil
}

// applyDelta puts and deletes one publication's entries in a bucket.
func applyDelta(
	ctx context.Context,
	bucket kv.Bucket,
	puts []StoredEntry,
	deletes []string,
) error {
	for _, entry := range puts {
		value, err := encodeRecord(ctx, entry)
		if err != nil {
			return err
		}
		if err := bucket.Put(ctx, []byte(entry.Filename), value); err != nil {
			return errors.Wrapf(ctx, err, "put store entry %s", entry.Filename)
		}
	}
	for _, name := range deletes {
		if err := bucket.Delete(ctx, []byte(name)); err != nil {
			return errors.Wrapf(ctx, err, "delete store entry %s", name)
		}
	}
	return nil
}

// Path returns the store file path.
func (s *boltStore) Path() string {
	return s.path
}

// Close releases the store file.
func (s *boltStore) Close() error {
	return s.db.Close()
}

// storeBucketName returns the entry bucket name for one key.
func storeBucketName(key Key) kv.BucketName {
	return kv.NewBucketName(storeBucketPrefix + key.VaultPath + "\x00" + key.PagesDir)
}

// discardedStore stands in for a store that could not be opened. It serves
// nothing, warns exactly once per Load, and fails Write so the caller warns.
type discardedStore struct {
	path   string
	reason string
	warnf  func(format string, args ...any)
}

// Load warns once and reports the key absent.
func (d *discardedStore) Load(
	ctx context.Context,
	key Key,
) ([]StoredEntry, bool, error) {
	d.warnf("page index store discarded (%s): %s", d.reason, d.path)
	return nil, false, nil
}

// Write fails; the caller owns the warning.
func (d *discardedStore) Write(
	ctx context.Context,
	key Key,
	puts []StoredEntry,
	deletes []string,
) error {
	return errors.Errorf(ctx, "page index store discarded (%s): %s", d.reason, d.path)
}

// Path returns the intended store file path.
func (d *discardedStore) Path() string {
	return d.path
}

// Close is a no-op: nothing was opened.
func (d *discardedStore) Close() error {
	return nil
}
