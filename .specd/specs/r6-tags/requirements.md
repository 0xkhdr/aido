# Requirements — r6-tags

> Task tags for cross-project organization and filtering.

## R6.6 — Task tags/labels

owner: craftsman
priority: should
risk: low

- **R6.6.1** When user adds tags to a task via task detail page (comma-separated input), the system shall create tag records if needed and link tags to task.
- **R6.6.2** When task is displayed in detail or list, the system shall show all assigned tags as colored pills/badges.
- **R6.6.3** When user removes a tag from a task, the system shall delete the tag-task link and remove the tag from DB if no other tasks use it.
- **R6.6.4** When tag name is invalid (empty, whitespace-only, or >50 chars), the system shall reject tag and display validation error.
- **R6.6.5** When user searches or filters by tag (future feature), the system shall find all tasks with that tag across all projects.

## Edge and failure behavior

- Tag names case-insensitive (stored lowercase, displayed as-is)
- Multiple tasks can share same tag (many-to-many)
- Concurrent tag updates: last write wins (replace all tags for task)
- Deleting task: automatically removes associated tag links (cascade via FK)
- Tag with no tasks: cleaned up on next tag update or scheduled job (optional)

## Non-goals

- Tag hierarchy or nested tags
- Tag permissions or visibility
- Auto-complete tag suggestions
- Tag renaming/merging
- Bulk tag operations
- Tag filtering in list view (separate feature)
