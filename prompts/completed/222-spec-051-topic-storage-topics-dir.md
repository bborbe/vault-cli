---
status: completed
spec: [051-topic-command-ladder]
summary: Added storage.TopicStorage with the TopicsDir config plumbing and a traversal-refusing FindTopicByName, plus 21 Ginkgo specs covering the configured-directory, absent-directory and traversal paths
execution_id: vault-cli-topic-ladder-exec-222-spec-051-topic-storage-topics-dir
dark-factory-version: v0.196.0
created: "2026-09-20T20:29:34Z"
queued: "2026-09-20T21:28:57Z"
started: "2026-09-20T21:34:02Z"
completed: "2026-09-20T21:40:08Z"
branch: dark-factory/topic-command-ladder
---

# Topic storage: `storage.TopicStorage`, the `TopicsDir` config plumbing, and the traversal refusal

<summary>
- The binary gains a storage layer for topic pages: read a topic by name, and write a topic back to disk.
- Reading a topic by name searches the vault's configured topics directory, and that directory is used exactly as configured with no fallback.
- A vault that declares no topics directory keeps working and resolves to `23 Topics`.
- A configured directory that does not exist yet yields an empty result rather than an error, and a write creates the path it needs instead of failing.
- A topic name that would resolve outside the topics directory is refused before any file is read or written, so a traversal name cannot reach a page elsewhere on disk.
- Writing a topic preserves every frontmatter key the topic entity did not recognise, and writes the file with the same permissions and the same frontmatter block shape the goal family uses.
- The goal, task, theme, objective and vision storage paths, and every existing test, behave exactly as they do today.
- No command, no flag and no output changes yet — this step adds the storage the command family will call.
</summary>

