# Design — r6-due-dates

> Optional task due dates with overdue warning indicator.

---

## D1: Due date field with date picker

- references: R6.2
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: due_date column (nullable), date input in task detail/edit, API endpoint, overdue warning badge
- Excludes: recurring tasks, deadline history, reminders/notifications, calendar view

### Interfaces
```
PATCH /api/projects/{projectId}/tasks/{taskId}
Request:  { "due_date": "2026-07-25" } or { "due_date": null }
Response: { "id": "...", "title": "...", "due_date": "2026-07-25", ... } (200 OK)

GET /project/{projectId}
Response: render list.html with tasks
          if due_date < today: render badge <span class="badge badge-overdue">Overdue</span>
          if due_date exists and future: render due date text "Due: 2026-07-25"
          if no due_date: render nothing
```

### Invariants
- Due date field is optional (can be NULL in DB)
- Due date stored as ISO 8601 string (YYYY-MM-DD) for comparison
- Overdue check uses local date only (no time component)
- Due date can be in past (task not auto-completed)

### Failure
- Invalid date format → reject with 400, display error
- Date string unparseable → reject with 400
- Task not found → 404
- Missing due_date field in request → preserve existing due_date (no change)

### Integration
- Task schema adds `due_date TEXT NULL` column
- Task detail page renders HTML `<input type="date">` element
- Task list template checks if `due_date < today()` and renders overdue badge
- Badge styling: overdue=red, future=gray

### Alternatives
- DateTime with time component: rejected (date-only is simpler for task management)
- Due date as separate table: rejected (denormalizes task data unnecessarily)
- Automatic overdue task marking: rejected (user decides task completion, not system)

### Verification
- Unit test: create task with due date, verify DB stores date
- Unit test: update task with invalid date, verify 400
- Unit test: task with due_date < today shows overdue indicator in list
- UI test: date picker allows selecting date, persists on save
- UI test: overdue badge displays correctly

### Deployment
- Database migration: add `due_date TEXT NULL` to tasks table
- Template: add date input in task detail edit form
- CSS: add overdue badge styling (red background)
- Rollback: drop due_date column, revert template

---

## D2: Due date sorting/filtering (stretch)

- disposition: future (separate task)
- scope: sort by due_date, filter by overdue/upcoming

Not included in this spec; deferred to follow-up work.
