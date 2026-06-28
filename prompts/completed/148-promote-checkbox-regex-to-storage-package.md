---
status: completed
summary: Promoted checkboxRegex to exported CheckboxRegex/CheckboxCompleteRegex/CheckboxUncompleteRegex in pkg/storage, replaced seven inline regexp.MustCompile call sites across pkg/ops/{update,complete,defer,workon}.go, dropped unused regexp imports from three of those four files, added refactor(checkbox) changelog bullet, and left lint.go untouched
execution_id: vault-cli-checkbox-shared-exec-148-promote-checkbox-regex-to-storage-package
dark-factory-version: v0.187.11
created: "2026-06-28T00:00:00Z"
queued: "2026-06-28T12:46:23Z"
started: "2026-06-28T12:46:25Z"
completed: "2026-06-28T12:50:57Z"
---
<summary>
- The checkbox-line regex pattern, currently inline-compiled in 7 places, is centralized in the `storage` package as exported package-level vars.
- Two additional shared patterns (the "force-complete" and "force-uncomplete" rewriters) are also promoted to the same place.
- Five files in `pkg/ops/` lose their local `regexp.MustCompile` calls and reference the shared vars instead; their `regexp` imports are dropped where no other usage remains.
- The lint package's intentionally-different `[ xX]` state class regexes are left alone — they are not duplicates of the parser regex.
- No regex literal content changes; this is a pure DRY refactor. No new behavior, no new tests.
- A single `refactor(checkbox):` bullet is added under `## Unreleased` in `CHANGELOG.md` describing the DRY consolidation.
- After edits, `make precommit` must exit 0 and `go test ./pkg/storage/... ./pkg/ops/...` must pass — existing PR #36 coverage already exercises both `-` and `*` marker forms.
</summary>

<objective>
Eliminate seven inline copies of the same checkbox parser regex across `pkg/storage` and `pkg/ops` by promoting the existing package-private `checkboxRegex` in `pkg/storage/base.go` to an exported var, adding two sibling exported vars for the rewriter patterns, and replacing the seven inline `regexp.MustCompile` call sites in `pkg/ops/` with references to those vars. No behavior change.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read /home/node/.claude/plugins/marketplaces/coding/docs/go-package-layout-guide.md — confirms the "flat default, no new sub-package" decision; the shared regex lives in `pkg/storage` because that is where the canonical copy already exists.
Read /home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md for the changelog bullet format.

Read these files fully before editing:
- `pkg/storage/base.go` — lines 23-26 hold the existing `var (...)` block with `frontmatterRegex` and the package-private `checkboxRegex`. Line 130's `parseCheckboxes` method consumes that var via the package-internal name; after promotion the same in-package code can keep using the new exported name `CheckboxRegex` (an exported name is still visible inside its own package).
- `pkg/ops/update.go` — line 151 inline-compiles the checkbox regex inside `parseCheckboxes`.
- `pkg/ops/complete.go` — line 272 (inside `countCheckboxStates`), line 357 (inside `updateDailyNote`), and lines 364-365 + 367-368 (the two rewriter regexes inside `updateDailyNote`).
- `pkg/ops/defer.go` — line 87 has a SEPARATE `regexp.MatchString(\`^\+\d+d$\`, ...)` for the `+Nd` shorthand — leave it alone; line 206 has the checkbox regex.
- `pkg/ops/workon.go` — line 244 has the checkbox regex.
- `pkg/ops/lint.go` — lines 460 and 771 use a DELIBERATELY DIFFERENT regex with character class `[ xX]` (uppercase-X state included). These are NOT duplicates of the parser regex and MUST NOT be touched in this prompt.

The four ops files (`update.go`, `complete.go`, `defer.go`, `workon.go`) already import `github.com/bborbe/vault-cli/pkg/storage` for unrelated reasons, so no new import lines are needed — only the `"regexp"` import will need to be removed from files where no other `regexp.` usage remains after the swap.

