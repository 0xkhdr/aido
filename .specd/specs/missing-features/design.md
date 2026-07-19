# Design — missing-features

> Decision contract for project rename, remove, task view, text handling, and design refresh.

---

## D1: Project rename via REST PATCH

- references: R1, R1.1, R1.2, R1.3
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: `PATCH /api/projects/:id` endpoint, project name validation, DB transaction
- Excludes: multi-language support, name history, uniqueness constraints

### Interfaces
```
PATCH /api/projects/{projectId}
Request:  { "name": "New Name" }
Response: { "id": "...", "name": "New Name", "created_at": "..." } (200 OK)
         { "error": "Project name cannot be empty" } (400 Bad Request)
         { "error": "Project not found" } (404 Not Found)
```

### Invariants
- Project name persists exactly as submitted (no trimming, no normalization)
- All tasks associated with project remain intact and linked
- Rename is atomic: either fully succeeds or fails with rollback

### Failure
- Invalid name → reject with 400, display validation message in UI
- Project not found → 404, redirect user to project list
- DB failure during rename → transaction rolls back, user sees error

### Integration
- Requires `db.UpdateProject()` method to support name-only update
- HTML form uses POST + `_method=PATCH` fallback for older browsers
- UI immediately re-renders project list after successful rename

### Alternatives
- DELETE+CREATE instead of PATCH: rejected (breaks task references, adds complexity)
- PUT (replace entire project): accepted but PATCH is more RESTful for partial updates

### Verification
- Unit test: rename project, verify name in DB
- Integration test: rename with empty name, verify 400 + no DB change
- UI test: rename via form, verify list updates immediately

### Deployment
- Database schema: no migration needed (name column already exists)
- Rollback: revert handler code, no data changes

---

## D2: Project delete with cascade

- references: R2, R2.1, R2.2, R2.3
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: `DELETE /api/projects/:id` endpoint, cascade delete via FK constraints
- Excludes: soft delete, archive, recovery, audit log

### Interfaces
```
DELETE /api/projects/{projectId}
Response: { "success": true } (200 OK)
         { "error": "Project not found" } (404 Not Found)
```

### Invariants
- Deletion is atomic: project and all its tasks removed in single transaction
- No orphaned tasks (FK constraint on tasks.project_id)
- After delete, project ID is no longer accessible (referential integrity)

### Failure
- Project not found → 404, safe no-op
- DB constraint violation → transaction rolls back, user sees error
- Concurrent delete attempts → last delete wins, second gets 404

### Integration
- SQLite FK constraints enabled in tech steering
- HTML confirmation dialog prevents accidental clicks
- After successful delete, user redirects to project list

### Alternatives
- Soft delete: deferred (could add archive feature later if needed)
- Async delete with queue: rejected (over-engineered for single-binary deployment)

### Verification
- Unit test: delete project, verify project and tasks removed from DB
- Unit test: delete non-existent project, verify 404
- UI test: confirm dialog appears, cancel cancels delete, confirm proceeds

### Deployment
- Database: no migration (FK already declared)
- Rollback: revert handler code, data is gone (no recovery)

---

## D3: Task detail page with inline edit

- references: R3, R3.1, R3.2, R3.3, R3.4
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: task detail template, edit handlers, task fetch by ID
- Excludes: comments, history, state machines, complex workflows

### Interfaces
```
GET /project/:projectId/task/:taskId
Response: render detail.html with task data

PATCH /api/projects/:projectId/tasks/:taskId
Request:  { "title": "...", "description": "...", "completed": false }
Response: { "id": "...", "title": "...", ... } (200 OK)
         { "error": "Task not found" } (404 Not Found)
```

### Invariants
- Task detail page always shows current state from DB
- Edit changes are persisted atomically
- Back button or project link returns to task list in correct project

### Failure
- Invalid task ID → 404 or redirect to project list
- Task deleted by another user → 404 on page load or edit attempt
- Validation error (empty title) → reject with 400, no DB change

### Integration
- Task list links to `/project/:pid/task/:tid` detail page
- Detail page has inline edit form (PATCH on submit)
- Detail page has "Back" link to project list
- Browser back button returns to list (navigate history)

### Alternatives
- Modal detail view instead of page: rejected (page navigation is clearer for bookmarking)
- Full-page edit view separate from list: rejected (inline edit is faster UX)

### Verification
- Unit test: fetch task detail, verify all fields present
- Integration test: edit task, verify DB updated and view refreshed
- UI test: navigate to task, edit, go back, verify list shows updated task

### Deployment
- Add task detail template (HTML)
- Add task detail handler and edit handler
- Database: no migration (task schema unchanged)
- Rollback: revert handler and template, no data changes

