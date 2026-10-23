---
description: Extract high-significance learnings from this conversation and document them in the vault's Knowledge Base — works in any vault via vault-cli config
argument-hint: "[--non-interactive]"
allowed-tools:
  - Read
  - Write
  - Edit
  - Grep
  - Glob
  - Bash
  - AskUserQuestion
  - mcp__semantic-search__search_related
  - mcp__semantic-search__check_duplicates
---

Extract high-significance learnings from the current conversation and document them in the active vault's Knowledge Base directory.

This command **must stay inline** — it analyzes the parent conversation; a sub-agent cannot see the conversation context.

## Non-interactive contract

If the arguments contain `--non-interactive`, strip that token and run under **NO-ASK**: never call `AskUserQuestion`. The caller is a session-close path invoking this command so a finished session can exit — an ask here would block the very exit it is meant to complete, which is worse than not running it at all.

Under NO-ASK:

- **Vault (the *Runtime context* section above):** never ask which vault. Use the vault matched from cwd. If none matches, **write nothing** and print `reflect: skipped — cwd matches no configured vault`. Never fall back to a guessed vault — a reflection written into the wrong vault is worse than none.
- **Selection (Step 2):** no picker. Take **every** learning that passed the significance bar; if none did, write nothing and print `reflect: skipped — no significant learnings`.
- **Action (Step 3):** no `Proceed?` prompt and no `REVIEW` menu. Take the recommended action — `ENHANCE` on a ≥ 0.6 match, `CREATE` below 0.4, and in the 0.4–0.6 band **prefer `ENHANCE` of the top match** over creating a near-duplicate. The caller has already decided to run reflect; the choices that remain are ones this command can make itself.
- **Sensitivity:** NO-ASK removes the operator's review of the learning list, so the filter must carry the check the operator would have made. **Never write a learning whose text carries a credential, token, connection string, private key, or personal data** — skip it and say so in the terminal line. **This is a content scan over the drafted learning text, not a filename check.**

  Run the mechanical half over each drafted learning's text before writing it:

  ```bash
  REFLECT_SCAN="$(ls -d ~/.claude/plugins/cache/vault-cli/vault-cli/*/scripts 2>/dev/null | sort -V | tail -1)/reflect-secret-scan.sh"
  printf '%s' "$LEARNING_TEXT" | bash "$REFLECT_SCAN"
  ```

  The first line resolves the installed plugin's `scripts/` **directory** the same way `commands/session-close.md` Phase 9.5 resolves `session-self-exit.py`, then appends this script's filename. The append is load-bearing: `ls -d` yields a directory, and `bash "$REFLECT_SCAN"` without it hands bash a directory, fails, and drops silently into the prose fallback below — which reads as a working scan. It prints `secret-scan: clean` (exit 0) or `secret-scan: rejected (<class>)` (exit 1), and it **never prints the matched value** — a scan that echoed it would put the secret into the very transcript it exists to keep it out of. **On any `rejected` line, skip that learning.**

  **Fallback when the script is unreachable** — the plugin-cache glob resolving to nothing, the same failure `commands/session-close.md` documents for a bare `scripts/…` path run from a vault cwd. Apply this list by hand, in the same order, skipping on any hit: `private-key` (`-----BEGIN … PRIVATE KEY-----`); `aws-key-id` (`AKIA` / `ASIA` + 16 uppercase alphanumerics); `github-token` (`ghp_` / `gho_` / `ghu_` / `ghs_` / `ghr_` / `github_pat_` + 20); `gitlab-token` (`glpat-` + 20); `npm-token` (`npm_` + 36); `api-key` (`sk-` **at a token boundary** — never the `sk-` inside `task-`, `risk-` or `disk-` — followed by 16+ token characters); `slack-token` (`xox[baprs]-` + 10); `stripe-key` (`sk_live_` / `rk_live_` / `sk_test_` / `rk_test_` + 16); `bearer-token` (any casing of `bearer` + a 20-character token); `connection-string` (`scheme://user:pass@`); `key-assignment` (a key whose name ends in `password` / `passwd` / `token` / `secret` / `api_key` / `apikey`, **case-insensitively and allowing a prefix** — `PASSWORD=`, `API_KEY=`, `client_secret=`, `access_token=` all count — with **at least 8 non-space characters after the `=`**, or 4+ inside quotes so a value containing spaces is not missed; a bare `token=` followed by a short word does not fire); `jwt` (three dot-separated `eyJ…` segments); `base64-blob` (a 40+-character base64 run that is **not** a pure hex digest — otherwise every git SHA is rejected).

  Those class names are the script's own `--classes` list; `scripts/mode-membership-rule-test.sh` pins the two together, so this fallback cannot drift away from the script. **Neither list is exhaustive** — base64url blobs, and provider tokens outside these prefixes (Google `AIza…`, for one), match nothing, which is why the semantic obligation at the top of this bullet still carries the check the patterns cannot.

  The Phase 3 filename list in `commands/session-close.md` (`.env`, `*.key`, `credentials.json`) is a floor for *files* and cannot see a secret pasted into the conversation — which is exactly what an unreviewed write would publish. The significance filter (reusable / non-obvious / not-in-code) does not test sensitivity at all, so this scan is the only thing standing between the conversation and the vault.
