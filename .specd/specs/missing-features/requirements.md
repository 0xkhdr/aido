# Requirements — missing-features

> Project management core features: rename/remove projects, task view page, rich text handling, modern design.

## R1 — Project rename

owner: craftsman
priority: must
risk: low

- **R1.1** When user edits project name field and submits, the system shall update project name in DB and reflect change immediately in project list.
- **R1.2** When project name is empty string, the system shall reject rename and display error message.
- **R1.3** When project rename succeeds, the system shall preserve all associated tasks unchanged.

## Edge and failure behavior

- Empty or whitespace-only names rejected with validation error
- Concurrent rename attempts: last write wins (DB enforces via transaction)
- Name length limit: 255 chars (DB column constraint)

## Non-goals

- Name uniqueness constraint (multiple projects can share names)
- Audit log of previous names

---

## R2 — Project remove

owner: craftsman
priority: must
risk: medium

- **R2.1** When user clicks delete project button and confirms deletion, the system shall remove project and all associated tasks atomically.
- **R2.2** When project is deleted, the system shall return user to project list view.
- **R2.3** When deletion succeeds, the system shall not display the project or its tasks in any list.

## Edge and failure behavior

- Attempting to view/edit deleted project: 404 or redirect to list
- Confirmation dialog required (accidental delete prevention)
- DB delete cascade enforced via foreign key constraints

## Non-goals

- Soft delete / archive (hard delete only)
- Recovery/undo after confirmation
- Delete history/audit trail

---

## R3 — Task detail view page

owner: craftsman
priority: must
risk: low

- **R3.1** When user clicks on a task in the list, the system shall navigate to a dedicated task detail page showing task title, description, status, and all task metadata.
- **R3.2** When task detail page is loaded, the system shall display inline edit controls for title, description, and status.
- **R3.3** When user edits task field on detail page and submits, the system shall save changes to DB and update the view without page reload.
- **R3.4** When user navigates back from task detail page, the system shall return to the project's task list with updated data.

## Edge and failure behavior

- Invalid task ID: 404 or redirect to project list
- Concurrent edits: last write wins
- Task deleted while viewing: 404 or redirect

## Non-goals

- Task comments/discussion threads
- Task history/revision log
- Complex workflows/state machines

---

## R4 — Chat/input box text handling

owner: craftsman
priority: should
risk: low

- **R4.1** When user pastes or types multi-line text into task description input, the system shall accept and preserve line breaks.
- **R4.2** When task description exceeds visible height, the system shall provide scrollable textarea with minimum height 200px.
- **R4.3** When user submits description with leading/trailing whitespace, the system shall preserve the text exactly as entered (no auto-trim).
- **R4.4** When description text contains URLs, the system shall render URLs as clickable links in task detail view.

## Edge and failure behavior

- Text length limit: 10000 chars (DB column size)
- Extremely long lines (no breaks): scrollable horizontally in detail view
- Special characters (quotes, newlines, HTML): escaped on render to prevent XSS

## Non-goals

- Rich text formatting (bold, italic, code blocks)
- Markdown parsing
- Emoji/image support
- Real-time sync of edits

---

## R5 — Claude-inspired color scheme

owner: craftsman
priority: should
risk: low

- **R5.1** When application loads, the system shall apply a color palette matching Claude's design: soft purples, cool grays, and clear contrast for accessibility.
- **R5.2** When user's OS sets dark mode, the system shall detect and apply dark variant of color scheme.
- **R5.3** When user's OS sets light mode, the system shall apply light variant of color scheme.
- **R5.4** When hovering over interactive elements, the system shall apply subtle highlight color change for visual feedback.

## Edge and failure behavior

- System prefers-color-scheme detection unavailable: default to light mode
- Custom CSS overrides: user styles respected (CSS cascade)
- Contrast ratio must meet WCAG AA minimum (4.5:1 for text)

## Non-goals

- User-switchable theme picker (OS preference only)
- Custom color configuration
- Animated theme transitions

---

## R6 — Domain-recommended features (stretch goals)

owner: craftsman
priority: could
risk: low

- **R6.1** Task priority levels (High, Medium, Low) with visual indicators in list view.
- **R6.2** Task due dates with optional deadline warning (visual indicator if overdue).
- **R6.3** Task dependencies: mark task as blocker, display blocked-by relationships.
- **R6.4** Project templates: pre-populate new projects with common task categories (e.g., "Roadmap", "Bugs", "In Progress").
- **R6.5** Bulk operations: select multiple tasks, apply action (delete, mark done, change priority) at once.
- **R6.6** Search across projects and tasks by keyword.
- **R6.7** Quick task creation from project list (without entering detail page).
- **R6.8** Task tags/labels for cross-project organization.

## Edge and failure behavior

- Bulk operations on deleted tasks: skip silently
- Search on empty/no results: display "No tasks found" message
- Due date in past: render with warning indicator

## Non-goals

- Recurring/recurring tasks
- Task time tracking/logging
- Calendar view
- Team collaboration/comments
