---
status: completed
spec: [042-prevent-duplicate-session-resume]
summary: 'Added the per-session lock section to docs/work-on-session-lifecycle.md (why, invariant, no-stale-lock, ErrSessionBusy refusal, lock directory, scope, detached-child safety) and created the CHANGELOG ## Unreleased section with the duplicate-resume guard bullet; docs+changelog only, scenario 005 untouched'
execution_id: vault-cli-session-lock-exec-204-spec-042-docs-and-changelog
dark-factory-version: dev
created: "2026-08-30T19:00:00Z"
queued: "2026-08-30T18:18:03Z"
started: "2026-08-30T19:01:06Z"
completed: "2026-08-30T19:03:46Z"
---

# Per-session lock docs and CHANGELOG (spec 042, prompt 2 of 2)

<summary>
- The work-on session lifecycle design document gains a section explaining the new per-session lock: why two writers on one transcript corrupt the jsonl, the invariant that the lock is held while a process writes the transcript, and how the kernel frees the lock on process death so a stale lock is impossible.
- The document records the `ErrSessionBusy` refusal semantics (hard failure on task and goal work-on, message names the session, never blocks or retries) and the lock-directory default under the user's home on a real local filesystem.
- The documented lock scope (launch path only) and the detached-child safety property make the follow-on vault-ui spec and future readers rely on a written contract rather than inference.
- The changelog gains an `## Unreleased` section with a bullet describing the duplicate-resume guard.
- No code changes: this prompt is docs and changelog only, and the interactive-TTY regression scenario stays byte-identical.
</summary>

<objective>
Give the per-session lock a durable written record so the design rationale lives in the project document rather than only in the spec, and record the duplicate-resume guard in the changelog for the next release. Covers spec 042 AC7.
</objective>

<context>
Read `/workspace/CLAUDE.md` and `/workspace/docs/dod.md` first (the dod changelog placement rule: `## Unreleased` goes below the preamble, above the newest `## vX.Y.Z`, never between the `# Changelog` title and the preamble). Then read fully:

- `/workspace/docs/work-on-session-lifecycle.md` — the design document from spec 040/041. It has sections for session-id ownership, post-exit write ordering, stream-json rejection, the TTY branch, `--output-format json`, the turn timeout, and the failure path. The lock section goes alongside these as a new `## ` section.
- `/workspace/CHANGELOG.md` — currently has NO `## Unreleased` section (the newest section is `## v0.117.3`). The AC7 evidence requires one to exist.
- `/workspace/scenarios/005-work-on-resume-auto-invokes-subtask.md` — do NOT touch (must stay byte-identical, AC5).
- `/workspace/pkg/ops/errors.go` — the `ErrSessionBusy` sentinel added by prompt 1, so the doc describes the real message.
- `/workspace/pkg/ops/session_lock.go` — the locker added by prompt 1 (default lock dir, flock mode, FD_CLOEXEC, fail-closed), so the doc matches the implementation.

Read these coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/documentation-guide.md` — structure for the design-doc section.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — bullet format and placement.
</context>

<requirements>
1. Add a new `## ` section to `/workspace/docs/work-on-session-lifecycle.md` titled something like `## The per-session lock` (exact title yours), placed after the existing sections. It must document, with real prose (not just headings):
   - **Why**: two claude processes working the same session id append to the single transcript `~/.claude/projects/<cwd>/<session-id>.jsonl`, and interleaved appends corrupt the jsonl — silent corruption, not merely a duplicate window. Nothing guarded against it before.
   - **The invariant**: the lock is held for the whole time a process writes the transcript. On the spawn path the parent holds it from the start of `StartSession` until it returns, released by a deferred unlock on every return path (clean turn, child exit error, ctx cancel, 30m bound). On the interactive resume path the lock fd survives the `syscall.Exec` into `claude --resume` (FD_CLOEXEC cleared), so the resumed claude itself holds the lock until it exits.
   - **No stale lock**: the kernel releases the flock when the holding process exits — normal exit, crash, or SIGKILL — so a re-work-on on the same session id succeeds immediately afterwards. No cleanup sweeps, no compensating clears, no lock TTL.
   - **The refusal**: a contended acquire returns `ErrSessionBusy` (a sentinel beside `ErrStarterUnavailable`, wrapped via `github.com/bborbe/errors` so `errors.Is` works), with a message naming the session id. `task work-on` and `goal work-on` surface it as a HARD failure (Success:false) — never downgraded to a warning — while the `ErrStarterUnavailable` soft path (warning, exit 0) is unchanged. `LOCK_EX|LOCK_NB` means the acquire never blocks and never retries: two racers collapse to one winner plus one refusal, zero corruption.
   - **The lock directory**: defaults under the user's home on a real local persistent filesystem (never tmpfs, never a shared/network mount — a lock on a mount cleared on reboot or shared across hosts would reopen the double-writer window). Created on demand with owner-only permissions; the directory must not be world-writable (an attacker-writable dir would let a local user pre-hold a lock and DoS work-on). The lock file is empty — no secret material.
   - **Lock scope**: the launch path only — `StartSession` (both interactive and non-interactive branches) and `ResumeSession`. The cached-id non-interactive re-persist spawns no writer and takes no lock; liveness gating there belongs to the vault-ui follow-on.
   - **The detached-child safety property**: on the spawn path, when the parent stops waiting (child exit error, ctx cancel, or the 30m bound), the detached child keeps running WITHOUT the parent's lock — safe because its id is never persisted on those paths, so no second engager can target it. Do not pre-persist the id.
   The section must contain the literal text `ErrSessionBusy`, `jsonl`, and `flock` (AC7 evidence greps the doc for `lock` case-insensitively; these make it substantive).
   - Do NOT change any existing section's content, and do NOT touch any code or test file.

