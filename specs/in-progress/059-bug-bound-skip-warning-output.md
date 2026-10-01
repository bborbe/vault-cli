---
status: prompted
approved: "2026-10-01T18:13:43Z"
generating: "2026-10-01T18:24:53Z"
prompted: "2026-10-01T18:36:29Z"
branch: dark-factory/bug-bound-skip-warning-output
---

## Summary

- `pkg/storage/page.go` skips a page it cannot parse (correct), but the single warning it emits per file carries an **unbounded** `error` field and, for one path, a full Go stack.
- Measured pre-fix (reproduced 2026-10-01 against this worktree): a file with 2,992 duplicate YAML keys produces **one** warning record whose `error` field is **≈332 MB** (316.9 MiB) — a single physical line; a frontmatter-less page produces a record of **≈2,457 bytes** embedding a ~30-line stack.
- A read-only listing command is taken out by a file it already correctly chooses to skip — the answer (225 task rows) is buried and unreachable.
- Fix has three parts: log `errors.Cause(err).Error()` (a plain string, no stack), truncate the embedded error to **≤200 bytes** UTF-8-safe keeping the **head**, and cap per-file warnings globally (after 10 skipped pages, one `skipping N unreadable pages` summary line).
- No change to the skip-and-continue behaviour itself; no change to the sibling `ListTasks` path (`pkg/storage/task.go:90`), which is out of scope.

## Problem

`ListPages` iterates a directory and, for each `.md` file that fails to parse, emits one `slog.Warn("skipping unreadable page", "file", filePath, "error", errors.Cause(err))`. The warn site is inside the **per-file** loop, so there is exactly one warning record per unreadable file — but two properties make that single record unbounded and, on one path, stack-bearing:

1. **The `error` field embeds the whole parse error.** A YAML document with N duplicate keys raises one error entry per *pair* — N(N−1)/2 entries (4,474,536 for the 2,992-key file) — and the returned error's `Error()` string contains all of them. `slog`'s text handler quotes the value and escapes its newlines, so the result is **one physical line of ~317 MB**, not thousands of lines. The line's size scales quadratically with the file's corruption.
2. **One path renders a Go stack.** `pkg/cli/cli.go:157` installs `slog.NewTextHandler`, which formats an `Any` attribute with `%+v`. `errors.Cause(err)` returns the *deepest* cause (it delegates to `github.com/pkg/errors.Cause`, which unwraps past `*dataError` because `dataError` implements `Cause()`). For a frontmatter-less page that deepest cause is a `*pkg/errors.fundamental`, whose `%+v` renders the full frame list; the human-readable reason (`no frontmatter found`) is its first four words.

The consequence is that a command whose contract is "list rows and exit 0" becomes unusable when one file is malformed — even though the skip is the right call. The operator loses the listing, not just the noise. The per-directory cap (DB5) covers the sibling case where *many* files are malformed at once: the producer loop tracked in `Fix Agent-task-controller Task_identifier Accumulator Loop` writes one malformed file per incident, so a directory can accumulate several.

## Assumptions

- The `%+v` stack path is the **frontmatter-less** branch (`errors.Errorf` → `*fundamental`); the **duplicate-key** branch reaches a `*yaml.TypeError`, which carries no stack but a multi-line message. Verified against `bborbe/errors` v1.6.1 source and by reproduction (below).
- The `file` attribute is the **absolute** path (`filepath.Join(targetDir, entry.Name())`, `page.go:60`) and is not truncated by this change.
- `slog`'s text handler always emits a `time=` prefix (no `ReplaceAttr` in `cli.go:157`); it is part of every line's byte budget.

## Reproduction

Both symptoms reproduced 2026-10-01 against this worktree (`vault-cli` at `d4f9315` / v0.158.2), by calling `ListPages` with a captured `slog` text handler at `Warn`.

### Symptom A — unbounded single record (a 2,992-duplicate-key file)

