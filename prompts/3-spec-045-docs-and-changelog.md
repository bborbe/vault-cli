---
spec: ["045-bug-exit-code-outranks-validated-turn"]
status: draft
created: "2026-09-06T13:20:00Z"
---

# Lifecycle doc contract rewrite and CHANGELOG bullet (spec 045, prompt 3 of 3)

<summary>
- The design document currently states, as intended behavior, the exact rule this spec removes: that a non-zero child exit always fails the turn. That sentence is deleted.
- It is replaced by a statement of the new precedence — the validated turn result decides, and the process exit status only decides when there is no usable result to judge.
- The document also records why the exit status is the weaker signal: it arrives with no explanation attached, while the result is a structured document the code already knows how to validate.
- The section explaining why the wait bound never inspects a still-running child's output is made explicit, so a future reader does not "simplify" the read out of its branch.
- The changelog records the precedence change for the next release.
- Documentation and changelog only — no code, no tests, no scenarios.
</summary>

<objective>
Bring `docs/work-on-session-lifecycle.md` in line with the behavior spec 045 shipped, replacing the now-wrong documented contract, and record the change in the changelog. Covers spec 045 Acceptance Criteria 7 and 8.
</objective>

<context>
This prompt depends on prompts 1 and 2 of spec 045 already being applied — the doc must describe what actually shipped on this branch, not an intention.

Read `CLAUDE.md` and `docs/dod.md` first. The changelog placement rule from `docs/dod.md` is load-bearing: `## Unreleased` goes **below** the preamble block (the `All notable changes…` line and the `* MAJOR / MINOR / PATCH` lines) and **above** the newest `## vX.Y.Z` section — never between the `# Changelog` title and the preamble.

Then read in full:

- `docs/work-on-session-lifecycle.md` — the whole file. The sections that matter here are `## The fate of --output-format json` and `## What the turn timeout does and does not cover`. The second of those contains the sentence this prompt must remove: *"Expiry, ctx cancellation, and a non-zero child exit all return an error, so the caller persists nothing and the UI keeps showing **Start** rather than offering a Resume that cannot work."*
- `CHANGELOG.md` — read the top ~20 lines before editing. At authoring time it had **no** `## Unreleased` section and the newest was `## v0.124.1`, but `.maintainer.yaml` sets `autoRelease: true`, so verify the current shape rather than trusting that.
- `pkg/ops/claude_session.go` as it now stands — `runDetachedTurn`'s child-exited branch and `validateSessionTurn`'s rejection messages. The doc must describe the real implementation, including the real predicate wording.
- `specs/in-progress/045-bug-exit-code-outranks-validated-turn.md` — the "Why this is a bug" section carries the argument the replacement prose should compress into the doc.

