---
page_type: guide
---

---

## Decision Frontmatter

| Field | Type | Description |
|-------|------|-------------|
| needs_review | bool | true if pending review |
| reviewed | bool | true after acknowledgement |
| reviewed_date | string | ISO date of review (YYYY-MM-DD) |
| status | string | pending, accepted, rejected, superseded |
| type | string | architecture, process, tooling, etc. |

## Lifecycle

1. Create decision file anywhere in the vault
2. Set `needs_review: true`
3. `vault-cli decision list` to see pending decisions
4. `vault-cli decision ack <name>` to mark reviewed
5. Optionally override status: `vault-cli decision ack <name> --status accepted`
