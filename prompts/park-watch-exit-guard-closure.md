---
status: draft
spec: [067-bug-parked-exit-requires-registered-watch]
created: "2026-11-18T23:39:18Z"
branch: dark-factory/bug-parked-exit-requires-registered-watch
---

<!--
AUDITOR NOTE — tree state verified 2026-11-18T23:39Z.

The behaviour this spec asks for is ALREADY PRESENT on this branch and shipped on
master in v0.194.1 (docs) and v0.194.2 (guard). Verified by reading the files:

  - scripts/session-self-exit.py: `claude_root` (~:510), `park_producer` (~:520),
    `park_store_dir` (~:534), `watched_session` (~:548), `park_guard` (~:603);
    called at the parked-task accept point (~:794) and the goal/topic accept
    point (~:828); `EXIT_REFUSED = 10` unchanged.
  - scripts/session-self-exit-test.sh: stub producer at :50-52, park fixtures at
    :302-425 and :678; 58 EXIT_REFUSED assertions; passes (98 tests).
  - docs/park-watches.md: exists, 56 lines, covers schema + failure modes.
  - commands/session-close.md: Phase 4.5 (248-388) names `park-event.py register`.
  - CHANGELOG.md: bullets present in v0.194.1 and v0.194.2.

The ONLY remaining delta is a stale sentence in `commands/session-close.md`
Phase 4.5 that still tells readers the guard "is not on master yet". This prompt
therefore (a) verifies every acceptance criterion against the shipped code with
exact commands, (b) corrects that sentence, and (c) records the correction in
`## Unreleased`. If this branch were ever cut before v0.194.2, requirement 2
carries the full contract so the executor can land it — the requirements are
idempotent, not a rewrite.

Open question for the auditor: if the intent was a fresh implementation, the spec
is stale relative to master and should be closed rather than re-implemented.
-->

# Close spec 067: confirm the park-watch exit guard and correct the stale doctrine note

<summary>
- Confirms the shipped rule that a parked session cannot exit unless a registered park watch names it.
- Confirms the refusal fires for a parked task and for a parked goal or topic page alike.
- Confirms a park that has a watch still exits cleanly, and a watch for a different session does not count.
- Confirms malformed records and lock files are tolerated, with exactly one warning per bad file.
- Confirms the real exit path is guarded, not only the dry-run preview.
- Confirms the guard fails open only when the park mechanism is not installed, and refuses when the store is merely absent.
- Corrects the session-close doctrine, which still tells readers the guard has not shipped.
- Records the doctrine correction in the changelog.
- Leaves the shipped guard code and its test harness unchanged.
</summary>

<objective>
Make the tree's park-watch behaviour match spec 067 end to end: a session may not exit "parked" unless a registered park watch names it, and the doctrine that teaches parking must stop claiming the guard is absent. The guard and its fixtures already exist on this branch, so the work is a contract audit plus one doctrine correction — not a reimplementation.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read these files before touching anything:

- `scripts/session-self-exit.py` — the decision script. Focus on `claude_root`, `park_producer`, `park_store_dir`, `watched_session`, `park_guard`, `terminal_verdict`, `page_verdict`, and `main`.
- `scripts/session-self-exit-test.sh` — the harness. Focus on the stub producer block near the top, `run` / `run_msg` / `run_live`, the park fixtures, and the fail-open fixtures.
- `commands/session-close.md` — read Phase 4.5 in full, from `### Phase 4.5` up to (not including) `### Phase 4.6`.
- `docs/park-watches.md` — the contract page for the store and the guard.
- `CHANGELOG.md` — the `## Unreleased` section only.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry style.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — verification expectations.

Environment facts that shape this prompt:

- `.git` is masked in this container (`hideGit`). Never run a `git` command; every check below is filesystem- or test-based.
- The localhost HTTP readiness probe inside the shell harness is filtered by the container's HTTP proxy. Prefix any run of the harness or `make` with `NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost`, or the harness exits 2 with `stub attention store never answered`.
- `make precommit` does NOT run `check-versions` in this repo; it runs `ensure format generate test check addlicense`. Do not bump any version manifest.
</context>

