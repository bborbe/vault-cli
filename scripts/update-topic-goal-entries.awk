# Entry rule for /vault-cli:update-topic.
#
# Prints one line per goal entry in a topic page's Goals section:
#
#     <goal name><TAB><status token or "none">
#
# Three properties are load-bearing and all are pinned by
# scripts/update-topic-goal-entries-test.sh:
#
#   LEVEL-RELATIVE BOUND — the section ends at the next heading of level <= the
#     Goals heading's OWN level. `## Goals` therefore runs past `### Lever A` to
#     the next `#` or `##`, while `# Goals` stops at the next `#`.
#
#     Bounding at ANY heading is the bug this script exists to prevent. The
#     Development Pipeline Throughput page keeps 29 of its 31 entries inside
#     `### Lever A/B/C/D` subsections; an any-level bound reads the 2 entries
#     above the first `###` and reports the other 28 as MISSING — a diff of 28
#     against a true diff of 0. That regression shipped in v0.178.0 and was
#     caught only by running the command against the real page.
#
#   DECLARATION, NOT MENTION — only a line beginning `- [[Name]]` is an entry.
#     A numbered list item (`1. [[X]]`), a bold prose line, and a wikilink
#     mid-sentence are the page's prose, not declarations. The same page carries
#     both shapes under `### Lever C`, one line apart from real entries.
#
#   NEVER SILENTLY SKIPPED — a page with no Goals heading exits 3 rather than
#     printing nothing. An empty entry list is indistinguishable from an absent
#     section, and the caller would then propose adding every tagging goal.

# A status token is the first backticked span that is a KNOWN goal status —
# never simply the first backticked span. The real page carries
#   `- [[Only the Most Important Work …]] — SC5 lever on `intake/spec`,
#    `in_progress` (phase `planning`); reworded …`
# where the first span is a flow-stage name and the status is the second. Phase
# values (`planning`, `execution`) are deliberately absent from the set: they are
# not statuses, and reporting one as a status would make the caller rewrite the
# wrong token.
#
# Residual risk, stated rather than hidden: an entry with NO status whose
# trailing clause happens to backtick a status word (e.g. "closes `completed`
# goals") reports that word. No such line exists on the page this ships against;
# the caller's drift check is what would surface it.
BEGIN {
  n = split("backlog next in_progress hold completed aborted draft todo", parts, " ")
  for (i = 1; i <= n; i++) KNOWN[parts[i]] = 1
}

function status_token(line,   rest, tok) {
  rest = line
  while (match(rest, /`[^`]+`/)) {
    tok = substr(rest, RSTART + 1, RLENGTH - 2)
    if (tok in KNOWN) return tok
    rest = substr(rest, RSTART + RLENGTH)
  }
  return "none"
}

function heading_level(line,   n) {
  n = 0
  while (substr(line, n + 1, 1) == "#") n++
  if (n == 0 || n > 6) return 0
  if (substr(line, n + 1, 1) != " ") return 0
  return n
}

{
  if (state == 0) {
    if ($0 ~ /^#{1,6}[[:space:]]+Goals[[:space:]]*$/) {
      level = heading_level($0)
      state = 1
    }
    next
  }

  lvl = heading_level($0)
  if (lvl > 0) {
    if (lvl <= level) exit    # section is over; END still runs and stays quiet
    next                      # a deeper grouping heading: keep scanning
  }

  if (match($0, /^- \[\[[^]]+\]\]/)) {
    name = substr($0, RSTART + 4, RLENGTH - 6)
    printf "%s\t%s\n", name, status_token($0)
  }
}

END {
  if (state == 0) {
    print "no Goals section" > "/dev/stderr"
    exit 3
  }
}
