# Design — r6-bulk-ops

> Select multiple tasks and apply bulk operations (delete, mark done, change priority).

---

## D1: Checkbox selection and bulk action toolbar

- references: R6.4
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: task checkboxes in list, bulk action toolbar, bulk delete/complete/priority handlers
- Excludes: filtering, undo, audit log, selective redo

### Interfaces
```html
<!-- Per task in list -->
<input type="checkbox" class="task-select" data-task-id="123">

<!-- Bulk action toolbar (shown when ≥1 task selected) -->
<div class="bulk-toolbar">
  <span>5 tasks selected</span>
  <button data-action="mark-done">Mark Done</button>
  <button data-action="delete">Delete Selected</button>
  <select name="priority">
    <option value="">Change Priority...</option>
    <option value="high">High</option>
    <option value="medium">Medium</option>
    <option value="low">Low</option>
  </select>
</div>

POST /api/projects/{projectId}/bulk-actions
Request:  { "task_ids": [1, 2, 3], "action": "mark-done" }
Response: { "success": true, "updated": 3 } (200 OK)
         { "error": "Invalid action" } (400 Bad Request)
```

### Invariants
- Checkbox state persists during page session (no reload)
- Bulk action applies to all selected task IDs
- Bulk delete requires confirmation dialog
- If task deleted concurrently, bulk operation skips it (no error)
- Bulk action clears selection after completion

### Failure
- Invalid action value → 400 Bad Request
- Project not found → 404
- Task deleted before bulk action executes → skip (no error)
- DB write error during bulk action → transaction rolls back, display error

### Integration
- Task list template renders checkbox per task
- JavaScript tracks selected task IDs in Set
- Toolbar shown/hidden based on selection count
- Bulk action handler in API calls DB update for each task
- After success, reload list or re-render affected tasks

### Alternatives
- Separate handlers per action: rejected (single bulk-actions endpoint is simpler)
- No confirmation for delete: rejected (accidental bulk delete too risky)
- Async bulk operations: rejected (single-binary deployment, sync is fine)

### Verification
- Integration test: POST bulk-actions with mark-done, verify tasks marked complete
- Integration test: POST bulk-actions with invalid action, verify 400
- Integration test: POST bulk-actions with deleted task ID, verify skipped
- UI test: select multiple tasks, verify toolbar shows, verify count updates
- UI test: delete confirmation dialog appears before bulk delete
- UI test: after bulk action, selection cleared, toolbar hidden

### Deployment
- Template: add checkboxes to task list rows
- JavaScript: add checkbox tracking, toolbar toggle logic
- Handler: add bulk-actions POST endpoint
- No database schema changes
- Rollback: revert template/JS, remove bulk-actions handler

---

## D2: Bulk filtering + actions (stretch)

- disposition: future
- scope: "select all matching filter" + bulk action

Not included in this spec; deferred to follow-up work.