1. A task file's frontmatter carries 2,992 duplicate `task_identifier` keys.
2. Run `vault-cli task list --status=in_progress --vault personal` (reaches the warn site via `pkg/ops/list.go:81` → `ListPages`).
3. Observed: **one** WARN record, whose `error` field embeds the whole YAML error — **332,280,825 bytes** (316.9 MiB) on a **single physical line**. The original incident's harness spilled it: `Output too large (316.9MB). Full output saved to: …`.
4. Warning shape (the `error` value contains ~4.47 M escaped `\n` sequences; the record itself is one line):

   ```
   level=WARN msg="skipping unreadable page" file="…/Fix x-crypto vulns in agent-task-controller (bump to v0.56.0).md" error="yaml: unmarshal errors:\n  line 2: mapping key \"task_identifier\" already defined at line 1\n  line 3: …"
   ```

5. After the file was repaired, the identical command returned 225 rows cleanly.

### Symptom B — stack trace in the record (frontmatter-less pages)

1. `20 Vision/` holds six pages with no frontmatter block.
2. Run `vault-cli vision list --vault private-personal`.
3. Observed: each of the six records embeds a ~30-line Go stack — measured **≈2,457 bytes** on this worktree via the real CLI (≈1,443 B in-process, with fewer frames), e.g. `error="no frontmatter found\ngithub.com/bborbe/errors.Errorf\n\t…/errors_new.go:18\ngithub.com/bborbe/vault-cli/pkg/storage.ParseFrontmatterMap\n\t…/base.go:78 … runtime.goexit\n\t…/asm_arm64.s:1039"`. Six files → screens of trace; the human-readable reason is the first four words.
4. Baseline measured 2026-10-01 on v0.156.0, reproduced twice: `vault-cli vision list --vault private-personal` writes **13,626 bytes** to stderr across **6** warn lines; the longest single line is **2,287 bytes**.

### Fixture generators

```bash
# Fixture A — one file with N duplicate task_identifier keys (N=2992 reproduces the flood)
{ echo '---'; for i in $(seq 1 2992); do echo 'task_identifier: 11111111-1111-4111-a111-111111111111'; done; echo '---'; echo '# Broken'; } > "$VAULT/25 Tasks/Broken.md"

# Fixture B — M frontmatter-less pages (M=12 exercises the ≥10 branch; M=15 forces the summary to name N)
for i in $(seq 1 12); do printf '# No Frontmatter %s\n' "$i" > "$VAULT/20 Vision/NoFm$i.md"; done

# Fixture C — duplicate-key file whose key names carry multibyte runes, so the 200-byte cut lands mid-rune.
# Calibration matters: 60 runes yields a 195-byte cause (under the 200-byte bound, so nothing is cut and
# there is no `…`); 100 runes yields a 275-byte cause with byte offset 197 a UTF-8 continuation byte (0xBC).
python3 - <<'PY' > "$VAULT/25 Tasks/MidRune.md"
print("---")
print("k" + "ü" * 100 + ": 1")
print("k" + "ü" * 100 + ": 2")
print("---")
print("# MidRune")
PY
```

### Mechanism (verified against source 2026-10-01)

- Warn site: `pkg/storage/page.go:66` — `slog.Warn("skipping unreadable page", "file", filePath, "error", errors.Cause(err))`; one call per file.
- Logger: `pkg/cli/cli.go:157` — `slog.NewTextHandler(os.Stderr, …)`; an `Any` attribute is rendered with `%+v` and the resulting string is quoted (newlines escaped).
- `errors.Cause` (`bborbe/errors` v1.6.1 `errors_cause.go:9`) delegates to `github.com/pkg/errors.Cause`, which loops on the `Cause() error` interface. `*dataError` implements `Cause()` (`errors_data-error.go:41`), so the unwrap goes **past** it to the deepest error:

  | Path | Deepest cause | `%+v` renders |
  |---|---|---|
  | duplicate-key YAML | `*yaml.TypeError` (no `Cause()`) | the multi-line message — no stack, but unbounded |
  | no frontmatter | `*pkg/errors.fundamental` (from `errors.Errorf`) | the ~30-line stack |

  Hence **`.Error()` is required to drop the stack** (Symptom B) and **truncation is required to bound the size** (Symptom A); neither alone fixes both.
- Reachability of the `task list` path: `vault-cli task list` → `pkg/ops/list.go:81` `pageStorage.ListPages` → `pkg/storage/page.go:66`. The page-loader fix therefore covers Symptom A's command.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| Warning records per unreadable file | one | one (already true) |
| `error` field content | the human-readable cause, e.g. `no frontmatter found` | the whole parse error; a Go stack on the frontmatter-less path |
| Warning record size (2,992-key file) | bounded, ≤1 KB | **332,280,825 bytes** (single line) |
| Warning record size (frontmatter-less page) | ≤300 B | **≈2,457 bytes** (stack) |
| `vault-cli vision list --vault private-personal` stderr | bounded; 6 lines | 13,626 bytes / 6 lines, longest line 2,287 bytes |
| Exit code | 0 | 0 (the flood is stderr, not a failure) |