- **Terminal line:** every NO-ASK run ends with exactly one line, so the calling session can record the outcome without re-reading the vault. One of:

  - `reflect: wrote <N> page(s): [[<a>]], [[<b>]]` — nothing skipped.
  - `reflect: wrote <N> page(s): [[<a>]]; skipped <M> — sensitivity` — a **mixed** run: pages written *and* learnings dropped by the sensitivity scan. It is one line because two lines cannot be recorded as one outcome, and the `skipped` count is lost the moment the write is reported alone — which is exactly how the skip used to vanish.
  - `reflect: skipped — <reason>` — nothing written.
  - `reflect: failed — <reason>` — **required, not optional:** a run that wrote some pages and then errored must print `failed` (naming what it did write), never `wrote`, so the caller's `failed:` branch is reachable.

  **The mixed form is what a correct run produces on a mixed input**, so it must be reachable, not theoretical. The canonical sample — the one `commands/session-close.md` Phase 9.4 and the post-deploy invocation exercise both use — is **two learnings, one clean and one carrying a credential-shaped fixture that is FAKE (an `sk-`-prefixed dummy, never a real secret)**: the first is written, the second is skipped by the scan, and the run ends on the mixed line. Never stage that sample from a real credential — the fixture must not become the leak the scan exists to prevent.

**If any step cannot proceed without a question, print the `skipped` line and stop** — never leave the caller waiting on a prompt.

## Runtime context

```bash
vault-cli config list --output json
```

Match cwd against each `path`. Use the matched vault's:
- `knowledge_dir` — Knowledge Base folder (e.g. `50 Knowledge Base`, `50 Knowledge`); falls back to default `50 Knowledge Base` if absent

If cwd is not inside any vault path, ask the user which vault to write to (AskUserQuestion with vault names from the config).

## Runtime detection

```
SEMANTIC_SEARCH_AVAILABLE = `mcp__semantic-search__search_related` present in session
DUPLICATE_CHECK_AVAILABLE = `mcp__semantic-search__check_duplicates` present in session
```

Both are optional — gracefully fall back to `Glob` / `Grep` when absent.

## Step 1: Analyze conversation for learnings

Scan the full conversation context. Look for:

- **Discoveries**: "I found that...", "Turns out...", "The issue was..."
- **Solutions**: Bug fixes, workarounds, successful approaches
- **Patterns**: Code patterns, API usage, tool configurations
- **Gotchas**: Things that didn't work, common mistakes, edge cases
- **New capabilities**: Tools created, features added, workflows established
- **Corrections**: Wrong assumptions corrected, better approaches found

### Significance filter

Each potential learning must pass all three:

| Question | Must be YES |
|---|---|
| **Reusable?** | Applies to 2+ future contexts? |
| **Non-obvious?** | Would forget in 6 months? |
| **Not in code?** | Insight beyond what's readable in source? |