2. Add the `## Unreleased` section to `/workspace/CHANGELOG.md`, below the preamble block and above `## v0.117.3` (the file has none yet — create it; do NOT rename an existing released section). Under it, one bullet describing the duplicate-resume guard, e.g.:
   ```
   - fix: a second `task work-on` / `goal work-on` (or a second Resume) against a session id whose per-session lock is already held is now refused with `ErrSessionBusy` instead of spawning a second claude process onto the same transcript — two writers on one jsonl silently corrupt it. An exclusive, non-blocking flock (`LOCK_EX|LOCK_NB`) is taken before any claude process is started or resumed, keyed by session id, released on every start-return path and by the kernel on process death (normal exit, crash, SIGKILL — no stale lock), and the interactive resume's lock fd survives the exec so the resumed claude holds it until exit. The interactive TTY branch and scenario 005 are unchanged.
   ```
   Do NOT bump version fields in `.claude-plugin/plugin.json` / `.claude-plugin/marketplace.json`, do NOT `git tag`, and do NOT rename the section to a version — version bumps and tags are a release-pipeline concern, never a prompt concern.

3. Do NOT modify: `scenarios/005-work-on-resume-auto-invokes-subtask.md` (must stay byte-identical — AC5), any file under `pkg/`, any test file, or `README.md`. This prompt is docs + changelog only.

4. Self-check before finishing: re-run every command in `<verification>` and confirm it passes, then walk spec 042's AC7 against the actual change and state which edit satisfies it.
</requirements>

<constraints>
- Docs and changelog only in this prompt: no behavior, signature, or test changes.
- The doc describes behavior that only exists after prompt 1 of this spec — do not claim the lock exists if it is not yet merged; the doc is written against the implementation already on this branch.
- `scenarios/005` must remain valid and unmodified — it is the regression lock on the TTY branch.
- `CHANGELOG.md` structure must satisfy `scripts/check-changelog.sh`: `# Changelog` → preamble → `## Unreleased` → `## v0.117.3` (newest first). Never place a section between the title and the preamble.
- Add the `## Unreleased` bullet only; do NOT bump version fields in `.claude-plugin/*.json` and do NOT `git tag` — version bumps and tags are a release-pipeline concern, never a prompt concern.
- Do NOT commit — dark-factory handles git. `.git` is visible in this container (`workflow: direct`, no `hideGit`), so git-based verification works; the prohibition is a scope rule.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0.

The lock is documented (AC7):

```
grep -ci 'lock' docs/work-on-session-lifecycle.md    # must be >= 1
grep -c 'ErrSessionBusy' docs/work-on-session-lifecycle.md   # must be >= 1
grep -c 'flock' docs/work-on-session-lifecycle.md    # must be >= 1
grep -c 'jsonl' docs/work-on-session-lifecycle.md    # must be >= 1
```

The changelog carries the guard bullet under `## Unreleased` (AC7):

```
grep -c '^## Unreleased' CHANGELOG.md                # must be >= 1
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume\|busy\|lock'   # must be >= 1
grep -m1 '^## v' CHANGELOG.md                        # must still print ## v0.117.3 (do not bump)
```

Scenario 005 is untouched (AC5):

```
git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md   # must exit 0
```

Then run the full gate once:

```
make precommit
```

Must exit 0 (its `check-changelog` target validates the `## Unreleased` placement). If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.

Before finishing, re-run every command in this block and confirm each passes, then walk spec 042's AC7 one criterion at a time against the actual change and state which edit satisfies it. Do not report success on any criterion whose evidence you have not run.
</verification>
