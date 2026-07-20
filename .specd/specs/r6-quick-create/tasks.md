# Tasks — r6-quick-create

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity | capabilities | context | evidence | checks |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1 | craftsman | `internal/handlers/templates/project-list.html` | - | `grep -n "quick-create" internal/handlers/templates/project-list.html` | quick-create form rendered at top of task list | R6.3.1 | feature | low | simple | write | template rendering | test/manual-form | form visible |
| T2 | craftsman | `internal/handlers/projects.go` | - | `curl -X POST http://localhost:8080/project/1/tasks -d "title=New Task"` | POST creates task with title, defaults priority="medium", empty description | R6.3.2 | feature | low | standard | read,write | HTTP form handler | test/quick-create-post | defaults applied |
| T3 | craftsman | `internal/handlers/templates/project-list.html` | T1 | `grep -n 'autofocus' internal/handlers/templates/project-list.html` | form input stays focused after submission, field cleared | R6.3.3 | feature | low | simple | write | JavaScript/template | test/manual-focus | rapid entry works |
| T4 | craftsman | `internal/handlers/projects.go` | T2 | `go test ./internal/handlers -run TestQuickCreateEmpty` | POST with empty title returns 400, no DB change | R6.3.4 | feature | low | simple | read,write | validation | test/quick-create-validation | validation fires |
| T5 | craftsman | `internal/handlers/projects_test.go` | T2,T4 | `go test ./internal/handlers -run TestQuickCreate -v` | full test suite for quick-create | - | test | low | simple | read,write | test fixtures | test/quick-create-full | edge cases |