`regexp.` usage AFTER all swaps in this prompt (verified before writing):
- `pkg/ops/update.go`: zero remaining → REMOVE `"regexp"` import.
- `pkg/ops/complete.go`: zero remaining → REMOVE `"regexp"` import.
- `pkg/ops/defer.go`: ONE remaining at line 87 (`regexp.MatchString`) → KEEP `"regexp"` import.
- `pkg/ops/workon.go`: zero remaining → REMOVE `"regexp"` import.

Recent CHANGELOG style (top of file) — mirror the bullet format `<area>(<scope>): <verb-led sentence>`:
```
- fix(checkbox): accept `*` as Markdown list marker alongside `-` in checkbox regex across storage and ops packages — ...
```
</context>

<requirements>

## 1. Promote and expand the storage-package regex block

In `pkg/storage/base.go`, replace the existing `var (...)` block at lines 23-26:

OLD:
```go
var (
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)
	checkboxRegex    = regexp.MustCompile(`^(\s*)[-*] \[([ x/])\] (.+)$`)
)
```

NEW:
```go
var (
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)

	// CheckboxRegex matches a Markdown checkbox line with either `-` or `*` as
	// the list marker. Capture groups: 1=leading whitespace, 2=state (` `, `x`,
	// or `/`), 3=task text. Shared across storage and ops packages to keep the
	// parser shape in one place.
	CheckboxRegex = regexp.MustCompile(`^(\s*)[-*] \[([ x/])\] (.+)$`)

	// CheckboxCompleteRegex matches an unchecked or in-progress checkbox marker
	// (` ` or `/`) and is used by rewriters that force a line to checked.
	// Capture groups: 1=list marker (`-` or `*`), 2=state (` ` or `/`).
	CheckboxCompleteRegex = regexp.MustCompile(`([-*]) \[([ /])\]`)

	// CheckboxUncompleteRegex matches a checked checkbox marker and is used by
	// rewriters that force a line to unchecked. Capture group 1=list marker.
	CheckboxUncompleteRegex = regexp.MustCompile(`([-*]) \[x\]`)
)
```

Then in the same file at the `parseCheckboxes` method (currently around line 130), update the reference from `checkboxRegex` to `CheckboxRegex` — the exported name is still visible from within its own package, so this is a simple rename of the call-site identifier on line 135.

## 2. Swap the seven inline call sites in `pkg/ops/`

For each replacement below, the surrounding code (loop body, switch, etc.) MUST remain untouched — only the `regexp.MustCompile(...)` expression and the local variable line it lives on change.

### 2a. `pkg/ops/update.go` line 151 (inside `parseCheckboxes`)

DELETE line 151 entirely (the local `checkboxRegex := regexp.MustCompile(\`^(\s*)[-*] \[([ x/])\] (.+)$\`)`). Replace the usage on line 154 with `storage.CheckboxRegex`:

OLD:
```go
checkboxRegex := regexp.MustCompile(`^(\s*)[-*] \[([ x/])\] (.+)$`)

for i, line := range lines {
	if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 {
```

NEW:
```go
for i, line := range lines {
	if matches := storage.CheckboxRegex.FindStringSubmatch(line); len(matches) == 4 {
```

### 2b. `pkg/ops/complete.go` line 272 (inside `countCheckboxStates`)

Same shape: delete the local `checkboxRegex := ...` line, swap the `.FindStringSubmatch` call to `storage.CheckboxRegex.FindStringSubmatch`.

### 2c. `pkg/ops/complete.go` line 357 (inside `updateDailyNote`)

Same shape: delete the local `checkboxRegex := ...` line, swap the `.FindStringSubmatch` call to `storage.CheckboxRegex.FindStringSubmatch` (this is the very next line below, currently using `checkboxRegex.FindStringSubmatch`).

### 2d. `pkg/ops/complete.go` lines 364-365 (force-complete rewriter)