**Skip (low significance):** API query parameters, script path changes, config tweaks, simple refactors, bug fixes for specific edge cases, implementation details visible in code.

**Keep (high significance):** Patterns combining multiple systems, architectural decisions with tradeoffs, non-obvious gotchas that caused wasted time, new capabilities/tools, workflow improvements affecting daily work.

Extract **0-3 high-significance learnings**. For each:

- **Title**: 3-7 words, descriptive
- **Category**: `code-pattern` | `tool-usage` | `workflow` | `gotcha` | `capability` | `configuration`
- **Summary**: 1-2 sentences
- **Context**: Why it matters, when it applies

## Step 2: Present learnings + ask user

**No learnings passed the filter:**

```
📚 No significant learnings detected.

This conversation contained:
- <reason 1>
- <reason 2>

Nothing to document — vault stays lean ✨
```

STOP.

**1-3 learnings found:**

```
📚 High-significance learnings (N found):

1. <Title> (<category>)
   <Summary>

2. ...

Which learnings should we document? (comma-separated numbers, "all", or "none"):
```

AskUserQuestion with options derived from the list. Wait for selection.

## Step 3: For each selected learning, find a documentation home

For each selected learning, process one at a time.

**Find candidates:**

If `SEMANTIC_SEARCH_AVAILABLE`:
- `mcp__semantic-search__search_related(query="<domain / problem, in your own words>", top_k=5)`
- **Run ≥2 queries with different framings.** Query the *domain and problem*, never the title you just drafted — a title-derived query returns pages phrased like your draft and misses pages stating the same idea in other vocabulary. Observed 2026-09-02: querying "page existed but was not consulted" missed [[A Written Warning Is Not a Control]] ("a warning informs whoever is already looking; a control changes what happens when nobody is") — the same thesis in different words. A near-duplicate page was written, then deleted.
- Filter to results under the vault's `<knowledge_dir>/` path

Else (fallback):
- `Glob: <knowledge_dir>/*<keyword>*.md`
- `Grep` for the strongest keyword in `<knowledge_dir>/`
- **The ≥2-framings rule applies here too, and matters more.** Keyword matching is strictly weaker than semantic search at finding a page that states your idea in different words — the exact miss this guard exists for. Use ≥2 keyword sets drawn from the *domain*, not from your drafted title, and grep the KB's vocabulary rather than your own (e.g. both `warning`/`control` and `documented`/`consulted`). Without semantic search, treat a single clean `Glob` as weak evidence of novelty, not proof.

**Decide action:**

| Top-match score | Action |
|---|---|
| ≥ 0.6 (or strong title/content overlap in fallback) | **ENHANCE** an existing page |
| 0.4–0.6 (or partial match) | **REVIEW** — show options |
| < 0.4 (or no usable match) | **CREATE** a new page |

Present:

```
📝 Learning: <title>

Recommended: <ENHANCE | CREATE | REVIEW>

[ENHANCE]   Add to: [[<existing page>]]  (similarity: 0.XX)
[CREATE]    Create new: <knowledge_dir>/<suggested name>.md
[REVIEW]    Options:
              1. [[<page A>]]  (0.XX)
              2. [[<page B>]]  (0.XX)
              3. Create new

Proceed? (y / n / skip)
```

Wait for user confirmation.

## Step 4: Apply documentation

**ENHANCE (add to existing page):**

1. Read the page
2. Find or create a `## Additional Insights` section
3. Append as a subsection:

   ```markdown
   ### <Learning Title>

   <Summary>

   **Context**: <when it applies>

   **Details**:
   - <key point 1>
   - <key point 2>
   ```