## Why this is a bug

The skip is correct and documented in the code comment at `pkg/storage/page.go:64-65` ("Warn and continue: the operator must be told the file was skipped, otherwise the page silently disappears from listings that still exit 0"). The defect is in the reporting, which contradicts the command's contract: a read-only listing whose whole purpose is to return rows becomes unreachable when a file it has already decided to ignore is damaged. Cost should scale with the number of affected files, not with the size of one file's corruption.

## Goal

`vault-cli` listing commands degrade gracefully in the presence of malformed pages: each skipped file yields one warning line carrying the human-readable cause and no Go stack, with the embedded error truncated to a bounded size — and a global cap so that a directory full of unreadable pages yields a bounded amount of stderr rather than one line per file.

## Non-goals

- Fixing whatever writes malformed frontmatter — tracked separately as `Fix Agent-task-controller Task_identifier Accumulator Loop` (the *producer*; this spec is about vault-cli's *reaction*).
- Repairing existing corrupt files.
- Changing the skip-and-continue behaviour itself, which is correct.
- `pkg/storage/task.go:90` (`ListTasks`, `slog.Debug`) and the `ListTasksStrict` split — a separate call path, not the one that flooded. Not folded in here.

## Acceptance Criteria

Each AC declares its evidence shape and the fixture it is measured on.

- [ ] **AC1 — size independent of parse-error count.** One unreadable file produces exactly one `skipping unreadable page` record, and that record's size does **not** grow with the number of parse errors the file contains.
  - fixture: two duplicate-key files, 3 keys and 2,992 keys.
  - evidence: a unit test asserts exactly one record per file and that the 2,992-key record is ≤1 KB — i.e. the same bounded magnitude as the 3-key record. Pre-fix the 2,992-key record is 332,280,825 bytes → fails.
- [ ] **AC2 — `error` field ≤200 bytes, UTF-8-safe, head-keeping.** The warning's `error` field value (including the `…` if appended) is **≤200 bytes**, decodes as valid UTF-8, and ends in `…` when cut; truncation **keeps the head**.
  - fixture: the 2,992-key file (Symptom A), plus Fixture C (a duplicate-key file whose key names carry multibyte runes, so the 200-byte cut lands mid-rune).
  - evidence: a unit test reads the captured `error` field, asserts `len(field) ≤ 200`, asserts `utf8.ValidString(field)`, and — for the mid-rune fixture — asserts the field ends in `…`; the existing `pkg/storage/page_test.go:73` assertion that the log contains `already defined` still passes (proving the head is kept). Pre-fix the raw field is ≈318,855,578 bytes (the escaped record ≈332 MB) → fails.
- [ ] **AC3 — no Go stack; one bounded line for a frontmatter-less page.** A page with no frontmatter logs `error="no frontmatter found"` — a single physical line, no stack, **≤300 bytes** total.
  - fixture: a frontmatter-less page.
  - evidence: a unit test asserts the record is one line (`strings.Count(log, "\n") == 1`), is ≤300 bytes, and contains none of `\n\t`, `errors_new.go`, `errors_wrap.go`, `runtime.goexit`, `asm_arm64.s`. Pre-fix the record is ≈2,457 bytes and contains `runtime.goexit`/`errors_new.go` → fails. (Note: the *existing* `page_test.go` stack assertions use the duplicate-key fixture, whose path carries no stack — they pass today and do not discriminate; this AC must use the frontmatter-less fixture.)
- [ ] **AC4 — `task list` survives a corrupt file.** `vault-cli task list` against a vault holding a file with 2,992 duplicate `task_identifier` keys exits **0**, prints its normal row output, and writes **≤1 KB** to stderr.
  - fixture: fixture A (2,992-key file) plus healthy rows.
  - evidence: exit code 0; stdout contains the healthy rows; `wc -c` of stderr ≤1024. Pre-fix stderr is 332,280,825 bytes → fails.
- [ ] **AC5 — global cap with summary line.** Warning output is bounded globally: **fewer than 10** unreadable pages → one line each, no summary; **10 or more** → per-file lines stop at the 10th and exactly one `skipping N unreadable pages` summary line follows, with total stderr **≤3 KB**.
  - fixture: a synthetic 12-page frontmatter-less fixture (the ≥10 branch), a synthetic 15-page one (forces the summary to name a second N), and a synthetic 6-page one (the <10 branch).
  - evidence: 12-page fixture → exactly **10** `skipping unreadable page` lines, exactly **1** `skipping 12 unreadable pages` line, total stderr ≤3072 bytes; 15-page fixture → exactly **1** `skipping 15 unreadable pages` line; 6-page fixture → exactly **6** `skipping unreadable page` lines and **no** summary line. Budget: 10 per-file lines at ~185 B + one summary (~90 B) ≈ 1,940 B ≤3,072. Pre-fix the 12-page fixture emits 12 unbounded lines → fails.
- [ ] **AC6 — regression lock fails pre-fix.** The new discriminating tests in `pkg/storage/page_test.go` fail against the pre-fix implementation.
  - evidence: with the fix's source change temporarily reverted, `make test` reports the AC1/AC2/AC3/AC5 specs failing; restored, it passes. (AC1's "one record per file" count assertion alone does NOT discriminate — it is the size/stack assertions that flip.)
