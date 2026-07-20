# Design — r6-priority

> Task priority levels (High, Medium, Low) with visual indicators and sort order.

---

## D1: Task priority field and UI badge

- references: R6.1
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: priority enum (High/Medium/Low), DB column, API endpoint, visual badge in list
- Excludes: sorting UI, filtering, priority history, custom priority levels

### Interfaces
```
PATCH /api/projects/{projectId}/tasks/{taskId}
Request:  { "priority": "high" }
Response: { "id": "...", "title": "...", "priority": "high", ... } (200 OK)

GET /project/{projectId}
Response: render list.html with tasks sorted by default (created_at)
          each task renders badge: <span class="badge badge-high">High</span>
```

### Invariants
- Priority defaults to "medium" for new tasks
- Valid values: "low", "medium", "high" (case-insensitive input, lowercase storage)
- Task created without priority specified → default to "medium"
- Priority persists atomically with task update

### Failure
- Invalid priority value → reject with 400, display error
- Task not found → 404
- Missing priority field in request → preserve existing priority (no change)

### Integration
- Task schema adds `priority TEXT DEFAULT 'medium'` column
- Task list template renders priority badge with color class
- Task detail page shows priority dropdown or radio buttons
- Badge colors: high=red, medium=orange, low=green (via CSS classes)

### Alternatives
- Numeric priority (1-5): rejected (High/Medium/Low is simpler and matches domain)
- Priority as separate entity: rejected (enum is simpler for this scope)

### Verification
- Unit test: create task without priority, verify defaults to medium
- Unit test: update task priority, verify DB change
- Integration test: update with invalid priority, verify 400
- UI test: verify badge displays with correct color and text

### Deployment
- Database migration: add `priority TEXT DEFAULT 'medium'` to tasks table
- CSS: add badge-high (red), badge-medium (orange), badge-low (green) styles
- Rollback: drop priority column, revert template/CSS

---

## D2: Priority sorting (stretch)

- disposition: future (separate task)
- scope: sort list by priority (high→low), then by created_at

Not included in this spec; deferred to follow-up work.