<requirements>
1. Audit the guard in `scripts/session-self-exit.py`. It is present; do not rewrite it. Confirm each contract below holds, and only if an element is genuinely absent add it in the style of the surrounding code:
   - `claude_root()` returns `os.environ.get("CLAUDE_CONFIG_DIR") or os.path.expanduser("~/.claude")`.
   - `park_producer()` globs `<claude_root>/plugins/cache/claude-supervisor/*/*/scripts/park-event.py`, returns the first sorted hit, and returns `None` when there is no hit. Producer presence — never store readability — is the fail-open key.
   - `park_store_dir()` returns `$PARK_EVENT_STATE_DIR` when set and non-empty, else `os.path.join(os.path.expanduser("~"), ".claude", "state", "park-events")`. It must NOT be derived from `CLAUDE_CONFIG_DIR`.
   - `watched_session(sid)` returns `(True, note)` when the producer is absent, with `note` naming the absent producer and saying the guard fails open. Otherwise it lists the store directory; an `OSError` reading the directory means "no watch" (never fail open). It reads only `*.json` files, ignores `*.lock` files silently, skips a record that is malformed JSON or is not a dict or has no `sessions` key or has a non-list `sessions`, and emits exactly one `WARN: <message>` line to stderr naming the offending file for each such record. It returns `(True, "")` when `sid` appears in any record's `sessions` list, and `(False, reason)` otherwise, where `reason` names `sid` and states that no registered park watch names the session.
   - `park_guard(sid, label, verdict)` returns `None` when `watched_session` allows the park (printing the fail-open note to stderr when one is returned), and returns a refusal string of the form `<label>: <verdict>, but <note>` when it does not.
   - `watched_session` and `park_guard` never write, create or delete anything under the store, and never import or shell out to `park-event.py`.
   - `park_guard` is called from the parked-task accept point in `main` (the branch where the held task's verdict starts with `parked until`) and from the goal/topic accept point inside the loop over `held` pages. Both calls sit on the decision path in `main`, before the final `SELF-EXIT: exit` print, so the live exit is guarded exactly as `--dry-run` is.
   - The headless branch inside `terminal_verdict` still returns its own refusal before any `defer_date` parsing, and for goal/topic holders the headless refusal still happens earlier in `main`. The new guard is therefore reached only for a non-headless park.
   - `EXIT_REFUSED = 10` is unchanged.
2. Audit the harness in `scripts/session-self-exit-test.sh`. It is present; do not delete or weaken any assertion. Confirm:
   - A stub producer exists at `$TMP/plugroot/plugins/cache/claude-supervisor/supervisor/0.0.0/scripts/park-event.py` and `CLAUDE_CONFIG_DIR="$TMP/plugroot"` is passed per invocation to fixtures that need the producer present. The store path is passed separately as `PARK_EVENT_STATE_DIR`; the two resolve by different rules.
   - Fixtures cover: an unwatched parked task refused; a parked goal/topic refused; a watch naming the session allowed; a watch naming only another session refused; a missing store directory refused (not failed open); a malformed record plus a schema-drift record plus `*.lock` files tolerated with the valid watch still found and each bad file warned about exactly once; the live exit path (`run_live`, no `--dry-run`) refused with a `SELF-EXIT: stay` message; fail-open under a `CLAUDE_CONFIG_DIR` with no producer; fail-open under a `HOME`-derived root with `CLAUDE_CONFIG_DIR` unset.
3. Correct the stale doctrine sentence in `commands/session-close.md` Phase 4.5. Replace exactly this sentence:

   `The exit guard that would refuse such a park (spec 067, PR #378) is **not on master yet** — until it ships, the exit is accepted silently, and this instruction is the only thing standing between a park and a session nothing resumes.`

   with:

   `The exit guard that refuses such a park (\`scripts/session-self-exit.py\`, shipped in v0.194.2) is live: an unwatched park is refused with \`SELF-EXIT: stay — … no registered park watch names session <id>\`, so the session stays alive and reports instead of exiting into silence.`

   Keep the surrounding text, including the "on a host that has the park mechanism" scoping and the `park-event.py register` instruction, unchanged. The replacement must stay inside Phase 4.5 (between `### Phase 4.5` and `### Phase 4.6`).
4. Append one bullet to the existing `## Unreleased` section of `CHANGELOG.md`:

   `- docs: \`session-close\` Phase 4.5 now states the park-watch exit guard is live (shipped v0.194.2) and refuses an unwatched park, replacing the stale "not on master yet" note so a session following the doctrine registers a watch instead of assuming the exit is unguarded`

   Append to the section; do not create a second `## Unreleased` heading and do not touch any `## vX.Y.Z` section.
5. Leave `docs/park-watches.md` as it is. Confirm it documents the store location, the record schema with the `sessions` list, the fail-open-on-producer-presence rule, the absent-store-refuses rule, the malformed/lock tolerance, and the read-only nature of the guard. If any of those is missing, add the missing paragraph; do not restructure the page.
6. Self-check before finishing: re-run the `<verification>` commands, confirm each expected outcome, and walk every acceptance criterion below against the change:
   - an unwatched parked task is refused with a message naming the session and the missing watch;
   - an unwatched parked goal/topic is refused the same way;
   - a watched park still exits with `parked until <defer_date>`;
   - a watch for another session does not count;
   - malformed records and lock files are tolerated with one warning per bad file;
   - the live exit path is guarded and prints `SELF-EXIT: stay`;
   - fail-open happens only when the producer is absent, under both root resolutions;
   - the doctrine names registration and no longer claims the guard is absent.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT renumber `EXIT_REFUSED`; 58 refusal assertions in `scripts/session-self-exit-test.sh` depend on the value 10.
- Do NOT change the park-due step. It is correct to read registered watches only.
- Do NOT change what counts as a park: `in_progress` plus a `defer_date` parsing to a future instant. A past, date-only, offset-less or unparseable `defer_date` keeps its current refusal wording, and the `no session id` refusal keeps exit 10.
- Do NOT relax the `no session id` refusal into a fail-open.
- Do NOT shell out to, or import, `park-event.py`. The store is read as files.
- The guard is read-only: never write, create or delete anything under the park-watch store.
- Fail-open is keyed on the producer's presence under the resolved plugin-cache root, never on the store directory's readability.
- Do NOT delete or weaken any assertion in `scripts/session-self-exit-test.sh`.
- Do NOT hand-bump version manifests or create a tag; the releaser owns them. The changelog change is one `## Unreleased` bullet only.
- Keep every path repo-relative in any code you write; do not introduce host-absolute paths.
- Existing tests must still pass.
</constraints>

<verification>
Run each command from the repo root. Every one must produce the stated outcome.

1. Shell harness passes (the proxy bypass is required):

   `NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost bash scripts/session-self-exit-test.sh`

   Expected: exit 0 and a final line `session-self-exit: 98 passed` (the count may be higher if assertions were added; it must not be lower).

2. Full precommit passes:

   `NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost make precommit`

   Expected: exit 0 and `ready to commit`.

3. The stale claim is gone:

   `! grep -q 'not on master yet' commands/session-close.md`

   Expected: no output, exit 0.

4. Registration is still taught inside Phase 4.5:

   `awk '/^### Phase 4\.5/{f=1} /^### Phase 4\.6/{f=0} f' commands/session-close.md | grep -q 'park-event.py register'`

   Expected: exit 0.

5. The guard is still wired at both accept points plus its definition:

   `[ "$(grep -c 'park_guard(' scripts/session-self-exit.py)" -eq 3 ]`

   Expected: exit 0.

6. The guard still references its contract page:

   `grep -q 'docs/park-watches.md' scripts/session-self-exit.py`

   Expected: exit 0.

7. The harness still carries the stub producer at four or more call sites:

   `[ "$(grep -c 'CLAUDE_CONFIG_DIR=' scripts/session-self-exit-test.sh)" -ge 4 ]`

   Expected: exit 0.

8. No harness lines were lost:

   `[ "$(wc -l < scripts/session-self-exit-test.sh)" -ge 776 ]`

   Expected: exit 0.

9. The refusal-assertion count did not drop:

   `[ "$(grep -cE '^[[:space:]]*run(_msg|_live)? 10 ' scripts/session-self-exit-test.sh)" -ge 58 ]`

   Expected: exit 0.

10. The changelog carries the correction in the right section:

    `awk '/^## /{sec=$0; next} sec=="## Unreleased"' CHANGELOG.md | grep -q '^- docs:.*park'`

    Expected: exit 0.
</verification>

## Completion report

Report `"status":"success"` only when `make precommit` exits 0. Include `"verification":{"command":"NO_PROXY=127.0.0.1,localhost make precommit","exitCode":N}` with the actual exit code. Report `"status":"partial"` if the doctrine correction and changelog bullet landed but `make precommit` failed on issues unrelated to this change; report `"status":"failed"` if the correction is incomplete or the harness fails.
