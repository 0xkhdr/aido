# Tasks — r6-priority

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity | capabilities | context | evidence | checks |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1 | craftsman | `internal/db/migrations/add_priority.sql`, `internal/db/store.go` | - | `sqlite3 aido.db ".schema tasks"` | schema includes priority column | R6.1, D1 | schema | low | simple | write | DB migration | schema/priority | column exists, default='medium' |
| T2 | craftsman | `internal/handlers/tasks.go` | T1 | `go test ./internal/handlers -run TestCreateTask` | new task without priority defaults to medium | R6.1.1 | feature | low | simple | read,write | task creation | test/default-priority | no priority field |
| T3 | craftsman | `internal/handlers/tasks.go` | T1 | `go test ./internal/handlers -run TestUpdateTaskPriority` | update priority, verify DB, verify 400 on invalid | R6.1.2, R6.1.4 | feature | low | standard | read,write | PATCH handler | test/priority-update | invalid value |
| T4 | craftsman | `internal/handlers/templates/project-list.html` | T1 | `grep -n "badge-" internal/handlers/templates/project-list.html` | task list renders priority badge with color class | R6.1.3 | feature | low | simple | write | template rendering | test/manual-badge | colors correct |
| T5 | craftsman | `internal/handlers/templates/static/style.css` | - | `grep -E "badge-(high\|medium\|low)" internal/handlers/templates/static/style.css` | badge-high, badge-medium, badge-low styles defined | R6.1.3 | feature | low | simple | write | CSS | test/manual-colors | contrast meets WCAG |
| T6 | craftsman | `internal/handlers/tasks_test.go` | T2,T3 | `go test ./internal/handlers -run TestPriority -v` | full test suite for priority CRUD | - | test | low | simple | read,write | test fixtures | test/priority-full | edge cases |
