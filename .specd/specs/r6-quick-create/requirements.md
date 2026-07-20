# Requirements — r6-quick-create

> Fast task creation without entering detail page.

## R6.3 — Quick task creation from project view

owner: craftsman
priority: should
risk: low

- **R6.3.1** When user types task title in quick-create form at top of project view and presses Enter or clicks Add, the system shall create task with that title and immediately reflect new task in list.
- **R6.3.2** When task is created via quick-create form, the system shall use default values: description empty, priority "medium", no due_date.
- **R6.3.3** When quick-create form is submitted successfully, the system shall clear the form input field and keep it focused for rapid re-entry.
- **R6.3.4** When quick-create form is submitted with empty title, the system shall reject creation and display validation error (no DB write).

## Edge and failure behavior

- Empty or whitespace-only title: rejected with validation error
- Rapid successive submissions: all tasks created (no rate limit)
- Form submission with default values: uses same task creation path as detail page
- Project not found: 404 or redirect to project list
- DB write error: display error, preserve form input

## Non-goals

- Bulk quick-create (multiple titles at once)
- Auto-complete suggestions
- Keyboard shortcuts
- Tags/labels in quick-create
- Priority/due-date selection in quick-create form
