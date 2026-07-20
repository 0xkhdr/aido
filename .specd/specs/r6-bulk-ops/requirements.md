# Requirements — r6-bulk-ops

> Select multiple tasks and apply bulk operations (delete, mark done, change priority).

## R6.4 — Bulk operations on tasks

owner: craftsman
priority: should
risk: medium

- **R6.4.1** When user checks task checkboxes in list view, the system shall display a bulk action toolbar with option to "Mark Done", "Delete Selected", or "Change Priority".
- **R6.4.2** When user selects bulk action "Mark Done", the system shall mark all selected tasks as completed and refresh the list.
- **R6.4.3** When user clicks "Delete Selected", the system shall display confirmation dialog; if confirmed, delete all selected tasks atomically.
- **R6.4.4** When user selects "Change Priority" and picks priority level, the system shall update priority for all selected tasks.
- **R6.4.5** When bulk operation completes, the system shall clear the checkbox selection and hide the bulk action toolbar.

## Edge and failure behavior

- Task deleted concurrently before bulk operation: skip silently (no error)
- Bulk delete on empty selection: toolbar not shown (no-op)
- Confirmation dialog cancelled: no action taken, selection remains
- Bulk operation error: display error message, selection remains for retry
- Large bulk operations (100+ tasks): no pagination/limit, but should complete within reasonable time

## Non-goals

- "Select all" checkbox
- Undo/redo for bulk operations
- Async bulk operations or progress bar
- Bulk operations across projects
- Bulk edit of description/due_date
