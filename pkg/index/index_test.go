// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index_test

import (
	"context"
	stderrors "errors"
	"sort"
	"strings"
	"sync"
	"time"

	libtime "github.com/bborbe/time"
	"github.com/bborbe/vault-cli/pkg/domain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/index"
)

// storageFake is a reader and lister pair whose per-key content and blocking
// behaviour the tests control. The counterfeiter fakes underneath record every
// call.
type storageFake struct {
	reader *mocks.IndexPageReader
	lister *mocks.IndexDirectoryLister

	mu       sync.Mutex
	pages    map[index.Key][]*domain.Page
	versions map[index.Key]map[string]int64
	gate     chan struct{}
	entered  chan struct{}
	failure  error

	// readMu guards the per-read gate: while readBlocked is set, a read of a
	// gated name registers its own gate and waits on it until the test opens it.
	readMu      sync.Mutex
	readBlocked bool
	readGated   map[string]bool
	readGates   map[string][]chan struct{}
	readEntered chan string
}

func newStorageFake() *storageFake {
	fake := &storageFake{
		reader:   &mocks.IndexPageReader{},
		lister:   &mocks.IndexDirectoryLister{},
		pages:    map[index.Key][]*domain.Page{},
		versions: map[index.Key]map[string]int64{},
	}
	fake.reader.ReadEntryStub = fake.readEntry
	fake.reader.ReadPageStub = fake.readFullPage
	fake.lister.ListFilesStub = fake.listFiles
	return fake
}

// fingerprint derives a file's fingerprint from its version counter, so a
// content change always changes the fingerprint.
func fingerprint(version int64) index.FileFingerprint {
	return index.FileFingerprint{
		Size:    version,
		ModTime: time.Unix(version, 0).UTC(),
	}
}

