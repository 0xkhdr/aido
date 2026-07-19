# Tasks — missing-features

> Add only real work. The optional columns beyond the six required ones may be omitted.
> Production rows declare full trace, risk, routing, context, capability, evidence, and edge-check intent.

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity | capabilities | context | evidence | checks |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1 | craftsman | `internal/handlers/projects.go`, `internal/db/store.go` | - | `go test ./internal/handlers -run TestUpdateProject` | R1.1, R1.2 | R1, D1 | feature | low | standard | read,write | DB CRUD, HTTP PATCH | test/unit-rename | empty name validation |
| T2 | craftsman | `internal/handlers/projects.go`, `internal/db/store.go` | - | `go test ./internal/handlers -run TestDeleteProject` | R2.1, R2.2, R2.3 | R2, D2 | feature | medium | standard | read,write | FK constraints, transaction | test/unit-delete | cascade verify |
| T3 | craftsman | `internal/handlers/templates/task-detail.html`, `internal/handlers/tasks.go` | - | `curl http://localhost:8080/project/1/task/1` | R3.1 | R3, D3 | feature | low | standard | read,write | template rendering | test/manual-view | 404 handling |
| T4 | craftsman | `internal/handlers/tasks.go`, `internal/handlers/templates/task-edit.html` | T3 | `go test ./internal/handlers -run TestPatchTask` | R3.2, R3.3 | R3, D3 | feature | low | standard | read,write | PATCH handler | test/unit-edit | concurrent edits |
| T5 | craftsman | `internal/handlers/templates/task-detail.html`, `internal/db/store.go` | T3 | `go test ./internal/handlers -run TestTextarea` | R4.1, R4.2, R4.3, R4.4 | R4, D4 | feature | low | simple | read,write | textarea CSS | test/unit-textarea | long lines |
| T6 | craftsman | `internal/handlers/templates/static/style.css` | - | `grep -n color-primary internal/handlers/templates/static/style.css` | R5.1, R5.2, R5.3 | R5, D5 | feature | low | simple | write | CSS variables | test/manual-theme | contrast ratios |
| T7 | craftsman | `internal/handlers/projects_test.go` | T1 | `go test ./internal/handlers -run TestUpdateProject -v` | - | R1 | test | low | standard | read,write | test fixtures | test/unit-rename-full | validation edges |
| T8 | craftsman | `internal/handlers/projects_test.go` | T2 | `go test ./internal/handlers -run TestDeleteProject -v` | - | R2 | test | low | standard | read,write | test fixtures | test/unit-delete-full | missing project |
| T-R6 | scout | - | - | - | - | R6 | deferred | low | simple | - | domain features | - | priority, due dates, bulk ops, search, tags |