Read this coding-plugin doc (in-container path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — bullet format and placement.
</context>

<requirements>

1. **Delete the stale contract sentence** from `## What the turn timeout does and does not cover` in `docs/work-on-session-lifecycle.md`. The literal string `a non-zero child exit all return an error` must not survive anywhere in the file.

2. **Replace it with prose stating the new precedence.** The replacement must contain the literal single-line phrase `validated result outranks the exit code` (spec AC7 greps for it verbatim, so do not hyphenate it, split it across a line wrap, or reword it). The replacement must state, in real prose:
   - Expiry and ctx cancellation still return an error and the caller persists nothing — those two are unchanged.
   - A non-zero child exit is no longer a failure by itself. Once the child has exited, the captured result is read and validated, and the **validated result outranks the exit code**: when the blob validates the turn is a success and the id persists.
   - Why the exit status is the weaker signal by construction: stderr goes to `os.DevNull`, so a non-zero exit arrives with no accompanying explanation, while the result blob is a structured document the code already knows how to validate. Trusting the opaque signal over the structured one was the inversion.
   - Why the mirror-image lie matters: discarding a session that *can* be resumed costs the whole turn, while the false positive it was guarding against costs one failed `claude --resume`.
   - The exit status remains the reason in exactly one case: the output is missing, unreadable, or not valid turn JSON, so there is no `result` text to surface.
   - When the blob parses but fails a predicate, the error leads with the child's own `result` text and names the failed predicate (`claude reported is_error: true`, `claude returned num_turns: 0`, `claude returned empty session_id`) — quote the real strings from `pkg/ops/claude_session.go`, do not paraphrase them.
   - The compensating clear is unchanged: it still fires on every error the detached turn returns. Only the definition of "failed" moved.

3. **Make the no-read-on-timeout rule explicit** in the same section, as its own short paragraph: the read lives inside the child-exited branch and is never hoisted above the `select`. On the timeout and cancellation paths the child is still running, so any bytes in the output file are partial by definition and must never be validated as success. Say plainly that this is a regression lock, with a unit test behind it, not a stylistic preference — a future reader who "simplifies" the read out of its branch reintroduces the bug in a worse form.

4. **Reconcile `## The fate of --output-format json`** with the new contract. Its closing line currently reads *"Stderr still goes to `os.DevNull`; a crash surfaces via exit code."* Keep the fact and correct the implication: stderr still goes to `os.DevNull`, which is precisely why the exit code carries no diagnostic content and is now the fallback signal rather than the primary one. Also update the sentence *"A turn whose result is `num_turns: 0`, `is_error: true`, or unparseable is an error, and no id is persisted."* so it says the validation verdict — not the process exit status — is what decides, and that the same shared `validateSessionTurn` still serves both branches.

5. **Do not rewrite other sections.** `## Session id ownership`, `## Why the TTY branch is untouched`, `## Failure path`, `## Post-exit write ordering`, `## Why stream-json was rejected`, and `## The per-session lock` stay as they are, apart from any sentence that directly contradicts the new precedence. **One is known**: in `## The per-session lock`, the *detached-child safety property* paragraph (~lines 168-176) lists "child exit error" alongside ctx cancel and the 30m bound as cases where the parent stops waiting and the clear removes the id. Under the new precedence a non-zero child exit with a validating blob is a success, not a failure — correct that enumeration to name only the paths that still fail, and leave the rest of the paragraph (the layered safety argument, the lock, the goal-path ordering) untouched. If you find any other contradicting sentence, correct only that sentence and say which in the completion summary.

6. **Add the `## Unreleased` section to `CHANGELOG.md`**, below the preamble block and above the newest `## vX.Y.Z` section (whatever it is when you run — do not assume a version number). If `## Unreleased` already exists, APPEND the bullet to it and do NOT create a second one. Never rename an existing released section. Under it, one `- fix:` bullet describing the precedence change. The bullet must contain the literal phrase `validated result outranks the exit code` (spec AC8 greps the topmost `## ` section for it). It should also name the user-visible payoff: a headless `work-on` turn that completed successfully is no longer discarded because its child process exited non-zero, so the session id persists and the Vault UI offers Resume; a genuinely failed turn still clears the id and now reports the child's own reason instead of `exit status 1`.

7. **Do NOT bump version fields** in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, do NOT tag, and do NOT rename `## Unreleased` to a version. `.maintainer.yaml` sets `autoRelease: true` — the release bot owns version bumps and tags, and hand-bumping races it.

8. **Do NOT modify** any file under `pkg/`, any `_test.go` file, or anything under `scenarios/`. This prompt is documentation and changelog only.

9. **Self-check before finishing.** Re-run every command in `<verification>` and confirm it passes. Then quote the deleted sentence and its replacement side by side, and state which edit satisfies spec 045 AC7 and which satisfies AC8.

</requirements>

<constraints>
- Documentation and changelog only in this prompt: no behavior, signature, or test changes.
- The doc describes behavior that exists on this branch after prompts 1 and 2 — write it in the present tense as shipped fact, and quote the real error strings from `pkg/ops/claude_session.go` rather than inventing wording.
- `CHANGELOG.md` structure must satisfy `scripts/check-changelog.sh`, which `make precommit` runs. The final order is always `# Changelog` → preamble → `## Unreleased` → newest `## vX.Y.Z` (newest first).
- The phrase `validated result outranks the exit code` must appear on a single line in both `docs/work-on-session-lifecycle.md` and the topmost `## ` section of `CHANGELOG.md`. A line wrap in the middle of it defeats the AC greps even though the prose reads fine.
- Do NOT weaken or remove the documented compensating clear — spec Non-goal. A genuinely failed turn must still clear the id, and the doc must keep saying so.
- Do NOT document Vault UI banner styling — spec Non-goal. Only the message content changed.
- Do NOT speculate in the doc about why a clean-`end_turn` child exits 1 — spec Non-goal, and the cause is unreproduced. State only that vault-cli must not depend on the exit code being trustworthy.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from the repo root:

```
make precommit
```

Must exit 0 (this runs `scripts/check-changelog.sh` and `scripts/check-versions.sh`).

Then, each of these must hold:

```
! grep -q 'a non-zero child exit all return an error' docs/work-on-session-lifecycle.md
```
must succeed — the stale contract sentence is gone. (Written as `! grep -q` on purpose: `grep -c` prints `0` but exits `1`, which would report a passing absence check as a failure.)

```
grep -c 'validated result outranks the exit code' docs/work-on-session-lifecycle.md
```
must print `1` or more.

```
awk '/^## /{n++} n==1' CHANGELOG.md | grep -c 'validated result outranks the exit code'
```
must print `1` or more — the phrase is inside the topmost `## ` section. Heading-independent on purpose: the release bot may rename `## Unreleased` to `## vX.Y.Z` between prompts.

```
test "$(grep -n -m1 '^## ' CHANGELOG.md | cut -d: -f1)" -gt "$(grep -n -m1 '^All notable changes to this project' CHANGELOG.md | cut -d: -f1)"
```
must exit 0 — the first `## ` heading sits below the preamble, never above it. Assertive on
purpose: a bare `head -12 CHANGELOG.md | grep -n '^## '` exits 0 even when the heading is
misplaced above the preamble, so it could never fail the case it targets.

```
! grep -rq 'a non-zero child exit all return an error' docs/
```
must succeed.

Nothing above distinguishes "the full rationale landed" from "one sentence containing the
magic phrase landed". The rewritten section must carry the argument, not just the string:

```
awk '/^## What the turn timeout does and does not cover$/,/^## Failure path$/' docs/work-on-session-lifecycle.md > /tmp/turn-section.txt
grep -q 'validated result outranks the exit code' /tmp/turn-section.txt
grep -q 'os.DevNull' /tmp/turn-section.txt
grep -qi 'hoist' /tmp/turn-section.txt
test "$(wc -l < /tmp/turn-section.txt)" -ge 30
```

All must exit 0: the precedence statement, the why-the-exit-code-is-the-weaker-signal
argument (`os.DevNull`), and the no-hoist regression lock all live in this section, which
was 16 lines before the rewrite.
</verification>