<objective>
Add the topic storage type and thread the vault's topics directory through the storage configuration, so prompt 3's command family has a read/write path for topic pages that honours the configured directory verbatim and cannot be walked out of by a crafted name. This is spec 051's prompt 2 of 4: it covers Desired Behavior 3 and Acceptance Criteria 11 and 12, and it depends on prompt 1's `domain.Topic` entity (it will not compile without it).
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `pkg/storage/storage.go` — the file you edit. Read `Config` (its directory-key block runs `TasksDir` → `DailyDir`, then `Excludes`), `NewConfigFromVault` (each key reads a `vault.GetXxxDir()` accessor), `DefaultConfig` (the same keys with literal defaults), the eight `//counterfeiter:generate` directives and the interfaces under them (`TaskStorage`, `GoalStorage`, `ThemeStorage`, `ObjectiveStorage`, `VisionStorage`, `DailyNoteStorage`, `PageStorage`, `DecisionStorage`), the composed `Storage` interface (it embeds all eight and then declares the five "Legacy methods"), `NewStorage`, the `markdownStorage` struct, and the per-entity constructors `NewGoalStorage` … `NewVisionStorage`.
- `pkg/storage/goal.go` — the storage implementation you mirror: `goalStorage` embedding `*baseStorage`, `ReadGoal`, `readGoalFromPath`, `WriteGoal`, `FindGoalByName`. Note where each one wraps its error and with what message.
- `pkg/storage/base.go` — the shared helpers you call rather than reimplement: `parseToFrontmatterMap`, `serializeMapAsFrontmatter`, `findFileByName` (note its exact not-found behaviour: it stats the exact `<dir>/<name>.md` path first, then walks `dir` for a case-insensitive match, and returns `errors.Wrapf(ctx, ErrNotFound, "%s", name)` on a miss; it also returns `ErrNotFound` when `dir` itself does not exist), `readEntityComponentsFromPath` (the shared read that returns `(map[string]any, domain.FileMetadata, domain.Content, error)`), `isSymlinkOutsideVault`, `isSymlink`, `isExcluded`.
- `pkg/storage/page.go` — `ListPages(ctx, vaultPath, pagesDir)` is the listing path prompt 3's `topic list` uses. Read its `fs.ErrNotExist` branch: an absent directory returns `nil, nil` with **no** error. That is the existing behaviour Acceptance Criterion 11 relies on; you do not change this file.
- `pkg/storage/errors.go` — `ErrNotFound`, a stdlib sentinel. `pkg/ops/vault_dispatcher.go`'s `FirstSuccess` tests it with `errors.Is` to decide whether to continue to the next vault, so any not-found you raise must keep it in the chain.
- `pkg/storage/goal_test.go` — the test shape you copy (external `package storage_test`, `storage.NewStorage(nil)`, a `os.MkdirTemp` vault, `BeforeEach`/`AfterEach` cleanup, `Describe`/`It`).
- `pkg/storage/export_test.go` — the test-only exports of unexported `baseStorage` methods (`NewBaseStorageForTest`, `ParseToFrontmatterMapForTest`, `SerializeMapAsFrontmatterForTest`, `FindFileByNameForTest`). Read it; you do not need to change it.
- `pkg/config/config.go` — READ ONLY. `Vault.TopicsDir` and `func (v *Vault) GetTopicsDir() string` already exist and already default to the literal `23 Topics`; they shipped under spec 048. `expandVaultPaths` copies the vault struct, and both `GetVault` and `GetAllVaults` return that copy, so the field survives every resolution path with no loader change. You consume this accessor; you do not touch this file.
- `docs/development-patterns.md` § "Adding a New Command" step 2 and § "Entity Structure" → **Storage** — the layering, the bare-wikilink invariant (`parseToFrontmatterMap`'s `quoteBareWikilinks` pass is the single chokepoint every read must go through, so delegate to `readEntityComponentsFromPath` rather than reading the file yourself), and the `Rendering caveat` about bare YAML dates re-serialising as RFC3339.
- `docs/dod.md` — this repo's `validationPrompt`.
- `specs/in-progress/051-topic-command-ladder.md` — the spec. Read its Non-goals, Acceptance Criteria 11 and 12, Constraints, the Failure Modes rows for the absent directory / the absent configured directory / the traversal attempt, and the Security section. Every requirement below comes from them.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — Interface → Constructor → Struct → Method, error wrapping.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` API; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-security-linting.md` — gosec file-permission rules: files `0600`, directories `0750`; `#nosec` only with a reason.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter directives and where the generated fakes land.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits, license headers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

**Current state of the pieces you copy, quoted verbatim:**

The `Config` struct:

```go
// Config holds the configuration for storage paths.
type Config struct {
	TasksDir      string
	GoalsDir      string
	ThemesDir     string
	ObjectivesDir string
	VisionDir     string
	DailyDir      string
	Excludes      []string
}
```

The vault-facing constructor:

```go
// NewConfigFromVault creates a Config from a Vault.
func NewConfigFromVault(vault *config.Vault) *Config {
	return &Config{
		TasksDir:      vault.GetTasksDir(),
		GoalsDir:      vault.GetGoalsDir(),
		ThemesDir:     vault.GetThemesDir(),
		ObjectivesDir: vault.GetObjectivesDir(),
		VisionDir:     vault.GetVisionDir(),
		DailyDir:      vault.GetDailyDir(),
		Excludes:      vault.GetExcludes(),
	}
}
```

The sibling interface you mirror:

```go
//counterfeiter:generate -o ../../mocks/goal-storage.go --fake-name GoalStorage . GoalStorage
type GoalStorage interface {
	WriteGoal(ctx context.Context, goal *domain.Goal) error
	FindGoalByName(ctx context.Context, vaultPath string, name string) (*domain.Goal, error)
}
```

The sibling implementation's find:

```go
// FindGoalByName searches for a goal by name in the vault.
func (g *goalStorage) FindGoalByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Goal, error) {
	goalsDir := filepath.Join(vaultPath, g.config.GoalsDir)
	matchedPath, matchedName, err := g.findFileByName(ctx, goalsDir, name)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "find goal file")
	}
	return g.readGoalFromPath(ctx, matchedPath, matchedName, vaultPath)
}
```

The shared read helper's signature:

```go
func (b *baseStorage) readEntityComponentsFromPath(
	ctx context.Context,
	filePath string,
	name string,
	vaultPath string,
) (map[string]any, domain.FileMetadata, domain.Content, error)
```

**Environment facts that shape this prompt:**

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies (`fatal: not a git repository`) reports a false pass. Acceptance Criterion 12's `git status` half is operator-side; section 6 below gives the byte-compare you satisfy instead.
2. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it — `go build`/`go test` otherwise try to read a masked `.git`.
3. **`make generate` regenerates `mocks/` from scratch** (`rm -rf mocks` then `go generate ./...`). You add the `//counterfeiter:generate` directive; you do not hand-write `mocks/topic-storage.go`. Never run `go mod vendor`.
</context>

<requirements>

## 0. Scope — three files, plus one generated mock

- `pkg/storage/storage.go` — edited: one `Config` field, two constructor lines, one interface, one counterfeiter directive, one `Storage` embed, one `markdownStorage` field, one `NewTopicStorage` constructor.
- `pkg/storage/topic.go` — NEW.
- `pkg/storage/topic_test.go` — NEW.
- `mocks/topic-storage.go` — GENERATED, never hand-written: the `//counterfeiter:generate` directive you add in § 1d produces it. `make generate` runs `rm -rf mocks` then `go generate ./...`, and `mocks/` is tracked in git (it is not gitignored), so this file lands in the change alongside the three above.

Nothing else. `pkg/storage/base.go`, `pkg/storage/goal.go`, `pkg/storage/task.go`, `pkg/storage/theme.go`, `pkg/storage/objective.go`, `pkg/storage/vision.go`, `pkg/storage/page.go`, `pkg/storage/decision.go`, `pkg/storage/errors.go`, `pkg/storage/export_test.go`, `pkg/config/**`, `pkg/domain/**`, `pkg/ops/**`, `pkg/cli/**`, `go.mod`, `go.sum`, `CHANGELOG.md`, `README.md` and `docs/**` are all untouched. No new dependency.

## 1. `pkg/storage/storage.go` — the `TopicsDir` config key

### 1a. The `Config` field

Add exactly this line immediately after the `DailyDir` line and immediately before the `Excludes` line:

```go
	TopicsDir     string
```

- Field name `TopicsDir`. No struct tag (the sibling keys have none), no alignment change to any other line.
- Do NOT reorder the other keys, do NOT add a second field, do NOT add a validation method or a default constant.

### 1b. `NewConfigFromVault`

Add exactly this line immediately after the `DailyDir:` line and immediately before the `Excludes:` line:

```go
		TopicsDir:     vault.GetTopicsDir(),
```

`Vault.GetTopicsDir()` already returns the configured value or the literal `23 Topics`. Do NOT inline a default here, do NOT stat or clean the value, do NOT fall back to a hardcoded path if the accessor returns empty — the accessor cannot return empty, and the configured path must be used verbatim.

### 1c. `DefaultConfig`

Add exactly this line immediately after the `DailyDir:` line:

```go
		TopicsDir:     "23 Topics",
```

`DefaultConfig()` is what `storage.NewStorage(nil)` and every `NewXxxStorage(nil)` use. `23 Topics` is the same literal `Vault.GetTopicsDir()` defaults to, and it must be the single literal — no constant, no second default, no environment variable.

### 1d. The `TopicStorage` interface

Add this block immediately after the `VisionStorage` interface block and immediately before the `DailyNoteStorage` interface block:

```go
//counterfeiter:generate -o ../../mocks/topic-storage.go --fake-name TopicStorage . TopicStorage
type TopicStorage interface {
	WriteTopic(ctx context.Context, topic *domain.Topic) error
	FindTopicByName(ctx context.Context, vaultPath string, name string) (*domain.Topic, error)
}
```

- Exactly two methods, in that order, with those signatures. `TopicStorage` is frozen; the counterfeiter directive's `--fake-name TopicStorage` and output path `../../mocks/topic-storage.go` are frozen.
- Do NOT add a `ReadTopic(ctx, vaultPath, topicID)` method and do NOT add a `ListTopics` method. `ListTopics` is not needed — prompt 3's `topic list` goes through the existing `PageStorage.ListPages` with the topics directory, exactly as `goal list` does. `ReadTopic` is not needed — the goal family's `ReadGoal` exists only as a legacy method with no ops caller, and nothing in this spec calls a topic read-by-id. Adding either would be dead code.
- Do NOT add a `domain.TopicID` type. It would have no consumer.

### 1e. The composed `Storage` interface

Add exactly this line to the `Storage` interface's embed block, immediately after the `VisionStorage` line and immediately before the `DailyNoteStorage` line:

```go
	TopicStorage
```

Do NOT add a `ReadTopic` entry to the "Legacy methods" block. That block exists for the goal/task/theme/objective/vision read-by-id methods that the storage tests use; the topic family has no such legacy caller, and adding one would require a `domain.TopicID` type (§ 1d).

### 1f. `NewStorage` and `markdownStorage`

In `NewStorage`, add exactly this line immediately after the `visionStorage:` line:

```go
		topicStorage:     &topicStorage{baseStorage: base},
```

In the `markdownStorage` struct, add exactly this field immediately after the `*visionStorage` line:

```go
	*topicStorage
```

Both are frozen in placement: the embed order in `markdownStorage` mirrors the constructor order in `NewStorage`, and `topicStorage` follows `visionStorage` in both.

### 1g. `NewTopicStorage`

Add this function immediately after `NewVisionStorage`:

```go
// NewTopicStorage creates a storage for topic operations only.
func NewTopicStorage(storageConfig *Config) TopicStorage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &topicStorage{baseStorage: &baseStorage{config: storageConfig}}
}
```

Its shape is frozen — it mirrors `NewGoalStorage` and `NewVisionStorage` exactly, including the `nil` → `DefaultConfig()` branch.

## 2. `pkg/storage/topic.go` — the storage implementation

Standard BSD license header (copy it from `pkg/storage/goal.go` verbatim), `package storage`. Imports: `context`, `os`, `path/filepath`, `strings`, `github.com/bborbe/errors`, `github.com/bborbe/vault-cli/pkg/domain`. Add nothing else — no `log/slog`, no `io/fs`, no `time`.

### 2a. The type

```go
type topicStorage struct {
	*baseStorage
}
```

Unexported, embedding `*baseStorage` by pointer, exactly like `goalStorage`, `pageStorage` and `visionStorage`. That embedding is what gives you `parseToFrontmatterMap`, `serializeMapAsFrontmatter`, `findFileByName` and `readEntityComponentsFromPath`.

### 2b. The traversal refusal helper

```go
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
```

- Unexported, two parameters, `bool` return, no error, no I/O, no context. Frozen signature.
- The `[[`/`]]` trim mirrors the first two lines of `baseStorage.findFileByName` — a bracket-wrapped name must be measured the same way it is resolved, or `[[../x]]` would slip past the check and then be unwrapped into a traversal by `findFileByName`.
- Do NOT add a guard that rejects any name containing a separator. A nested name inside the topics directory is legitimate; only a name that *resolves outside* is refused.
- Do NOT add a symlink check here. `readEntityComponentsFromPath` already runs `isSymlinkOutsideVault` and `WriteTopic` already runs `isSymlink`; duplicating either here is dead code.

### 2c. `FindTopicByName`

```go
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
```

Non-negotiable properties:

- The topics directory is `filepath.Join(vaultPath, t.config.TopicsDir)` — the configured value, used verbatim. No existence check, no `os.Stat`, no fallback to `23 Topics` when the configured directory is absent. `findFileByName` already returns an `ErrNotFound`-class error when `dir` does not exist, which is exactly the "configured-but-absent directory is a miss, not a fallback" behaviour Acceptance Criterion 11 requires.
- The refusal is `errors.Wrapf(ctx, ErrNotFound, ...)`, **not** `errors.Errorf`. `pkg/ops/vault_dispatcher.go`'s `FirstSuccess` only continues to the next vault when `errors.Is(err, storage.ErrNotFound)` holds; a bare `errors.Errorf` would turn a traversal name in one vault into a hard failure that aborts a multi-vault search. `github.com/bborbe/errors`'s `Wrapf` delegates to `github.com/pkg/errors`, which preserves the `Unwrap` chain, so `errors.Is` reaches the sentinel.
- The message names both the name and the directory. The spec's Failure Modes row for a `show` on a topic that does not exist requires the error to name "the topic and the directory searched", and this branch plus the `find topic file in %s` wrap below are the two places that produce it.
- The `find topic file in %s` wrap is what names the directory on the genuine-miss path. Do NOT collapse it to `errors.Wrap(ctx, err, "find topic file")` — the directory would then be missing from the message.
- Do NOT change `pkg/storage/base.go`'s `findFileByName` to add containment there. The goal and task families go through the same helper, and moving the check into it would change their behaviour on a traversal name — which the spec's Constraint "The goal binary surface stays at exactly twelve leaves with unchanged names, flags, argument counts, output, and exit codes" forbids. This is the one intentional divergence from a byte-for-byte goal mirror, and it exists because Acceptance Criterion 12 asserts it.

### 2d. `readTopicFromPath`

```go
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
```

- Unexported, three parameters plus `ctx`, pointer return. It mirrors `goalStorage.readGoalFromPath` in shape and delegates to the shared helper, which is the single chokepoint carrying the bare-wikilink quoting pass. Do NOT read the file with `os.ReadFile` here — bypassing `parseToFrontmatterMap` reintroduces the `[[X]]` → nested-list corruption.
- Return the helper's error unchanged. Do NOT wrap it a second time; `readEntityComponentsFromPath` already wraps "read file %s" and "parse frontmatter", and the extra layer adds noise without adding information.

### 2e. `WriteTopic`

```go
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
```

Non-negotiable properties:

- The symlink refusal, the `serializeMapAsFrontmatter(ctx, topic.RawMap(), string(topic.Content))` call, the `errors.Wrap(ctx, err, "serialize frontmatter")` message, the `0600` file mode and the `errors.Wrapf(ctx, err, "write file %s", topic.FilePath)` message are all copied from `goalStorage.WriteGoal` verbatim. Do not "improve" any of them.
- `os.MkdirAll(filepath.Dir(topic.FilePath), 0750)` is the one addition over `WriteGoal`. It is required by the spec's Failure Modes row "Topics directory absent from the vault … a write creates the directory path it needs" and by the row for a configured directory that does not exist. `0750` is the gosec-compliant directory mode per the security-linting guide; do NOT use `0777` or `0755`.
- Do NOT add a temp-file-plus-rename atomic write. The spec's Failure Modes rows for "two writers mutate the same page at once" and "crash mid-write" describe last-write-wins and prior-or-fully-written behaviour, which is exactly what the goal path already does; adding a mechanism the goal family does not have would be a divergence the spec does not ask for. The round-trip test in § 3 is the evidence for those rows.
- Do NOT add logging, a file lock, a backup, or a `fsync`.
- Do NOT write to `DefaultConfig().TopicsDir` on any path. The only directory this function touches is the parent of the `File.Path` it was handed, which the caller derived from the configured topics directory.

## 3. `pkg/storage/topic_test.go` — the storage specs

External test package `package storage_test`, standard BSD license header, imports: `context`, `errors` (stdlib), `os`, `path/filepath`, Ginkgo v2 + Gomega dot-imports, `github.com/bborbe/vault-cli/pkg/config`, `github.com/bborbe/vault-cli/pkg/domain`, `github.com/bborbe/vault-cli/pkg/storage`. Copy `goal_test.go`'s fixture style: a `os.MkdirTemp` vault, a `BeforeEach` that creates it and the topics directory, an `AfterEach` that removes it.

The fixture declares two package-level variables, both set in `BeforeEach` and used throughout § 3b and § 3c:

```go
var (
	vaultDir string
	topicsDir string
)
```

- `vaultDir` is the `os.MkdirTemp` result — the vault root the storage calls are handed as `vaultPath`.
- `topicsDir = filepath.Join(vaultDir, "23 Topics")` — the **default** topics directory, because § 3a's `store` is `storage.NewStorage(nil)`, which resolves through `DefaultConfig()` to the literal `23 Topics`. This mirrors `goal_test.go`, whose `BeforeEach` sets `goalsDir = filepath.Join(vaultDir, "Goals")` to match `DefaultConfig()`'s `GoalsDir`. Create it in `BeforeEach` with `os.MkdirAll(topicsDir, 0750)`. The specs in § 3d and § 3e that need a *configured* directory build their own `storage.NewStorage(&storage.Config{TopicsDir: "Custom Topics"})` and a matching path; they do not use `topicsDir`.

Use a helper for the page content:

```go
topicContent := func() string {
	return `---
status: in_progress
phase: planning
tags:
  - attention
unknown_key: kept
---
# Attention Routing

Body text.
`
}
```

The specs below are mandatory. Name them exactly as written; the names listed in `<verification>` are grep targets there and the rest are mandatory but asserted by inspection.

### 3a. `FindTopicByName` — the happy paths

```
Describe("FindTopicByName", ...)
  It("finds a topic by bare name")
      topic, err := store.FindTopicByName(ctx, vaultDir, "Attention Routing")
      Expect(err).To(BeNil())
      Expect(topic).NotTo(BeNil())
      Expect(topic.Name).To(Equal("Attention Routing"))
      Expect(topic.GetField("phase")).To(Equal("planning"))

  It("finds a topic by bracket-wrapped name")
      topic, err := store.FindTopicByName(ctx, vaultDir, "[[Attention Routing]]")
      Expect(err).To(BeNil())
      Expect(topic).NotTo(BeNil())
      Expect(topic.Name).To(Equal("Attention Routing"))

  It("returns an error for a nonexistent name")
      _, err := store.FindTopicByName(ctx, vaultDir, "Nonexistent")
      Expect(err).NotTo(BeNil())
      Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())

  It("returns an error whose message names the topic and the searched directory")
      _, err := store.FindTopicByName(ctx, vaultDir, "Nonexistent")
      Expect(err.Error()).To(ContainSubstring("Nonexistent"))
      Expect(err.Error()).To(ContainSubstring(filepath.Join(vaultDir, "23 Topics")))
```

`store` is `storage.NewStorage(nil)` (so `DefaultConfig()`, hence `23 Topics`). The fourth spec is the storage-level half of the spec's Failure Modes row "Topic page named by `show` does not exist … stderr names the topic".

### 3b. `FindTopicByName` — the traversal refusal (Acceptance Criterion 12)

```
Describe("FindTopicByName traversal refusal", ...)
  BeforeEach: write a file at filepath.Join(vaultDir, "outside-file.md") with the
  content "---\nstatus: in_progress\n---\noutside\n" and capture its bytes.

  It("refuses a parent-directory name and leaves the outside file byte-identical")
      before, err := os.ReadFile(filepath.Join(vaultDir, "outside-file.md"))
      Expect(err).To(BeNil())

      _, err = store.FindTopicByName(ctx, vaultDir, "../outside-file")
      Expect(err).NotTo(BeNil())
      Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())

      after, err := os.ReadFile(filepath.Join(vaultDir, "outside-file.md"))
      Expect(err).To(BeNil())
      Expect(after).To(Equal(before))

  It("refuses a name that climbs out of the vault entirely")
      _, err := store.FindTopicByName(ctx, vaultDir, "../../outside-file")
      Expect(err).NotTo(BeNil())
      Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())

  It("still resolves a name nested inside the topics directory")
      // write <vaultDir>/23 Topics/sub/nested.md, then
      topic, err := store.FindTopicByName(ctx, vaultDir, "sub/nested")
      Expect(err).To(BeNil())
      Expect(topic.Name).To(Equal("sub/nested"))
      Expect(topic.FilePath).To(Equal(filepath.Join(topicsDir, "sub", "nested.md")))
```

The first spec is the point of the whole section: without the refusal, `filepath.Join(vaultDir, "23 Topics", "../outside-file.md")` cleans to `<vaultDir>/outside-file.md`, which exists, and the read would succeed. The third spec is the negative control that stops the check from degenerating into "reject every name containing a separator".

### 3c. `WriteTopic`

```
Describe("WriteTopic", ...)
  It("writes a topic and reads it back with every key preserved")
      topic := domain.NewTopic(
          map[string]any{"status": "in_progress", "phase": "planning", "unknown_key": "kept"},
          domain.FileMetadata{Name: "Attention Routing", FilePath: filepath.Join(topicsDir, "Attention Routing.md")},
          domain.Content("---\nstatus: in_progress\n---\n# Attention Routing\n"),
      )
      Expect(store.WriteTopic(ctx, topic)).To(Succeed())

      reread, err := store.FindTopicByName(ctx, vaultDir, "Attention Routing")
      Expect(err).To(BeNil())
      Expect(reread.GetField("phase")).To(Equal("planning"))
      Expect(reread.GetField("unknown_key")).To(Equal("kept"))
      Expect(reread.GetField("status")).To(Equal("in_progress"))

  It("creates a missing parent directory")
      nestedDir := filepath.Join(vaultDir, "Custom Topics", "deeper")
      topic := domain.NewTopic(
          map[string]any{"status": "in_progress"},
          domain.FileMetadata{Name: "Fresh", FilePath: filepath.Join(nestedDir, "Fresh.md")},
          domain.Content("---\nstatus: in_progress\n---\n"),
      )
      Expect(store.WriteTopic(ctx, topic)).To(Succeed())
      _, err := os.Stat(filepath.Join(nestedDir, "Fresh.md"))
      Expect(err).To(BeNil())

  It("writes the file with 0600 permissions")
      topic := domain.NewTopic(
          map[string]any{"status": "in_progress"},
          domain.FileMetadata{Name: "Mode", FilePath: filepath.Join(topicsDir, "Mode.md")},
          domain.Content("---\nstatus: in_progress\n---\n"),
      )
      Expect(store.WriteTopic(ctx, topic)).To(Succeed())
      info, err := os.Stat(filepath.Join(topicsDir, "Mode.md"))
      Expect(err).To(BeNil())
      Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))

  It("round-trips a bare wikilink value without destroying it")
      // write a topic whose frontmatter carries `related: [[A Topic]]`, read it back,
      // and assert the on-disk bytes still contain `[[A Topic]]` exactly once and
      // not the nested-list form `- - A Topic`
```

The first spec is the boundary test: the values the wrapper holds are what `yaml.Marshal` receives, so a key dropped on the write path is a key dropped on disk. The fourth is the boundary test for the shared `quoteBareWikilinks` pass — it fails if `readTopicFromPath` ever bypasses `readEntityComponentsFromPath`.

### 3d. The configuration plumbing

```
Describe("topic storage configuration", ...)
  It("carries the vault's configured topics directory")
      cfg := storage.NewConfigFromVault(&config.Vault{TopicsDir: "Custom Topics"})
      Expect(cfg.TopicsDir).To(Equal("Custom Topics"))

  It("falls back to the 23 Topics default when the vault declares none")
      cfg := storage.NewConfigFromVault(&config.Vault{})
      Expect(cfg.TopicsDir).To(Equal("23 Topics"))

  It("carries 23 Topics in DefaultConfig")
      Expect(storage.DefaultConfig().TopicsDir).To(Equal("23 Topics"))

  It("writes under the configured directory and not under the default")
      customStore := storage.NewStorage(&storage.Config{TopicsDir: "Custom Topics"})
      topic := domain.NewTopic(
          map[string]any{"status": "in_progress"},
          domain.FileMetadata{Name: "Scoped", FilePath: filepath.Join(vaultDir, "Custom Topics", "Scoped.md")},
          domain.Content("---\nstatus: in_progress\n---\n"),
      )
      Expect(customStore.WriteTopic(ctx, topic)).To(Succeed())
      _, err := os.Stat(filepath.Join(vaultDir, "Custom Topics", "Scoped.md"))
      Expect(err).To(BeNil())
      _, err = os.Stat(filepath.Join(vaultDir, "23 Topics", "Scoped.md"))
      Expect(err).NotTo(BeNil())

  It("finds a page under the configured directory and not under the default")
      // write the same file under Custom Topics, then
      // customStore.FindTopicByName(ctx, vaultDir, "Scoped") succeeds while
      // storage.NewStorage(nil).FindTopicByName(ctx, vaultDir, "Scoped") fails
```

The fourth and fifth specs are Acceptance Criterion 11's "a topic written through the command family lands under that directory … the written file appears under the configured directory and not under the default", at the storage layer. The fifth's second assertion is the anti-fallback guard: a silent fallback to `23 Topics` would make it pass.

### 3e. The absent directory (Acceptance Criterion 11's second half)

```
Describe("absent directories", ...)
  It("returns an ErrNotFound-class error when the configured directory is absent")
      store := storage.NewStorage(&storage.Config{TopicsDir: "Custom Topics"})
      _, err := store.FindTopicByName(ctx, vaultDir, "Anything")
      Expect(err).NotTo(BeNil())
      Expect(errors.Is(err, storage.ErrNotFound)).To(BeTrue())
      // and the searched (configured) directory was NOT created by the miss:
      _, statErr := os.Stat(filepath.Join(vaultDir, "Custom Topics"))
      Expect(os.IsNotExist(statErr)).To(BeTrue())

  It("lists an absent configured directory as an empty result with no error")
      pageStore := storage.NewPageStorage(&storage.Config{TopicsDir: "Custom Topics"})
      pages, err := pageStore.ListPages(ctx, vaultDir, "Custom Topics")
      Expect(err).To(BeNil())
      Expect(pages).To(BeEmpty())
```

The first spec's second half is the Failure Modes row "Configuration names a topics directory that does not exist … the default topics directory is not created". It asserts on the *configured* directory (`Custom Topics`) rather than on the literal `23 Topics`, because the `BeforeEach` fixture already creates the default topics directory — it mirrors `goal_test.go`, whose `BeforeEach` runs `os.MkdirAll(goalsDir, 0755)` — so the configured directory is the one this spec can prove untouched. The second spec is the "list returns an empty result set and exits 0" half — `PageStorage.ListPages` already has the `fs.ErrNotExist` → `nil, nil` branch, and this spec pins that the topics directory reaches it. Do NOT change `pkg/storage/page.go` to make this pass; it already passes.

## 4. Failure modes and security — the mapping

Map the spec's Failure Modes table onto this change and state the mapping in your completion report:

- **Topics directory absent from the vault.** `FindTopicByName` → `findFileByName` → `os.Stat(dir)` is `IsNotExist` → `ErrNotFound`. `ListPages` on an absent directory returns `nil, nil`. A write creates the directory (§ 2e). Covered by § 3e and § 3c's second spec.
- **Topic page named by `show` does not exist.** Non-zero exit comes from the CLI in prompt 3; the error message naming the topic and the directory comes from § 2c's two wraps. Covered by § 3a's third and fourth specs.
- **Configuration names a topics directory that does not exist.** Used verbatim with no fallback — `filepath.Join(vaultPath, t.config.TopicsDir)` with no existence check and no default substitution. Covered by § 3e.
- **Topic page carries a non-canonical `phase` value.** `readTopicFromPath` → `parseToFrontmatterMap` → `yaml.Unmarshal` never validates a value; the wrapper's raw read is prompt 1's. Covered by § 3a's first spec's `phase` assertion against `planning`, and by prompt 1's non-canonical-value spec.
- **Topic page carries a duplicate frontmatter key.** The page still reads (the duplicate is a lint concern, not a read concern). Covered by prompt 3's lint spec; name it as out of scope for this prompt.
- **`defer` with a relative date or a non-UTC timezone.** The stored value comes from `libtime.DateOrDateTime` via prompt 1's `SetDeferDate`; the write path here serialises the map unchanged. Out of scope for this prompt — name it as owned by prompt 3.
- **`defer` with a past date.** Refused in the ops layer (prompt 3) before `WriteTopic` is reached. Out of scope here.
- **Two writers mutate the same topic page at once / crash mid-write.** `WriteTopic` uses the same single `os.WriteFile` as `WriteGoal`; last write wins and no partial frontmatter block is produced. Covered by § 3c's first and fourth specs (write, then read back and parse).

Security properties to preserve, not to build: the topic name is user-supplied and resolves a file path, and § 2b/§ 2c are the containment. Do NOT add a regex allowlist, a character-class restriction, a shell-quoting pass, or a `filepath.Abs` comparison — the cleaned-prefix check is the mechanism. Topic file content is parsed as YAML frontmatter and markdown; malformed YAML already fails through `parseToFrontmatterMap` with an error naming the file and leaves the file untouched — do not add a panic-recovery wrapper. No network, no subprocess and no credential access is introduced by this prompt.

## 5. Self-check before finishing

- Re-read the changed hunks and confirm: the `Config` field sits between `DailyDir` and `Excludes`; `NewConfigFromVault` reads `vault.GetTopicsDir()` with no fallback; `DefaultConfig` carries `23 Topics`; `TopicStorage` has exactly two methods; `Storage` embeds it with no `ReadTopic` entry; `markdownStorage` and `NewStorage` both gained `topicStorage` after `visionStorage`; `pkg/storage/base.go`, `pkg/storage/page.go` and `pkg/config/config.go` are untouched.
- Walk spec 051's Acceptance Criteria 11 and 12 and state in your completion report which requirement and which spec satisfies each, plus which evidence covers each Failure Modes row listed in § 4.
- Walk `docs/dod.md`: every exported type and function has a doc comment; no `fmt.Print*` and no `os.Stdout` in `pkg/storage/`; every error goes through `github.com/bborbe/errors` with a `ctx`; no `context.Background()`; no new `go.mod` dependency; tests use Ginkgo v2 / Gomega in an external test package.
- Confirm each check in `<verification>` passes by **running** it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 051 — non-goals.** Do NOT add any binary leaf. Do NOT write, backfill, default or normalize a `phase` field onto any topic page. Do NOT modify, extend or reuse the goal phase type, the task phase type, or their normalizers for the topic entity. Do NOT change the goal command family's output, flags, exit codes, or the goal-side frontmatter allowlists. Do NOT extend the hardcoded entity-kind lists elsewhere in the binary — the watch command's accepted type list, its watch-directory set, and the resolve/type set each name task, goal, theme and objective by hand, and "mirror the goal family" is not license to add a topic kind to any of them. Do NOT add a `docs/topic-writing.md`. Do NOT migrate, rewrite or reformat existing topic pages.
- **Copied from spec 051 — constraints.** The topics directory default is the vault's existing topics default; a configuration override wins over the default and is used verbatim. Topic pages with no `phase` line parse and show without error, and no file is backfilled. Unknown frontmatter keys on a topic page survive a read-write cycle. The layered recipe in `docs/development-patterns.md` § "Adding a New Command" (Domain → Storage → Ops → CLI) governs the shape of this work.
- **Frozen names.** Type `topicStorage`; interface `TopicStorage`; methods `WriteTopic`, `FindTopicByName`, `readTopicFromPath`, `isTopicNameWithinDir`; constructor `NewTopicStorage`; config field `TopicsDir`; default literal `23 Topics`; fake name `TopicStorage` at `mocks/topic-storage.go`. All are grep targets in the acceptance criteria.
- **Depends on prompt 1.** `domain.Topic`, `domain.NewTopic` and `domain.TopicFrontmatter` must already exist — this prompt will not compile without them. If `domain.NewTopic` is missing, stop and report `"status":"failed"` naming prompt 1 as the missing dependency; do NOT define a topic entity here to make it compile.
- **Do NOT modify `pkg/storage/base.go`.** The traversal refusal lives in the topic path (§ 2b/§ 2c), not in the shared `findFileByName`. Moving it into the shared helper would change the goal and task families' behaviour on a traversal name, which the spec forbids. This is the one intentional divergence from a byte-for-byte goal mirror.
- **Do NOT modify `pkg/config/config.go`.** `Vault.TopicsDir` and `Vault.GetTopicsDir()` already exist and already default to `23 Topics`.
- **Do NOT modify `pkg/storage/page.go`.** Its `fs.ErrNotExist` → `nil, nil` branch is the absent-directory behaviour Acceptance Criterion 11 relies on, and it already works.
- **No `ReadTopic`, no `ListTopics`, no `domain.TopicID`.** All three would be dead code in this spec.
- **Permissions.** Files `0600`, directories `0750` (gosec). No `#nosec` comment is needed on the `os.ReadFile`/`os.WriteFile` calls that mirror the existing ones, but if you add one it must carry a reason.
- **Tests.** Ginkgo v2 + Gomega, external test package `storage_test`, no stdlib `t.Run` table tests. Every named `It` must contain a real assertion — a spec that is only named does not satisfy the acceptance criteria. Every Go file keeps its BSD license header.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass. Acceptance Criterion 12's `git status` half is operator-side; the byte-compare in § 3b is the container-executable evidence.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
- Do NOT run `go mod vendor`. No new dependency; `go.mod` and `go.sum` are untouched.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**The storage test run, with its frozen spec names.** Capture the run's output, check its exit status separately from the name greps (a failing run still prints the names), and never pipe a test command:

```
go test ./pkg/storage/... -v -ginkgo.v -count=1 > /tmp/topic-storage.log 2>&1; test "$?" = "0"
grep -F -q -- 'finds a topic by bare name' /tmp/topic-storage.log
grep -F -q -- 'finds a topic by bracket-wrapped name' /tmp/topic-storage.log
grep -F -q -- 'returns an error whose message names the topic and the searched directory' /tmp/topic-storage.log
grep -F -q -- 'refuses a parent-directory name and leaves the outside file byte-identical' /tmp/topic-storage.log
grep -F -q -- 'refuses a name that climbs out of the vault entirely' /tmp/topic-storage.log
grep -F -q -- 'still resolves a name nested inside the topics directory' /tmp/topic-storage.log
grep -F -q -- 'writes a topic and reads it back with every key preserved' /tmp/topic-storage.log
grep -F -q -- 'creates a missing parent directory' /tmp/topic-storage.log
grep -F -q -- 'writes the file with 0600 permissions' /tmp/topic-storage.log
grep -F -q -- 'round-trips a bare wikilink value without destroying it' /tmp/topic-storage.log
grep -F -q -- 'writes under the configured directory and not under the default' /tmp/topic-storage.log
grep -F -q -- 'finds a page under the configured directory and not under the default' /tmp/topic-storage.log
grep -F -q -- 'lists an absent configured directory as an empty result with no error' /tmp/topic-storage.log
```

**The new file exists and the storage implementation carries the frozen names:**

```
test -f pkg/storage/topic.go
test -f pkg/storage/topic_test.go
test "$(grep -c 'type topicStorage struct' pkg/storage/topic.go)" = "1"
test "$(grep -c 'func (t \*topicStorage) FindTopicByName(' pkg/storage/topic.go)" = "1"
test "$(grep -c 'func (t \*topicStorage) WriteTopic(' pkg/storage/topic.go)" = "1"
test "$(grep -c 'func (t \*topicStorage) readTopicFromPath(' pkg/storage/topic.go)" = "1"
test "$(grep -c 'func isTopicNameWithinDir(dir string, name string) bool' pkg/storage/topic.go)" = "1"
```

**The traversal refusal is an `ErrNotFound`-class error and the directory is named:**

```
test "$(sed -n '/func (t \*topicStorage) FindTopicByName(/,/^}/p' pkg/storage/topic.go | grep -c 'ErrNotFound')" = "1"
test "$(sed -n '/func (t \*topicStorage) FindTopicByName(/,/^}/p' pkg/storage/topic.go | grep -c 'does not resolve inside')" = "1"
test "$(sed -n '/func (t \*topicStorage) FindTopicByName(/,/^}/p' pkg/storage/topic.go | grep -c 'find topic file in %s')" = "1"
test "$(grep -c 'errors.Errorf(ctx, ErrNotFound' pkg/storage/topic.go)" = "0"
```

The first three use the `sed` range form, not `grep -A<n>`: the range runs from the `FindTopicByName` signature to the function's own closing brace, so the assertion cannot false-fail when a doc comment or an added line shifts the body. A fixed `-A12` window reaches only three lines past `ErrNotFound` on the mandated body and misses `find topic file in %s` entirely if the refusal block grows by one line — the same brittleness class the `-ginkgo.v` flag avoids for the spec-name greps. The last line is the trap guard: `errors.Errorf` would lose the `ErrNotFound` chain and break the multi-vault dispatcher's `errors.Is` test.

**The shared helper was NOT changed — the goal and task families keep their behaviour:**

```
test "$(grep -c 'isTopicNameWithinDir' pkg/storage/base.go)" = "0"
test "$(grep -c 'topic' pkg/storage/base.go)" = "0"
test "$(grep -c 'topic' pkg/storage/page.go)" = "0"
test "$(grep -c 'func (v \*Vault) GetTopicsDir() string' pkg/config/config.go)" = "1"
```

`pkg/storage/base.go` and `pkg/storage/page.go` genuinely carry no `topic` text, so those two assertions are absence checks. `pkg/config/config.go` is different: `Vault.TopicsDir`, `GetTopicsDir` and its doc comment already exist (spec 048) and this prompt forbids modifying that file, so the only honest check is that the accessor is still present exactly once — do NOT rewrite it as `grep -c 'topic' … = "0"`, which the pre-existing text can never satisfy.

**`pkg/storage/storage.go` carries the config plumbing and the interface:**

```
test "$(grep -c 'TopicsDir     string' pkg/storage/storage.go)" = "1"
test "$(grep -c 'TopicsDir:     vault.GetTopicsDir(),' pkg/storage/storage.go)" = "1"
test "$(grep -c 'TopicsDir:     "23 Topics",' pkg/storage/storage.go)" = "1"
test "$(grep -c 'mocks/topic-storage.go --fake-name TopicStorage' pkg/storage/storage.go)" = "1"
test "$(grep -c 'func NewTopicStorage(storageConfig \*Config) TopicStorage' pkg/storage/storage.go)" = "1"
test "$(grep -c 'ReadTopic' pkg/storage/storage.go)" = "0"
test "$(grep -c 'ListTopics' pkg/storage/storage.go)" = "0"
```

**`Config.TopicsDir` sits between `DailyDir` and `Excludes`, and `topicStorage` follows `visionStorage` in both places:**

```
test "$(grep -n 'DailyDir      string' pkg/storage/storage.go | head -1 | cut -d: -f1)" -lt "$(grep -n 'TopicsDir     string' pkg/storage/storage.go | head -1 | cut -d: -f1)"
test "$(grep -n 'TopicsDir     string' pkg/storage/storage.go | head -1 | cut -d: -f1)" -lt "$(grep -n 'Excludes      \[\]string' pkg/storage/storage.go | head -1 | cut -d: -f1)"
test "$(grep -n 'visionStorage:' pkg/storage/storage.go | head -1 | cut -d: -f1)" -lt "$(grep -n 'topicStorage:' pkg/storage/storage.go | head -1 | cut -d: -f1)"
test "$(grep -n '\*visionStorage' pkg/storage/storage.go | head -1 | cut -d: -f1)" -lt "$(grep -n '\*topicStorage' pkg/storage/storage.go | head -1 | cut -d: -f1)"
```

**Permissions and formatting:**

```
test "$(grep -c 'os.MkdirAll(dir, 0750)' pkg/storage/topic.go)" = "1"
test "$(grep -c 'os.WriteFile(topic.FilePath, \[\]byte(content), 0600)' pkg/storage/topic.go)" = "1"
test -z "$(gofmt -e -l pkg/storage/topic.go pkg/storage/topic_test.go pkg/storage/storage.go)"
go test ./pkg/... -count=1 > /tmp/topic-storage-pkg.log 2>&1; test "$?" = "0"
```

Finally, walk spec 051's Acceptance Criteria 11 and 12 against the change and state in your completion report which requirement and which spec satisfies each one, and which evidence covers each row of the spec's Failure Modes table listed in `<requirements>` § 4.
</verification>
