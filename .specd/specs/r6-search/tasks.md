# Tasks — r6-search

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity | capabilities | context | evidence | checks |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1 | craftsman | `internal/handlers/templates/base.html` or `internal/handlers/templates/header.html` | - | `grep -n "search.*form" internal/handlers/templates/base.html` | search form in navigation bar, submits to /search | R6.5.1 | feature | low | simple | write | template rendering | test/manual-search-box | form visible |
| T2 | craftsman | `internal/handlers/search.go` | - | `curl "http://localhost:8080/search?q=test"` | search handler queries tasks by title+description, returns matching results | R6.5.1, R6.5.4 | feature | low | standard | read | LIKE query, DB | test/search-query | LIKE works |
| T3 | craftsman | `internal/handlers/templates/search-results.html` | T2 | `grep -n "project.*task" internal/handlers/templates/search-results.html` | results page displays task title, project name, link to detail | R6.5.2 | feature | low | simple | write | template rendering | test/manual-results | links work |
| T4 | craftsman | `internal/handlers/templates/search-results.html` | T2 | `grep -n "No tasks" internal/handlers/templates/search-results.html` | empty results display "No tasks found" message | R6.5.3 | feature | low | simple | write | template rendering | test/manual-no-results | message shown |
| T5 | craftsman | `internal/handlers/search_test.go` | T2 | `go test ./internal/handlers -run TestSearch -v` | full test suite for search: title match, description match, case-insensitive, special chars | R6.5.1, R6.5.4 | test | low | standard | read | test fixtures | test/search-full | edge cases |
