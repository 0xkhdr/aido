# Design — home-page

> Traces every requirement to a mechanism. Entities + UI/UX first; keeps the
> existing Go + htmx + SQLite stack.

## Architecture

Server-rendered HTML over htmx, same as today. No SPA, no client framework.
Two-pane layout served by the home handler; htmx swaps drive selection and task
mutations without full reloads.

```
Browser (htmx)
  └── GET  /                     home: sidebar + active project + composer + tasks
  └── POST /projects             create project        → swap #sidebar
  └── GET  /projects/{id}        select project (active) → swap #main
  └── POST /projects/{id}/tasks  create task in project  → swap #task-list, clear composer
  └── POST /tasks/{id}/toggle    toggle done             → swap #task-list
  └── DELETE /tasks/{id}         delete task             → swap #task-list
```

## Data model (SQLite)

```sql
CREATE TABLE projects (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tasks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title      TEXT NOT NULL,
  done       BOOLEAN NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_tasks_project ON tasks(project_id);
```

Migration: existing `tasks` gain `project_id`; a default project is created and
orphan tasks are backfilled to it. Satisfies R1.4.

## Go layer

- `internal/db/db.go`
  - `Project{ID int64; Name string; CreatedAt time.Time}`
  - `Task{ID, ProjectID int64; Title string; Done bool; CreatedAt time.Time}`
  - `EnsureDefaultProject()` → R1.1
  - `CreateProject(name)` with trim+non-empty guard → R1.2, R1.3
  - `ListProjects()` → R2.1
  - `GetProject(id)` → R2.2 (rejects unknown id)
  - `CreateTask(projectID, title)` with trim+non-empty+project-exists guard → R3.2, R3.3, R1.4
  - `ListTasksByProject(projectID)` → R4.1
  - `ToggleTask(id)` → R4.2; `DeleteTask(id)` → R4.3
- `internal/handlers/handlers.go`
  - `Home` renders full page with active project (default when none) → R2.3
  - `CreateProject`, `SelectProject`, `CreateTask`, `ToggleTask`, `DeleteTask`
  - Active project resolved from path; falls back to first project → R2.3

## Templates

- `index.html` — shell: `#sidebar` + `#main`.
- `sidebar.html` — project list, active marked, new-project form → R2.1, R1.2.
- `main.html` — active project header + composer + `#task-list` → R2.2, R3.1.
- `composer.html` — centered single input + send, posts to active project → R3.1, R3.4.
- `list.html` — tasks for active project, empty state → R4.1, R3.5.

## UI/UX

- Two-pane desktop layout: fixed-width left sidebar, fluid main pane. Composer is
  the visual focal point, centered in the main pane like Claude's message box →
  R5.1.
- One design token set (spacing scale, single accent color, system font stack),
  applied across all partials for a coherent look → R5.2.
- Empty state: friendly line + focused composer when a project has no tasks → R3.5.
- htmx swaps keep composer clear-and-refocus after send → R3.4.

## Requirement trace

| Requirement | Mechanism |
|---|---|
| R1.1 | `EnsureDefaultProject` on startup |
| R1.2 / R1.3 | `CreateProject` trim+guard, sidebar swap |
| R1.4 | `project_id NOT NULL` FK + `CreateTask` guard |
| R2.1 / R2.2 / R2.3 | sidebar list, `SelectProject`, default fallback |
| R3.1–R3.5 | composer partial, `CreateTask`, empty state, htmx swap |
| R4.1–R4.3 | `ListTasksByProject`, `ToggleTask`, `DeleteTask` |
| R5.1 / R5.2 | two-pane layout, shared design tokens |

## Edge/failure handling

- Whitespace project name / task title → guard returns error, no row written.
- Unknown or foreign project id on select/create → 404/400, active unchanged.
- Startup on empty DB → default project ensured before first render.

## Non-goals (design)

No AI, no multi-task parse, no inbox, no auth, no rename/delete-project.