OLD:
```go
lines[i] = regexp.MustCompile(`([-*]) \[([ /])\]`).
	ReplaceAllString(line, "$1 [x]")
```

NEW:
```go
lines[i] = storage.CheckboxCompleteRegex.ReplaceAllString(line, "$1 [x]")
```

### 2e. `pkg/ops/complete.go` lines 367-368 (force-uncomplete rewriter)

OLD:
```go
lines[i] = regexp.MustCompile(`([-*]) \[x\]`).
	ReplaceAllString(line, "$1 [ ]")
```

NEW:
```go
lines[i] = storage.CheckboxUncompleteRegex.ReplaceAllString(line, "$1 [ ]")
```

### 2f. `pkg/ops/defer.go` line 206 (inside the daily-note filter)

Delete the local `checkboxRegex := ...` line, swap the `.FindStringSubmatch` call to `storage.CheckboxRegex.FindStringSubmatch`. KEEP the `regexp` import — line 87 (`regexp.MatchString` for `^\+\d+d$`) is unrelated and still needs it.

### 2g. `pkg/ops/workon.go` line 244 (inside `findAndUpdateTaskCheckbox`)

Delete the local `checkboxRegex := ...` line, swap the `.FindStringSubmatch` call to `storage.CheckboxRegex.FindStringSubmatch`.

## 3. Drop now-unused `regexp` imports

After the swaps above, three files no longer import-use `regexp`:

- `pkg/ops/update.go` — remove the `"regexp"` line from the import block.
- `pkg/ops/complete.go` — remove the `"regexp"` line from the import block.
- `pkg/ops/workon.go` — remove the `"regexp"` line from the import block.

Do NOT touch `pkg/ops/defer.go`'s import block — it still uses `regexp.MatchString` at line 87.

If `goimports`/`gofmt` reorganizes the import groups during precommit, accept whatever ordering it produces; do not hand-tune.

## 4. DO NOT touch `pkg/ops/lint.go`

Lines 460 and 771 in `pkg/ops/lint.go` use a deliberately different state class (`[ xX]` — uppercase X allowed) because the linter intentionally accepts shapes the parser rejects, then warns about them. Leave both lines exactly as they are. `git diff pkg/ops/lint.go` MUST report zero changes at the end of this prompt.

## 5. Changelog entry

