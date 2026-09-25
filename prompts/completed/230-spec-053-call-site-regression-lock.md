---
status: completed
spec: [053-fleet-spawned-metrics-sessions]
summary: Added scripts/metrics-append-call-site-test.sh, a section-scoped self-checking regression lock for the session-connect metrics call site, and wired it into the Makefile test target
execution_id: vault-cli-metrics-exec-230-spec-053-call-site-regression-lock
dark-factory-version: v0.196.0
created: "2026-09-25T17:18:12Z"
queued: "2026-09-25T17:37:23Z"
started: "2026-09-25T17:51:02Z"
completed: "2026-09-25T17:54:22Z"
branch: dark-factory/fleet-spawned-metrics-sessions
---

# The call-site regression lock: a `make test` script that pins § Session connect

<summary>
- A new test runs on every `make test` and fails the build if the plugin's session-connect step stops invoking the metrics append.
- It checks the invocation is inside § Session connect, not merely somewhere in the file, so a passing mention elsewhere cannot satisfy it.
- It checks the invocation is an actionable step line in the branch that writes the session id — between the two branch bullets — and not a comment or a fenced code block.
- It checks the section still states the non-fatal warning for a failed append.
- It proves it measures rather than recites: it strips the invocation in a throwaway copy and re-runs itself there, requiring a non-zero exit, then re-runs itself against the real tree requiring zero.
- No Go code, no test fixture, no scenario is added — this is the regression lock for prose that no unit test can reach.
</summary>

<objective>
Add `scripts/metrics-append-call-site-test.sh` — a self-checking regression lock for the call site prompt 2 added — and wire it into the `make test` target beside the existing `scripts/struck-row-rule-test.sh`. Without it, a later edit to `agents/work-on-task-assistant.md` § Session connect silently drops the invocation and fleet-spawned runs vanish from `metrics_sessions` again, exactly as they are absent today.
</objective>

<context>
This prompt depends on prompt 2 of spec 053 already being applied: the script asserts against the invocation prompt 2 inserted, and fails if it is absent. Read `CLAUDE.md` first for project conventions.

Then read in full:

- `scripts/struck-row-rule-test.sh` — **the frozen template.** Copy its structure: the header comment explaining why prose needs a test at all, `set -uo pipefail` without `-e`, `ROOT=$(cd "$(dirname "$0")/.." && pwd)` followed by `cd "$ROOT" || exit 2`, the `ARTIFACTS` array, the `PATTERN`, the `pass` / `fail` counters with a `check <label> <want> <got>` function, the `missing <root>` helper that prints the artifacts lacking the pattern, the `section <file> <heading>` awk helper, and the self-check that strips the pattern in a `mktemp -d` throwaway copy and requires exactly the stripped artifact to be reported.
- `scripts/daily-note-has-entry-test.sh` — the second template, for the **exit-code direction**. Its `expect <want-exit> <label> …` helper runs a script and asserts its exit code, and its header documents the `awk | grep -q` + `pipefail` SIGPIPE hazard (bug 4) that makes a present match read as absent. Your script has the same hazard: never put a `grep -nF … | head -1 | grep -q …` pipeline directly in an `&&` condition.
- `Makefile` § `.PHONY: test` — the target that already runs `scripts/daily-note-has-entry-test.sh`, `scripts/goal-task-link-count-test.sh` and `scripts/struck-row-rule-test.sh`. You add one line there.
- `agents/work-on-task-assistant.md` — the artifact under test. Confirm by reading it that § Session connect now carries the invocation as a sub-bullet between the `If EXACTLY ONE UUID is returned` bullet and the `If zero OR multiple UUIDs are returned` bullet, and that the section states the warning. If the invocation is absent, STOP and report `status: failed` with the message "session-connect call site not yet deployed (prompt 2)" — do NOT add it here.
- `specs/in-progress/053-fleet-spawned-metrics-sessions.md` — the spec. Read its Desired Behavior 7, Acceptance Criterion 6 and the Verification section's `bash scripts/metrics-append-call-site-test.sh` line. Every requirement below comes from them.

**Environment facts that shape this prompt:**

1. **Every check in `<verification>` is git-free.** The daemon's executor does not check `<verification>` exit codes, so a command that dies for an environmental reason still reports a pass. The spec's git-shaped evidence is reproduced here by running the script itself in both directions. The git forms stay on the spec's `# Verification` § "Operator-executable" rung.
2. **This prompt is bash only.** No Go file, no test file, no scenario, no documentation changes. The docs are prompt 4.
3. **`make test` runs the script, and `make precommit` runs `make test`.** A failure in this script therefore fails `make precommit` — which is the point.
</context>

