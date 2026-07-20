# Requirements — r6-priority

> Task priority levels with visual indicators in list view.

## R6.1 — Task priority levels

owner: craftsman
priority: should
risk: low

- **R6.1.1** When task is created without explicit priority, the system shall default priority to "medium".
- **R6.1.2** When user updates task priority via dropdown in task detail or list, the system shall persist priority to DB and display updated badge in list immediately.
- **R6.1.3** When task is displayed in list, the system shall render a visual badge (High=red, Medium=orange, Low=green) showing task priority.
- **R6.1.4** When priority value is invalid (not "high", "medium", or "low"), the system shall reject update with 400 error and display validation message.

## Edge and failure behavior

- Missing priority field in update request: preserve existing priority (no-op)
- Concurrent priority updates: last write wins
- New task without priority field in request: default to "medium"
- Task priority visible in all list views (project task list only, not search/filter yet)

## Non-goals

- Sorting by priority
- Filtering by priority
- Priority history/change log
- Custom priority levels or weights
- Bulk priority change
