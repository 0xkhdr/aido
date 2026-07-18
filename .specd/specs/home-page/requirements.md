# Requirements — home-page

> Stable requirement/criterion IDs. Testable EARS behavior. UI/UX and core
> entities only; no AI summarization in scope.

## Context

aido is a Go + htmx + SQLite task app. Today: flat task list, no projects. This
spec redesigns the home page into a clean, Claude-style workspace where a user
picks or creates a project and adds tasks to it through a chat-style composer.
Scope is the entity model and the UI/UX; task functionality beyond
create/list/toggle/delete stays as-is.

Entities: a Project (id, name, created_at) is a container for tasks; a Task (id,
project_id, title, done, created_at) belongs to exactly one project.

## R1 — Project model

owner: Mohamed Khedr
priority: must
risk: medium

- R1.1: When the app starts against an empty database, the system shall provision at least one default project so the home page is never empty.
- R1.2: When a user submits a new project name, the system shall create a project with a unique id and that name and return it in the sidebar.
- R1.3: When a user submits a project name that is empty or only whitespace, the system shall reject it and create no project.
- R1.4: When a task is created, the system shall associate it with exactly one existing project via project_id.

## R2 — Sidebar and project selection

owner: Mohamed Khedr
priority: must
risk: low

- R2.1: When the home page loads, the system shall render a left sidebar listing all projects with the active project visibly marked.
- R2.2: When a user selects a project in the sidebar, the system shall make it the active project and show that project's tasks in the main pane.
- R2.3: When no project has been explicitly selected, the system shall treat the first default project as active.

## R3 — Chat-style composer

owner: Mohamed Khedr
priority: must
risk: medium

- R3.1: When a project is active, the system shall render a centered Claude-style composer (single input with a send action) bound to that project.
- R3.2: When a user submits composer text, the system shall create exactly one task in the active project with that text as the title.
- R3.3: When composer text is empty or only whitespace on submit, the system shall create no task and leave the composer state unchanged.
- R3.4: When a task is created, the system shall clear the composer and show the new task in the task list without a full page reload.
- R3.5: When the active project has no tasks, the system shall show a clean empty state prompting the user to add the first task.

## R4 — Task list within a project

owner: Mohamed Khedr
priority: must
risk: low

- R4.1: When a project is active, the system shall list only that project's tasks in the main pane.
- R4.2: When a user toggles a task, the system shall persist its done state and reflect it in the list.
- R4.3: When a user deletes a task, the system shall remove it from the active project and from the list.

## R5 — Clean UI/UX

owner: Mohamed Khedr
priority: should
risk: low

- R5.1: When the viewport is desktop-width, the system shall present a two-pane layout (sidebar plus main) with the composer as the focal element.
- R5.2: When any home-page pane renders, the system shall apply a single coherent visual style (spacing, typography, color) across sidebar, composer, and task list.

## Edge and failure behavior

- Empty or whitespace project name is rejected with no create (R1.3).
- Empty or whitespace composer text creates no task (R3.3).
- Selecting a non-existent or foreign project id is rejected, active project unchanged.
- Creating a task with no valid project_id is rejected (R1.4).

## Non-goals

- AI summarization, task parsing, or any LLM feature.
- Multi-task creation from one submit (one task per submit).
- Unassigned/inbox tasks (every task belongs to a project).
- Auth, multi-user, sharing, project deletion/rename (future specs).
- Mobile-specific layout beyond not breaking.