<requirements>

## 1. Scope — two files

- `scripts/metrics-append-call-site-test.sh` — NEW, mode `0755` (run `chmod 0755 scripts/metrics-append-call-site-test.sh`; every sibling in `scripts/` is `0755`).
- `Makefile` — MODIFIED, one line added to the `test` target.

Nothing else. Do NOT touch `agents/**`, `commands/**`, `docs/**`, `scenarios/**`, `README.md`, `CHANGELOG.md`, any Go file, any test file, or any other script. In particular do NOT modify `scripts/struck-row-rule-test.sh` or `scripts/check-changelog.sh`.

## 2. `scripts/metrics-append-call-site-test.sh` — the frozen constants

```bash
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

AGENT=agents/work-on-task-assistant.md
PATTERN='vault-cli task append-metrics-session'
SECTION='### Session connect (MANDATORY when Obsidian task file exists)'
ONE_BRANCH='If EXACTLY ONE UUID is returned'
ANY_BRANCH='If zero OR multiple UUIDs are returned'

# The artifacts that must carry the invocation. One entry today: § Session connect
# in the work-on-task agent definition is the only place a session-connect may
# invoke the verb. It stays a list so a second call site is one line away.
ARTIFACTS=("$AGENT")
```

The file's first line is `#!/usr/bin/env bash`, then the header comment block of requirement 7, then a blank `#` line, then `set -uo pipefail` — the same order as `scripts/struck-row-rule-test.sh`. The five strings above are contractual and must appear verbatim — they are grep targets in `<verification>`.

## 3. `scripts/metrics-append-call-site-test.sh` — the helpers

Define these three helpers, in this order, **above** the `--check-only` branch of requirement 4 (bash defines a function only when its definition is executed, so a helper used by `--check-only` must precede it):

```bash
# missing <root> — artifacts under <root> that do NOT name the invocation.
missing() {
	local root=$1 f
	for f in "${ARTIFACTS[@]}"; do
		grep -qF -- "$PATTERN" "$root/$f" 2>/dev/null || printf '%s\n' "$f"
	done
}

# section <file> <heading> — the lines under an exact heading, until the next
# heading. Exact whole-line match: a substring match would also open on a mention
# of the heading inside prose.
section() {
	awk -v h="$2" '$0 == h { f = 1; next } f && /^#/ { exit } f' "$1"
}

# lineOf <file> <fixed-string> — the first matching line number, empty when absent.
lineOf() {
	grep -nF -- "$2" "$1" 2>/dev/null | head -1 | cut -d: -f1
}
```

`missing` and `section` mirror the template's helpers member for member. `lineOf` is new and exists so no `grep … | head -1 | grep -q …` pipeline appears in an `&&` condition — under `pipefail` that shape can report a present match as absent (see `scripts/daily-note-has-entry-test.sh`'s bug-4 note). Capture the line number with `lineOf`, then read the line with `sed -n "${n}p"` and match on that string.

## 4. `scripts/metrics-append-call-site-test.sh` — `--check-only`, the exit-code direction

Immediately after the helpers, before any check runs, add:

```bash
# --check-only <root>: the presence check alone, against <root>. Exits 0 when
# <root>'s artifact carries the invocation and 1 when it does not. The self-check
# below re-invokes this script in this mode; nothing else uses it.
if [ "${1:-}" = "--check-only" ]; then
	[ -z "$(missing "${2:?--check-only needs a root}")" ] || exit 1
	exit 0
fi
```

This mode is what makes "the script exits non-zero when the invocation is missing" **executed** evidence rather than a claim: the self-check in requirement 6 re-invokes this same script in this mode against a stripped throwaway copy and against the real tree, and requires the two exits to differ.

## 5. `scripts/metrics-append-call-site-test.sh` — the checks

After the `--check-only` branch, define the counters and the `check` function exactly as the template does (`pass=0`, `fail=0`, and `check <label> <want> <got>` comparing the two and incrementing the counters), then run these checks in order:

1. **The invocation is present in every artifact.**
   ```bash
   check "every artifact names the invocation" "" "$(missing "$ROOT")"
   ```
