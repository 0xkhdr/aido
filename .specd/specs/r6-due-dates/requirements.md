# Requirements — r6-due-dates

> Optional task due dates with visual overdue warning.

## R6.2 — Task due dates

owner: craftsman
priority: should
risk: low

- **R6.2.1** When user sets task due date via date picker in task detail, the system shall persist due_date to DB in ISO 8601 format (YYYY-MM-DD).
- **R6.2.2** When task due date is in the past (< today's date), the system shall display an "Overdue" badge in task list view with red indicator.
- **R6.2.3** When task due date is optional, the system shall allow user to clear due date (set to null) via date picker.
- **R6.2.4** When due date value is invalid (unparseable date string), the system shall reject update with 400 error and display validation message.

## Edge and failure behavior

- Missing due_date field in update request: preserve existing due_date (no-op)
- Due date in past but task not marked complete: still shows overdue badge (no auto-completion)
- Concurrent due date updates: last write wins
- Date comparison uses local date only; no time zone handling
- Task with no due_date: renders no date indicator in list

## Non-goals

- Reminder notifications or email alerts
- Recurring/recurring task dates
- Calendar view of tasks by due date
- Sorting/filtering by due date (separate feature)
- Time component (date only)
- Task auto-completion on due date