func (f *storageFake) listFiles(
	_ context.Context,
	vaultPath string,
	pagesDir string,
) ([]index.FileEntry, error) {
	key := index.NewKey(vaultPath, pagesDir)

	f.mu.Lock()
	gate, entered := f.gate, f.entered
	entries := make([]index.FileEntry, 0, len(f.pages[key]))
	for _, page := range f.pages[key] {
		name := page.Name
		entries = append(entries, index.FileEntry{
			Name:        name + ".md",
			Fingerprint: fingerprint(f.versions[key][name]),
		})
	}
	f.mu.Unlock()

	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if gate != nil {
		<-gate
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failure != nil {
		return nil, f.failure
	}
	// The production lister reads os.ReadDir, which is filename ascending.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// readFullPage reads the file's full page by its bare name.
func (f *storageFake) readFullPage(
	ctx context.Context,
	vaultPath string,
	pagesDir string,
	name string,
) (*domain.Page, error) {
	entry, _, err := f.readEntry(ctx, vaultPath, pagesDir, name+".md")
	if err != nil {
		return nil, err
	}
	return entry.Page, nil
}

// readEntry reads the file's current page, captured before any gate wait so a
// blocked read still reports the content the caller would have seen.
func (f *storageFake) readEntry(
	_ context.Context,
	vaultPath string,
	pagesDir string,
	filename string,
) (*index.Entry, index.FileFingerprint, error) {
	key := index.NewKey(vaultPath, pagesDir)
	name := strings.TrimSuffix(filename, ".md")

	f.mu.Lock()
	var page *domain.Page
	for _, candidate := range f.pages[key] {
		if candidate.Name == name {
			page = candidate
			break
		}
	}
	version := f.versions[key][name]
	f.mu.Unlock()

	if gate, entered := f.enterRead(filename); gate != nil {
		select {
		case entered <- filename:
		default:
		}
		<-gate
	}

	if page == nil {
		return nil, index.FileFingerprint{}, stderrors.New("page not found: " + name)
	}
	return &index.Entry{Page: page, OpenQuestions: []domain.OpenQuestion{}}, fingerprint(version), nil
}

// enterRead registers a read of filename against the active read gate, if the
// filename is gated, and returns the gate the read must wait on.
func (f *storageFake) enterRead(filename string) (chan struct{}, chan string) {
	f.readMu.Lock()
	defer f.readMu.Unlock()
	if !f.readBlocked || !f.readGated[filename] {
		return nil, nil
	}
	gate := make(chan struct{})
	f.readGates[filename] = append(f.readGates[filename], gate)
	return gate, f.readEntered
}

// blockReads makes every read of the named files wait on its own gate until the
// test opens it. The returned channel reports the filename of each read that
// entered, and the release func opens the nth gate registered for a filename,
// so the test decides the order the reads finish in.
func (f *storageFake) blockReads(names ...string) (<-chan string, func(name string, n int)) {
	entered := make(chan string, 64)
	gated := map[string]bool{}
	for _, name := range names {
		gated[name] = true
	}
	f.readMu.Lock()
	f.readBlocked = true
	f.readGated = gated
	f.readGates = map[string][]chan struct{}{}
	f.readEntered = entered
	f.readMu.Unlock()
	return entered, func(name string, n int) {
		f.readMu.Lock()
		gate := f.readGates[name][n]
		f.readMu.Unlock()
		close(gate)
	}
}

// block makes every listing wait until the returned release func runs. The
// returned channel reports each call that entered the stub.
func (f *storageFake) block() (<-chan struct{}, func()) {
	gate := make(chan struct{})
	entered := make(chan struct{}, 64)
	f.mu.Lock()
	f.gate = gate
	f.entered = entered
	f.mu.Unlock()
	return entered, func() {
		f.mu.Lock()
		f.gate = nil
		f.entered = nil
		f.mu.Unlock()
		close(gate)
	}
}

func (f *storageFake) setPages(key index.Key, titles ...string) {
	key = index.NewKey(key.VaultPath, key.PagesDir)
	pages := make([]*domain.Page, 0, len(titles))
	for _, title := range titles {
		pages = append(pages, newPage(title))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	previous := map[string]*domain.Page{}
	for _, page := range f.pages[key] {
		previous[page.Name] = page
	}
	versions := f.versions[key]
	if versions == nil {
		versions = map[string]int64{}
		f.versions[key] = versions
	}
	for _, page := range pages {
		name := page.Name
		if old, ok := previous[name]; !ok || old.Content != page.Content {
			versions[name]++
		}
	}
	f.pages[key] = pages
}

// putPage writes one file into the key's folder, replacing any file of the same
// name, and bumps its version so its fingerprint changes. Content is given
// separately from the name, so a test can modify a file without renaming it.
func (f *storageFake) putPage(key index.Key, name, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key = index.NewKey(key.VaultPath, key.PagesDir)
	replaced := false
	pages := make([]*domain.Page, 0, len(f.pages[key])+1)
	for _, page := range f.pages[key] {
		if page.Name == name {
			pages = append(pages, newPageContent(name, content))
			replaced = true
			continue
		}
		pages = append(pages, page)
	}
	if !replaced {
		pages = append(pages, newPageContent(name, content))
	}
	f.pages[key] = pages
	if f.versions[key] == nil {
		f.versions[key] = map[string]int64{}
	}
	f.versions[key][name]++
}

// dropPage removes one file from the key's folder.
func (f *storageFake) dropPage(key index.Key, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key = index.NewKey(key.VaultPath, key.PagesDir)
	pages := make([]*domain.Page, 0, len(f.pages[key]))
	for _, page := range f.pages[key] {
		if page.Name == name {
			continue
		}
		pages = append(pages, page)
	}
	f.pages[key] = pages
}

func (f *storageFake) setFailure(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failure = err
}

func newPage(title string) *domain.Page {
	return newPageContent(title, "# "+title)
}

// newPageContent builds a page whose file name and content differ, so a test
// can change a file's content without changing its name.
func newPageContent(name, content string) *domain.Page {
	return domain.NewPage(
		map[string]any{"title": name},
		domain.FileMetadata{Name: name, FilePath: "/vault/" + name + ".md"},
		domain.Content(content),
	)
}

func titles(pages []*domain.Page) []string {
	result := make([]string, 0, len(pages))
	for _, page := range pages {
		result = append(result, page.GetString("title"))
	}
	return result
}

func callCountFor(fake *storageFake, key index.Key) int {
	want := index.NewKey(key.VaultPath, key.PagesDir)
	count := 0
	for i := 0; i < fake.lister.ListFilesCallCount(); i++ {
		_, vaultPath, pagesDir := fake.lister.ListFilesArgsForCall(i)
		if index.NewKey(vaultPath, pagesDir) == want {
			count++
		}
	}
	return count
}

func fastWaiter() libtime.WaiterDuration {
	return libtime.WaiterDurationFunc(func(ctx context.Context, duration libtime.Duration) error {
		timer := time.NewTimer(time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	})
}

func newIndex(fake *storageFake) index.PageIndex {
	return index.NewPageIndex(
		fake.reader, fake.lister, libtime.NewCurrentDateTime(), fastWaiter(), index.TestMetrics,
	)
}

var _ = Describe("PageIndex", func() {
	var (
		ctx  context.Context
		fake *storageFake
	)

	BeforeEach(func() {
		ctx = context.Background()
		fake = newStorageFake()
	})

	Describe("ListPages", func() {
		It("serves repeat reads of a clean key without touching storage", func() {
			fake.setPages(index.NewKey("/vault-a", "24 Tasks"), "one", "two")
			idx := newIndex(fake)

			first, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(first)).To(Equal([]string{"one", "two"}))
			Expect(fake.lister.ListFilesCallCount()).To(Equal(1))

			for i := 0; i < 5; i++ {
				again, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
				Expect(err).To(BeNil())
				Expect(again[0]).To(BeIdenticalTo(first[0]))
			}
			Expect(fake.lister.ListFilesCallCount()).To(Equal(1))
		})

	})

	Describe("Cold read", func() {
		It("shares a single build across concurrent cold reads", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "one")
			idx := newIndex(fake)
			entered, release := fake.block()

			results := make([][]*domain.Page, 8)
			errs := make([]error, 8)
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					defer GinkgoRecover()
					results[i], errs[i] = idx.ListPages(ctx, "/vault-a", "24 Tasks")
				}(i)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				Expect(idx.Build(ctx, []index.Key{key})).To(BeNil())
			}()

			Eventually(entered).Should(Receive())
			Consistently(fake.lister.ListFilesCallCount, "200ms").Should(Equal(1))

			release()
			wg.Wait()

			Expect(fake.lister.ListFilesCallCount()).To(Equal(1))
			for i := range results {
				Expect(errs[i]).To(BeNil())
				Expect(titles(results[i])).To(Equal([]string{"one"}))
			}
		})
	})

	Describe("Refresh", func() {
		It("rebuilds only the requested key", func() {
			aTasks := index.NewKey("/vault-a", "24 Tasks")
			aGoals := index.NewKey("/vault-a", "25 Goals")
			bTasks := index.NewKey("/vault-b", "24 Tasks")
			bGoals := index.NewKey("/vault-b", "25 Goals")
			all := []index.Key{aTasks, aGoals, bTasks, bGoals}
			for _, key := range all {
				fake.setPages(key, "old")
			}
			idx := newIndex(fake)
			Expect(idx.Build(ctx, all)).To(BeNil())
			Expect(fake.lister.ListFilesCallCount()).To(Equal(4))

			fake.setPages(aTasks, "new")
			Expect(idx.Refresh(ctx, aTasks)).To(BeNil())

			Expect(callCountFor(fake, aTasks)).To(Equal(2))
			for _, key := range []index.Key{aGoals, bTasks, bGoals} {
				Expect(callCountFor(fake, key)).To(Equal(1))
			}
			pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(pages)).To(Equal([]string{"new"}))
		})

		It("keeps serving the previous snapshot while a rebuild is blocked", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "old")
			idx := newIndex(fake)
			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())

			entered, release := fake.block()
			fake.setPages(key, "new")
			refreshDone := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				refreshDone <- idx.Refresh(ctx, key)
			}()
			Eventually(entered).Should(Receive())

			start := time.Now()
			pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(pages)).To(Equal([]string{"old"}))
			Expect(time.Since(start)).To(BeNumerically("<", 100*time.Millisecond))

			release()
			Expect(<-refreshDone).To(BeNil())
			pages, err = idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(pages)).To(Equal([]string{"new"}))
		})

		It("coalesces a burst of refreshes into one follow-up", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "old")
			idx := newIndex(fake)
			entered, release := fake.block()

			first := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				first <- idx.Refresh(ctx, key)
			}()
			Eventually(entered).Should(Receive())

			rest := make([]chan error, 3)
			for i := range rest {
				rest[i] = make(chan error, 1)
				done := rest[i]
				go func() {
					defer GinkgoRecover()
					done <- idx.Refresh(ctx, key)
				}()
			}
			Consistently(fake.lister.ListFilesCallCount, "200ms").Should(Equal(1))

			fake.setPages(key, "new")
			release()

			Expect(<-first).To(BeNil())
			for _, done := range rest {
				Expect(<-done).To(BeNil())
				pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
				Expect(err).To(BeNil())
				Expect(titles(pages)).To(Equal([]string{"new"}))
			}
			Expect(fake.lister.ListFilesCallCount()).To(Equal(2))
		})
	})

	Describe("dirty marks", func() {
		It("rebuilds once on the next read and serves the new content", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "old")
			idx := newIndex(fake)
			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())

			fake.setPages(key, "new")
			idx.MarkDirty(key)
			Expect(fake.lister.ListFilesCallCount()).To(Equal(1))

			pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(pages)).To(Equal([]string{"new"}))
			Expect(fake.lister.ListFilesCallCount()).To(Equal(2))
		})

		It("shares one rebuild between two concurrent reads after one mark", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "old")
			idx := newIndex(fake)
			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())

			fake.setPages(key, "new")
			idx.MarkDirty(key)
			entered, release := fake.block()

			var wg sync.WaitGroup
			results := make([][]*domain.Page, 2)
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					defer GinkgoRecover()
					results[i], _ = idx.ListPages(ctx, "/vault-a", "24 Tasks")
				}(i)
			}
			Eventually(entered).Should(Receive())
			release()
			wg.Wait()

			Expect(fake.lister.ListFilesCallCount()).To(Equal(2))
			for i := range results {
				Expect(titles(results[i])).To(Equal([]string{"new"}))
			}
		})

		It("forces a reader onto a rebuild that started after the mark", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "old")
			idx := newIndex(fake)
			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())

			entered, release := fake.block()
			refreshDone := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				refreshDone <- idx.Refresh(ctx, key)
			}()
			Eventually(entered).Should(Receive())

			idx.MarkDirty(key)
			fake.setPages(key, "new")

			readDone := make(chan []*domain.Page, 1)
			readErr := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
				readDone <- pages
				readErr <- err
			}()
			Consistently(fake.lister.ListFilesCallCount, "150ms").Should(Equal(2))

			release()
			Expect(<-refreshDone).To(BeNil())
			Expect(<-readErr).To(BeNil())
			Expect(titles(<-readDone)).To(Equal([]string{"new"}))
			Expect(fake.lister.ListFilesCallCount()).To(Equal(3))
		})

		It("re-reads every file of every known key on ForceReload", func() {
			aTasks := index.NewKey("/vault-a", "24 Tasks")
			bTasks := index.NewKey("/vault-b", "24 Tasks")
			fake.setPages(aTasks, "a")
			fake.setPages(bTasks, "b")
			idx := newIndex(fake)
			Expect(idx.Build(ctx, []index.Key{aTasks, bTasks})).To(BeNil())
			Expect(fake.lister.ListFilesCallCount()).To(Equal(2))
			before := fake.reader.ReadEntryCallCount()

			idx.ForceReload()
			// The mark alone reads and lists nothing.
			Expect(fake.lister.ListFilesCallCount()).To(Equal(2))
			Expect(fake.reader.ReadEntryCallCount()).To(Equal(before))

			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			_, err = idx.ListPages(ctx, "/vault-b", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(fake.lister.ListFilesCallCount()).To(Equal(4))
			// Every file is re-read although no fingerprint changed.
			Expect(fake.reader.ReadEntryCallCount() - before).To(Equal(2))
		})

		It("ignores a dirty mark for an unknown key", func() {
			idx := newIndex(fake)
			idx.MarkDirty(index.NewKey("/unknown", "24 Tasks"))
			Expect(fake.lister.ListFilesCallCount()).To(Equal(0))
		})
	})

	Describe("Failed rebuild", func() {
		It("keeps the previous snapshot and retries while the key stays dirty", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "old")
			idx := newIndex(fake)
			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())

			idx.MarkDirty(key)
			fake.setFailure(stderrors.New("boom"))

			pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(pages)).To(Equal([]string{"old"}))
			Expect(fake.lister.ListFilesCallCount()).To(Equal(2))

			Expect(idx.Refresh(ctx, key)).To(BeNil())
			Expect(fake.lister.ListFilesCallCount()).To(Equal(3))

			fake.setFailure(nil)
			fake.setPages(key, "new")
			pages, err = idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(pages)).To(Equal([]string{"new"}))
			Expect(fake.lister.ListFilesCallCount()).To(Equal(4))
		})

		It("returns an error when the very first build fails", func() {
			idx := newIndex(fake)
			fake.setFailure(stderrors.New("boom"))
			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Rescan", func() {
		It("refreshes every known key once the interval has passed", func() {
			clock := libtime.NewCurrentDateTime()
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			clock.SetNow(libtime.DateTime(start))

			aTasks := index.NewKey("/vault-a", "24 Tasks")
			bTasks := index.NewKey("/vault-b", "24 Tasks")
			fake.setPages(aTasks, "a")
			fake.setPages(bTasks, "b")
			idx := index.NewPageIndex(fake.reader, fake.lister, clock, fastWaiter(), index.TestMetrics)
			Expect(idx.Build(ctx, []index.Key{aTasks, bTasks})).To(BeNil())
			Expect(fake.lister.ListFilesCallCount()).To(Equal(2))

			rescanCtx, cancel := context.WithCancel(ctx)
			rescanDone := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				rescanDone <- idx.Rescan(rescanCtx)
			}()

			Consistently(fake.lister.ListFilesCallCount, "100ms").Should(Equal(2))

			fake.setPages(aTasks, "a2")
			fake.setPages(bTasks, "b2")
			clock.SetNow(libtime.DateTime(start.Add(time.Minute)))

			Eventually(fake.lister.ListFilesCallCount, "2s").Should(Equal(4))
			Eventually(func() []string {
				pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
				Expect(err).To(BeNil())
				return titles(pages)
			}, "2s").Should(Equal([]string{"a2"}))
			Consistently(fake.lister.ListFilesCallCount, "100ms").Should(Equal(4))

			cancel()
			Eventually(rescanDone, "2s").Should(Receive(BeNil()))
		})

		It("keeps the rescan interval inside the 60s ceiling", func() {
			Expect(index.RescanInterval).To(BeNumerically("<=", 60*time.Second))
		})
	})

	Describe("context cancellation", func() {
		It("fails a waiting reader without aborting the build it waits on", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "old")
			idx := newIndex(fake)
			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())

			idx.MarkDirty(key)
			fake.setPages(key, "new")
			entered, release := fake.block()

			builderDone := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
				builderDone <- err
			}()
			Eventually(entered).Should(Receive())

			readCtx, cancelRead := context.WithCancel(ctx)
			readDone := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				_, err := idx.ListPages(readCtx, "/vault-a", "24 Tasks")
				readDone <- err
			}()
			Consistently(fake.lister.ListFilesCallCount, "100ms").Should(Equal(2))
			cancelRead()
			Eventually(readDone).Should(Receive(HaveOccurred()))

			release()
			Expect(<-builderDone).To(BeNil())
			pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(pages)).To(Equal([]string{"new"}))
		})

		It("runs an owned follow-up even when the caller's ctx is cancelled", func() {
			key := index.NewKey("/vault-a", "24 Tasks")
			fake.setPages(key, "old")
			idx := newIndex(fake)
			_, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())

			entered, release := fake.block()
			fake.setPages(key, "new")
			refreshDone := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				refreshDone <- idx.Refresh(ctx, key)
			}()
			Eventually(entered).Should(Receive())

			idx.MarkDirty(key)

			readCtx, cancelRead := context.WithCancel(ctx)
			readDone := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				_, err := idx.ListPages(readCtx, "/vault-a", "24 Tasks")
				readDone <- err
			}()
			Consistently(fake.lister.ListFilesCallCount, "150ms").Should(Equal(2))

			cancelRead()
			release()
			Expect(<-refreshDone).To(BeNil())
			Eventually(readDone, "2s").Should(Receive(HaveOccurred()))

			Expect(fake.lister.ListFilesCallCount()).To(Equal(3))
			pages, err := idx.ListPages(ctx, "/vault-a", "24 Tasks")
			Expect(err).To(BeNil())
			Expect(titles(pages)).To(Equal([]string{"new"}))
			Expect(fake.lister.ListFilesCallCount()).To(Equal(3))
		})

		It("returns an error from Build when ctx is already cancelled", func() {
			idx := newIndex(fake)
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			err := idx.Build(cancelled, []index.Key{index.NewKey("/vault-a", "24 Tasks")})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("NewKey", func() {
		It("cleans both parts so equivalent paths share a key", func() {
			Expect(index.NewKey("/v/", "24 Tasks/")).To(Equal(index.NewKey("/v", "24 Tasks")))
		})
	})
})