In `CHANGELOG.md`, the `## Unreleased` section already exists (verified — contains one `fix(checkbox):` bullet from PR #36). Append a SECOND bullet directly below the existing one, inside the same `## Unreleased` section:

```
- refactor(checkbox): DRY out duplicated checkbox parser regex — promote `checkboxRegex` in `pkg/storage` to exported `CheckboxRegex`, add sibling `CheckboxCompleteRegex` and `CheckboxUncompleteRegex` for the force-complete / force-uncomplete rewriters, and replace seven inline `regexp.MustCompile` call sites across `pkg/ops/{update,complete,defer,workon}.go` with references to the shared vars. No behavior change; lint.go's intentionally-broader `[ xX]` regex shape is left untouched.
```

Use a real em-dash (`—`, U+2014), matching the style of the existing entry above it. The `refactor(checkbox):` prefix is required so dark-factory's prefix-based version bump classifies the change correctly (no version bump on `refactor:`).

## 6. Verification grep gates

Run these greps from the repo root after edits — each MUST produce the stated output before reporting completion:

```bash
# AC1: three exported vars defined in storage/base.go
grep -nE '^\tCheckboxRegex\s+=' pkg/storage/base.go            # 1 match
grep -nE '^\tCheckboxCompleteRegex\s+=' pkg/storage/base.go    # 1 match
grep -nE '^\tCheckboxUncompleteRegex\s+=' pkg/storage/base.go  # 1 match

# AC2a: zero inline parser-shape regexes remain in pkg/ops
git grep -nE 'regexp\.MustCompile\(`\^\(\\s\*\)\[' pkg/ops/    # zero matches

# AC2b: four call sites now use storage.CheckboxRegex
git grep -nE 'storage\.CheckboxRegex\b' pkg/ops/               # 4 matches
git grep -nE 'storage\.CheckboxCompleteRegex' pkg/ops/         # 1 match
git grep -nE 'storage\.CheckboxUncompleteRegex' pkg/ops/       # 1 match

# AC3: lint.go untouched
git diff -- pkg/ops/lint.go                                    # empty output

# AC6: refactor bullet present under Unreleased
grep -nE '^- refactor\(checkbox\):' CHANGELOG.md               # 1 match
awk '/^## /{section=$0} /^- refactor\(checkbox\):/{print section}' CHANGELOG.md
# Expected: "## Unreleased"
```

If any gate fails, fix the underlying issue and re-run the gates before invoking `make precommit`.

## 7. Final acceptance gate

From the repo root, run:

```bash
make precommit
go test ./pkg/storage/... ./pkg/ops/...
```

Both MUST exit 0. If `make precommit` fails on `check-versions`, do NOT touch any version string — that target is unrelated to this refactor; report the divergence in the completion report under `## Improvements` and stop. For any other failure, fix the root cause (most likely a missed `regexp` import removal or a typo in the exported var name) and re-run.

</requirements>

<constraints>
- Do NOT modify any regex literal content. The patterns being promoted are byte-for-byte identical to the patterns being deleted; this is pure DRY.
- Do NOT touch `pkg/ops/lint.go` lines 460 or 771. Their `[ xX]` state class is intentional.
- Do NOT add new tests. Existing Ginkgo coverage from PR #36 already exercises both `-` and `*` marker forms across `pkg/storage` and `pkg/ops`.
- Do NOT introduce a new sub-package. Per `go-package-layout-guide.md`, flat default holds; the shared vars live in the existing `pkg/storage` package.
- Public API additions are limited to exactly three exported names: `CheckboxRegex`, `CheckboxCompleteRegex`, `CheckboxUncompleteRegex`. No method signatures change.
- Do NOT rename `frontmatterRegex` or any other existing var.
- Do NOT bump any version string. The `refactor(checkbox):` prefix instructs dark-factory not to bump.
- Do NOT touch `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`.
- Do NOT commit — dark-factory handles git.
- Run `make precommit` and `go test ./pkg/storage/... ./pkg/ops/...` ONCE at the end. Do not preemptively re-run individual targets.
</constraints>

<verification>
Run from the repo root:

```bash
# Acceptance criteria gates
grep -nE '^\tCheckboxRegex\s+=' pkg/storage/base.go            # AC1: 1 match
grep -nE '^\tCheckboxCompleteRegex\s+=' pkg/storage/base.go    # AC1: 1 match
grep -nE '^\tCheckboxUncompleteRegex\s+=' pkg/storage/base.go  # AC1: 1 match
git grep -nE 'regexp\.MustCompile\(`\^\(\\s\*\)\[' pkg/ops/    # AC2: 0 matches
git grep -nE 'storage\.CheckboxRegex\b' pkg/ops/               # AC2: 4 matches
git grep -nE 'storage\.CheckboxCompleteRegex' pkg/ops/         # AC2: 1 match
git grep -nE 'storage\.CheckboxUncompleteRegex' pkg/ops/       # AC2: 1 match
git diff -- pkg/ops/lint.go                                    # AC3: empty
grep -nE '^- refactor\(checkbox\):' CHANGELOG.md               # AC6: 1 match

# Build + test gates
make precommit                                                 # AC4: exit 0
go test ./pkg/storage/... ./pkg/ops/...                        # AC5: exit 0

# Sanity: regexp import dropped from the three files that no longer use it
grep -n '"regexp"' pkg/ops/update.go    # zero output
grep -n '"regexp"' pkg/ops/complete.go  # zero output
grep -n '"regexp"' pkg/ops/workon.go    # zero output
grep -n '"regexp"' pkg/ops/defer.go     # one match (line 87 still uses MatchString)
```
</verification>
