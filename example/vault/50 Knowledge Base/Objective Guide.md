---
page_type: guide
---

---

## Objective Frontmatter

| Field | Type | Description |
|-------|------|-------------|
| status | string | todo, in_progress, completed, hold |
| page_type | string | Always "objective" |

## Structure

Objectives live in `22 Objectives/` and represent time-bound outcomes (3-12 months). They link upward to themes via Tags.

```
Tags: [[Theme Name]]
```

Goals link to objectives to form the delivery path.

## Lifecycle

1. Create objective in `22 Objectives/`
2. Link to a theme
3. Create goals that deliver this objective
4. Track via `vault-cli objective list`
