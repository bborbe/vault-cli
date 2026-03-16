---
page_type: guide
---

---

## Goal Frontmatter

| Field | Type | Description |
|-------|------|-------------|
| status | string | todo, in_progress, backlog, completed, hold, aborted |
| page_type | string | Always "goal" |

## Structure

Goals live in `23 Goals/` and link upward to themes via Tags.

```
Tags: [[Theme Name]]
```

Tasks link to goals via their `goals` frontmatter field.

## Lifecycle

1. Create goal file in `23 Goals/`
2. Link to a theme
3. Create tasks that reference this goal
4. Track progress via `vault-cli goal list`
