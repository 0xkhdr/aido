# Design — r6-tags

> Task tags for cross-project organization and filtering.

---

## D1: Tags on task with add/remove UI

- references: R6.6
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: tags table, task_tags join table, tag input/display in task detail, tag API endpoint
- Excludes: tag autocomplete, tag hierarchy, tag permissions, bulk tag operations

### Interfaces
```
GET /project/{projectId}/task/{taskId}
Response: render detail.html with task and tags list

PATCH /api/projects/{projectId}/tasks/{taskId}/tags
Request:  { "tags": ["urgent", "design", "review"] }
Response: { "id": "...", "tags": ["urgent", "design", "review"] } (200 OK)

Database schema:
CREATE TABLE tags (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL
);
CREATE TABLE task_tags (
  task_id INTEGER NOT NULL,
  tag_id INTEGER NOT NULL,
  PRIMARY KEY (task_id, tag_id),
  FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  FOREIGN KEY (tag_id) REFERENCES tags(id)
);
```

### Invariants
- Tags are case-insensitive and stored lowercase
- Each tag is unique across all projects
- Task can have multiple tags (0 or more)
- Tag-task relationship is many-to-many
- Tag deletion: if last task with tag is untagged, tag removed from DB (optional cleanup)

### Failure
- Invalid tag format (empty, >50 chars): reject with 400
- Task not found → 404
- Tag not found → 404
- Concurrent tag updates: last write wins (replace all tags for task)

### Integration
- Task detail page displays tag pills/badges
- Task detail page has input field to add/edit tags (comma-separated or click to add)
- Tag display in task list (optional, shows 2-3 tags with "..." if more)
- Tag link or filter not in scope (deferred)

### Alternatives
- Tag as single text field: rejected (many-to-many allows cross-project search later)
- Tag hierarchy/nested tags: rejected (flat tags sufficient for MVP)
- Auto-complete tag suggestions: deferred (stretch goal)

### Verification
- Unit test: add tag to task, verify task_tags join table
- Unit test: remove tag from task, verify join entry deleted
- Unit test: update task with multiple tags, verify all persisted
- Unit test: invalid tag (empty or >50 chars), verify 400
- UI test: add tag via detail page, verify displayed and saved
- UI test: remove tag, verify UI updated and DB cleaned

### Deployment
- Database migration: create tags table and task_tags join table
- Template: add tag input/display in task detail page
- Handler: add PATCH /api/projects/{projectId}/tasks/{taskId}/tags endpoint
- CSS: add tag pill/badge styling
- Rollback: drop tags and task_tags tables, revert template/handler

---

## D2: Tag filtering and search (stretch)

- disposition: future
- scope: filter tasks by tag, search within tag

Not included in this spec; deferred to follow-up work.