2. **It is inside § Session connect, section-scoped — not merely somewhere in the file.**
   ```bash
   check "the invocation is inside § Session connect" "yes" \
       "$(section "$ROOT/$AGENT" "$SECTION" | grep -qF -- "$PATTERN" && echo yes)"
   ```
3. **It is an actionable step line, not a comment and not a fenced code block.** Capture the line with `lineOf` + `sed`, then assert it starts with optional whitespace and `- `:
   ```bash
   INV_NUM=$(lineOf "$ROOT/$AGENT" "$PATTERN")
   INV_TEXT=""
   [ -n "$INV_NUM" ] && INV_TEXT=$(sed -n "${INV_NUM}p" "$ROOT/$AGENT")
   check "the invocation line is a bulleted step" "yes" \
       "$(printf '%s\n' "$INV_TEXT" | grep -qE '^[[:space:]]*- ' && echo yes)"
   ```
   `printf` of one short line into `grep -q` cannot hit the SIGPIPE hazard; a `grep -nF … | head -1 | grep -q …` pipeline can.
4. **It sits in the branch that writes the session id: strictly between the two branch bullets.**
   ```bash
   ONE=$(lineOf "$ROOT/$AGENT" "$ONE_BRANCH")
   ANY=$(lineOf "$ROOT/$AGENT" "$ANY_BRANCH")
   check "the invocation is after the EXACTLY ONE bullet" "yes" \
       "$([ -n "$ONE" ] && [ -n "$INV_NUM" ] && [ "$INV_NUM" -gt "$ONE" ] && echo yes)"
   check "the invocation is before the zero OR multiple bullet" "yes" \
       "$([ -n "$ANY" ] && [ -n "$INV_NUM" ] && [ "$INV_NUM" -lt "$ANY" ] && echo yes)"
   ```
5. **The section still states the non-fatal warning.**
   ```bash
   check "the section states the warning" "yes" \
       "$(section "$ROOT/$AGENT" "$SECTION" | grep -qE 'warning.*metrics|metrics.*warning' && echo yes)"
   ```

## 6. `scripts/metrics-append-call-site-test.sh` — the self-check

After the five checks, add the self-check. It strips the invocation from the artifact in a throwaway copy and requires **exactly that artifact** to be reported, then runs the two exit-code directions:

```bash
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/$(dirname "$AGENT")"
sed "s/$PATTERN/STRIPPED/" "$ROOT/$AGENT" >"$TMP/$AGENT"
check "self-check: only the stripped artifact is reported" "$AGENT" "$(missing "$TMP")"

# The non-zero direction: the same script, against a tree whose invocation is gone.
if bash "$ROOT/scripts/metrics-append-call-site-test.sh" --check-only "$TMP" >/dev/null 2>&1; then
	fail=$((fail + 1))
	echo "❌ self-check: --check-only exited 0 on a tree without the invocation" >&2
else
	pass=$((pass + 1))
fi

# The zero direction: the same script, against the real tree.
if bash "$ROOT/scripts/metrics-append-call-site-test.sh" --check-only "$ROOT" >/dev/null 2>&1; then
	pass=$((pass + 1))
else
	fail=$((fail + 1))
	echo "❌ self-check: --check-only exited non-zero on the real tree" >&2
fi
```

End the script exactly as the template does:

```bash
echo "metrics-append-call-site: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
```

The `sed` pattern is the literal invocation string; it contains no `/`, so it needs no escaping. The child invocations pass an absolute root, and they exit inside the `--check-only` branch before reaching `mktemp`, so there is no recursion and no nested `trap`.

## 7. The header comment

Between the `#!/usr/bin/env bash` shebang and the `set -uo pipefail` line, the script carries a `#`-prefixed header explaining **why prose needs a test at all**, in the template's voice. It must state, in prose:

- A fleet-spawned worker records its session in the task's metrics only because § Session connect in `agents/work-on-task-assistant.md` invokes `vault-cli task append-metrics-session` in the branch that writes `claude_session_id`.
- That step is a Claude Code agent definition: it has no test harness and runs only inside a real LLM in a real spawned worker behind `mcp__supervisor__spawn_agent`. An integration test can exercise the verb, never the call site firing. The prose IS the implementation, so an edit that drops the invocation is a behaviour regression no unit or integration test can see.
- The pattern is the discriminating fact: an artifact that does not name the invocation cannot be performing it.
- The self-check strips the invocation in a throwaway copy and re-invokes this script in `--check-only` mode against it, requiring a non-zero exit, then against the real tree requiring zero — so "the check can fail" is executed evidence rather than a claim.
- `Run from repo root (Makefile target \`test\`).`

