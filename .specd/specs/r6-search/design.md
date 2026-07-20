# Design — r6-search

> Global search for tasks by title and description across all projects.

---

## D1: Search box in navigation with results page

- references: R6.5
- disposition: accepted
- owner: craftsman

### Boundaries
- Owns: search input in nav bar, search handler, results page with links to task detail
- Excludes: advanced filters, saved searches, search history, result ranking/relevance

### Interfaces
```html
<input type="text" name="q" placeholder="Search tasks..." form="search-form">
<form id="search-form" method="GET" action="/search"></form>

GET /search?q=<query>
Response: render search-results.html with matching tasks
          each result: <a href="/project/{id}/task/{id}">Project / Task Title</a>
         if no results: display "No tasks found"
```

### Invariants
- Search is case-insensitive and across all projects
- Search matches task title and description (full-text or simple substring)
- Results display task title, associated project name, and link to detail page
- Empty search query → empty results (no "show all tasks")
- Special characters in query: literal match (no regex/operators)

### Failure
- Empty query (q=""): redirect to home or display empty results
- Query too long (>1000 chars): reject with 400
- Database timeout on large search: display error message

### Integration
- Search form in navigation template (header/nav bar)
- Search handler queries tasks table with LIKE on title and description
- Results page template lists matching tasks with project context
- Simple LIKE query for MVP (no full-text index)

### Alternatives
- Full-text search with indexes: deferred (LIKE is sufficient for MVP)
- Search filters (by project, priority, due date): deferred (stretch goal)
- Real-time search suggestions: rejected (complexity not justified)

### Verification
- Integration test: search for task title, verify matching tasks in results
- Integration test: search for description text, verify results include task
- Integration test: search with special characters, verify literal match
- Integration test: empty query, verify no error (empty results or redirect)
- UI test: search box in nav, submit query, verify results page loads

### Deployment
- Template: add search form to nav/header
- Handler: add GET /search endpoint
- No database schema changes (search via LIKE on existing columns)
- Rollback: revert template/handler, no data changes

---

## D2: Advanced search filters (stretch)

- disposition: future
- scope: filter by project, priority, due date, tag

Not included in this spec; deferred to follow-up work.