- [ ] **AC7 — sibling path untouched (negative).** The `ListTasks` path in `pkg/storage/task.go` is unchanged.
  - evidence: `git fetch origin` then `git diff origin/master -- pkg/storage/task.go` is empty after the change.
- [ ] **AC8 — `make precommit` green** and `CHANGELOG.md` carries a `## Unreleased` `fix:` bullet.
  - evidence: `make precommit` exits 0; `awk '/^## /{sec=$0} /skipping unreadable page/ {print sec}' CHANGELOG.md` prints `## Unreleased`. The repo has **no** `## Unreleased` section today (top is `## v0.158.2`), so the prompt must create it below the preamble and above the newest versioned section.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint / format / generate / test / checks clean.
- `make test` — unit + integration suite passes, including the new `pkg/storage/page_test.go` specs.
- `grep -n 'errors.Cause(err).Error()' pkg/storage/page.go` — the warn site logs the string form.
- `grep -n 'unreadable pages' pkg/storage/page.go` — the summary line exists.

### Operator-executable (runs on the host after PR merge, spec verification ladder)

- Build the **post-merge** tree: `cd ~/Documents/workspaces/vault-cli && git fetch origin && git merge origin/master` (this checkout, not the feature worktree), then `go build -o /tmp/new-vault-cli .`.
- **Fixture A** (2,992-key file), **Fixture B** (12 and 15 frontmatter-less pages) and **Fixture C** (mid-rune) walks, and the live `private-personal` check — see `# Desired Behavior` DB7.
- `make install` + `claude plugin update vault-cli@vault-cli` after `github-releaser-agent` tags — release gate per `docs/releasing-vault-cli.md`.

## Desired Behavior

1. **Per-file warning.** For each page that fails to parse, `ListPages` emits exactly one `level=WARN msg="skipping unreadable page"` line, with `file` naming the path and `error` carrying the cause — independent of how many errors the underlying YAML raised.
2. **Plain-string cause.** The `error` attribute is `errors.Cause(err).Error()` — a `string`, so `slog`'s text handler renders it verbatim and no `fundamental.Format` / `%+v` frame list is produced.
3. **Bounded, head-keeping truncation.** The cause string is truncated to ≤200 bytes before logging. Truncation is UTF-8-safe (never splits a multibyte rune) and appends `…` when any bytes were removed. The **head** of the message is preserved, so `already defined` (and the `line N` prefix that carries it) survives for the 2,992-key case.
4. **Bounded line.** For a frontmatter-less page the whole line — `time=` + `level=` + `msg=` + `file=` + `error=` — stays ≤300 bytes. For the 2,992-key file the line stays ≤1 KB (its long absolute `file` path plus the ≤200 B `error` field).
5. **Global cap.** The page loader counts skipped pages. While the count is <10 it emits the per-file warning as in DB1. On reaching 10 it suppresses further per-file lines and, at the end of the walk, emits a single `skipping N unreadable pages` summary line naming the total N. The threshold is a named constant (value 10). Consumer: the producer loop that accumulates duplicate keys ([[Fix Agent-task-controller Task_identifier Accumulator Loop]]) writes *many* malformed files, not one — the cap bounds a directory-level listing over such a directory, which the per-file truncation alone does not.
6. **Healthy rows unaffected.** Pages that parse are returned and listed exactly as before; only stderr reporting changes. `vault-cli task list` still exits 0 and prints its rows.
7. **Runtime confirmation.** Against fixture A: exit 0, healthy rows present, stderr ≤1 KB with exactly one per-file warning. Against fixture B (12 pages): exit 0, stderr ≤3 KB, per-file lines stop at the 10th, one `skipping 12 unreadable pages` line follows. Against `private-personal`: exit 0, exactly 6 `skipping unreadable page` lines, ≤3 KB.