## 8. `Makefile` — wire it into `make test`

In the `.PHONY: test` target, add one line immediately after `@bash scripts/struck-row-rule-test.sh`:

```make
	@bash scripts/metrics-append-call-site-test.sh
```

The line must be tab-indented like its siblings, and it must be the last line of the target's recipe. Do NOT change any other target, and do NOT reorder the existing script lines.

## 9. Self-check before finishing

- Run `bash scripts/metrics-append-call-site-test.sh` and confirm it prints `metrics-append-call-site: 9 passed, 0 failed` and exits 0. (Nine: the six checks of requirement 5 plus the stripped-artifact check plus the two exit-code directions.)
- Strip the invocation from `agents/work-on-task-assistant.md` in a scratch copy of the repo, run the script there, confirm it exits non-zero and names the artifact — then discard the scratch copy. Do NOT leave the real file modified.
- Run `shellcheck scripts/metrics-append-call-site-test.sh` if it is available and fix what it reports; if it is not installed, say so in the completion report rather than skipping silently.
- Walk spec 053's Acceptance Criterion 6's script half and Desired Behavior 7 and state in your completion report which requirement satisfies each, and confirm the spec's Verification line `bash scripts/metrics-append-call-site-test.sh — exits 0 with the invocation present; exits non-zero when the invocation is stripped from a throwaway copy` holds in both directions.
- Walk spec 053's Failure Modes row "The agent definition loses the invocation (a later edit to § Session connect)" and state that this script is its detector.
- Confirm each check in `<verification>` passes by **running** it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 053 — non-goals (hard vetoes).** Do NOT modify `agents/work-on-task-assistant.md` — the call site is prompt 2's artifact and this prompt only asserts against it. Do NOT change the Phase 3 status-flip rule. Do NOT add the field to any allowlist. Do NOT add a scenario. Do NOT touch `commands/**`, `docs/**`, `README.md` or `CHANGELOG.md` — prompt 4 owns those.
- **Copied from spec 053 — constraints.** The accumulator's semantics are frozen (accumulate, never suppress); this script does not test the accumulator, only the call site. `make precommit` must pass in the repo root, and it runs `make test`, which now runs this script. The script must not require network access, a built binary, a vault, or any environment variable.
- **Frozen strings.** `AGENT=agents/work-on-task-assistant.md`; `PATTERN='vault-cli task append-metrics-session'`; `SECTION='### Session connect (MANDATORY when Obsidian task file exists)'`; `ONE_BRANCH='If EXACTLY ONE UUID is returned'`; `ANY_BRANCH='If zero OR multiple UUIDs are returned'`; the `--check-only` mode; the summary line `metrics-append-call-site: $pass passed, $fail failed`. All are grep targets in `<verification>`.
- **Shell conventions.** `#!/usr/bin/env bash`, `set -uo pipefail` (NOT `set -e` — the script must survive a non-zero exit from the child it invokes), tab indentation, mode `0755`. No `tail -f`, no `watch`, no background jobs, no network. Do not pipe a long producer into `grep -q` inside an `&&` condition — see the SIGPIPE hazard in `scripts/daily-note-has-entry-test.sh`.
- **No git.** Do NOT commit — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite (which includes `./integration` and now this script), `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code. Absence is asserted with `! grep -q`, never with a `grep -c` that must print `0`. Never pipe a test command.

**The script exists, is executable, and passes with the invocation present:**

```
test -f scripts/metrics-append-call-site-test.sh
test -x scripts/metrics-append-call-site-test.sh
test "$(stat -c '%a' scripts/metrics-append-call-site-test.sh)" = "755"
bash scripts/metrics-append-call-site-test.sh
```

The last line must exit 0 and print `metrics-append-call-site: 9 passed, 0 failed`.

**The script exits non-zero when the invocation is absent — the throwaway-copy direction, executed:**

```
TMPDIR_CHECK=$(mktemp -d)
mkdir -p "$TMPDIR_CHECK/agents"
sed 's/vault-cli task append-metrics-session/STRIPPED/' agents/work-on-task-assistant.md > "$TMPDIR_CHECK/agents/work-on-task-assistant.md"
! bash scripts/metrics-append-call-site-test.sh --check-only "$TMPDIR_CHECK"
bash scripts/metrics-append-call-site-test.sh --check-only .
rm -rf "$TMPDIR_CHECK"
```

Both directions must hold: non-zero on the stripped copy, zero on the repo root.

**The script carries the frozen constants and the check labels:**

```
test "$(grep -cF "AGENT=agents/work-on-task-assistant.md" scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF "PATTERN='vault-cli task append-metrics-session'" scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF "SECTION='### Session connect (MANDATORY when Obsidian task file exists)'" scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF "ONE_BRANCH='If EXACTLY ONE UUID is returned'" scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF "ANY_BRANCH='If zero OR multiple UUIDs are returned'" scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF -- '--check-only' scripts/metrics-append-call-site-test.sh)" -ge 4
test "$(grep -cF 'every artifact names the invocation' scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF 'the invocation is inside § Session connect' scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF 'the invocation line is a bulleted step' scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF 'the invocation is after the EXACTLY ONE bullet' scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF 'the invocation is before the zero OR multiple bullet' scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF 'the section states the warning' scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF 'self-check: only the stripped artifact is reported' scripts/metrics-append-call-site-test.sh)" = "1"
test "$(grep -cF 'metrics-append-call-site: $pass passed, $fail failed' scripts/metrics-append-call-site-test.sh)" = "1"
```

**The script follows the shell conventions, and the SIGPIPE shape is absent:**

```
test "$(head -1 scripts/metrics-append-call-site-test.sh)" = "#!/usr/bin/env bash"
test "$(grep -c '^set -uo pipefail$' scripts/metrics-append-call-site-test.sh)" = "1"
! grep -q '^set -e' scripts/metrics-append-call-site-test.sh
! grep -qE '\| *head -1 *\| *grep -q' scripts/metrics-append-call-site-test.sh
! grep -qE 'tail -f|watch |curl|wget' scripts/metrics-append-call-site-test.sh
test "$(grep -c 'mktemp -d' scripts/metrics-append-call-site-test.sh)" -ge 1
test "$(grep -cF 'trap ' scripts/metrics-append-call-site-test.sh)" -ge 1
```

**The Makefile wires it in, after the struck-row line:**

```
test "$(grep -cF -- 'bash scripts/metrics-append-call-site-test.sh' Makefile)" = "1"
test "$(grep -nF -- 'bash scripts/struck-row-rule-test.sh' Makefile | cut -d: -f1)" -lt "$(grep -nF -- 'bash scripts/metrics-append-call-site-test.sh' Makefile | cut -d: -f1)"
test "$(grep -cF -- 'bash scripts/daily-note-has-entry-test.sh' Makefile)" = "1"
test "$(grep -cF -- 'bash scripts/goal-task-link-count-test.sh' Makefile)" = "1"
```

**The artifact under test is unchanged by this prompt** — if any of these is not as stated, prompt 2's edit was reverted or weakened and the script's target must be restored rather than the assertion adjusted:

```
test "$(grep -cF -- 'vault-cli task append-metrics-session' agents/work-on-task-assistant.md)" = "1"
test "$(grep -cF -- 'ℹ️ Metrics: not recorded' agents/work-on-task-assistant.md)" = "1"
test "$(grep -cF -- 'do NOT append a metrics entry' agents/work-on-task-assistant.md)" = "1"
```

**The other scripts and the documentation are untouched:**

```
test "$(grep -cF 'struck-row-rule' scripts/struck-row-rule-test.sh)" -ge 1
! grep -qF -- 'append-metrics-session' scripts/struck-row-rule-test.sh
! grep -qF -- 'append-metrics-session' README.md
! grep -qF -- 'append-metrics-session' CHANGELOG.md
! grep -qF -- 'append-metrics-session' docs/work-on-session-lifecycle.md
! grep -qF -- 'append-metrics-session' commands/work-on-task.md
```

**The whole suite, including the new script via `make test`:**

```
make test > /tmp/spec053-make-test.log 2>&1; test "$?" = "0"
grep -qF -- 'metrics-append-call-site: 9 passed, 0 failed' /tmp/spec053-make-test.log
```

Finally, walk spec 053's Acceptance Criterion 6 (script half) and Desired Behavior 7 against the change and state in your completion report which requirement satisfies each, confirm both exit-code directions were executed, and state that this script is the detector for the spec's Failure Modes row "The agent definition loses the invocation".
</verification>