4. **If the page uses a numbered sequence** (`### 1.`, `### 2.` …), derive the next number from the WHOLE file — never from a truncated listing:

   ```bash
   # next number
   grep -oE '^#{2,6} [0-9]+\.' <page> | grep -oE '[0-9]+' | sort -n | tail -1
   # after appending: must print nothing
   grep -oE '^#{2,6} [0-9]+\.' <page> | grep -oE '[0-9]+' | sort -n | uniq -d
   ```

   Append at `max + 1`, then run the `uniq -d` check; any output is a collision you just created — renumber yours, never the existing one (its number may already be cited elsewhere). Same read → write → verify shape as `/email-inbox`'s rule-table numbering.

   **`#{2,6}`, not `#+` — a fenced `# 1.` bash comment is not a heading.** `^#+` matches any depth, so a code block containing `# 1. what is suppressed` / `# 2. the unfiltered source` is counted as sections 1 and 2, and `uniq -d` then reports collisions that do not exist. Observed 2026-09-17 on the first real run of this step: the check returned `1 2 17` on a page whose only true duplicate was `17`. Excluding depth-1 is safe because vault notes never use `# H1` — the filename is the title. Note the synthetic fixtures used to test this step all passed, because they had no code fences; only a real page exposed it.

   **The trap is surveying the page with `head`.** A structure grep piped through `head -N` reads exactly like a complete outline and is not. Observed 2026-09-17: `grep -nE '^#{2,3} ' <page> | head -20` showed 17 numbered shapes on a page that had **37**, and the new section was appended as `### 18.` — colliding with an existing one. Survey the headings unpiped, or compute the max directly with the command above.

**CREATE (new page):**

1. Pick a filename (PascalCase or natural language matching existing KB pages)
2. **Pre-write duplicate check.** The file does not exist yet, so `check_duplicates(file_path=…)` cannot be called on it — it takes a path to an *existing* file. Instead run one final `mcp__semantic-search__search_related` on the **drafted summary sentence** (not the title), and treat any hit ≥ 0.6 under `<knowledge_dir>/` as a duplicate → open that page and ask the user whether to ENHANCE it instead. **Under `--non-interactive` there is no one to ask: take the ENHANCE branch directly** — the Action bullet above already prescribes exactly that for the ≥ 0.6 band. Only if `DUPLICATE_CHECK_AVAILABLE` and the file already exists on a re-run does `check_duplicates` apply.
3. Create the file at `<knowledge_dir>/<filename>.md`:

   ```markdown
   ---
   tags:
     - knowledge
     - <category>
   created: <YYYY-MM-DD>
   ---
   Tags: [[Knowledge Base]]

   ---

   <Summary paragraph>

   ## Context

   <when and why this matters>

   ## Details

   - <key point 1>
   - <key point 2>
   - <code examples if applicable>

   ## Related

   - [[<related page 1>]]
   - [[<related page 2>]]
   ```

4. **Post-write audit.** Now that the file exists, if `DUPLICATE_CHECK_AVAILABLE` run `mcp__semantic-search__check_duplicates(file_path="<knowledge_dir>/<filename>.md")` as a backstop. A hit here means step 2's search missed — delete the new page and ENHANCE the match instead, rather than leaving both.

   When `DUPLICATE_CHECK_AVAILABLE` is false this backstop cannot run, and step 2's search is the *only* guard. Say so in the Step 5 report (`duplicate check: pre-write only — check_duplicates unavailable`) rather than reporting the page as duplicate-checked; an unrun check is not a pass. See [[Checks That Report False Green]].

## Step 5: Report

```
✅ Reflection complete

Documented N learnings:
- [[<page 1>]] (enhanced)
- [[<page 2>]] (created)

Skipped: M learnings
```

## Guidelines

- Be selective — not everything is worth documenting
- Always check existing pages before creating new ones
- Keep KB pages scannable, not verbose
- Link generously to related pages
- Match existing KB organization patterns in the active vault
- Skip the obvious — basic operations, one-time fixes

## Notes

- No hardcoded `50 Knowledge Base/` — the vault's `knowledge_dir` config drives the folder
- Works in any vault registered with `vault-cli config` that has `knowledge_dir` set (or falls back to the documented default)
- Multiple Atlassian / semantic-search MCPs supported via the same unified detection pattern