## Constraints

- **Preserve the existing assertion.** `pkg/storage/page_test.go:73` asserts the log contains `already defined`; truncation must keep the head, not the tail. Do not weaken or delete that assertion.
- **Do not fold in `pkg/storage/task.go`.** `ListTasks` (Debug-level) is a different call path and stays out of scope.
- **Pinned bounds are load-bearing.** ≤200 B `error` field, ≤300 B line (frontmatter-less page), ≤1 KB / ≤3 KB stderr, threshold 10 — these are what make the criteria falsifiable; do not loosen them. They are jointly satisfiable only when each is measured on the fixture named in its AC (a 2,992-key line is bounded by AC4's ≤1 KB, not AC3's ≤300 B).
- **Skip-and-continue unchanged.** The `continue` after the warn stays; `ListPages` still returns the pages it could read plus `nil` error.
- **`pkg/ops/` stays output-free.** No formatting moves into `pkg/ops/`; the warn site remains in `pkg/storage/`.
- **No hand-bump of version strings or tags.** `.maintainer.yaml: release.autoRelease: true` — **Driver 1** (`github-releaser-agent`) owns the tag and bumps all four version strings post-merge. Add a `## Unreleased` bullet and let it run. (`.dark-factory.yaml: autoRelease: false` is Driver 2, deliberately off — it does not contradict this.) Do not edit `.claude-plugin/*.json` version fields or run `git tag`.

## Security / Abuse

Not a security fix, but a side benefit: the bound also removes an unbounded stack-trace disclosure on stderr (frame paths, module names) and caps a local stderr-flood vector. No new input surface is added.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Page with 2,992 duplicate keys | one ≤1 KB warning line; listing exits 0 | none needed — bounded by design |
| Page with no frontmatter | one ≤300 B line, `error="no frontmatter found"`, no stack | none needed |
| Parse error cuts mid-multibyte-rune at byte 200 | field truncated to valid UTF-8, ends in `…` | none needed — `utf8`-safe cut |
| Directory with ≥10 unreadable pages | per-file lines stop at 10th; one `skipping N unreadable pages` summary | none needed |
| Pathological file, cap not yet reached (single-file flood) | stderr ≤1 KB (AC2/AC4) | none needed — the field bound caps it before the cap counts |
| Truncation slices past the rune boundary | (must not happen) invalid UTF-8 in log | the `utf8.ValidString` assertion fails; fix the cut helper before proceeding |
| Fix accidentally changes `ListTasks` | (must not happen) out-of-scope behaviour drift | AC7's empty `git diff` fails; revert the stray edit |

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Log `errors.Cause(err).Error()`; truncate the cause to ≤200 B UTF-8-safe, head-keeping, `…` suffix; unit tests incl. the mid-rune fixture | 1, 2, 3, 4 | 1, 2, 3 | — |
| 2 | Global cap at threshold 10 + `skipping N unreadable pages` summary line; unit tests (6/10/12/15-page fixtures) | 5 | 5 | 1 |
| 3 | Regression lock: 2,992-key fixture through the real binary, pre-fix-fails evidence; `## Unreleased` `fix:` CHANGELOG bullet | 6, 7 | 4, 6, 7, 8 | 1, 2 |

Rationale: prompt 1 is the core fix and stands alone; prompt 2 builds on the counting prompt 1 introduces; prompt 3 adds the end-to-end fixture coverage and the changelog entry once both code paths exist. AC6 (the pre-fix-fails lock) is owned by prompt 3 alone.
