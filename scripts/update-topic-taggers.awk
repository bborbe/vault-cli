# Membership rule for /vault-cli:update-topic.
#
# Prints FILENAME for each goal file whose frontmatter `topics:` (or the legacy
# singular `topic:`) names the topic passed as -v t=<topic>.
#
# Two properties are load-bearing and both are pinned by
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

FNR == 1 { fm = 0; infield = 0 }

/^---[[:space:]]*$/ { fm++; infield = 0; next }

fm != 1 { next }

/^(topics|topic):/ {
  infield = 1
  if (element_has_topic($0)) { print FILENAME; nextfile }
  next
}

infield && /^[[:space:]]+-/ {
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
