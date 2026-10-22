# Membership rule for /vault-cli:update-topic.
#
# Prints FILENAME for each goal file whose frontmatter `topics:` (or the legacy
# singular `topic:`) names the topic passed as -v t=<topic>.
#
# Three properties are load-bearing and all are pinned by
# scripts/update-topic-taggers-test.sh:
#
#   FIELD, NOT FILE — a goal that merely mentions the topic in its body, title,
#     tags or description is not a member. Matching the whole file reports
#     members that do not exist and inflates the diff.
#
#   WHOLE ELEMENT, NOT SUBSTRING — "German" must not match "German-Spanish".
#     index() is used for the wikilink form (literal, never regex, so a topic
#     carrying regex metacharacters cannot misfire), and the bare/quoted form is
#     compared token-by-token after stripping the list punctuation.
#
#   NEVER SILENTLY SKIPPED — a file this script fails to parse reads as "not a
#     member", which is indistinguishable from a correct exclusion and removes a
#     goal from the diff without a word. So the opener is anchored to line 1 (a
#     later `---` horizontal rule is not frontmatter), a UTF-8 BOM is stripped
#     before matching, and a list continuation is accepted at column 0 as well
#     as indented.

FNR == 1 {
  fm = 0; infield = 0
  # Strip a UTF-8 BOM: without this, line 1 fails the `---` match and the whole
  # file is skipped — a member silently absent from the set.
  #
  # Built with sprintf("%c%c%c", …) and compared via substr() rather than written
  # as an octal escape in a regex: not every awk (BSD awk on macOS among them)
  # interprets `\357` in an ERE, and a silently-unstripped BOM is exactly the
  # failure this line exists to prevent.
  bom = sprintf("%c%c%c", 239, 187, 191)
  if (substr($0, 1, 3) == bom) $0 = substr($0, 4)
}

# The opening delimiter is line 1 only. A `---` further down is a horizontal
# rule, and treating it as an opener would scan the body as frontmatter.
FNR == 1 && /^---[[:space:]]*$/ { fm = 1; infield = 0; next }

fm == 1 && /^---[[:space:]]*$/ { fm = 2; infield = 0; next }

fm != 1 { next }

/^(topics|topic):/ {
  infield = 1
  if (element_has_topic($0)) { print FILENAME; nextfile }
  next
}

# A continuation is a list item, indented or at column 0 — `topics:` followed by
# `- '[[X]]'` at the margin is valid YAML and is matched here too. This rule must
# precede the top-level-key reset below, which would otherwise consume the item.
infield && /^[[:space:]]*-/ {
  if (element_has_topic($0)) { print FILENAME; nextfile }
  next
}

# A new top-level key ends the topics field.
/^[^[:space:]]/ { infield = 0 }

function element_has_topic(line,   s, n, i, parts) {
  if (index(line, "[[" t "]]") > 0) return 1
  s = line
  gsub(/\[\[|\]\]/, " ", s)
  gsub(/[][,'"]/, " ", s)
  n = split(s, parts, /[[:space:]]+/)
  for (i = 1; i <= n; i++) if (parts[i] == t) return 1
  return 0
}
