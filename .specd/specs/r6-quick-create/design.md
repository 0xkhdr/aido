# Design — r6-quick-create

> Fast task creation from project view without entering detail page.

---

## D1: Quick create form at top of project view

- references: R6.3
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: quick-create form at top of task list, POST handler, immediate list refresh
- Excludes: bulk creation, templates, multi-field editing during creation

### Interfaces
```html
<form method="POST" action="/project/{projectId}/tasks" class="quick-create">
  <input type="text" name="title" placeholder="New task..." required>
  <button type="submit">Add</button>
</form>

POST /project/{projectId}/tasks (from quick-create form)
Request:  FormData { title: "Task title" }
Response: redirect to /project/{projectId} (flash message: "Task created")
         or render list.html with task appended and form cleared
```

### Invariants
- Quick create accepts title only (description, priority, due_date optional/default)
- Task created with title, empty description, priority="medium", no due_date
- Form field focused after submission for rapid re-entry
- Submission clears input field
- Success message displayed (toast or flash message)

### Failure
- Empty title → reject with validation error (no DB write)
- Project not found → 404
- DB write error → display error, preserve form input

### Integration
- Quick create form placed above task list in project view template
- Form action POSTs to existing `/project/{projectId}/tasks` handler (or new dedicated handler)
- Task created via same handler as detail-page creation (code reuse)
- After success, scroll to new task in list or show toast notification
- Form input remains focused for rapid task entry

### Alternatives
- Modal dialog for quick create: rejected (inline form is faster, no modal overhead)
- Separate page for quick create: rejected (defeats purpose of "quick")
- AJAX submission without page reload: accepted (better UX if simple to implement)

### Verification
- Integration test: POST to quick-create endpoint, verify task created in DB
- Integration test: POST with empty title, verify 400 + no DB change
- UI test: submit task via quick-create form, verify list updates and form clears
- UI test: rapid task entry (multiple POSTs in succession), verify all tasks created

### Deployment
- Template: add quick-create form at top of project task list
- Handler: ensure POST /project/{projectId}/tasks accepts form submission
- Optional: add AJAX handler for form submission if simple
- Rollback: revert template form, no data schema changes

---

## D2: Quick-create with keyboard shortcut (stretch)

- disposition: future
- scope: Cmd+N or Ctrl+N to focus quick-create input

Not included in this spec; deferred to follow-up work.