---

## D4: Textarea with multi-line and scrolling

- references: R4, R4.1, R4.2, R4.3, R4.4
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: HTML textarea element, CSS for min-height and scrolling, form submission
- Excludes: markdown parsing, rich text editor, emoji support

### Interfaces
```html
<textarea name="description" 
          style="min-height: 200px; overflow-y: auto; word-wrap: break-word;"
          placeholder="Task description (multiline OK, max 10000 chars)">
  {{ .Task.Description }}
</textarea>
```

### Invariants
- Text input preserves line breaks and whitespace exactly as entered
- Text longer than 10000 chars rejected on server validation
- URLs in text are displayed as clickable links in read-only view

### Failure
- Text > 10000 chars → reject with 400, display char count warning in UI
- Special characters (HTML, script tags) → escaped on render to prevent XSS

### Integration
- Task update endpoint validates description length
- Task detail view HTML-escapes description before rendering
- CSS ensures textarea is usable on mobile (scrollable within container)

### Alternatives
- `contenteditable` div instead of textarea: rejected (textarea is more accessible)
- Rich text editor library: deferred (stretch goal, not core feature)
- Auto-expand textarea height: rejected (fixed height + scroll is simpler)

### Verification
- Unit test: validate description length, reject > 10000 chars
- Integration test: submit multi-line text, verify newlines preserved in DB
- UI test: paste large text, verify scrollbar appears, submit succeeds
- Security test: submit text with HTML tags, verify escaped on render

### Deployment
- No database migration (description column already exists and sized)
- Rollback: revert template changes, no data changes

---

## D5: Claude-inspired color scheme

- references: R5, R5.1, R5.2, R5.3, R5.4
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: global CSS variables, light/dark theme stylesheets, prefers-color-scheme media query
- Excludes: user theme picker UI, custom color picker, animated transitions

### Interfaces
```css
:root {
  --color-primary: #8B5CF6;        /* Purple (Claude primary) */
  --color-secondary: #EC4899;      /* Pink */
  --color-background: #FFFFFF;     /* Light bg */
  --color-surface: #F9FAFB;        /* Light surface */
  --color-text: #111827;           /* Dark text */
  --color-border: #E5E7EB;         /* Light border */
}

@media (prefers-color-scheme: dark) {
  :root {
    --color-background: #0F172A;   /* Dark bg */
    --color-surface: #1E293B;      /* Dark surface */
    --color-text: #F1F5F9;         /* Light text */
    --color-border: #334155;       /* Dark border */
  }
}
```

### Invariants
- Text contrast ratio ≥ 4.5:1 WCAG AA (measured for all color combos)
- Color scheme persists across page reloads (OS preference used, not stored in app)
- All interactive elements have `:hover` state with subtle highlight

### Failure
- Prefers-color-scheme unavailable → default to light mode
- Custom user CSS overrides: respected (cascade preserved)

### Integration
- All templates inherit CSS variables
- Buttons, links, forms use semantic color classes
- Icons and images respect current theme

### Alternatives
- User theme toggle in settings: deferred (stretch goal, simpler to use OS preference)
- Animated transition on theme change: rejected (users change system preference rarely)

### Verification
- Visual test: verify light and dark mode renders correctly
- Accessibility test: measure contrast ratios, verify WCAG AA compliance
- Cross-browser test: verify prefers-color-scheme detection works

### Deployment
- CSS file deployed with app binary (embedded)
- No database changes
- Rollback: revert CSS file, users stay on current OS theme

---

## D6: Feature recommendations (domain best practices)

- references: R6
- disposition: accepted (stretch goals, separate tasks)
- owner: craftsman

### Features to implement (in priority order)

1. **Task Priority (High/Medium/Low)**
   - Visual indicator in list (colored badge)
   - Sortable/filterable by priority
   - Default to Medium for new tasks

2. **Due Dates**
   - Date picker in task edit
   - Overdue warning (red indicator if date < today)
   - Optional field (can be empty)

3. **Quick Task Create**
   - Form at top of project view: type title, hit Enter
   - Skips detail page, immediately adds to list
   - Faster than "New Task" button + detail page

4. **Bulk Operations**
   - Checkbox select multiple tasks
   - Bulk "Mark Done", "Delete", "Change Priority"
   - Confirmation dialog for bulk delete

5. **Search**
   - Search box in nav bar
   - Searches task title + description across all projects
   - Results link to task detail page

6. **Task Tags**
   - Add/remove tags from task detail
   - Filter by tag in list view
   - Cross-project tag search

Each feature deferred to separate spec; track in project roadmap.
